package live

import (
	"net/http"
	"strconv"
)

// Params event params.
type Params map[string]interface{}

// String helper to get a string from the params.
func (p Params) String(key string) string {
	return mapString(p, key)
}

// Checkbox helper to return a boolean from params referring to
// a checkbox input.
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

func mapString(p map[string]interface{}, key string) string {
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

// Int helper to return and int from the params.
func (p Params) Int(key string) int {
	return mapInt(p, key)
}

func mapInt(p map[string]interface{}, key string) int {
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

// Float32 helper to return a float32 from the params.
func (p Params) Float32(key string) float32 {
	return mapFloat32(p, key)
}

func mapFloat32(p map[string]interface{}, key string) float32 {
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

// NewParamsFromRequest helper to generate Params from an http request.
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
