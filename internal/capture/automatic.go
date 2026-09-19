package capture

import (
	"fmt"

	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/nativeprotocol"
)

// NativeCapability stays inside the native host and app transport.
type NativeCapability struct {
	SessionID      string
	Secret         string
	DiagnosticMode bool
}

// SetDiagnostic publishes the latest safe diagnostic in capture state.
func (m *Manager) SetDiagnostic(diagnostic diagnostics.Diagnostic) {
	m.mu.Lock()
	mode := m.mode
	status := m.nativeStatus
	session := m.sessionLocked()
	session.Mode = mode
	session.NativeStatus = status
	session.Diagnostic = &diagnostic
	m.mu.Unlock()
	m.emit(Event{Type: EventSessionChanged, Session: &session})
}

// StartAutomatic starts the one app-wide automatic capture session.
func (m *Manager) StartAutomatic(browser string, tabID int64) (NativeCapability, error) {
	m.Stop()
	sessionID, err := randomToken()
	if err != nil {
		return NativeCapability{}, fmt.Errorf("create automatic capture session: %w", err)
	}
	secret, err := randomToken()
	if err != nil {
		return NativeCapability{}, fmt.Errorf("create automatic capture capability: %w", err)
	}

	m.mu.Lock()
	m.mode = ModeAutomatic
	m.nativeStatus = NativeConnected
	m.browser = browser
	m.tabID = tabID
	m.autoSessionID = sessionID
	m.autoSecret = secret
	m.streams = make(map[string]storedStream)
	m.streamOrder = nil
	m.seen = make(map[string]struct{})
	session := m.sessionLocked()
	m.mu.Unlock()
	m.emit(Event{Type: EventSessionChanged, Session: &session})
	return NativeCapability{SessionID: sessionID, Secret: secret}, nil
}

// AcceptAutomaticStream validates host capability and selected-tab isolation.
func (m *Manager) AcceptAutomaticStream(sessionID, secret string, tabID int64, input nativeprotocol.Stream) error {
	m.mu.Lock()
	valid := m.mode == ModeAutomatic && m.autoSessionID == sessionID && m.autoSecret == secret && m.tabID == tabID
	m.mu.Unlock()
	if !valid {
		return fmt.Errorf("automatic capture capability is invalid")
	}

	stream, headers, err := validate(receivedStream{URL: input.URL, Type: input.Type, Headers: input.Headers}, m.now())
	if err != nil {
		return err
	}

	m.mu.Lock()
	if m.mode != ModeAutomatic || m.autoSessionID != sessionID || m.autoSecret != secret || m.tabID != tabID {
		m.mu.Unlock()
		return fmt.Errorf("automatic capture session was replaced")
	}
	if _, exists := m.seen[stream.URL]; exists {
		m.mu.Unlock()
		return nil
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
	return nil
}

// StopAutomatic stops a session only when its capability still owns the app.
func (m *Manager) StopAutomatic(sessionID, secret string) {
	m.mu.Lock()
	owned := m.mode == ModeAutomatic && m.autoSessionID == sessionID && m.autoSecret == secret
	m.mu.Unlock()
	if owned {
		m.Stop()
	}
}
