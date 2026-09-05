package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opendownload/opendownload/internal/transport"
)

type HTTPDownloaderConfig struct {
	Workers    int
	Verbose    bool
	OnProgress ProgressCallback
}

type HTTPDownloader struct {
	client *transport.HTTPClient
	config HTTPDownloaderConfig
}

// NewHTTPDownloader creates a direct HTTP transfer engine. The caller owns
// output publication and must supply an uncreated work file path to Download.
func NewHTTPDownloader(client *transport.HTTPClient, config HTTPDownloaderConfig) *HTTPDownloader {
	if config.Workers <= 0 {
		config.Workers = 8
	}
	return &HTTPDownloader{client: client, config: config}
}

// Download transfers url into the exact caller supplied work path. It never
// renames or removes a separate final destination.
func (d *HTTPDownloader) Download(ctx context.Context, url, outPath string) error {
	size, _, _, err := d.client.Head(ctx, url)
	if err != nil {
		return transportError("inspect URL", err)
	}
	reporter := newProgressReporter(d.config.OnProgress)
	defer reporter.finish()
	reporter.bytes(0, size, true)

	if d.config.Workers > 1 {
		if rangeSize, supported, probeErr := d.probeRange(ctx, url); probeErr != nil {
			return probeErr
		} else if supported {
			size = rangeSize
			return d.downloadMultiSegment(ctx, url, outPath, size, reporter)
		}
	}

	return d.downloadSingle(ctx, url, outPath, size, reporter)
}

func (d *HTTPDownloader) probeRange(ctx context.Context, url string) (int64, bool, error) {
	resp, err := d.client.GetRange(ctx, url, 0, 0)
	if err != nil {
		return 0, false, transportError("probe range support", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return 0, false, nil
	}
	start, end, total, ok := parseContentRange(resp.Header.Get("Content-Range"))
	if !ok || start != 0 || end != 0 || total <= 0 {
		return 0, false, nil
	}
	return total, true, nil
}

func parseContentRange(value string) (int64, int64, int64, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bytes") {
		return 0, 0, 0, false
	}
	rangeAndSize := strings.Split(parts[1], "/")
	if len(rangeAndSize) != 2 || rangeAndSize[1] == "*" {
		return 0, 0, 0, false
	}
	rangeParts := strings.Split(rangeAndSize[0], "-")
	if len(rangeParts) != 2 {
		return 0, 0, 0, false
	}
	start, startErr := strconv.ParseInt(rangeParts[0], 10, 64)
	end, endErr := strconv.ParseInt(rangeParts[1], 10, 64)
	total, totalErr := strconv.ParseInt(rangeAndSize[1], 10, 64)
	if startErr != nil || endErr != nil || totalErr != nil || start < 0 || end < start || total <= end {
		return 0, 0, 0, false
	}
	return start, end, total, true
}

func (d *HTTPDownloader) downloadSingle(ctx context.Context, url, outPath string, totalSize int64, reporter *progressReporter) error {
	resp, err := d.client.GetBody(ctx, url)
	if err != nil {
		return transportError("fetch URL", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return validationError("HTTP response", fmt.Sprintf("received %d", resp.StatusCode))
	}

	file, err := createWorkOutput(outPath)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		_ = file.Close()
		if !succeeded {
			removeWorkOutput(outPath)
		}
	}()

	progress := &progressWriter{report: func(downloaded int64) { reporter.bytes(downloaded, totalSize, false) }}
	_, err = io.Copy(file, io.TeeReader(resp.Body, progress))
	reporter.bytes(progress.written.Load(), totalSize, true)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close work output: %w", err)
	}
	succeeded = true
	return nil
}

func (d *HTTPDownloader) downloadMultiSegment(ctx context.Context, url, outPath string, totalSize int64, reporter *progressReporter) error {
	workers := d.config.Workers
	segmentSize := totalSize / int64(workers)
	if segmentSize < 1024*1024 {
		return d.downloadSingle(ctx, url, outPath, totalSize, reporter)
	}

	file, err := createWorkOutput(outPath)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		_ = file.Close()
		if !succeeded {
			removeWorkOutput(outPath)
		}
	}()

	if err := file.Truncate(totalSize); err != nil {
		_ = file.Close()
		return fmt.Errorf("allocate work output: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close work output: %w", err)
	}

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
			if err := d.downloadSegment(ctx, url, outPath, start, end, totalSize, &downloaded); err != nil {
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

	succeeded = true
	return nil
}

func (d *HTTPDownloader) downloadSegment(ctx context.Context, url, outPath string, start, end, totalSize int64, downloaded *atomic.Int64) error {
	resp, err := d.client.GetRange(ctx, url, start, end)
	if err != nil {
		return transportError("fetch HTTP byte range", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return validationError("HTTP range response", fmt.Sprintf("expected HTTP 206, received %d", resp.StatusCode))
	}
	responseStart, responseEnd, responseTotal, validRange := parseContentRange(resp.Header.Get("Content-Range"))
	if !validRange || responseStart != start || responseEnd != end || responseTotal != totalSize {
		return validationError("HTTP range response", fmt.Sprintf("Content Range did not match requested bytes %d-%d", start, end))
	}

	file, err := os.OpenFile(outPath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, 32*1024)
	offset := start
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.WriteAt(buf[:n], offset); writeErr != nil {
				return writeErr
			}
			offset += int64(n)
			downloaded.Add(int64(n))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return transportError("read HTTP byte range", readErr)
		}
	}
	if offset != end+1 {
		return fmt.Errorf("range response wrote %d bytes, expected %d", offset-start, end-start+1)
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
