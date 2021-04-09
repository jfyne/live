package live

import (
	"net/http"
	"net/url"
	"testing"
)

func TestParamString(t *testing.T) {
	p := Params{"test": "output"}
	out := p.String("test")
	if out != "output" {
		t.Error("unexpected output of ParamString", out)
	}

	empty := p.String("nokey")
	if empty != "" {
		t.Error("unexpected output of ParamString", empty)
	}
}

func TestParamCheckbox(t *testing.T) {
	p := Params{"test": "on"}
	state := p.Checkbox("test")
	if state != true {
		t.Error("unexpected output of ParamCheckbox", state)
	}
	p["test"] = "noton"
	state = p.Checkbox("test")
	if state != false {
		t.Error("unexpected output of ParamCheckbox", state)
	}
	state = p.Checkbox("nottest")
	if state != false {
		t.Error("unexpected output of ParamCheckbox", state)
	}
}

func TestParamInt(t *testing.T) {
	var out int

	p := Params{"test": 1}
	out = p.Int("test")
	if out != 1 {
		t.Error("unexpected output of ParamInt", out)
	}
	p["test"] = "1"
	out = p.Int("test")
	if out != 1 {
		t.Error("unexpected output of ParamInt", out)
	}
	p["test"] = "aaa"
	out = p.Int("test")
	if out != 0 {
		t.Error("unexpected output of ParamInt", out)
	}
	p["test"] = 1
	out = p.Int("nottest")
	if out != 0 {
		t.Error("unexpected output of ParamInt", out)
	}
}

func TestParamFloat32(t *testing.T) {
	var out float32

	p := Params{"test": 1.0}
	out = p.Float32("test")
	if out != 1.0 {
		t.Error("unexpected output of ParamFloat32", out)
	}
	p["test"] = "1.0"
	out = p.Float32("test")
	if out != 1.0 {
		t.Error("unexpected output of ParamFloat32", out)
	}
	p["test"] = "aaa"
	out = p.Float32("test")
	if out != 0.0 {
		t.Error("unexpected output of ParamFloat32", out)
	}
	p["test"] = 1.0
	out = p.Float32("nottest")
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
	params := NewParamsFromRequest(r)
	var out int
	out = params.Int("one")
	if out != 1 {
		t.Error("did not get expected params", params)
	}
	out = params.Int("two")
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
