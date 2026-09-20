package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/downloader"
	"github.com/opendownload/opendownload/internal/media"
	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/transport"
)

func TestServicePublishesReservedOutput(t *testing.T) {
	payload := []byte("fresh media")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", "11")
		if request.Method != http.MethodHead {
			_, _ = writer.Write(payload)
		}
	}))
	defer server.Close()
	directory := t.TempDir()
	service := NewService(Config{DefaultOutputDir: func() (string, error) { return directory, nil }, DefaultWorkers: 1})
	snapshot, err := service.Queue(context.Background(), Request{ID: "one", URL: server.URL + "/movie.mp4", OutputDir: directory})
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	completed, err := service.Wait(context.Background(), snapshot.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if completed.Status != StatusCompleted {
		t.Fatalf("status = %s: %s", completed.Status, completed.Message)
	}
	contents, err := os.ReadFile(completed.OutputPath)
	if err != nil || string(contents) != string(payload) {
		t.Fatalf("published output = %q, err = %v", contents, err)
	}
	if _, err := os.Stat(completed.OutputPath + temporaryOutputSuffix + ".one"); !os.IsNotExist(err) {
		t.Fatalf("work file remains: %v", err)
	}
}

func TestServiceRejectsExistingOutputWithoutForce(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "movie.mp4")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	service := NewService(Config{DefaultOutputDir: func() (string, error) { return directory, nil }})
	_, err := service.Queue(context.Background(), Request{ID: "one", URL: "https://example.test/movie.mp4", Filename: "movie.mp4", OutputDir: directory})
	if err == nil {
		t.Fatal("expected existing output conflict")
	}
	if contents, _ := os.ReadFile(path); string(contents) != "old" {
		t.Fatal("existing output changed")
	}
}

func TestPublishForcePreservesFinalOnFailedReplacement(t *testing.T) {
	directory := t.TempDir()
	finalPath := filepath.Join(directory, "movie.mp4")
	if err := os.WriteFile(finalPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	err := publishOutput(filepath.Join(directory, "missing.part"), finalPath, "job", true)
	if err == nil {
		t.Fatal("expected publication failure")
	}
	contents, readErr := os.ReadFile(finalPath)
	if readErr != nil || string(contents) != "old" {
		t.Fatalf("final output was not restored: %q, %v", contents, readErr)
	}
}

func TestServiceDeliversOrderedTerminalSnapshots(t *testing.T) {
	gate := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead {
			writer.Header().Set("Content-Length", "4")
			return
		}
		<-gate
		_, _ = writer.Write([]byte("data"))
	}))
	defer server.Close()
	service := NewService(Config{DefaultOutputDir: func() (string, error) { return t.TempDir(), nil }, DefaultWorkers: 1})
	var mu sync.Mutex
	updates := make([]JobSnapshot, 0)
	stop := service.Subscribe(func(snapshot JobSnapshot) { mu.Lock(); updates = append(updates, snapshot); mu.Unlock() })
	defer stop()
	snapshot, err := service.Queue(context.Background(), Request{ID: "one", URL: server.URL + "/movie.mp4", OutputDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	close(gate)
	completed, err := service.Wait(context.Background(), snapshot.ID)
	if err != nil || completed.Status != StatusCompleted {
		t.Fatalf("completion = %#v, %v", completed, err)
	}
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(updates) < 3 {
		t.Fatalf("events = %#v", updates)
	}
	for index := 1; index < len(updates); index++ {
		if updates[index].Version <= updates[index-1].Version {
			t.Fatalf("versions not monotonic: %#v", updates)
		}
	}
	if updates[len(updates)-1].Status != StatusCompleted {
		t.Fatalf("terminal event = %#v", updates[len(updates)-1])
	}
}

func TestClassifyDownloadFailure(t *testing.T) {
	tests := []struct {
		name       string
		kind       media.SourceKind
		err        error
		wantCode   diagnostics.Code
		wantStatus int
	}{
		{
			name:       "authentication required",
			err:        &transport.HTTPStatusError{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized"},
			wantCode:   diagnostics.DownloadAuthRequired,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "access forbidden",
			err:        &transport.HTTPStatusError{StatusCode: http.StatusForbidden, Status: "403 Forbidden"},
			wantCode:   downloadAccessForbiddenCode,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "source not found",
			err:        &transport.HTTPStatusError{StatusCode: http.StatusNotFound, Status: "404 Not Found"},
			wantCode:   diagnostics.DownloadSourceNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "rate limited",
			err:        &transport.HTTPStatusError{StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests"},
			wantCode:   downloadRateLimitedCode,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:     "invalid HLS manifest",
			kind:     media.SourceHLS,
			err:      &parser.ValidationError{Subject: "HLS playlist", Detail: "invalid"},
			wantCode: diagnostics.DownloadManifestInvalid,
		},
		{
			name:     "failed segment",
			kind:     media.SourceHLS,
			err:      fmt.Errorf("segment 2: %w", &downloader.TransportError{Operation: "fetch HLS segment", Err: errors.New("connection reset")}),
			wantCode: diagnostics.DownloadSegmentFailed,
		},
		{
			name:     "network failure",
			err:      &downloader.TransportError{Operation: "fetch URL", Err: errors.New("connection reset")},
			wantCode: diagnostics.DownloadTransportFailed,
		},
		{
			name:     "filesystem failure",
			err:      &os.PathError{Op: "write", Path: "video.mp4", Err: os.ErrPermission},
			wantCode: diagnostics.DownloadWriteFailed,
		},
		{
			name:     "cancelled",
			err:      context.Canceled,
			wantCode: diagnostics.DownloadCancelled,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classification := classifyDownloadFailure(test.kind, test.err)
			if classification.code != test.wantCode {
				t.Fatalf("code = %q, want %q", classification.code, test.wantCode)
			}
			if classification.httpStatus != test.wantStatus {
				t.Fatalf("HTTP status = %d, want %d", classification.httpStatus, test.wantStatus)
			}
			if classification.message == "" {
				t.Fatal("classification returned an empty safe message")
			}
		})
	}
}

func TestServicePublishesSafeDiagnosticAndTechnicalContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	technicalStore := diagnostics.NewTechnicalStore()
	technicalStore.SetEnabled(true)
	service := NewService(Config{
		DefaultOutputDir: func() (string, error) { return t.TempDir(), nil },
		DefaultWorkers:   1,
		TechnicalStore:   technicalStore,
	})
	const sensitiveHeader = "Authorization: Bearer browser-secret"
	snapshot, err := service.Queue(context.Background(), Request{
		ID:        "captured-job",
		URL:       server.URL + "/video.mp4",
		OutputDir: t.TempDir(),
		TechnicalContext: func() *diagnostics.TechnicalDiagnostic {
			return &diagnostics.TechnicalDiagnostic{
				RawError:  "capture-side context",
				RequestID: "browser-request",
				RequestHeaders: map[string]string{
					"Authorization": sensitiveHeader,
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	failed, err := service.Wait(context.Background(), snapshot.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if failed.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", failed.Status)
	}
	if failed.Diagnostic == nil {
		t.Fatal("failed snapshot has no safe diagnostic")
	}
	if failed.Diagnostic.Code != diagnostics.DownloadAuthRequired {
		t.Fatalf("diagnostic code = %q, want %q", failed.Diagnostic.Code, diagnostics.DownloadAuthRequired)
	}
	if failed.Message != "Authentication is required to download this source." {
		t.Fatalf("message = %q, want safe compatibility text", failed.Message)
	}
	if failed.OutputPath != "" {
		t.Fatalf("failed download retained an output path: %q", failed.OutputPath)
	}
	if failed.Diagnostic.SafeContext["httpStatus"] != "401" {
		t.Fatalf("safe HTTP status = %#v", failed.Diagnostic.SafeContext)
	}

	serialized, err := json.Marshal(failed)
	if err != nil {
		t.Fatalf("marshal failed snapshot: %v", err)
	}
	if strings.Contains(string(serialized), "browser-secret") || strings.Contains(string(serialized), "capture-side context") {
		t.Fatalf("safe snapshot exposed technical context: %s", serialized)
	}

	technical := technicalStore.Get(failed.Diagnostic.DiagnosticID)
	if technical == nil {
		t.Fatal("technical diagnostic was not correlated to the failed job")
	}
	if technical.RequestID != "browser-request" || technical.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("unexpected technical context: %#v", technical)
	}
	if !strings.Contains(technical.RawError, "capture-side context") || !strings.Contains(technical.RawError, "HTTP 401") {
		t.Fatalf("technical failure was not combined: %q", technical.RawError)
	}
	if technical.RequestHeaders["Authorization"] != sensitiveHeader {
		t.Fatalf("capture headers were not retained in technical context: %#v", technical.RequestHeaders)
	}
}

func TestServiceDoesNotResolveTechnicalContextWhenDisabled(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	technicalStore := diagnostics.NewTechnicalStore()
	resolved := false
	service := NewService(Config{
		DefaultOutputDir: func() (string, error) { return t.TempDir(), nil },
		DefaultWorkers:   1,
		TechnicalStore:   technicalStore,
	})
	snapshot, err := service.Queue(context.Background(), Request{
		ID:        "safe-job",
		URL:       server.URL + "/missing.mp4",
		OutputDir: t.TempDir(),
		TechnicalContext: func() *diagnostics.TechnicalDiagnostic {
			resolved = true
			return &diagnostics.TechnicalDiagnostic{RawError: "must not be retained"}
		},
	})
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	failed, err := service.Wait(context.Background(), snapshot.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if failed.Diagnostic == nil || failed.Diagnostic.Code != diagnostics.DownloadSourceNotFound {
		t.Fatalf("unexpected safe diagnostic: %#v", failed.Diagnostic)
	}
	if resolved || technicalStore.Count() != 0 {
		t.Fatalf("technical context was retained while diagnostics were disabled: resolved=%v count=%d", resolved, technicalStore.Count())
	}
}
