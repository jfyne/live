package live

import (
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

	e.Data = []byte("wrong")
	p, err = e.Params()
	if err != ErrMessageMalformed {
		t.Error("expected ErrMessageMalformed, got", err)
	}
}
