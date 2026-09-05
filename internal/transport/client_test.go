package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPClientRetriesTransientServerFailure(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if attempts.Add(1) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write([]byte("ready"))
	}))
	defer server.Close()

	client := NewHTTPClient(HTTPClientConfig{MaxRetries: 1})
	body, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(body) != "ready" {
		t.Fatalf("Get() body = %q, want %q", body, "ready")
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestHTTPClientReturnsTypedStatusError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	_, err := NewHTTPClient(HTTPClientConfig{MaxRetries: -1}).Get(context.Background(), server.URL)
	var statusError *HTTPStatusError
	if !errors.As(err, &statusError) {
		t.Fatalf("Get() error = %v, want HTTPStatusError", err)
	}
	if statusError.StatusCode != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", statusError.StatusCode, http.StatusNotFound)
	}
}

func TestHTTPClientLimiterReleasesWhenBodyCloses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("body"))
	}))
	defer server.Close()

	limiter := NewConnectionLimiter(1)
	client := NewHTTPClient(HTTPClientConfig{Limiter: limiter, MaxRetries: -1})
	first, err := client.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("first GetBody() error = %v", err)
	}
	if limiter.Active() != 1 {
		t.Fatalf("active connections = %d, want 1", limiter.Active())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err = client.GetBody(ctx, server.URL)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second GetBody() error = %v, want deadline exceeded", err)
	}

	if _, err := io.Copy(io.Discard, first.Body); err != nil {
		t.Fatalf("read first response: %v", err)
	}
	if err := first.Body.Close(); err != nil {
		t.Fatalf("close first response: %v", err)
	}
	if limiter.Active() != 0 {
		t.Fatalf("active connections = %d, want 0", limiter.Active())
	}

	third, err := client.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("third GetBody() error = %v", err)
	}
	if err := third.Body.Close(); err != nil {
		t.Fatalf("close third response: %v", err)
	}
}
