package capture

import (
	"time"

	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/nativeprotocol"
)

// SetTechnicalStore supplies the desktop-owned, memory-only sensitive
// diagnostic store. A nil value disables capture of technical context.
func (m *Manager) SetTechnicalStore(store *diagnostics.TechnicalStore) {
	m.mu.Lock()
	m.technicalStore = store
	m.mu.Unlock()
}

// SetDiagnosticReporter receives safe diagnostics after they have been
// published to capture state. Reporters must not mutate the manager.
func (m *Manager) SetDiagnosticReporter(reporter func(diagnostics.Diagnostic)) {
	m.mu.Lock()
	m.diagnosticReporter = reporter
	m.mu.Unlock()
}

// DeveloperDiagnosticsEnabled reports whether new capture sessions may retain
// sensitive browser context. The setting is intentionally sampled at start.
func (m *Manager) DeveloperDiagnosticsEnabled() bool {
	m.mu.Lock()
	store := m.technicalStore
	m.mu.Unlock()
	return store != nil && store.Enabled()
}

// ClearTechnicalCapture removes capture-scoped sensitive data immediately.
func (m *Manager) ClearTechnicalCapture() {
	m.mu.Lock()
	store := m.technicalStore
	ids := m.clearTechnicalCaptureLocked()
	for id, stream := range m.streams {
		stream.debug = nil
		m.streams[id] = stream
	}
	m.diagnosticMode = false
	m.mu.Unlock()
	for _, id := range ids {
		if store != nil {
			store.Delete(id)
		}
	}
}

func (m *Manager) clearTechnicalCaptureLocked() []string {
	ids := make([]string, 0, len(m.technicalDiagnosticIDs))
	for id := range m.technicalDiagnosticIDs {
		ids = append(ids, id)
	}
	m.technicalDiagnosticIDs = make(map[string]struct{})
	return ids
}

func (m *Manager) diagnosticModeLocked() bool {
	return m.diagnosticMode && m.technicalStore != nil && m.technicalStore.Enabled()
}

func (m *Manager) reportCaptureDiagnostic(diagnostic diagnostics.Diagnostic, technical diagnostics.TechnicalDiagnostic) {
	m.SetDiagnostic(diagnostic)

	m.mu.Lock()
	store := m.technicalStore
	reporter := m.diagnosticReporter
	m.mu.Unlock()
	if store != nil && store.Enabled() {
		technical.DiagnosticID = diagnostic.DiagnosticID
		technical.OccurredAt = diagnostic.OccurredAt
		if store.Record(technical) != "" {
			m.mu.Lock()
			m.technicalDiagnosticIDs[diagnostic.DiagnosticID] = struct{}{}
			m.mu.Unlock()
		}
	}
	if reporter != nil {
		reporter(diagnostic)
	}
}

// TechnicalContext returns a copy of the selected stream's raw Gecko request
// context while developer diagnostics remains enabled. It is intentionally
// absent from all stream summaries and events.
func (m *Manager) TechnicalContext(id string) *diagnostics.TechnicalDiagnostic {
	m.mu.Lock()
	stored, ok := m.streams[id]
	enabled := m.diagnosticModeLocked()
	m.mu.Unlock()
	if !ok || !enabled || stored.debug == nil {
		return nil
	}
	return technicalFromDebug(stored.debug)
}

func technicalFromDebug(debug *nativeprotocol.DebugContext) *diagnostics.TechnicalDiagnostic {
	if debug == nil {
		return nil
	}
	return &diagnostics.TechnicalDiagnostic{
		OccurredAt:           time.Now().UTC(),
		RequestID:            debug.RequestID,
		RequestMethod:        debug.RequestMethod,
		RequestType:          debug.RequestType,
		RequestTimestamp:     debug.RequestTimestamp,
		RequestFrameID:       debug.RequestFrameID,
		RequestParentFrameID: debug.RequestParentFrameID,
		RequestURL:           debug.RequestURL,
		RequestDocumentURL:   debug.RequestDocumentURL,
		RequestOriginURL:     debug.RequestOriginURL,
		RequestInitiator:     debug.RequestInitiator,
		RequestHeaders:       cloneHeaders(debug.RequestHeaders),
		ResponseHeaders:      cloneHeaders(debug.ResponseHeaders),
		ResponseStatusLine:   debug.ResponseStatusLine,
		ResponseFromCache:    debug.ResponseFromCache,
		ResponseIP:           debug.ResponseIP,
		HTTPStatus:           debug.ResponseStatus,
	}
}

func cloneDebug(debug *nativeprotocol.DebugContext) *nativeprotocol.DebugContext {
	if debug == nil {
		return nil
	}
	copy := *debug
	copy.RequestHeaders = cloneHeaders(debug.RequestHeaders)
	copy.ResponseHeaders = cloneHeaders(debug.ResponseHeaders)
	return &copy
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	copy := make(map[string]string, len(headers))
	for key, value := range headers {
		copy[key] = value
	}
	return copy
}
