package diagnostics

import (
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	// DefaultTechnicalStoreMaxRecords bounds the number of sensitive diagnostic
	// records retained for the current process.
	DefaultTechnicalStoreMaxRecords = 20
	// DefaultTechnicalStoreMaxBytes bounds sensitive diagnostic payload retained
	// by the current process.
	DefaultTechnicalStoreMaxBytes = 2 << 20
	// DefaultTechnicalHeaderValueBytes bounds each retained header value.
	DefaultTechnicalHeaderValueBytes = 8 << 10

	maxTechnicalHeadersPerSide    = 64
	maxTechnicalHeaderKeyBytes    = 256
	maxTechnicalRequestIDBytes    = 8 << 10
	maxTechnicalRequestFieldBytes = 8 << 10
	maxTechnicalRawErrorBytes     = 128 << 10
	maxTechnicalRequestURLBytes   = 64 << 10
	maxTechnicalHostDetailBytes   = 128 << 10
	maxTechnicalPipeDetailBytes   = 128 << 10

	technicalDiagnosticOverhead = 1024
	technicalHeaderOverhead     = 64
)

// TechnicalDiagnostic contains sensitive debugging information. It is valid
// only in process memory while developer diagnostics is enabled. Do not put it
// into Diagnostic, History, logs, events, exports, or extension messages.
type TechnicalDiagnostic struct {
	DiagnosticID         string            `json:"diagnosticId"`
	OccurredAt           time.Time         `json:"occurredAt"`
	RawError             string            `json:"rawError,omitempty"`
	RequestID            string            `json:"requestId,omitempty"`
	RequestMethod        string            `json:"requestMethod,omitempty"`
	RequestType          string            `json:"requestType,omitempty"`
	RequestTimestamp     float64           `json:"requestTimestamp,omitempty"`
	RequestFrameID       int64             `json:"requestFrameId,omitempty"`
	RequestParentFrameID int64             `json:"requestParentFrameId,omitempty"`
	RequestURL           string            `json:"requestUrl,omitempty"`
	RequestDocumentURL   string            `json:"requestDocumentUrl,omitempty"`
	RequestOriginURL     string            `json:"requestOriginUrl,omitempty"`
	RequestInitiator     string            `json:"requestInitiator,omitempty"`
	RequestHeaders       map[string]string `json:"requestHeaders,omitempty"`
	ResponseHeaders      map[string]string `json:"responseHeaders,omitempty"`
	ResponseStatusLine   string            `json:"responseStatusLine,omitempty"`
	ResponseFromCache    bool              `json:"responseFromCache,omitempty"`
	ResponseIP           string            `json:"responseIp,omitempty"`
	NativeHostDetail     string            `json:"nativeHostDetail,omitempty"`
	PipeDetail           string            `json:"pipeDetail,omitempty"`
	HTTPStatus           int               `json:"httpStatus,omitempty"`
	Truncated            bool              `json:"truncated"`
}

// TechnicalStoreOptions controls TechnicalStore memory limits. Zero values use
// the package defaults.
type TechnicalStoreOptions struct {
	MaxRecords          int
	MaxBytes            int
	MaxHeaderValueBytes int
}

type storedTechnicalDiagnostic struct {
	diagnostic TechnicalDiagnostic
	bytes      int
}

// TechnicalStore holds sensitive diagnostics in memory only. It is disabled by
// default and clears all records when disabled.
type TechnicalStore struct {
	mu                  sync.RWMutex
	enabled             bool
	entries             map[string]storedTechnicalDiagnostic
	order               []string
	bytes               int
	maxRecords          int
	maxBytes            int
	maxHeaderValueBytes int
}

// NewTechnicalStore creates a disabled in-memory technical diagnostic store.
func NewTechnicalStore() *TechnicalStore {
	return NewTechnicalStoreWithOptions(TechnicalStoreOptions{})
}

// NewTechnicalStoreWithOptions creates a disabled in-memory technical
// diagnostic store with explicit limits. Zero-valued limits use defaults.
func NewTechnicalStoreWithOptions(options TechnicalStoreOptions) *TechnicalStore {
	maxRecords := options.MaxRecords
	if maxRecords <= 0 {
		maxRecords = DefaultTechnicalStoreMaxRecords
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultTechnicalStoreMaxBytes
	}
	maxHeaderValueBytes := options.MaxHeaderValueBytes
	if maxHeaderValueBytes <= 0 {
		maxHeaderValueBytes = DefaultTechnicalHeaderValueBytes
	}

	return &TechnicalStore{
		entries:             make(map[string]storedTechnicalDiagnostic),
		maxRecords:          maxRecords,
		maxBytes:            maxBytes,
		maxHeaderValueBytes: maxHeaderValueBytes,
	}
}

// Enabled reports whether the store can retain technical diagnostics.
func (s *TechnicalStore) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// SetEnabled changes runtime diagnostic capture. Disabling clears all
// sensitive records immediately. This state is never persisted.
func (s *TechnicalStore) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	if !enabled {
		s.clearLocked()
	}
}

// Record retains one technical diagnostic while enabled and returns its
// diagnostic ID. It returns an empty string when diagnostics are disabled or
// the record cannot fit within the configured memory limit.
func (s *TechnicalStore) Record(diagnostic TechnicalDiagnostic) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		return ""
	}

	diagnostic = normalizeTechnicalDiagnostic(diagnostic, s.maxHeaderValueBytes)
	if diagnostic.DiagnosticID == "" {
		diagnostic.DiagnosticID = NewID()
	}
	if diagnostic.OccurredAt.IsZero() {
		diagnostic.OccurredAt = time.Now().UTC()
	}
	diagnostic = fitTechnicalDiagnostic(diagnostic, s.maxBytes)
	size := technicalDiagnosticSize(diagnostic)
	if size > s.maxBytes {
		return ""
	}

	s.removeLocked(diagnostic.DiagnosticID)
	for len(s.order) >= s.maxRecords || s.bytes+size > s.maxBytes {
		if len(s.order) == 0 {
			return ""
		}
		s.removeLocked(s.order[0])
	}

	s.entries[diagnostic.DiagnosticID] = storedTechnicalDiagnostic{
		diagnostic: cloneTechnicalDiagnostic(diagnostic),
		bytes:      size,
	}
	s.order = append(s.order, diagnostic.DiagnosticID)
	s.bytes += size
	return diagnostic.DiagnosticID
}

// Get returns a copy of one technical diagnostic while enabled. It returns nil
// when diagnostics are disabled or the ID is not retained.
func (s *TechnicalStore) Get(diagnosticID string) *TechnicalDiagnostic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.enabled {
		return nil
	}
	record, ok := s.entries[diagnosticID]
	if !ok {
		return nil
	}
	diagnostic := cloneTechnicalDiagnostic(record.diagnostic)
	return &diagnostic
}

// Delete clears one technical diagnostic. It is safe to call for an unknown
// diagnostic ID.
func (s *TechnicalStore) Delete(diagnosticID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeLocked(diagnosticID)
}

// Clear removes all technical diagnostics from memory.
func (s *TechnicalStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked()
}

// Count returns the number of retained technical diagnostics.
func (s *TechnicalStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.order)
}

// Bytes returns the bounded payload size of retained technical diagnostics.
func (s *TechnicalStore) Bytes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bytes
}

func (s *TechnicalStore) removeLocked(diagnosticID string) {
	record, ok := s.entries[diagnosticID]
	if !ok {
		return
	}
	delete(s.entries, diagnosticID)
	s.bytes -= record.bytes
	for index, id := range s.order {
		if id == diagnosticID {
			s.order = append(s.order[:index], s.order[index+1:]...)
			break
		}
	}
}

func (s *TechnicalStore) clearLocked() {
	s.entries = make(map[string]storedTechnicalDiagnostic)
	s.order = nil
	s.bytes = 0
}

func normalizeTechnicalDiagnostic(diagnostic TechnicalDiagnostic, maxHeaderValueBytes int) TechnicalDiagnostic {
	var truncated bool
	diagnostic.DiagnosticID, truncated = truncateTechnicalString(diagnostic.DiagnosticID, maxTechnicalHeaderKeyBytes)
	diagnostic.RawError, truncated = truncateWithFlag(diagnostic.RawError, maxTechnicalRawErrorBytes, truncated)
	diagnostic.RequestID, truncated = truncateWithFlag(diagnostic.RequestID, maxTechnicalRequestIDBytes, truncated)
	diagnostic.RequestMethod, truncated = truncateWithFlag(diagnostic.RequestMethod, maxTechnicalRequestFieldBytes, truncated)
	diagnostic.RequestType, truncated = truncateWithFlag(diagnostic.RequestType, maxTechnicalRequestFieldBytes, truncated)
	diagnostic.RequestURL, truncated = truncateWithFlag(diagnostic.RequestURL, maxTechnicalRequestURLBytes, truncated)
	diagnostic.RequestDocumentURL, truncated = truncateWithFlag(diagnostic.RequestDocumentURL, maxTechnicalRequestURLBytes, truncated)
	diagnostic.RequestOriginURL, truncated = truncateWithFlag(diagnostic.RequestOriginURL, maxTechnicalRequestURLBytes, truncated)
	diagnostic.RequestInitiator, truncated = truncateWithFlag(diagnostic.RequestInitiator, maxTechnicalRequestURLBytes, truncated)
	diagnostic.NativeHostDetail, truncated = truncateWithFlag(diagnostic.NativeHostDetail, maxTechnicalHostDetailBytes, truncated)
	diagnostic.PipeDetail, truncated = truncateWithFlag(diagnostic.PipeDetail, maxTechnicalPipeDetailBytes, truncated)
	diagnostic.ResponseStatusLine, truncated = truncateWithFlag(diagnostic.ResponseStatusLine, maxTechnicalRequestFieldBytes, truncated)
	diagnostic.ResponseIP, truncated = truncateWithFlag(diagnostic.ResponseIP, maxTechnicalRequestFieldBytes, truncated)
	diagnostic.RequestHeaders, truncated = normalizeHeaders(diagnostic.RequestHeaders, maxHeaderValueBytes, truncated)
	diagnostic.ResponseHeaders, truncated = normalizeHeaders(diagnostic.ResponseHeaders, maxHeaderValueBytes, truncated)
	diagnostic.Truncated = diagnostic.Truncated || truncated
	return diagnostic
}

func truncateWithFlag(value string, maxBytes int, truncated bool) (string, bool) {
	trimmed, didTruncate := truncateTechnicalString(value, maxBytes)
	return trimmed, truncated || didTruncate
}

func normalizeHeaders(headers map[string]string, maxValueBytes int, truncated bool) (map[string]string, bool) {
	if len(headers) == 0 {
		return nil, truncated
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > maxTechnicalHeadersPerSide {
		keys = keys[:maxTechnicalHeadersPerSide]
		truncated = true
	}

	result := make(map[string]string, len(keys))
	for _, key := range keys {
		trimmedKey, keyTruncated := truncateTechnicalString(key, maxTechnicalHeaderKeyBytes)
		trimmedValue, valueTruncated := truncateTechnicalString(headers[key], maxValueBytes)
		if keyTruncated || valueTruncated {
			truncated = true
		}
		uniqueKey := trimmedKey
		for suffix := 1; ; suffix++ {
			if _, exists := result[uniqueKey]; !exists {
				result[uniqueKey] = trimmedValue
				break
			}
			truncated = true
			suffixText := "#" + strconv.Itoa(suffix)
			prefix, _ := truncateTechnicalString(trimmedKey, maxTechnicalHeaderKeyBytes-len(suffixText))
			uniqueKey = prefix + suffixText
		}
	}
	return result, truncated
}

func fitTechnicalDiagnostic(diagnostic TechnicalDiagnostic, maxBytes int) TechnicalDiagnostic {
	if technicalDiagnosticSize(diagnostic) <= maxBytes {
		return diagnostic
	}

	limited := TechnicalDiagnostic{
		DiagnosticID:         diagnostic.DiagnosticID,
		OccurredAt:           diagnostic.OccurredAt,
		RequestTimestamp:     diagnostic.RequestTimestamp,
		RequestFrameID:       diagnostic.RequestFrameID,
		RequestParentFrameID: diagnostic.RequestParentFrameID,
		ResponseFromCache:    diagnostic.ResponseFromCache,
		HTTPStatus:           diagnostic.HTTPStatus,
		Truncated:            true,
	}
	addLimitedString(&limited.RequestID, diagnostic.RequestID, &limited, maxBytes)
	addLimitedString(&limited.RequestMethod, diagnostic.RequestMethod, &limited, maxBytes)
	addLimitedString(&limited.RequestType, diagnostic.RequestType, &limited, maxBytes)
	addLimitedString(&limited.RequestURL, diagnostic.RequestURL, &limited, maxBytes)
	addLimitedString(&limited.RequestDocumentURL, diagnostic.RequestDocumentURL, &limited, maxBytes)
	addLimitedString(&limited.RequestOriginURL, diagnostic.RequestOriginURL, &limited, maxBytes)
	addLimitedString(&limited.RequestInitiator, diagnostic.RequestInitiator, &limited, maxBytes)
	addLimitedString(&limited.RawError, diagnostic.RawError, &limited, maxBytes)
	addLimitedString(&limited.ResponseStatusLine, diagnostic.ResponseStatusLine, &limited, maxBytes)
	addLimitedString(&limited.ResponseIP, diagnostic.ResponseIP, &limited, maxBytes)
	addLimitedString(&limited.NativeHostDetail, diagnostic.NativeHostDetail, &limited, maxBytes)
	addLimitedString(&limited.PipeDetail, diagnostic.PipeDetail, &limited, maxBytes)
	addLimitedHeaders(&limited.RequestHeaders, diagnostic.RequestHeaders, &limited, maxBytes)
	addLimitedHeaders(&limited.ResponseHeaders, diagnostic.ResponseHeaders, &limited, maxBytes)
	return limited
}

func addLimitedString(destination *string, value string, diagnostic *TechnicalDiagnostic, maxBytes int) {
	if value == "" || technicalDiagnosticSize(*diagnostic) >= maxBytes {
		return
	}
	*destination = value
	if technicalDiagnosticSize(*diagnostic) <= maxBytes {
		return
	}
	*destination = ""
	remaining := maxBytes - technicalDiagnosticSize(*diagnostic)
	if remaining <= 0 {
		return
	}
	trimmed, _ := truncateTechnicalString(value, remaining)
	*destination = trimmed
}

func addLimitedHeaders(destination *map[string]string, headers map[string]string, diagnostic *TechnicalDiagnostic, maxBytes int) {
	if len(headers) == 0 || technicalDiagnosticSize(*diagnostic) >= maxBytes {
		return
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if technicalDiagnosticSize(*diagnostic) >= maxBytes {
			return
		}
		if *destination == nil {
			*destination = make(map[string]string)
		}
		(*destination)[key] = headers[key]
		if technicalDiagnosticSize(*diagnostic) <= maxBytes {
			continue
		}
		delete(*destination, key)
		remaining := maxBytes - technicalDiagnosticSize(*diagnostic) - len(key) - technicalHeaderOverhead
		if remaining <= 0 {
			continue
		}
		trimmed, _ := truncateTechnicalString(headers[key], remaining)
		if trimmed == "" && headers[key] != "" {
			continue
		}
		(*destination)[key] = trimmed
		if technicalDiagnosticSize(*diagnostic) > maxBytes {
			delete(*destination, key)
		}
	}
	if len(*destination) == 0 {
		*destination = nil
	}
}

func truncateTechnicalString(value string, maxBytes int) (string, bool) {
	if len(value) <= maxBytes {
		return value, false
	}
	if maxBytes <= 0 {
		return "", true
	}
	cut := maxBytes
	for cut > 0 && (value[cut]&0xc0) == 0x80 {
		cut--
	}
	return value[:cut], true
}

func technicalDiagnosticSize(diagnostic TechnicalDiagnostic) int {
	size := technicalDiagnosticOverhead + len(diagnostic.DiagnosticID) + len(diagnostic.RawError) + len(diagnostic.RequestID) + len(diagnostic.RequestMethod) + len(diagnostic.RequestType) + len(diagnostic.RequestURL) + len(diagnostic.RequestDocumentURL) + len(diagnostic.RequestOriginURL) + len(diagnostic.RequestInitiator) + len(diagnostic.ResponseStatusLine) + len(diagnostic.ResponseIP) + len(diagnostic.NativeHostDetail) + len(diagnostic.PipeDetail)
	for key, value := range diagnostic.RequestHeaders {
		size += technicalHeaderOverhead + len(key) + len(value)
	}
	for key, value := range diagnostic.ResponseHeaders {
		size += technicalHeaderOverhead + len(key) + len(value)
	}
	return size
}

func cloneTechnicalDiagnostic(diagnostic TechnicalDiagnostic) TechnicalDiagnostic {
	diagnostic.RequestHeaders = cloneTechnicalHeaders(diagnostic.RequestHeaders)
	diagnostic.ResponseHeaders = cloneTechnicalHeaders(diagnostic.ResponseHeaders)
	return diagnostic
}

func cloneTechnicalHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	copyHeaders := make(map[string]string, len(headers))
	for key, value := range headers {
		copyHeaders[key] = value
	}
	return copyHeaders
}
