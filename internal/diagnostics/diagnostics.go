package diagnostics

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var localPathPattern = regexp.MustCompile(`(?i)(^|[\s(])(?:[a-z]:/|/users/|/home/)[^\s]+`)

// Stage identifies the subsystem that reported a diagnostic.
type Stage string

const (
	StageNativeConnection Stage = "native_connection"
	StageCapture          Stage = "capture"
	StageDownload         Stage = "download"
)

// Code is a stable, transport-safe diagnostic identifier.
type Code string

const (
	NativeHostNotRegistered Code = "NATIVE_HOST_NOT_REGISTERED"
	NativeHostForbidden     Code = "NATIVE_HOST_FORBIDDEN"
	NativeHostDisconnected  Code = "NATIVE_HOST_DISCONNECTED"
	AppNotRunning           Code = "APP_NOT_RUNNING"
	IPCAccessDenied         Code = "IPC_ACCESS_DENIED"
	IPCProtocolInvalid      Code = "IPC_PROTOCOL_INVALID"
	CaptureSessionReplaced  Code = "CAPTURE_SESSION_REPLACED"
	CaptureSessionStopped   Code = "CAPTURE_SESSION_STOPPED"
	CaptureRequestRejected  Code = "CAPTURE_REQUEST_REJECTED"
	CapturePayloadInvalid   Code = "CAPTURE_PAYLOAD_INVALID"
	CaptureUnsupported      Code = "CAPTURE_UNSUPPORTED"
	DownloadURLInvalid      Code = "DOWNLOAD_URL_INVALID"
	DownloadAuthRequired    Code = "DOWNLOAD_AUTH_REQUIRED"
	DownloadSourceNotFound  Code = "DOWNLOAD_SOURCE_NOT_FOUND"
	DownloadHTTPFailed      Code = "DOWNLOAD_HTTP_FAILED"
	DownloadTransportFailed Code = "DOWNLOAD_TRANSPORT_FAILED"
	DownloadManifestInvalid Code = "DOWNLOAD_MANIFEST_INVALID"
	DownloadSegmentFailed   Code = "DOWNLOAD_SEGMENT_FAILED"
	DownloadWriteFailed     Code = "DOWNLOAD_WRITE_FAILED"
	DownloadCancelled       Code = "DOWNLOAD_CANCELLED"
)

// Diagnostic is safe to send over Wails, native messaging, or local history.
type Diagnostic struct {
	Code         Code              `json:"code"`
	Stage        Stage             `json:"stage"`
	Retryable    bool              `json:"retryable"`
	UserMessage  string            `json:"userMessage"`
	DiagnosticID string            `json:"diagnosticId"`
	OccurredAt   time.Time         `json:"occurredAt"`
	SafeContext  map[string]string `json:"safeContext,omitempty"`
}

// Error carries a stable diagnostic while preserving a user-safe error string.
type Error struct{ Diagnostic Diagnostic }

func (e *Error) Error() string { return e.Diagnostic.UserMessage }

func NewError(diagnostic Diagnostic) error { return &Error{Diagnostic: diagnostic} }

// New creates a diagnostic with a caller-supplied ID.
func New(code Code, stage Stage, retryable bool, userMessage, diagnosticID string, occurredAt time.Time) Diagnostic {
	return Diagnostic{Code: code, Stage: stage, Retryable: retryable, UserMessage: userMessage, DiagnosticID: diagnosticID, OccurredAt: occurredAt}
}

// NewID creates an opaque diagnostic identifier without embedding user data.
func NewID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "diagnostic-unknown"
	}
	return hex.EncodeToString(bytes)
}

// WithTechnicalDetail remains for source compatibility with older callers.
// Sensitive values must be recorded through TechnicalStore, so this method
// intentionally discards its input and returns a safe Diagnostic.
func (d Diagnostic) WithTechnicalDetail(_ string) Diagnostic {
	return d
}

// WithContext keeps only explicitly safe context fields.
func (d Diagnostic) WithContext(values map[string]string) Diagnostic {
	if len(values) == 0 {
		return d
	}
	d.SafeContext = make(map[string]string, len(values))
	for key, value := range values {
		switch key {
		case "browser", "hostname", "httpStatus", "retryCount", "operationId":
			d.SafeContext[key] = Redact(value)
		}
	}
	if len(d.SafeContext) == 0 {
		d.SafeContext = nil
	}
	return d
}

// Redact removes credentials, tokens, queries, and local path details.
func Redact(value string) string {
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.User = nil
		parsed.RawQuery = ""
		parsed.Fragment = ""
		value = parsed.String()
	}
	value = localPathPattern.ReplaceAllString(value, "$1[LOCAL_PATH]")
	for _, marker := range []string{"Authorization:", "Cookie:", "Set-Cookie:", "token=", "secret=", "pairingCode=", "sessionSecret="} {
		if index := strings.Index(strings.ToLower(value), strings.ToLower(marker)); index >= 0 {
			value = value[:index] + marker + "[REDACTED]"
		}
	}
	return value
}
