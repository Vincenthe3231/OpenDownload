package capture

import "time"

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
	ID         string    `json:"id"`
	Host       string    `json:"host"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	CapturedAt time.Time `json:"capturedAt"`
}

// Pairing contains the short lived pairing code returned directly to the desktop client.
type Pairing struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// SessionSnapshot is safe to publish because it omits the pairing code.
type SessionSnapshot struct {
	Active    bool      `json:"active"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	Paired    bool      `json:"paired"`
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

func summarize(stream Stream) StreamSummary {
	return StreamSummary{
		ID:         stream.ID,
		Host:       stream.Host,
		Name:       stream.Name,
		Type:       stream.Type,
		CapturedAt: stream.CapturedAt,
	}
}
