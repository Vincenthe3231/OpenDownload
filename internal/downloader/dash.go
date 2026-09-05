package downloader

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/util"
)

type DASHDownloaderConfig struct {
	Workers    int
	Verbose    bool
	OnProgress ProgressCallback
}

type DASHDownloader struct {
	client *util.HTTPClient
	config DASHDownloaderConfig
}

func NewDASHDownloader(client *util.HTTPClient, config DASHDownloaderConfig) *DASHDownloader {
	if config.Workers <= 0 {
		config.Workers = 8
	}
	return &DASHDownloader{client: client, config: config}
}

func (d *DASHDownloader) Download(ctx context.Context, rep *parser.DASHRepresentation, outPath string) error {
	total := len(rep.Segments)
	var completed atomic.Int64
	var downloaded atomic.Int64
	reporter := newProgressReporter(d.config.OnProgress)
	defer reporter.finish()
	reporter.segments(0, 0, int64(total), true)

	results := make([][]byte, total)
	var mu sync.Mutex
	var downloadErr error

	sem := make(chan struct{}, d.config.Workers)
	var wg sync.WaitGroup

	for i, seg := range rep.Segments {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, segment parser.DASHSegment) {
			defer wg.Done()
			defer func() { <-sem }()

			data, err := d.downloadSegment(ctx, segment)
			if err != nil {
				mu.Lock()
				if downloadErr == nil {
					downloadErr = fmt.Errorf("segment %d: %w", idx, err)
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			results[idx] = data
			mu.Unlock()

			done := completed.Add(1)
			bytes := downloaded.Add(int64(len(data)))
			reporter.segments(bytes, done, int64(total), done == int64(total))
		}(i, seg)
	}

	wg.Wait()

	if downloadErr != nil {
		return downloadErr
	}

	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	for i, data := range results {
		if data == nil {
			return fmt.Errorf("missing data for segment %d", i)
		}
		if _, err := file.Write(data); err != nil {
			return fmt.Errorf("failed to write segment %d: %w", i, err)
		}
	}

	return nil
}

func (d *DASHDownloader) downloadSegment(ctx context.Context, seg parser.DASHSegment) ([]byte, error) {
	if seg.Range != "" {
		return d.downloadSegmentRange(ctx, seg)
	}
	return d.client.Get(ctx, seg.URL)
}

func (d *DASHDownloader) downloadSegmentRange(ctx context.Context, seg parser.DASHSegment) ([]byte, error) {
	var start, end int64
	_, err := fmt.Sscanf(seg.Range, "%d-%d", &start, &end)
	if err != nil {
		return nil, fmt.Errorf("failed to parse byte range: %w", err)
	}

	resp, err := d.client.GetRange(ctx, seg.URL, start, end)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data := make([]byte, 0, end-start+1)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if readErr != nil {
			break
		}
	}

	return data, nil
}
