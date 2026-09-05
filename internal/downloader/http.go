package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opendownload/opendownload/internal/util"
)

type HTTPDownloaderConfig struct {
	Workers    int
	Verbose    bool
	OnProgress ProgressCallback
}

type HTTPDownloader struct {
	client *util.HTTPClient
	config HTTPDownloaderConfig
}

func NewHTTPDownloader(client *util.HTTPClient, config HTTPDownloaderConfig) *HTTPDownloader {
	if config.Workers <= 0 {
		config.Workers = 8
	}
	return &HTTPDownloader{client: client, config: config}
}

func (d *HTTPDownloader) Download(ctx context.Context, url, outPath string) error {
	size, resumable, _, err := d.client.Head(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to inspect URL: %w", err)
	}
	reporter := newProgressReporter(d.config.OnProgress)
	defer reporter.finish()
	reporter.bytes(0, size, true)

	if size > 0 && resumable && d.config.Workers > 1 {
		return d.downloadMultiSegment(ctx, url, outPath, size, reporter)
	}

	return d.downloadSingle(ctx, url, outPath, size, reporter)
}

func (d *HTTPDownloader) downloadSingle(ctx context.Context, url, outPath string, totalSize int64, reporter *progressReporter) error {
	resp, err := d.client.GetBody(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	progress := &progressWriter{report: func(downloaded int64) { reporter.bytes(downloaded, totalSize, false) }}
	_, err = io.Copy(file, io.TeeReader(resp.Body, progress))
	reporter.bytes(progress.written.Load(), totalSize, true)
	return err
}

func (d *HTTPDownloader) downloadMultiSegment(ctx context.Context, url, outPath string, totalSize int64, reporter *progressReporter) error {
	workers := d.config.Workers
	segmentSize := totalSize / int64(workers)
	if segmentSize < 1024*1024 {
		return d.downloadSingle(ctx, url, outPath, totalSize, reporter)
	}

	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	if err := file.Truncate(totalSize); err != nil {
		file.Close()
		return fmt.Errorf("failed to allocate file: %w", err)
	}
	file.Close()

	var downloaded atomic.Int64

	done := make(chan struct{})
	var progressWG sync.WaitGroup
	progressWG.Add(1)
	go func() {
		defer progressWG.Done()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				reporter.bytes(downloaded.Load(), totalSize, false)
			}
		}
	}()

	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		start := int64(i) * segmentSize
		end := start + segmentSize - 1
		if i == workers-1 {
			end = totalSize - 1
		}

		wg.Add(1)
		go func(start, end int64) {
			defer wg.Done()
			if err := d.downloadSegment(ctx, url, outPath, start, end, &downloaded); err != nil {
				errCh <- err
			}
		}(start, end)
	}

	wg.Wait()
	close(done)
	progressWG.Wait()
	close(errCh)
	reporter.bytes(downloaded.Load(), totalSize, true)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *HTTPDownloader) downloadSegment(ctx context.Context, url, outPath string, start, end int64, downloaded *atomic.Int64) error {
	resp, err := d.client.GetRange(ctx, url, start, end)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	file, err := os.OpenFile(outPath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return err
	}

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			downloaded.Add(int64(n))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	return nil
}

type progressWriter struct {
	written atomic.Int64
	report  func(int64)
}

func (p *progressWriter) Write(data []byte) (int, error) {
	n := len(data)
	current := p.written.Add(int64(n))
	p.report(current)
	return n, nil
}
