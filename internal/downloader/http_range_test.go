package downloader

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opendownload/opendownload/internal/util"
)

func TestHTTPDownloaderUsesVerifiedByteRanges(t *testing.T) {
	payload := bytes.Repeat([]byte("open-download-range-data"), 400000)
	var rangeRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			return
		}
		rawRange := r.Header.Get("Range")
		if rawRange == "" {
			http.Error(w, "range required", http.StatusBadRequest)
			return
		}
		rangeRequests.Add(1)
		var start, end int
		if _, err := fmt.Sscanf(rawRange, "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start || end >= len(payload) {
			http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(payload[start : end+1])
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "movie.bin")
	err := NewHTTPDownloader(util.NewHTTPClient(util.HTTPClientConfig{}), HTTPDownloaderConfig{Workers: 4}).Download(context.Background(), server.URL, path)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Equal(contents, payload) {
		t.Fatal("output did not match source data")
	}
	if rangeRequests.Load() < 2 {
		t.Fatalf("range requests = %d, want probe plus transfer ranges", rangeRequests.Load())
	}
}

func TestHTTPDownloaderFallsBackWhenRangeProbeIsInvalid(t *testing.T) {
	payload := []byte("fallback payload")
	var fullRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			return
		}
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Range", "bytes 1-1/15")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(payload[:1])
			return
		}
		fullRequests.Add(1)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "movie.bin")
	err := NewHTTPDownloader(util.NewHTTPClient(util.HTTPClientConfig{}), HTTPDownloaderConfig{Workers: 4}).Download(context.Background(), server.URL, path)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != string(payload) {
		t.Fatalf("fallback output = %q, err = %v", contents, err)
	}
	if fullRequests.Load() != 1 {
		t.Fatalf("full requests = %d, want 1", fullRequests.Load())
	}
}

func TestOrderedSegmentsWritesSourceOrderAndCleansPartialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playlist.ts")
	data := []string{"first", "second", "third"}
	err := downloadOrderedSegments(context.Background(), len(data), 2, path, func(ctx context.Context, index int) ([]byte, error) {
		if index == 0 {
			time.Sleep(25 * time.Millisecond)
		}
		return []byte(data[index]), nil
	}, nil)
	if err != nil {
		t.Fatalf("ordered segment download failed: %v", err)
	}
	contents, readErr := os.ReadFile(path)
	if readErr != nil || string(contents) != "firstsecondthird" {
		t.Fatalf("ordered output = %q, err = %v", contents, readErr)
	}

	failurePath := filepath.Join(t.TempDir(), "failed.ts")
	err = downloadOrderedSegments(context.Background(), 2, 2, failurePath, func(ctx context.Context, index int) ([]byte, error) {
		if index == 1 {
			return nil, fmt.Errorf("source failure")
		}
		return []byte("first"), nil
	}, nil)
	if err == nil {
		t.Fatal("expected segment error")
	}
	if _, statErr := os.Stat(failurePath + ".part"); !os.IsNotExist(statErr) {
		t.Fatalf("partial file remains: %v", statErr)
	}
}
