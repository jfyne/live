package live

import (
	"context"
	"net/http"
	"strconv"
)

// EventHandler a function to handle events, returns the data that should
// be set to the socket after handling.
type EventHandler func(context.Context, *Socket, map[string]interface{}) (interface{}, error)

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
	switch out := v.(type) {
	case int:
		return out
	case string:
		i, err := strconv.Atoi(out)
		if err != nil {
			return 0
		}
		return i
	}
	return 0
}

// ParamFloat32 helper to return a float32 from the params.
func ParamFloat32(params map[string]interface{}, key string) float32 {
	v, ok := params[key]
	if !ok {
		return 0.0
	}
	switch out := v.(type) {
	case float32:
		return out
	case float64:
		return float32(out)
	case string:
		f, err := strconv.ParseFloat(out, 32)
		if err != nil {
			return 0.0
		}
		return float32(f)
	}
	return 0.0
}

// ParamsFromRequest given an *http.Request extract the params.
func ParamsFromRequest(r *http.Request) map[string]interface{} {
	out := map[string]interface{}{}
	values := r.URL.Query()
	for k, v := range values {
		if len(v) == 1 {
			out[k] = v[0]
		} else {
			out[k] = v
		}
	}
	return out
}
