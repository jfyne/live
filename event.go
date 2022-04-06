package live

import (
	"encoding/json"
)

// EventConfig configures an event.
type EventConfig func(e *Event) error

const (
	// EventError indicates an error has occured.
	EventError = "err"
	// EventPatch a patch event containing a diff.
	EventPatch = "patch"
	// EventAck sent when an event is ackknowledged.
	EventAck = "ack"
	// EventConnect sent as soon as the server accepts the
	// WS connection.
	EventConnect = "connect"
	// EventParams sent for a URL parameter update. Can be
	// sent both directions.
	EventParams = "params"
	// EventRedirect sent in order to trigger a browser
	// redirect.
	EventRedirect = "redirect"
)

// Event messages that are sent and received by the
// socket.
type Event struct {
	T        string          `json:"t"`
	ID       int             `json:"i,omitempty"`
	Data     json.RawMessage `json:"d,omitempty"`
	SelfData interface{}     `json:"s,omitempty"`
}

// Params extract params from inbound message.
func (e Event) Params() (Params, error) {
	if e.Data == nil {
		return Params{}, nil
	}
	var p Params
	if err := json.Unmarshal(e.Data, &p); err != nil {
		return nil, ErrMessageMalformed
	}
	return p, nil
}

// WithID sets an ID on an event.
func WithID(ID int) EventConfig {
	return func(e *Event) error {
		e.ID = ID
		return nil
	}
}

type ErrorEvent struct {
	Source Event  `json:"source"`
	Err    string `json:"err"`
}
