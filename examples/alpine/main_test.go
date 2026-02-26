package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// mockTransport is a transport implementation for alpine example testing
// that tracks sent events and provides a channel for injecting inbound events.
type mockTransport struct {
	events chan live.Event
	sent   []live.Event
	mu     sync.Mutex
	closed bool
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		events: make(chan live.Event, 16),
		sent:   []live.Event{},
	}
}

func (m *mockTransport) Send(e live.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, e)
	return nil
}

func (m *mockTransport) Events() <-chan live.Event {
	return m.events
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		close(m.events)
		m.closed = true
	}
	return nil
}

func (m *mockTransport) GetSent() []live.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]live.Event{}, m.sent...)
}

// ---------------------------------------------------------------------------
// Regression tests (must PASS with existing framework before alpine is created)
// ---------------------------------------------------------------------------

// TestFramework_NewIsland verifies that the live package NewIsland constructor
// works correctly — a basic regression of core framework behaviour.
func TestFramework_NewIsland(t *testing.T) {
	island, err := live.NewIsland("test-island",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return map[string]any{"value": 42}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewIsland failed: %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}
}

// TestFramework_EventHandlerRegistration verifies that event handlers can be
// registered and retrieved — a basic regression of the island event handler API.
func TestFramework_EventHandlerRegistration(t *testing.T) {
	island, err := live.NewIsland("event-test",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return map[string]any{}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewIsland failed: %v", err)
	}

	err = island.HandleEvent("myevent", func(ctx context.Context, state any, params live.Params) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}

	handler, err := island.GetEventHandler("myevent")
	if err != nil {
		t.Fatalf("GetEventHandler failed: %v", err)
	}
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestFramework_MountIsland verifies that the engine mounts an island, calls
// the mount handler, and returns a valid instance — regression for the engine
// mount path.
func TestFramework_MountIsland(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("basic", func() (*live.Island, error) {
		return live.NewIsland("basic",
			live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
				return map[string]any{"ok": true}, nil
			}),
		)
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newMockTransport()
	session := live.NewSession(ctx, "session-reg", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-reg", "basic-1", "basic", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}
	state := instance.State().(map[string]any)
	if state["ok"] != true {
		t.Errorf("expected state ok=true, got %v", state["ok"])
	}
}

// ---------------------------------------------------------------------------
// New feature tests (RED — fail to compile until alpine/main.go is created)
// ---------------------------------------------------------------------------

// TestAlpineIsland_Suggest verifies that calling the "suggest" event handler
// with search params {"search": "go"} returns Suggestions containing at least
// one item whose Name contains "Go".
//
// Scenario: Typeahead suggests matching items
func TestAlpineIsland_Suggest(t *testing.T) {
	island, err := NewAlpineIsland()
	if err != nil {
		t.Fatalf("NewAlpineIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("suggest")
	if err != nil {
		t.Fatalf("expected 'suggest' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()
	initialState := &AlpineState{}
	newState, err := handler(ctx, initialState, live.Params{"search": "go"})
	if err != nil {
		t.Fatalf("'suggest' handler returned error: %v", err)
	}

	alpineState, ok := newState.(*AlpineState)
	if !ok {
		t.Fatalf("expected *AlpineState, got %T", newState)
	}

	if len(alpineState.Suggestions) == 0 {
		t.Fatal("expected at least one suggestion for search 'go', got none")
	}

	found := false
	for _, item := range alpineState.Suggestions {
		if item.Name == "Go" || item.ID == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Suggestions to contain an item matching 'Go', got: %+v", alpineState.Suggestions)
	}
}

// TestAlpineIsland_SuggestNoMatch verifies that calling the "suggest" event
// handler with a search that matches no items returns an empty Suggestions list.
//
// Scenario: Typeahead shows no suggestions for unmatched input
func TestAlpineIsland_SuggestNoMatch(t *testing.T) {
	island, err := NewAlpineIsland()
	if err != nil {
		t.Fatalf("NewAlpineIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("suggest")
	if err != nil {
		t.Fatalf("expected 'suggest' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()
	initialState := &AlpineState{}
	newState, err := handler(ctx, initialState, live.Params{"search": "xyz"})
	if err != nil {
		t.Fatalf("'suggest' handler returned error: %v", err)
	}

	alpineState, ok := newState.(*AlpineState)
	if !ok {
		t.Fatalf("expected *AlpineState, got %T", newState)
	}

	if len(alpineState.Suggestions) != 0 {
		t.Errorf("expected empty Suggestions for search 'xyz', got: %+v", alpineState.Suggestions)
	}
}

// TestAlpineIsland_Selected verifies that calling the "selected" event handler
// with params {"id": "go"} adds the Go item to the Selected list.
//
// Scenario: User selects an item from suggestions
func TestAlpineIsland_Selected(t *testing.T) {
	island, err := NewAlpineIsland()
	if err != nil {
		t.Fatalf("NewAlpineIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("selected")
	if err != nil {
		t.Fatalf("expected 'selected' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()
	// Build initial state by mounting the island first to get the items list
	mountedState, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("island.Mount() failed: %v", err)
	}
	initialState, ok := mountedState.(*AlpineState)
	if !ok {
		t.Fatalf("expected *AlpineState from Mount, got %T", mountedState)
	}

	newState, err := handler(ctx, initialState, live.Params{"id": "go"})
	if err != nil {
		t.Fatalf("'selected' handler returned error: %v", err)
	}

	alpineState, ok := newState.(*AlpineState)
	if !ok {
		t.Fatalf("expected *AlpineState, got %T", newState)
	}

	if len(alpineState.Selected) == 0 {
		t.Fatal("expected Selected to contain at least one item after selecting 'go', got none")
	}

	found := false
	for _, item := range alpineState.Selected {
		if item.ID == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Selected to contain item with ID 'go', got: %+v", alpineState.Selected)
	}
}

// TestAlpineIsland_SelectedNoDuplicate verifies that selecting the same item
// twice results in it appearing only once in the Selected list.
//
// Scenario: Selecting same item twice does not duplicate it
func TestAlpineIsland_SelectedNoDuplicate(t *testing.T) {
	island, err := NewAlpineIsland()
	if err != nil {
		t.Fatalf("NewAlpineIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("selected")
	if err != nil {
		t.Fatalf("expected 'selected' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()
	// Build initial state by mounting the island first to get the items list
	mountedState, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("island.Mount() failed: %v", err)
	}
	initialState, ok := mountedState.(*AlpineState)
	if !ok {
		t.Fatalf("expected *AlpineState from Mount, got %T", mountedState)
	}

	// Select "go" the first time
	afterFirst, err := handler(ctx, initialState, live.Params{"id": "go"})
	if err != nil {
		t.Fatalf("first 'selected' call returned error: %v", err)
	}

	// Select "go" a second time
	afterSecond, err := handler(ctx, afterFirst, live.Params{"id": "go"})
	if err != nil {
		t.Fatalf("second 'selected' call returned error: %v", err)
	}

	alpineState, ok := afterSecond.(*AlpineState)
	if !ok {
		t.Fatalf("expected *AlpineState, got %T", afterSecond)
	}

	count := 0
	for _, item := range alpineState.Selected {
		if item.ID == "go" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected item 'go' to appear exactly once in Selected, but found %d times: %+v",
			count, alpineState.Selected)
	}
}
