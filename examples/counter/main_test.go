package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// counterMockTransport is a transport implementation for counter example testing.
// It tracks sent events and provides a channel for injecting inbound events.
type counterMockTransport struct {
	events chan live.Event
	sent   []live.Event
	mu     sync.Mutex
	closed bool
}

func newCounterMockTransport() *counterMockTransport {
	return &counterMockTransport{
		events: make(chan live.Event, 16),
		sent:   []live.Event{},
	}
}

func (m *counterMockTransport) Send(e live.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, e)
	return nil
}

func (m *counterMockTransport) Events() <-chan live.Event {
	return m.events
}

func (m *counterMockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		close(m.events)
		m.closed = true
	}
	return nil
}

func (m *counterMockTransport) GetSent() []live.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]live.Event{}, m.sent...)
}

// ---------------------------------------------------------------------------
// Regression tests (must PASS with existing counter/main.go code)
// ---------------------------------------------------------------------------

// TestCounterIsland_NewCounterIsland verifies that NewCounterIsland constructs
// a valid Island without error.
//
// Scenario: CounterState registers correctly
func TestCounterIsland_NewCounterIsland(t *testing.T) {
	island, err := NewCounterIsland()
	if err != nil {
		t.Fatalf("NewCounterIsland() error = %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}
}

// TestCounterIsland_MountWithInitial verifies that mounting the counter island
// with props.Int("initial") sets the CounterState.Count to the provided value.
//
// Scenario: MountWithInitial (props.Int for initial count)
func TestCounterIsland_MountWithInitial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewCounterIsland()
	if err != nil {
		t.Fatalf("NewCounterIsland() failed: %v", err)
	}

	// Call the mount handler directly with an initial count of 7.
	state, err := island.Mount(ctx, live.Props{"initial": 7}, "")
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	cs, ok := state.(*CounterState)
	if !ok {
		t.Fatalf("expected *CounterState from Mount, got %T", state)
	}

	if cs.Count != 7 {
		t.Errorf("expected initial Count = 7, got %d", cs.Count)
	}
}

// TestCounterIsland_MountDefault verifies that mounting with no "initial" prop
// defaults to Count = 0.
//
// Scenario: MountWithInitial (props.Int for initial count)
func TestCounterIsland_MountDefault(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewCounterIsland()
	if err != nil {
		t.Fatalf("NewCounterIsland() failed: %v", err)
	}

	state, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	cs, ok := state.(*CounterState)
	if !ok {
		t.Fatalf("expected *CounterState, got %T", state)
	}

	if cs.Count != 0 {
		t.Errorf("expected Count = 0, got %d", cs.Count)
	}
}

// TestCounterIsland_Increment verifies that the "inc" event handler increments
// CounterState.Count by 1 each time it is invoked.
//
// Scenario: Increment event handler
func TestCounterIsland_Increment(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewCounterIsland()
	if err != nil {
		t.Fatalf("NewCounterIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("inc")
	if err != nil {
		t.Fatalf("GetEventHandler('inc') error = %v", err)
	}

	state := &CounterState{Count: 3}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("inc handler error = %v", err)
	}

	cs, ok := newState.(*CounterState)
	if !ok {
		t.Fatalf("expected *CounterState from inc handler, got %T", newState)
	}

	if cs.Count != 4 {
		t.Errorf("expected Count = 4 after increment, got %d", cs.Count)
	}
}

// TestCounterIsland_IncrementMultiple verifies that calling "inc" multiple times
// accumulates the count correctly.
//
// Scenario: Increment event handler
func TestCounterIsland_IncrementMultiple(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewCounterIsland()
	if err != nil {
		t.Fatalf("NewCounterIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("inc")
	if err != nil {
		t.Fatalf("GetEventHandler('inc') error = %v", err)
	}

	var current any = &CounterState{Count: 0}

	for i := 0; i < 5; i++ {
		newState, err := handler(ctx, current, live.Params{})
		if err != nil {
			t.Fatalf("inc handler error on iteration %d: %v", i, err)
		}
		current = newState
	}

	cs, ok := current.(*CounterState)
	if !ok {
		t.Fatalf("expected *CounterState, got %T", current)
	}

	if cs.Count != 5 {
		t.Errorf("expected Count = 5 after 5 increments, got %d", cs.Count)
	}
}

// TestCounterIsland_Decrement verifies that the "dec" event handler decrements
// CounterState.Count by 1.
//
// Scenario: Decrement event handler
func TestCounterIsland_Decrement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewCounterIsland()
	if err != nil {
		t.Fatalf("NewCounterIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("dec")
	if err != nil {
		t.Fatalf("GetEventHandler('dec') error = %v", err)
	}

	state := &CounterState{Count: 10}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("dec handler error = %v", err)
	}

	cs, ok := newState.(*CounterState)
	if !ok {
		t.Fatalf("expected *CounterState from dec handler, got %T", newState)
	}

	if cs.Count != 9 {
		t.Errorf("expected Count = 9 after decrement, got %d", cs.Count)
	}
}

// TestCounterIsland_DecrementBelowZero verifies that decrement works correctly
// even when the count goes negative.
//
// Scenario: Decrement event handler
func TestCounterIsland_DecrementBelowZero(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewCounterIsland()
	if err != nil {
		t.Fatalf("NewCounterIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("dec")
	if err != nil {
		t.Fatalf("GetEventHandler('dec') error = %v", err)
	}

	state := &CounterState{Count: 0}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("dec handler error = %v", err)
	}

	cs, ok := newState.(*CounterState)
	if !ok {
		t.Fatalf("expected *CounterState from dec handler, got %T", newState)
	}

	if cs.Count != -1 {
		t.Errorf("expected Count = -1 after decrement from 0, got %d", cs.Count)
	}
}

// TestCounterIsland_MountAndRenderViaEngine verifies the full mount-and-render
// path through the engine: registering the island, mounting it, and confirming
// that at least one patch event is sent to the transport.
//
// Scenario: MountAndRender via engine
func TestCounterIsland_MountAndRenderViaEngine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("counter", NewCounterIsland)
	if err != nil {
		t.Fatalf("registry.Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newCounterMockTransport()
	session := live.NewSession(ctx, "session-counter", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-counter", "counter-1", "counter", live.Props{"initial": 5})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Verify the mounted state has the correct initial count.
	cs, ok := instance.State().(*CounterState)
	if !ok {
		t.Fatalf("expected *CounterState, got %T", instance.State())
	}
	if cs.Count != 5 {
		t.Errorf("expected initial Count = 5, got %d", cs.Count)
	}

	// Verify at least one patch event was sent to the transport after mount.
	sent := transport.GetSent()
	if len(sent) == 0 {
		t.Fatal("expected at least one event sent to transport after mount")
	}

	hasPatch := false
	for _, e := range sent {
		if e.T == live.EventPatch {
			hasPatch = true
			break
		}
	}
	if !hasPatch {
		t.Error("expected at least one EventPatch event to be sent after mount")
	}
}

// TestCounterIsland_IncrementViaEngine verifies that routing an "inc" event
// through the engine updates the counter's state and triggers a patch event.
//
// Scenario: Increment event handler
func TestCounterIsland_IncrementViaEngine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("counter", NewCounterIsland)
	if err != nil {
		t.Fatalf("registry.Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newCounterMockTransport()
	session := live.NewSession(ctx, "session-counter", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-counter", "counter-1", "counter", live.Props{"initial": 0})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Clear events from mount.
	transport.mu.Lock()
	transport.sent = []live.Event{}
	transport.mu.Unlock()

	// Route an increment event.
	err = engine.RouteEvent("session-counter", live.Event{
		T:      "inc",
		Island: "counter-1",
		Data:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("RouteEvent('inc') failed: %v", err)
	}

	cs, ok := instance.State().(*CounterState)
	if !ok {
		t.Fatalf("expected *CounterState, got %T", instance.State())
	}
	if cs.Count != 1 {
		t.Errorf("expected Count = 1 after increment via engine, got %d", cs.Count)
	}

	// Verify a patch event was sent.
	sent := transport.GetSent()
	hasPatch := false
	for _, e := range sent {
		if e.T == live.EventPatch {
			hasPatch = true
			break
		}
	}
	if !hasPatch {
		t.Error("expected an EventPatch event after increment via engine")
	}
}

// TestCounterIsland_DecrementViaEngine verifies that routing a "dec" event
// through the engine decrements the counter's state.
//
// Scenario: Decrement event handler
func TestCounterIsland_DecrementViaEngine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("counter", NewCounterIsland)
	if err != nil {
		t.Fatalf("registry.Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newCounterMockTransport()
	session := live.NewSession(ctx, "session-counter", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-counter", "counter-1", "counter", live.Props{"initial": 10})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Clear events from mount.
	transport.mu.Lock()
	transport.sent = []live.Event{}
	transport.mu.Unlock()

	// Route a decrement event.
	err = engine.RouteEvent("session-counter", live.Event{
		T:      "dec",
		Island: "counter-1",
		Data:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("RouteEvent('dec') failed: %v", err)
	}

	cs, ok := instance.State().(*CounterState)
	if !ok {
		t.Fatalf("expected *CounterState, got %T", instance.State())
	}
	if cs.Count != 9 {
		t.Errorf("expected Count = 9 after decrement via engine, got %d", cs.Count)
	}
}
