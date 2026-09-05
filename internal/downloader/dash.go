package downloader

import (
	"context"
	"fmt"

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
	reporter := newProgressReporter(d.config.OnProgress)
	defer reporter.finish()
	reporter.segments(0, 0, int64(total), true)

	var completed, downloaded int64
	return downloadOrderedSegments(ctx, total, d.config.Workers, outPath, func(fetchCtx context.Context, index int) ([]byte, error) {
		data, err := d.downloadSegment(fetchCtx, rep.Segments[index])
		if err != nil {
			return nil, fmt.Errorf("segment %d: %w", index, err)
		}
		return data, nil
	}, func(bytes int) {
		completed++
		downloaded += int64(bytes)
		reporter.segments(downloaded, completed, int64(total), completed == int64(total))
	})
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
