package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// mockTransport is a transport implementation for forms example testing
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
// Regression tests (must PASS with existing framework before forms is created)
// ---------------------------------------------------------------------------

// TestFramework_NewIsland_Forms verifies that the live package NewIsland
// constructor works correctly — a basic regression of core framework behaviour.
func TestFramework_NewIsland_Forms(t *testing.T) {
	island, err := live.NewIsland("test-island",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return map[string]any{"value": 1}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewIsland failed: %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}
}

// TestFramework_MountIsland_Forms verifies that the engine mounts an island
// and calls the mount handler — a regression of the engine mount path.
func TestFramework_MountIsland_Forms(t *testing.T) {
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
	defer transport.Close()
	session := live.NewSession(ctx, "session-forms-reg", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-forms-reg", "basic-1", "basic", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}
	state := instance.State().(map[string]any)
	if state["ok"] != true {
		t.Errorf("expected state ok=true, got %v", state["ok"])
	}
}

// ---------------------------------------------------------------------------
// New feature tests (RED — fail to compile until forms/main.go is created)
// ---------------------------------------------------------------------------

// TestTodoIsland_Validate verifies that calling the "validate" event handler
// with an empty task param sets Errors["message"] to a non-empty error string.
//
// Scenario: Validate task input on change
func TestTodoIsland_Validate(t *testing.T) {
	island, err := NewTodoIsland()
	if err != nil {
		t.Fatalf("NewTodoIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("validate")
	if err != nil {
		t.Fatalf("expected 'validate' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()

	// Initial state from mount
	initialState, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Call validate with no "task" key (empty params)
	newState, err := handler(ctx, initialState, live.Params{})
	if err != nil {
		t.Fatalf("validate handler returned unexpected error: %v", err)
	}

	todoState, ok := newState.(*TodoState)
	if !ok {
		t.Fatalf("expected *TodoState, got %T", newState)
	}

	if todoState.Errors == nil {
		t.Fatal("expected Errors map to be non-nil after validate with empty task")
	}

	errMsg, exists := todoState.Errors["message"]
	if !exists {
		t.Fatal("expected Errors[\"message\"] to be set after validate with empty task")
	}
	if errMsg == "" {
		t.Error("expected Errors[\"message\"] to be a non-empty error string")
	}
}

// TestTodoIsland_Save verifies that calling the "save" event handler with a
// valid task name appends the task to the list and increments NextID from 1 to 2.
//
// Scenario: Add a task via form submission
func TestTodoIsland_Save(t *testing.T) {
	island, err := NewTodoIsland()
	if err != nil {
		t.Fatalf("NewTodoIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("save")
	if err != nil {
		t.Fatalf("expected 'save' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()

	// Initial state from mount
	initialState, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	todoState, ok := initialState.(*TodoState)
	if !ok {
		t.Fatalf("expected *TodoState from mount, got %T", initialState)
	}
	if todoState.NextID != 1 {
		t.Errorf("expected initial NextID == 1, got %d", todoState.NextID)
	}

	// Call save with a valid task name
	newState, err := handler(ctx, initialState, live.Params{"task": "Buy groceries"})
	if err != nil {
		t.Fatalf("save handler returned unexpected error: %v", err)
	}

	updatedState, ok := newState.(*TodoState)
	if !ok {
		t.Fatalf("expected *TodoState from save handler, got %T", newState)
	}

	if len(updatedState.Tasks) != 1 {
		t.Fatalf("expected 1 task after save, got %d", len(updatedState.Tasks))
	}

	task := updatedState.Tasks[0]
	if task.Name != "Buy groceries" {
		t.Errorf("expected task Name == 'Buy groceries', got %q", task.Name)
	}

	if updatedState.NextID != 2 {
		t.Errorf("expected NextID == 2 after save, got %d", updatedState.NextID)
	}
}

// TestTodoIsland_Done verifies that calling the "done" event handler with the
// ID of a saved task toggles the task's Complete field to true.
//
// Scenario: Toggle task completion
func TestTodoIsland_Done(t *testing.T) {
	island, err := NewTodoIsland()
	if err != nil {
		t.Fatalf("NewTodoIsland() failed: %v", err)
	}

	saveHandler, err := island.GetEventHandler("save")
	if err != nil {
		t.Fatalf("expected 'save' event handler to be registered, got error: %v", err)
	}

	doneHandler, err := island.GetEventHandler("done")
	if err != nil {
		t.Fatalf("expected 'done' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()

	// Mount to get initial state
	initialState, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Save a task first
	afterSave, err := saveHandler(ctx, initialState, live.Params{"task": "Buy groceries"})
	if err != nil {
		t.Fatalf("save handler returned unexpected error: %v", err)
	}

	savedState, ok := afterSave.(*TodoState)
	if !ok {
		t.Fatalf("expected *TodoState from save handler, got %T", afterSave)
	}
	if len(savedState.Tasks) != 1 {
		t.Fatalf("expected 1 task after save, got %d", len(savedState.Tasks))
	}

	taskID := savedState.Tasks[0].ID
	if taskID == "" {
		t.Fatal("expected task ID to be non-empty after save")
	}

	// Verify task starts as not complete
	if savedState.Tasks[0].Complete {
		t.Fatal("expected task Complete == false before done handler")
	}

	// Call done with the task's ID
	afterDone, err := doneHandler(ctx, afterSave, live.Params{"id": taskID})
	if err != nil {
		t.Fatalf("done handler returned unexpected error: %v", err)
	}

	doneState, ok := afterDone.(*TodoState)
	if !ok {
		t.Fatalf("expected *TodoState from done handler, got %T", afterDone)
	}
	if len(doneState.Tasks) != 1 {
		t.Fatalf("expected 1 task after done, got %d", len(doneState.Tasks))
	}

	if !doneState.Tasks[0].Complete {
		t.Errorf("expected task Complete == true after done handler, got false")
	}
}

// TestPrefillIsland_Mount verifies that mounting the prefill island with
// Props{"name": "Test User", "age": 35} populates Name and Age in the state.
//
// Scenario: Form loads with initial values
func TestPrefillIsland_Mount(t *testing.T) {
	island, err := NewPrefillIsland()
	if err != nil {
		t.Fatalf("NewPrefillIsland() failed: %v", err)
	}

	ctx := context.Background()

	state, err := island.Mount(ctx, live.Props{
		"name": "Test User",
		"age":  35,
	}, "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	prefillState, ok := state.(*PrefillState)
	if !ok {
		t.Fatalf("expected *PrefillState, got %T", state)
	}

	if prefillState.Name != "Test User" {
		t.Errorf("expected Name == 'Test User', got %q", prefillState.Name)
	}
	if prefillState.Age != 35 {
		t.Errorf("expected Age == 35, got %d", prefillState.Age)
	}
}

// TestPrefillIsland_Validate verifies that calling the "validate" handler with
// an empty name sets Validation to "Name is required".
//
// Scenario: Validate prefill form
func TestPrefillIsland_Validate(t *testing.T) {
	island, err := NewPrefillIsland()
	if err != nil {
		t.Fatalf("NewPrefillIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("validate")
	if err != nil {
		t.Fatalf("expected 'validate' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()

	// Mount with some initial state
	initialState, err := island.Mount(ctx, live.Props{"name": "Test User", "age": 35}, "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Call validate with empty name
	newState, err := handler(ctx, initialState, live.Params{"name": ""})
	if err != nil {
		t.Fatalf("validate handler returned unexpected error: %v", err)
	}

	prefillState, ok := newState.(*PrefillState)
	if !ok {
		t.Fatalf("expected *PrefillState, got %T", newState)
	}

	if prefillState.Validation != "Name is required" {
		t.Errorf("expected Validation == 'Name is required', got %q", prefillState.Validation)
	}
}

// TestPrefillIsland_Save verifies that the "save" handler updates name and age in state.
func TestPrefillIsland_Save(t *testing.T) {
	island, err := NewPrefillIsland()
	if err != nil {
		t.Fatalf("NewPrefillIsland failed: %v", err)
	}

	handler, err := island.GetEventHandler("save")
	if err != nil {
		t.Fatalf("expected 'save' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()

	// Mount with initial values
	initialState, err := island.Mount(ctx, live.Props{"name": "Test User", "age": 35}, "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Call save with new values
	newState, err := handler(ctx, initialState, live.Params{"name": "New Name", "age": "40"})
	if err != nil {
		t.Fatalf("save handler returned unexpected error: %v", err)
	}

	prefillState, ok := newState.(*PrefillState)
	if !ok {
		t.Fatalf("expected *PrefillState, got %T", newState)
	}

	if prefillState.Name != "New Name" {
		t.Errorf("expected Name == 'New Name', got %q", prefillState.Name)
	}
	if prefillState.Age != 40 {
		t.Errorf("expected Age == 40, got %d", prefillState.Age)
	}
}
