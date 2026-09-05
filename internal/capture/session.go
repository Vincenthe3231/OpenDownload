package capture

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	pairingLifetime = 5 * time.Minute
	maxPayloadBytes = 64 << 10
)

var allowedHeaders = map[string]string{
	"accept":          "Accept",
	"accept-language": "Accept-Language",
	"authorization":   "Authorization",
	"cookie":          "Cookie",
	"origin":          "Origin",
	"referer":         "Referer",
	"user-agent":      "User-Agent",
}

type Stream struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Host       string    `json:"host"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	CapturedAt time.Time `json:"capturedAt"`
}

type Pairing struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type receivedStream struct {
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers"`
}

type storedStream struct {
	Stream
	headers map[string]string
}

type Manager struct {
	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	token    string
	expires  time.Time
	paired   bool
	streams  map[string]storedStream
	seen     map[string]struct{}
	now      func() time.Time
}

func NewManager() *Manager {
	return &Manager{
		streams: make(map[string]storedStream),
		seen:    make(map[string]struct{}),
		now:     time.Now,
	}
}

func (m *Manager) Start() (Pairing, error) {
	m.Stop()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Pairing{}, fmt.Errorf("start Firefox capture receiver: %w", err)
	}
	token, err := randomToken()
	if err != nil {
		listener.Close()
		return Pairing{}, err
	}

	m.mu.Lock()
	m.listener = listener
	m.token = token
	m.expires = m.now().Add(pairingLifetime)
	m.paired = false
	m.streams = make(map[string]storedStream)
	m.seen = make(map[string]struct{})
	server := &http.Server{Handler: http.HandlerFunc(m.handleStream)}
	m.server = server
	pairing := Pairing{
		Code:      fmt.Sprintf("http://%s#%s", listener.Addr().String(), token),
		ExpiresAt: m.expires,
	}
	m.mu.Unlock()

	go server.Serve(listener)
	return pairing, nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	server := m.server
	m.server = nil
	m.listener = nil
	m.token = ""
	m.expires = time.Time{}
	m.paired = false
	m.streams = make(map[string]storedStream)
	m.seen = make(map[string]struct{})
	m.mu.Unlock()
	if server != nil {
		server.Close()
	}
}

func (m *Manager) List() []Stream {
	m.mu.Lock()
	defer m.mu.Unlock()

	streams := make([]Stream, 0, len(m.streams))
	for _, stream := range m.streams {
		streams = append(streams, stream.Stream)
	}
	return streams
}

func (m *Manager) Get(id string) (Stream, map[string]string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.streams[id]
	if !ok {
		return Stream{}, nil, false
	}
	headers := make(map[string]string, len(stream.headers))
	for key, value := range stream.headers {
		headers[key] = value
	}
	return stream.Stream, headers, true
}

func (m *Manager) handleStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, OpenDownloadSession")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/v1/firefox/streams" || !isLoopback(r.RemoteAddr) {
		http.NotFound(w, r)
		return
	}

	m.mu.Lock()
	valid := m.token != "" && (m.paired || m.now().Before(m.expires)) && r.Header.Get("OpenDownloadSession") == m.token
	m.mu.Unlock()
	if !valid {
		http.Error(w, "invalid or expired pairing code", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadBytes)
	defer r.Body.Close()
	var received receivedStream
	if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
		http.Error(w, "invalid capture payload", http.StatusBadRequest)
		return
	}

	stream, headers, err := validate(received, m.now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	if m.token == "" {
		m.mu.Unlock()
		http.Error(w, "capture session stopped", http.StatusUnauthorized)
		return
	}
	m.paired = true
	if _, exists := m.seen[stream.URL]; !exists {
		m.seen[stream.URL] = struct{}{}
		m.streams[stream.ID] = storedStream{Stream: stream, headers: headers}
	}
	m.mu.Unlock()
	w.WriteHeader(http.StatusCreated)
}

func validate(received receivedStream, capturedAt time.Time) (Stream, map[string]string, error) {
	parsed, err := url.Parse(received.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return Stream{}, nil, fmt.Errorf("capture URL must be http or https")
	}
	headers := make(map[string]string)
	for key, value := range received.Headers {
		canonical, ok := allowedHeaders[strings.ToLower(key)]
		if !ok || value == "" || len(value) > 8192 {
			continue
		}
		headers[canonical] = value
	}
	name := path.Base(parsed.Path)
	if name == "." || name == "/" || name == "" {
		name = parsed.Host
	}
	id, err := randomToken()
	if err != nil {
		return Stream{}, nil, err
	}
	return Stream{
		ID:         id,
		URL:        received.URL,
		Host:       parsed.Host,
		Name:       name,
		Type:       normalizeType(received.Type, parsed.Path),
		CapturedAt: capturedAt,
	}, headers, nil
}

func normalizeType(value, rawPath string) string {
	switch strings.ToLower(value) {
	case "hls", "dash", "video", "audio":
		return strings.ToLower(value)
	}
	switch strings.ToLower(path.Ext(rawPath)) {
	case ".m3u8":
		return "hls"
	case ".mpd":
		return "dash"
	default:
		return "media"
	}
}

func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("create capture token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
