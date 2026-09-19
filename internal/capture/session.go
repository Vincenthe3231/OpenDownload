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

	"github.com/opendownload/opendownload/internal/diagnostics"
)

// Manager owns one local browser capture session and its in memory request data.
type Manager struct {
	mu                     sync.Mutex
	server                 *http.Server
	listener               net.Listener
	token                  string
	expires                time.Time
	paired                 bool
	streams                map[string]storedStream
	streamOrder            []string
	seen                   map[string]struct{}
	mode                   Mode
	nativeStatus           NativeConnectionStatus
	browser                string
	tabID                  int64
	autoSessionID          string
	autoSecret             string
	diagnosticMode         bool
	technicalStore         *diagnostics.TechnicalStore
	diagnosticReporter     func(diagnostics.Diagnostic)
	technicalDiagnosticIDs map[string]struct{}
	now                    func() time.Time
	listeners              map[uint64]Listener
	nextListener           uint64
}

// NewManager returns an idle capture manager.
func NewManager() *Manager {
	return &Manager{
		streams:                make(map[string]storedStream),
		seen:                   make(map[string]struct{}),
		now:                    time.Now,
		listeners:              make(map[uint64]Listener),
		technicalStore:         diagnostics.NewTechnicalStore(),
		technicalDiagnosticIDs: make(map[string]struct{}),
		mode:                   ModeManual,
		nativeStatus:           NativeDisconnected,
	}
}

// Subscribe registers a local listener and returns an unsubscribe function.
func (m *Manager) Subscribe(listener Listener) func() {
	if listener == nil {
		return func() {}
	}
	m.mu.Lock()
	id := m.nextListener
	m.nextListener++
	m.listeners[id] = listener
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		delete(m.listeners, id)
		m.mu.Unlock()
	}
}

// Session returns the current safe capture state.
func (m *Manager) Session() SessionSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionLocked()
}

// Start replaces any active capture session with a fresh loopback session.
func (m *Manager) Start() (Pairing, error) {
	m.Stop()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Pairing{}, fmt.Errorf("start browser capture receiver: %w", err)
	}
	token, err := randomToken()
	if err != nil {
		_ = listener.Close()
		return Pairing{}, err
	}

	m.mu.Lock()
	m.listener = listener
	m.mode = ModeManual
	m.nativeStatus = NativeDisconnected
	m.browser = ""
	m.tabID = 0
	m.autoSessionID = ""
	m.autoSecret = ""
	m.token = token
	m.expires = m.now().Add(pairingLifetime)
	m.paired = false
	m.streams = make(map[string]storedStream)
	m.streamOrder = nil
	m.seen = make(map[string]struct{})
	server := &http.Server{Handler: http.HandlerFunc(m.handleStream)}
	m.server = server
	pairing := Pairing{
		Code:      fmt.Sprintf("http://%s#%s", listener.Addr().String(), token),
		ExpiresAt: m.expires,
	}
	session := m.sessionLocked()
	m.mu.Unlock()

	go func() {
		_ = server.Serve(listener)
	}()
	m.emit(Event{Type: EventSessionChanged, Session: &session})
	return pairing, nil
}

// Stop clears the active session, captures, and private headers.
func (m *Manager) Stop() {
	m.mu.Lock()
	server := m.server
	wasActive := server != nil || m.token != "" || m.autoSessionID != ""
	m.server = nil
	m.listener = nil
	m.token = ""
	m.expires = time.Time{}
	m.paired = false
	m.mode = ModeManual
	m.nativeStatus = NativeDisconnected
	m.browser = ""
	m.tabID = 0
	m.autoSessionID = ""
	m.autoSecret = ""
	m.streams = make(map[string]storedStream)
	m.streamOrder = nil
	m.seen = make(map[string]struct{})
	m.mu.Unlock()
	if server != nil {
		_ = server.Close()
	}
	if wasActive {
		session := SessionSnapshot{Mode: ModeManual, NativeStatus: NativeDisconnected}
		m.emit(Event{Type: EventSessionChanged, Session: &session})
	}
}

// List returns ordered public summaries without private request data.
func (m *Manager) List() []StreamSummary {
	m.mu.Lock()
	defer m.mu.Unlock()

	streams := make([]StreamSummary, 0, len(m.streamOrder))
	for _, id := range m.streamOrder {
		stream, ok := m.streams[id]
		if ok {
			streams = append(streams, summarize(stream.Stream))
		}
	}
	return streams
}

// Get returns the stored request URL and a private header copy for a download job.
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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+sessionHeader)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != captureRoute || !isLoopback(r.RemoteAddr) {
		http.NotFound(w, r)
		return
	}

	m.mu.Lock()
	valid := m.token != "" && (m.paired || m.now().Before(m.expires)) && r.Header.Get(sessionHeader) == m.token
	m.mu.Unlock()
	if !valid {
		http.Error(w, "invalid or expired pairing code", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadBytes)
	defer func() { _ = r.Body.Close() }()
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
	if _, exists := m.seen[stream.URL]; exists {
		m.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		return
	}
	m.evictOldestLocked()
	m.seen[stream.URL] = struct{}{}
	m.streams[stream.ID] = storedStream{Stream: stream, headers: headers}
	m.streamOrder = append(m.streamOrder, stream.ID)
	summary := summarize(stream)
	session := m.sessionLocked()
	m.mu.Unlock()

	m.emit(Event{Type: EventSessionChanged, Session: &session})
	m.emit(Event{Type: EventStreamAdded, Stream: &summary})
	w.WriteHeader(http.StatusCreated)
}

func (m *Manager) evictOldestLocked() {
	if len(m.streamOrder) < maxCapturedStreams {
		return
	}
	oldestID := m.streamOrder[0]
	m.streamOrder = m.streamOrder[1:]
	if stream, ok := m.streams[oldestID]; ok {
		delete(m.seen, stream.URL)
	}
	delete(m.streams, oldestID)
}

func (m *Manager) sessionLocked() SessionSnapshot {
	return SessionSnapshot{
		Active:       m.token != "" || m.autoSessionID != "",
		ExpiresAt:    m.expires,
		Paired:       m.paired,
		Mode:         m.mode,
		NativeStatus: m.nativeStatus,
		Browser:      m.browser,
		TabID:        m.tabID,
	}
}

func (m *Manager) emit(event Event) {
	m.mu.Lock()
	listeners := make([]Listener, 0, len(m.listeners))
	for _, listener := range m.listeners {
		listeners = append(listeners, listener)
	}
	m.mu.Unlock()
	for _, listener := range listeners {
		listener(event)
	}
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
