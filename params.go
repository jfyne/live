package live

import (
	"net/http"
	"strconv"
)

// Params represents event parameters as a flexible key-value map.
// Params are typically extracted from client events and contain form data,
// event metadata, or other structured information.
//
// The type provides helper methods for safe type conversion of common types.
type Params map[string]any

// String retrieves a string value from the params.
// Returns an empty string if the key is not found or the value is not a string.
func (p Params) String(key string) string {
	return mapString(p, key)
}

// Checkbox retrieves a boolean value from params for checkbox inputs.
// Returns true if the value is the string "on" (the default for checked checkboxes).
// Returns false if the key is not found or the value is not "on".
func (p Params) Checkbox(key string) bool {
	v, ok := p[key]
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

func mapString(p map[string]any, key string) string {
	v, ok := p[key]
	if !ok {
		return ""
	}
	out, ok := v.(string)
	if !ok {
		return ""
	}
	return out
}

// Int retrieves an integer value from the params.
// Handles conversion from int, string, float32, and float64 types.
// Returns 0 if the key is not found or the value cannot be converted to int.
func (p Params) Int(key string) int {
	return mapInt(p, key)
}

func mapInt(p map[string]any, key string) int {
	v, ok := p[key]
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
	case float32:
		return int(out)
	case float64:
		return int(out)
	}
	return 0
}

// Float32 retrieves a float32 value from the params.
// Handles conversion from float32, float64, and string types.
// Returns 0.0 if the key is not found or the value cannot be converted to float32.
func (p Params) Float32(key string) float32 {
	return mapFloat32(p, key)
}

func mapFloat32(p map[string]any, key string) float32 {
	v, ok := p[key]
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

// NewParamsFromRequest creates a Params map from URL query parameters.
// If a parameter appears multiple times, the value will be a []string slice.
// If a parameter appears once, the value will be a string.
func NewParamsFromRequest(r *http.Request) Params {
	out := Params{}
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
