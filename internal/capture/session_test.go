package capture

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleStreamStoresSafeMetadataAndPrivateHeaders(t *testing.T) {
	manager := NewManager()
	manager.token = "test-token"
	manager.expires = time.Now().Add(time.Minute)
	payload, _ := json.Marshal(receivedStream{
		URL:  "https://cdn.example.test/live/video.m3u8",
		Type: "hls",
		Headers: map[string]string{
			"Cookie":        "session=secret",
			"Referer":       "https://example.test/watch",
			"X-Untrusted":   "discard",
			"Authorization": "Bearer secret",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/firefox/streams", bytes.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:50001"
	req.Header.Set("OpenDownloadSession", "test-token")
	response := httptest.NewRecorder()

	manager.handleStream(response, req)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	streams := manager.List()
	if len(streams) != 1 || streams[0].Name != "video.m3u8" || streams[0].Type != "hls" {
		t.Fatalf("unexpected public streams: %#v", streams)
	}
	_, headers, ok := manager.Get(streams[0].ID)
	if !ok || headers["Cookie"] != "session=secret" || headers["X-Untrusted"] != "" {
		t.Fatalf("unexpected private headers: %#v", headers)
	}
}

func TestHandleStreamRejectsExpiredPairing(t *testing.T) {
	manager := NewManager()
	manager.token = "expired"
	manager.expires = time.Now().Add(-time.Minute)
	req := httptest.NewRequest(http.MethodPost, "/v1/firefox/streams", bytes.NewBufferString(`{"url":"https://example.test/video.mp4"}`))
	req.RemoteAddr = "127.0.0.1:50001"
	req.Header.Set("OpenDownloadSession", "expired")
	response := httptest.NewRecorder()

	manager.handleStream(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestHandleStreamKeepsPairedSessionAfterPairingExpiry(t *testing.T) {
	manager := NewManager()
	manager.token = "paired"
	manager.paired = true
	manager.expires = time.Now().Add(-time.Minute)
	req := httptest.NewRequest(http.MethodPost, "/v1/firefox/streams", bytes.NewBufferString(`{"url":"https://example.test/video.mp4"}`))
	req.RemoteAddr = "127.0.0.1:50001"
	req.Header.Set("OpenDownloadSession", "paired")
	response := httptest.NewRecorder()

	manager.handleStream(response, req)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
}

func TestStopClearsStreams(t *testing.T) {
	manager := NewManager()
	manager.streams["stream"] = storedStream{Stream: Stream{ID: "stream"}}
	manager.Stop()
	if len(manager.List()) != 0 {
		t.Fatal("streams remain after stop")
	}
}

func TestStartUsesLoopbackPairingAddress(t *testing.T) {
	manager := NewManager()
	pairing, err := manager.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()

	if !strings.HasPrefix(pairing.Code, "http://127.0.0.1:") || !strings.Contains(pairing.Code, "#") {
		t.Fatalf("unexpected pairing code: %q", pairing.Code)
	}
	if pairing.ExpiresAt.Before(time.Now()) {
		t.Fatal("pairing code is already expired")
	}
}
