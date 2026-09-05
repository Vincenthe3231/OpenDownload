package capture

// EventType describes a capture notification sent to local subscribers.
type EventType string

const (
	EventSessionChanged EventType = "capture.session_changed"
	EventStreamAdded    EventType = "capture.stream_added"
)

// Event carries safe capture metadata only.
type Event struct {
	Type    EventType        `json:"type"`
	Session *SessionSnapshot `json:"session,omitempty"`
	Stream  *StreamSummary   `json:"stream,omitempty"`
}

// Listener receives a capture event after the manager has updated its state.
type Listener func(Event)
