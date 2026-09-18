package capture

import (
	"time"

	"github.com/opendownload/opendownload/internal/diagnostics"
)

// Stream contains request metadata and private request headers retained only in memory.
type Stream struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Host       string    `json:"host"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	CapturedAt time.Time `json:"capturedAt"`
}

// StreamSummary is safe to send to the desktop client.
type StreamSummary struct {
	ID           string           `json:"id"`
	Host         string           `json:"host"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	CapturedAt   time.Time        `json:"capturedAt"`
	ErrorCode    diagnostics.Code `json:"errorCode,omitempty"`
	DiagnosticID string           `json:"diagnosticId,omitempty"`
}

// Pairing contains the short lived pairing code returned directly to the desktop client.
type Pairing struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// SessionSnapshot is safe to publish because it omits the pairing code.
type SessionSnapshot struct {
	Active       bool                    `json:"active"`
	ExpiresAt    time.Time               `json:"expiresAt,omitempty"`
	Paired       bool                    `json:"paired"`
	Mode         Mode                    `json:"mode"`
	NativeStatus NativeConnectionStatus  `json:"nativeStatus"`
	Browser      string                  `json:"browser,omitempty"`
	TabID        int64                   `json:"tabId,omitempty"`
	Diagnostic   *diagnostics.Diagnostic `json:"diagnostic,omitempty"`
}

// Mode identifies how browser capture connects to the app.
type Mode string

const (
	ModeManual    Mode = "manual"
	ModeAutomatic Mode = "automatic"
)

// NativeConnectionStatus describes automatic capture transport state.
type NativeConnectionStatus string

const (
	NativeDisconnected NativeConnectionStatus = "disconnected"
	NativeConnecting   NativeConnectionStatus = "connecting"
	NativeConnected    NativeConnectionStatus = "connected"
)

// CaptureDiagnostic aliases the shared transport-safe diagnostic contract.
type CaptureDiagnostic = diagnostics.Diagnostic

type receivedStream struct {
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers"`
}

type storedStream struct {
	Stream
	headers map[string]string
}

func summarize(stream Stream) StreamSummary {
	return StreamSummary{
		ID:         stream.ID,
		Host:       stream.Host,
		Name:       stream.Name,
		Type:       stream.Type,
		CapturedAt: stream.CapturedAt,
	}
}
