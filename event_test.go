package live

import (
	"net/http"
	"net/url"
	"testing"
)

func TestEventParams(t *testing.T) {
	e := Event{}
	p, err := e.Params()
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	if len(p) != 0 {
		t.Fatal("expected zero length map, got", p)
	}

	e.Data = "wrong"
	p, err = e.Params()
	if err != ErrMessageMalformed {
		t.Error("expected ErrMessageMalformed, got", err)
	}
}

func TestParamString(t *testing.T) {
	p := map[string]interface{}{"test": "output"}
	out := ParamString(p, "test")
	if out != "output" {
		t.Error("unexpected output of ParamString", out)
	}

	empty := ParamString(p, "nokey")
	if empty != "" {
		t.Error("unexpected output of ParamString", empty)
	}
}

func TestParamCheckbox(t *testing.T) {
	p := map[string]interface{}{"test": "on"}
	state := ParamCheckbox(p, "test")
	if state != true {
		t.Error("unexpected output of ParamCheckbox", state)
	}
	p["test"] = "noton"
	state = ParamCheckbox(p, "test")
	if state != false {
		t.Error("unexpected output of ParamCheckbox", state)
	}
	state = ParamCheckbox(p, "nottest")
	if state != false {
		t.Error("unexpected output of ParamCheckbox", state)
	}
}

func TestParamInt(t *testing.T) {
	var out int

	out = ParamInt(map[string]interface{}{"test": 1}, "test")
	if out != 1 {
		t.Error("unexpected output of ParamInt", out)
	}
	out = ParamInt(map[string]interface{}{"test": "1"}, "test")
	if out != 1 {
		t.Error("unexpected output of ParamInt", out)
	}
	out = ParamInt(map[string]interface{}{"test": "aaa"}, "test")
	if out != 0 {
		t.Error("unexpected output of ParamInt", out)
	}
	out = ParamInt(map[string]interface{}{"test": 1}, "nottest")
	if out != 0 {
		t.Error("unexpected output of ParamInt", out)
	}
}

func TestParamFloat32(t *testing.T) {
	var out float32

	out = ParamFloat32(map[string]interface{}{"test": 1.0}, "test")
	if out != 1.0 {
		t.Error("unexpected output of ParamFloat32", out)
	}
	out = ParamFloat32(map[string]interface{}{"test": "1.0"}, "test")
	if out != 1.0 {
		t.Error("unexpected output of ParamFloat32", out)
	}
	out = ParamFloat32(map[string]interface{}{"test": "aaa"}, "test")
	if out != 0.0 {
		t.Error("unexpected output of ParamFloat32", out)
	}
	out = ParamFloat32(map[string]interface{}{"test": 1.0}, "nottest")
	if out != 0.0 {
		t.Error("unexpected output of ParamFloat32", out)
	}
}

func TestParamsFromRequest(t *testing.T) {
	var err error
	r := &http.Request{}
	r.URL, err = url.Parse("http://example.com?one=1&two=2&three=3&three=4")
	if err != nil {
		t.Fatal(err)
	}
	params := ParamsFromRequest(r)
	var out int
	out = ParamInt(params, "one")
	if out != 1 {
		t.Error("did not get expected params", params)
	}
	out = ParamInt(params, "two")
	if out != 2 {
		t.Error("did not get expected params", params)
	}

	sliceout, ok := params["three"].([]string)
	if !ok {
		t.Error("did not get expected params", params)
	}
	if len(sliceout) != 2 {
		t.Error("did not get expected params", params)
	}
}
