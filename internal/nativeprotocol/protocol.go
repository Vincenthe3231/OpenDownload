package nativeprotocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const MaxPayloadBytes = 1 << 20

// ProtocolVersion is the version shared by the Gecko extension, native host,
// and desktop named-pipe transport. A message is intentionally rejected by a
// receiver when it carries another version so old extensions cannot silently
// exchange an unsafe payload with a newer desktop app.
const ProtocolVersion = 1

// Stream is the redacted request metadata forwarded by the native host.
type Stream struct {
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers,omitempty"`
}

// DebugContext contains browser-observable request metadata. It is sent only
// from the extension to the desktop app when desktop developer diagnostics
// explicitly enabled it for the active capture session. Nothing in this type
// may be sent back to the extension or included in a public capture snapshot.
type DebugContext struct {
	RequestID            string            `json:"requestId,omitempty"`
	RequestURL           string            `json:"requestUrl,omitempty"`
	RequestMethod        string            `json:"requestMethod,omitempty"`
	RequestType          string            `json:"requestType,omitempty"`
	RequestTimestamp     float64           `json:"requestTimestamp,omitempty"`
	RequestFrameID       int64             `json:"requestFrameId,omitempty"`
	RequestParentFrameID int64             `json:"requestParentFrameId,omitempty"`
	RequestDocumentURL   string            `json:"requestDocumentUrl,omitempty"`
	RequestOriginURL     string            `json:"requestOriginUrl,omitempty"`
	RequestInitiator     string            `json:"requestInitiator,omitempty"`
	RequestHeaders       map[string]string `json:"requestHeaders,omitempty"`
	ResponseHeaders      map[string]string `json:"responseHeaders,omitempty"`
	ResponseStatus       int               `json:"responseStatus,omitempty"`
	ResponseStatusLine   string            `json:"responseStatusLine,omitempty"`
	ResponseFromCache    bool              `json:"responseFromCache,omitempty"`
	ResponseIP           string            `json:"responseIp,omitempty"`
}

// Failure is the complete error payload permitted to cross into the browser
// extension. It deliberately has no technical-detail field.
type Failure struct {
	Code         string `json:"code"`
	UserMessage  string `json:"userMessage"`
	Retryable    bool   `json:"retryable"`
	DiagnosticID string `json:"diagnosticId"`
}

// Message is shared by browser native messaging and the app pipe.
type Message struct {
	ProtocolVersion int           `json:"protocolVersion"`
	Type            string        `json:"type"`
	Browser         string        `json:"browser,omitempty"`
	TabID           int64         `json:"tabId,omitempty"`
	SessionID       string        `json:"sessionId,omitempty"`
	SessionSecret   string        `json:"sessionSecret,omitempty"`
	Stream          *Stream       `json:"stream,omitempty"`
	Debug           *DebugContext `json:"debug,omitempty"`
	Failure         *Failure      `json:"failure,omitempty"`
	DiagnosticMode  bool          `json:"diagnosticMode,omitempty"`
}

// ValidateProtocol verifies the message version after framing has completed.
// Callers use this rather than failing inside Read so they can return a safe
// structured failure to a peer that sent a valid frame with the wrong version.
func (m Message) ValidateProtocol() error {
	if m.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported native protocol version: %d", m.ProtocolVersion)
	}
	return nil
}

// Read reads one browser-native or pipe-framed JSON message.
func Read(r io.Reader, message *Message) error {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return err
	}
	if length == 0 || length > MaxPayloadBytes {
		return fmt.Errorf("native message payload exceeds limit")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return err
	}
	if err := json.Unmarshal(payload, message); err != nil {
		return fmt.Errorf("invalid native message JSON: %w", err)
	}
	return nil
}

// Write writes one browser-native or pipe-framed JSON message.
func Write(w io.Writer, message Message) error {
	if message.ProtocolVersion == 0 {
		message.ProtocolVersion = ProtocolVersion
	}
	if err := message.ValidateProtocol(); err != nil {
		return err
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(payload) == 0 || len(payload) > MaxPayloadBytes {
		return fmt.Errorf("native message payload exceeds limit")
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(payload))); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}
