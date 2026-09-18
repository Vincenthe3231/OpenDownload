package nativeprotocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const MaxPayloadBytes = 1 << 20

// Stream is the redacted request metadata forwarded by the native host.
type Stream struct {
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers,omitempty"`
}

// Message is shared by browser native messaging and the app pipe.
type Message struct {
	Type          string  `json:"type"`
	Browser       string  `json:"browser,omitempty"`
	TabID         int64   `json:"tabId,omitempty"`
	SessionID     string  `json:"sessionId,omitempty"`
	SessionSecret string  `json:"sessionSecret,omitempty"`
	Stream        *Stream `json:"stream,omitempty"`
	ErrorCode     string  `json:"errorCode,omitempty"`
	DiagnosticID  string  `json:"diagnosticId,omitempty"`
	Error         string  `json:"error,omitempty"`
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
