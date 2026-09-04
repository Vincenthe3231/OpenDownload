package downloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opendownload/opendownload/server/internal/util"
)

type HTTPDownloaderConfig struct {
	Workers int
	Verbose bool
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

	if size > 0 && resumable && d.config.Workers > 1 {
		return d.downloadMultiSegment(ctx, url, outPath, size)
	}

	return d.downloadSingle(ctx, url, outPath, size)
}

func (d *HTTPDownloader) downloadSingle(ctx context.Context, url, outPath string, totalSize int64) error {
	resp, err := d.client.GetBody(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	progress := &progressWriter{total: totalSize, start: time.Now()}
	_, err = io.Copy(file, io.TeeReader(resp.Body, progress))
	fmt.Println()
	return err
}

func (d *HTTPDownloader) downloadMultiSegment(ctx context.Context, url, outPath string, totalSize int64) error {
	workers := d.config.Workers
	segmentSize := totalSize / int64(workers)
	if segmentSize < 1024*1024 {
		return d.downloadSingle(ctx, url, outPath, totalSize)
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
	progress := &progressWriter{total: totalSize, start: time.Now()}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				progress.printProgress(downloaded.Load())
				fmt.Println()
				return
			case <-ticker.C:
				progress.printProgress(downloaded.Load())
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
	close(errCh)

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
	total    int64
	written  atomic.Int64
	start    time.Time
}

func (p *progressWriter) Write(data []byte) (int, error) {
	n := len(data)
	p.written.Add(int64(n))
	p.printProgress(p.written.Load())
	return n, nil
}

func (p *progressWriter) printProgress(current int64) {
	elapsed := time.Since(p.start).Seconds()
	if elapsed == 0 {
		elapsed = 0.001
	}

	speed := float64(current) / elapsed

	if p.total > 0 {
		pct := float64(current) / float64(p.total) * 100
		fmt.Printf("\r  %s / %s (%.1f%%) @ %s/s   ",
			util.FormatBytes(current),
			util.FormatBytes(p.total),
			pct,
			util.FormatBytes(int64(speed)))
	} else {
		fmt.Printf("\r  %s @ %s/s   ",
			util.FormatBytes(current),
			util.FormatBytes(int64(speed)))
	}
}
