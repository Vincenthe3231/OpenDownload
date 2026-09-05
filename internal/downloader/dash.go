package downloader

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/transport"
)

type DASHDownloaderConfig struct {
	Workers    int
	Verbose    bool
	OnProgress ProgressCallback
}

type DASHDownloader struct {
	client *transport.HTTPClient
	config DASHDownloaderConfig
}

// NewDASHDownloader creates a DASH transfer engine. The caller owns output
// publication and must supply an uncreated work file path to Download.
func NewDASHDownloader(client *transport.HTTPClient, config DASHDownloaderConfig) *DASHDownloader {
	if config.Workers <= 0 {
		config.Workers = 8
	}
	return &DASHDownloader{client: client, config: config}
}

// Download transfers representation segments into the exact caller supplied
// work path. It never renames or removes a separate final destination.
func (d *DASHDownloader) Download(ctx context.Context, rep *parser.DASHRepresentation, outPath string) error {
	if rep == nil {
		return validationError("DASH representation", "must not be nil")
	}
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
	data, err := d.client.Get(ctx, seg.URL)
	if err != nil {
		return nil, transportError("fetch DASH segment", err)
	}
	return data, nil
}

func (d *DASHDownloader) downloadSegmentRange(ctx context.Context, seg parser.DASHSegment) ([]byte, error) {
	start, end, err := parseDASHByteRange(seg.Range)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.GetRange(ctx, seg.URL, start, end)
	if err != nil {
		return nil, transportError("fetch DASH byte range", err)
	}
	defer resp.Body.Close()
	return readValidatedRangeResponse(resp, start, end)
}

func parseDASHByteRange(value string) (int64, int64, error) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, validationError("DASH byte range", "must use start-end syntax")
	}
	start, startErr := strconv.ParseInt(parts[0], 10, 64)
	end, endErr := strconv.ParseInt(parts[1], 10, 64)
	if startErr != nil || endErr != nil || start < 0 || end < start || end == math.MaxInt64 {
		return 0, 0, validationError("DASH byte range", "must contain a valid nonnegative inclusive range")
	}
	return start, end, nil
}


func readValidatedRangeResponse(resp *http.Response, start, end int64) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, validationError("DASH range response", "must include a response body")
	}
	if resp.StatusCode != http.StatusPartialContent {
		return nil, validationError("DASH range response", fmt.Sprintf("expected HTTP 206, received %d", resp.StatusCode))
	}
	responseStart, responseEnd, responseTotal, validRange := parseContentRange(resp.Header.Get("Content-Range"))
	if !validRange || responseStart != start || responseEnd != end || responseTotal <= end {
		return nil, validationError("DASH range response", fmt.Sprintf("Content Range does not match requested bytes %d-%d", start, end))
	}

	expectedLength := end - start + 1
	if resp.ContentLength >= 0 && resp.ContentLength != expectedLength {
		return nil, validationError("DASH range response", "Content Length does not match requested bytes")
	}
	maxInt := int64(^uint(0) >> 1)
	if expectedLength > maxInt {
		return nil, validationError("DASH byte range", "is too large for this platform")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, expectedLength+1))
	if err != nil {
		return nil, transportError("read DASH byte range", err)
	}
	if int64(len(data)) != expectedLength {
		return nil, validationError("DASH range response", fmt.Sprintf("body has %d bytes, expected %d", len(data), expectedLength))
	}

	return data, nil
}
