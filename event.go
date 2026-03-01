package live

import (
	"encoding/json"
)

// EventConfig is a function that configures an Event.
// It is used with functional options pattern for event construction.
type EventConfig func(e *Event) error

// Event type constants define the standard event types used in the live protocol.
const (
	// EventError indicates an error has occurred during event processing.
	// The Data field contains error details.
	EventError = "err"

	// EventPatch contains DOM patches to apply on the client.
	// The Data field contains a JSON-encoded array of Patch objects.
	EventPatch = "patch"

	// EventAck acknowledges that a client event was received and processed.
	// This is sent from server to client after successfully handling an event.
	EventAck = "ack"

	// EventConnect is sent from server to client immediately after
	// the transport connection is established.
	EventConnect = "connect"

	// EventParams communicates URL parameter updates.
	// Can be sent in both directions to synchronize browser state.
	EventParams = "params"

	// EventRedirect instructs the client browser to navigate to a new URL.
	// The Data field contains the redirect destination.
	EventRedirect = "redirect"
)

// Event represents a message exchanged between client and server over a transport.
// Events are the fundamental unit of communication in the live protocol.
//
// Events can represent:
// - Client actions (button clicks, form submissions, etc.)
// - Server responses (patches, errors, redirects)
// - System events (connection, acknowledgment)
type Event struct {
	// T is the event type (e.g., EventPatch, EventError, EventConnect, or custom event names).
	T string `json:"t"`

	// ID is an optional event identifier used for request/response correlation.
	ID int `json:"i,omitempty"`

	// Island identifies which island instance this event targets.
	// Format: the island instance ID (e.g., "counter-1").
	// Empty for session-level events.
	Island string `json:"island,omitempty"`

	// Data contains the event payload as raw JSON.
	// The structure depends on the event type.
	// Use the Params() method to decode client event data.
	Data json.RawMessage `json:"d,omitempty"`

	// SelfData is used for server-originated events targeting island self-handlers.
	// This bypasses JSON encoding and passes data directly to the handler.
	// The "-" JSON tag ensures clients cannot set this field via the wire protocol.
	SelfData any `json:"-"`
}

// Params extracts and decodes parameters from the event's Data field.
// This is typically used for client-originated events that contain form data
// or other structured parameters.
//
// Returns an empty Params map if Data is nil.
// Returns ErrMessageMalformed if the Data cannot be decoded as Params.
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

// ErrorEvent represents an error response sent from server to client.
// It includes the original event that caused the error and the error message.
type ErrorEvent struct {
	// Source is the original event that triggered the error.
	Source Event `json:"source"`

	// Err is the error message describing what went wrong.
	Err string `json:"err"`
}
