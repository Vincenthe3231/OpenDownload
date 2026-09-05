package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/transport"
)

func TestHTTPDownloaderReportsByteProgress(t *testing.T) {
	payload := []byte("download progress payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "25")
		w.Header().Set("Accept-Ranges", "bytes")
		if r.Method != http.MethodHead {
			_, _ = w.Write(payload)
		}
	}))
	defer server.Close()

	updates, callback := collectProgress()
	path := filepath.Join(t.TempDir(), "video.mp4")
	client := transport.NewHTTPClient(transport.HTTPClientConfig{})
	err := NewHTTPDownloader(client, HTTPDownloaderConfig{Workers: 1, OnProgress: callback}).Download(context.Background(), server.URL, path)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	assertProgress(t, updates(), int64(len(payload)), 0, 0)

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(contents) != string(payload) {
		t.Fatalf("unexpected output: %q", contents)
	}
}

func TestHLSDownloaderReportsSegmentProgress(t *testing.T) {
	segments := [][]byte{[]byte("first"), []byte("second")}
	server := testSegmentServer(t, segments)
	defer server.Close()

	updates, callback := collectProgress()
	playlist := &parser.HLSPlaylist{Segments: []parser.HLSSegment{
		{URI: server.URL + "/0"},
		{URI: server.URL + "/1"},
	}}
	path := filepath.Join(t.TempDir(), "video.ts")
	client := transport.NewHTTPClient(transport.HTTPClientConfig{})
	err := NewHLSDownloader(client, HLSDownloaderConfig{Workers: 1, OnProgress: callback}).Download(context.Background(), playlist, path)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	assertProgress(t, updates(), int64(len(segments[0])+len(segments[1])), 2, 2)
}

func TestDASHDownloaderReportsSegmentProgress(t *testing.T) {
	segments := [][]byte{[]byte("first"), []byte("second")}
	server := testSegmentServer(t, segments)
	defer server.Close()

	updates, callback := collectProgress()
	representation := &parser.DASHRepresentation{Segments: []parser.DASHSegment{
		{URL: server.URL + "/0"},
		{URL: server.URL + "/1"},
	}}
	path := filepath.Join(t.TempDir(), "video.mp4")
	client := transport.NewHTTPClient(transport.HTTPClientConfig{})
	err := NewDASHDownloader(client, DASHDownloaderConfig{Workers: 1, OnProgress: callback}).Download(context.Background(), representation, path)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	assertProgress(t, updates(), int64(len(segments[0])+len(segments[1])), 2, 2)
}

func testSegmentServer(t *testing.T, segments [][]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) != 2 || r.URL.Path[0] != '/' || r.URL.Path[1] < '0' || int(r.URL.Path[1]-'0') >= len(segments) {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(segments[r.URL.Path[1]-'0'])
	}))
}

func collectProgress() (func() []Progress, ProgressCallback) {
	var mu sync.Mutex
	updates := make([]Progress, 0)
	return func() []Progress {
			mu.Lock()
			defer mu.Unlock()
			return append([]Progress(nil), updates...)
		}, func(update Progress) {
			mu.Lock()
			updates = append(updates, update)
			mu.Unlock()
		}
}

func assertProgress(t *testing.T, updates []Progress, downloaded, completed, total int64) {
	t.Helper()
	if len(updates) == 0 {
		t.Fatal("expected progress updates")
	}
	for index := 1; index < len(updates); index++ {
		if updates[index].DownloadedBytes < updates[index-1].DownloadedBytes {
			t.Fatalf("downloaded bytes decreased: %#v", updates)
		}
		if updates[index].CompletedUnits < updates[index-1].CompletedUnits {
			t.Fatalf("completed units decreased: %#v", updates)
		}
	}
	last := updates[len(updates)-1]
	if last.DownloadedBytes != downloaded {
		t.Fatalf("downloaded bytes = %d, want %d", last.DownloadedBytes, downloaded)
	}
	if last.CompletedUnits != completed || last.TotalUnits != total {
		t.Fatalf("units = %d/%d, want %d/%d", last.CompletedUnits, last.TotalUnits, completed, total)
	}
}
