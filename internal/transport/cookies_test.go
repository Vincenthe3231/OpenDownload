package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestHTTPClientLoadsNetscapeCookieFile(t *testing.T) {
	var receivedCookie string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedCookie = request.Header.Get("Cookie")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	cookieFile := filepath.Join(t.TempDir(), "cookies.txt")
	cookieLine := serverURL.Hostname() + "\tFALSE\t/\tFALSE\t2147483647\tsession\ttrusted\n"
	if err := os.WriteFile(cookieFile, []byte(cookieLine), 0o600); err != nil {
		t.Fatalf("write cookie file: %v", err)
	}

	client := NewHTTPClient(HTTPClientConfig{CookieFile: cookieFile, MaxRetries: -1})
	response, err := client.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("GetBody() error = %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response: %v", err)
	}
	if receivedCookie != "session=trusted" {
		t.Fatalf("received cookie = %q, want %q", receivedCookie, "session=trusted")
	}
}
