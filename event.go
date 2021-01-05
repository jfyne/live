package live

// EventHandler a function to handle events, returns the data that should
// be set to the socket after handling.
type EventHandler func(*Socket, map[string]interface{}) (interface{}, error)

const (
	// EventError indicates an error has occured.
	EventError = "err"
	// EventPatch a patch event containing a diff.
	EventPatch = "patch"
	// EventAck sent when an event is ackknowledged.
	EventAck = "ack"
	// EventHello sent as soon as the server accepts the
	// WS connection.
	EventHello = "hello"
)

// Event messages that are sent and received by the
// socket.
type Event struct {
	T    string      `json:"t"`
	ID   int         `json:"i,omitempty"`
	Data interface{} `json:"d,omitempty"`
}

// Params extract params from inbound message.
func (e Event) Params() (map[string]interface{}, error) {
	if e.Data == nil {
		return map[string]interface{}{}, nil
	}
	p, ok := e.Data.(map[string]interface{})
	if !ok {
		return nil, ErrMessageMalformed
	}
	return p, nil
}

// ParamString helper to return a string from the params.
func ParamString(params map[string]interface{}, key string) string {
	v, ok := params[key]
	if !ok {
		return ""
	}
	out, ok := v.(string)
	if !ok {
		return ""
	}
	return out
}

// ParamCheckbox helper to return a boolean from params referring to
// a checkbox input.
func ParamCheckbox(params map[string]interface{}, name string) bool {
	v, ok := params[name]
	if !ok {
		return false
	}
	out, ok := v.(string)
	if !ok {
		return false
	}
	if out == "on" {
		return true
	}
	return false
}

// ParamInt helper to return an int from the params.
func ParamInt(params map[string]interface{}, key string) int {
	v, ok := params[key]
	if !ok {
		return 0
	}
	out, ok := v.(int)
	if !ok {
		return 0
	}
	return out
}

// ParamFloat32 helper to return a float32 from the params.
func ParamFloat32(params map[string]interface{}, key string) float32 {
	v, ok := params[key]
	if !ok {
		return 0.0
	}
	out, ok := v.(float32)
	if !ok {
		return 0.0
	}
	return out
}
