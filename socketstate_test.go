package live

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMemorySocketStateStore(t *testing.T) {
	ctx := context.Background()
	m := &MemorySocketStateStore{
		janitorFrequency: 50 * time.Millisecond,
		gets:             make(chan mssGetop),
		sets:             make(chan mssSetop),
		dels:             make(chan mssDelop),
		clean:            make(chan bool),
	}
	go m.operate(ctx)
	go m.janitor(ctx)

	ID := SocketID("a")
	state := SocketState{
		Render: []byte("test"),
	}

	if err := m.Set(ID, state, 100*time.Millisecond); err != nil {
		t.Error(err)
	}
	s, err := m.Get(ID)
	if err != nil {
		t.Error(fmt.Errorf("initial get: %w", err))
	}
	if string(s.Render) != string(state.Render) {
		t.Error("state doesnt match")
	}
	time.Sleep(150 * time.Millisecond)
	_, err = m.Get(ID)
	if err == nil {
		t.Error(fmt.Errorf("state should be clear: %w", err))
	}
}
