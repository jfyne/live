package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// buttonsMockTransport is a transport implementation for buttons example testing.
// It tracks sent events and provides a channel for injecting inbound events.
type buttonsMockTransport struct {
	events chan live.Event
	sent   []live.Event
	mu     sync.Mutex
	closed bool
}

func newButtonsMockTransport() *buttonsMockTransport {
	return &buttonsMockTransport{
		events: make(chan live.Event, 16),
		sent:   []live.Event{},
	}
}

func (m *buttonsMockTransport) Send(e live.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, e)
	return nil
}

func (m *buttonsMockTransport) Events() <-chan live.Event {
	return m.events
}

func (m *buttonsMockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		close(m.events)
		m.closed = true
	}
	return nil
}

func (m *buttonsMockTransport) GetSent() []live.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]live.Event{}, m.sent...)
}

// ---------------------------------------------------------------------------
// New feature tests (RED — fail to compile until examples/buttons/main.go exists)
// ---------------------------------------------------------------------------

// TestButtonsIsland_NewButtonsIsland verifies that NewButtonsIsland constructs
// a valid non-nil Island without error.
//
// Scenario: NewButtonsIsland construction
func TestButtonsIsland_NewButtonsIsland(t *testing.T) {
	island, err := NewButtonsIsland()
	if err != nil {
		t.Fatalf("NewButtonsIsland() error = %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}
}

// TestButtonsIsland_Mount verifies that mounting the buttons island initializes
// the count to 0 by default when no props are provided.
//
// Scenario: Mount handler initializes count to 0
func TestButtonsIsland_Mount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewButtonsIsland()
	if err != nil {
		t.Fatalf("NewButtonsIsland() failed: %v", err)
	}

	state, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	bs, ok := state.(*ButtonsState)
	if !ok {
		t.Fatalf("expected *ButtonsState from Mount, got %T", state)
	}

	if bs.Count != 0 {
		t.Errorf("expected initial Count = 0, got %d", bs.Count)
	}
}

// TestButtonsIsland_Increment verifies that the "inc" event handler increments
// ButtonsState.Count by 1.
//
// Scenario: Increment event handler (live-click inc button)
func TestButtonsIsland_Increment(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewButtonsIsland()
	if err != nil {
		t.Fatalf("NewButtonsIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("inc")
	if err != nil {
		t.Fatalf("GetEventHandler('inc') error = %v", err)
	}

	state := &ButtonsState{Count: 3}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("inc handler error = %v", err)
	}

	bs, ok := newState.(*ButtonsState)
	if !ok {
		t.Fatalf("expected *ButtonsState from inc handler, got %T", newState)
	}

	if bs.Count != 4 {
		t.Errorf("expected Count = 4 after increment, got %d", bs.Count)
	}
}

// TestButtonsIsland_Decrement verifies that the "dec" event handler decrements
// ButtonsState.Count by 1.
//
// Scenario: Decrement event handler (live-click dec button)
func TestButtonsIsland_Decrement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewButtonsIsland()
	if err != nil {
		t.Fatalf("NewButtonsIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("dec")
	if err != nil {
		t.Fatalf("GetEventHandler('dec') error = %v", err)
	}

	state := &ButtonsState{Count: 10}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("dec handler error = %v", err)
	}

	bs, ok := newState.(*ButtonsState)
	if !ok {
		t.Fatalf("expected *ButtonsState from dec handler, got %T", newState)
	}

	if bs.Count != 9 {
		t.Errorf("expected Count = 9 after decrement, got %d", bs.Count)
	}
}

// TestButtonsIsland_Up verifies that the "up" event handler increments
// ButtonsState.Count by 1. The "up" event is sent by live-window-keyup with
// live-key="ArrowUp".
//
// Scenario: Up event (ArrowUp keyboard shortcut via live-window-keyup)
func TestButtonsIsland_Up(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewButtonsIsland()
	if err != nil {
		t.Fatalf("NewButtonsIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("up")
	if err != nil {
		t.Fatalf("GetEventHandler('up') error = %v", err)
	}

	state := &ButtonsState{Count: 5}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("up handler error = %v", err)
	}

	bs, ok := newState.(*ButtonsState)
	if !ok {
		t.Fatalf("expected *ButtonsState from up handler, got %T", newState)
	}

	if bs.Count != 6 {
		t.Errorf("expected Count = 6 after up (ArrowUp), got %d", bs.Count)
	}
}

// TestButtonsIsland_Down verifies that the "down" event handler decrements
// ButtonsState.Count by 1. The "down" event is sent by live-window-keyup with
// live-key="ArrowDown".
//
// Scenario: Down event (ArrowDown keyboard shortcut via live-window-keyup)
func TestButtonsIsland_Down(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewButtonsIsland()
	if err != nil {
		t.Fatalf("NewButtonsIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("down")
	if err != nil {
		t.Fatalf("GetEventHandler('down') error = %v", err)
	}

	state := &ButtonsState{Count: 7}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("down handler error = %v", err)
	}

	bs, ok := newState.(*ButtonsState)
	if !ok {
		t.Fatalf("expected *ButtonsState from down handler, got %T", newState)
	}

	if bs.Count != 6 {
		t.Errorf("expected Count = 6 after down (ArrowDown), got %d", bs.Count)
	}
}

// TestButtonsIsland_MountAndRender verifies the full mount-and-render path
// through the engine: registering the island, mounting it, confirming that
// at least one patch event containing valid HTML is sent to the transport.
//
// Scenario: Mount and render via engine produces valid HTML
func TestButtonsIsland_MountAndRender(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("buttons", NewButtonsIsland)
	if err != nil {
		t.Fatalf("registry.Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newButtonsMockTransport()
	session := live.NewSession(ctx, "session-buttons", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-buttons", "buttons-1", "buttons", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Verify the mounted state has Count initialized to 0.
	bs, ok := instance.State().(*ButtonsState)
	if !ok {
		t.Fatalf("expected *ButtonsState, got %T", instance.State())
	}
	if bs.Count != 0 {
		t.Errorf("expected initial Count = 0 after mount, got %d", bs.Count)
	}

	// Verify at least one patch event was sent to the transport after mount.
	sent := transport.GetSent()
	if len(sent) == 0 {
		t.Fatal("expected at least one event sent to transport after mount")
	}

	// Find a patch event and confirm its data contains HTML.
	var patchEvent *live.Event
	for i := range sent {
		if sent[i].T == live.EventPatch {
			patchEvent = &sent[i]
			break
		}
	}
	if patchEvent == nil {
		t.Fatal("expected at least one EventPatch event to be sent after mount")
	}

	// The patch data should be non-empty HTML.
	html := string(patchEvent.Data)
	if strings.TrimSpace(html) == "" {
		t.Error("expected non-empty HTML in patch event data")
	}
}
