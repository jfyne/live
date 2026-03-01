package live

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// engineMockTransport is a transport implementation for engine testing
// that tracks sent events.
type engineMockTransport struct {
	events chan Event
	sent   []Event
	mu     sync.Mutex
	closed bool
}

func newEngineMockTransport() *engineMockTransport {
	return &engineMockTransport{
		events: make(chan Event, 16),
		sent:   []Event{},
	}
}

func (m *engineMockTransport) Send(e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrTransportClosed
	}

	m.sent = append(m.sent, e)
	return nil
}

func (m *engineMockTransport) Events() <-chan Event {
	return m.events
}

func (m *engineMockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		close(m.events)
		m.closed = true
	}
	return nil
}

func (m *engineMockTransport) GetSent() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Event{}, m.sent...)
}

// TestEngineSessionLifecycle tests adding, getting, and deleting sessions
func TestEngineSessionLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create a session
	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)

	// Add session to engine
	engine.AddSession(session)

	// Give the session manager goroutine time to process
	time.Sleep(10 * time.Millisecond)

	// Get session should succeed
	retrievedSession, ok := engine.GetSession("session-1")
	if !ok {
		t.Fatal("expected to find session")
	}
	if retrievedSession.ID != "session-1" {
		t.Errorf("expected session ID 'session-1', got %s", retrievedSession.ID)
	}

	// Delete session
	engine.DeleteSession("session-1")

	// Give the session manager goroutine time to process
	time.Sleep(10 * time.Millisecond)

	// Get session should fail
	_, ok = engine.GetSession("session-1")
	if ok {
		t.Error("expected session to be deleted")
	}
}

// TestEngineMountIsland tests mounting an island with props
func TestEngineMountIsland(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a test island
	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		return NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				initialValue := props.Int("initial")
				return map[string]any{"count": initialValue}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(map[string]any)
				count := state["count"].(int)
				tmpl := template.Must(template.New("counter").Parse("<div>Count: {{.}}</div>"))
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, count); err != nil {
					return nil, err
				}
				return &buf, nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create a session
	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	// Mount an island with props
	props := Props{"initial": 5}
	instance, err := engine.MountIsland("session-1", "counter-1", "counter", props)
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Verify the instance was created
	if instance.ID != "counter-1" {
		t.Errorf("expected instance ID 'counter-1', got %s", instance.ID)
	}
	if instance.Type != "counter" {
		t.Errorf("expected instance type 'counter', got %s", instance.Type)
	}

	// Verify the mount handler was called and state was set
	state := instance.State().(map[string]any)
	if state["count"] != 5 {
		t.Errorf("expected count 5, got %v", state["count"])
	}

	// Verify the instance is in the session
	retrievedInstance, ok := session.GetIsland("counter-1")
	if !ok {
		t.Fatal("expected to find island in session")
	}
	if retrievedInstance.ID != "counter-1" {
		t.Errorf("expected instance ID 'counter-1', got %s", retrievedInstance.ID)
	}

	// Verify state was saved to state store
	savedState, ok := stateStore.Get("session-1", "counter-1")
	if !ok {
		t.Fatal("expected to find state in state store")
	}
	savedStateMap := savedState.(map[string]any)
	if savedStateMap["count"] != 5 {
		t.Errorf("expected saved count 5, got %v", savedStateMap["count"])
	}

	// Verify a patch event was sent to the client
	sent := transport.GetSent()
	if len(sent) == 0 {
		t.Fatal("expected at least one event to be sent")
	}
	patchEvent := sent[0]
	if patchEvent.T != EventPatch {
		t.Errorf("expected patch event, got %s", patchEvent.T)
	}
	if patchEvent.Island != "counter-1" {
		t.Errorf("expected island ID 'counter-1', got %s", patchEvent.Island)
	}
}

// TestEngineRouteEvent tests routing events to the correct island
func TestEngineRouteEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a test island with an event handler
	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		island, _ := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(map[string]any)
				count := state["count"].(int)
				tmpl := template.Must(template.New("counter").Parse("<div>Count: {{.}}</div>"))
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, count); err != nil {
					return nil, err
				}
				return &buf, nil
			}),
		)
		island.HandleEvent("increment", func(ctx context.Context, state any, params Params) (any, error) {
			stateMap := state.(map[string]any)
			stateMap["count"] = stateMap["count"].(int) + 1
			return stateMap, nil
		})
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create a session and mount an island
	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-1", "counter-1", "counter", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Clear sent events from mounting
	transport.mu.Lock()
	transport.sent = []Event{}
	transport.mu.Unlock()

	// Route an increment event
	event := Event{
		T:      "increment",
		Island: "counter-1",
		Data:   []byte(`{}`),
	}
	err = engine.RouteEvent("session-1", event)
	if err != nil {
		t.Fatalf("failed to route event: %v", err)
	}

	// Verify the state was updated
	state := instance.State().(map[string]any)
	if state["count"] != 1 {
		t.Errorf("expected count 1, got %v", state["count"])
	}

	// Verify state was saved to state store
	savedState, ok := stateStore.Get("session-1", "counter-1")
	if !ok {
		t.Fatal("expected to find state in state store")
	}
	savedStateMap := savedState.(map[string]any)
	if savedStateMap["count"] != 1 {
		t.Errorf("expected saved count 1, got %v", savedStateMap["count"])
	}

	// Verify a patch event was sent to the client
	sent := transport.GetSent()
	if len(sent) == 0 {
		t.Fatal("expected at least one event to be sent")
	}
	patchEvent := sent[0]
	if patchEvent.T != EventPatch {
		t.Errorf("expected patch event, got %s", patchEvent.T)
	}
}

// TestEngineUnmountIsland tests unmounting cleanup
func TestEngineUnmountIsland(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a test island with unmount handler
	unmountCalled := false
	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		return NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithUnmount(func(ctx context.Context, state any) error {
				unmountCalled = true
				return nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>test</div>")), nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create a session and mount an island
	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err = engine.MountIsland("session-1", "counter-1", "counter", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Verify state is in the store
	_, ok := stateStore.Get("session-1", "counter-1")
	if !ok {
		t.Fatal("expected state to be in store before unmount")
	}

	// Unmount the island
	err = engine.UnmountIsland("session-1", "counter-1")
	if err != nil {
		t.Fatalf("failed to unmount island: %v", err)
	}

	// Verify unmount handler was called
	if !unmountCalled {
		t.Error("expected unmount handler to be called")
	}

	// Verify instance was removed from session
	_, ok = session.GetIsland("counter-1")
	if ok {
		t.Error("expected island to be removed from session")
	}

	// Verify state was deleted from store
	_, ok = stateStore.Get("session-1", "counter-1")
	if ok {
		t.Error("expected state to be deleted from store")
	}
}

// TestEngineBroadcastToIslandType tests broadcasting to all islands of a type
func TestEngineBroadcastToIslandType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a test island
	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		return NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>test</div>")), nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create two sessions with counter islands
	transport1 := newEngineMockTransport()
	session1 := NewSession(ctx, "session-1", transport1)
	engine.AddSession(session1)

	transport2 := newEngineMockTransport()
	session2 := NewSession(ctx, "session-2", transport2)
	engine.AddSession(session2)

	time.Sleep(10 * time.Millisecond)

	// Mount counter islands in both sessions
	_, err = engine.MountIsland("session-1", "counter-1", "counter", Props{})
	if err != nil {
		t.Fatalf("failed to mount island 1: %v", err)
	}
	_, err = engine.MountIsland("session-2", "counter-2", "counter", Props{})
	if err != nil {
		t.Fatalf("failed to mount island 2: %v", err)
	}

	// Clear sent events
	transport1.mu.Lock()
	transport1.sent = []Event{}
	transport1.mu.Unlock()
	transport2.mu.Lock()
	transport2.sent = []Event{}
	transport2.mu.Unlock()

	// Broadcast a message to all counter islands
	event := Event{
		T:    "message",
		Data: []byte(`{"text":"hello"}`),
	}
	engine.BroadcastToIslandType("counter", event)

	// Give time for events to be sent
	time.Sleep(10 * time.Millisecond)

	// Verify both transports received the event
	sent1 := transport1.GetSent()
	if len(sent1) == 0 {
		t.Error("expected session 1 to receive broadcast")
	} else {
		if sent1[0].T != "message" {
			t.Errorf("expected message event, got %s", sent1[0].T)
		}
		if sent1[0].Island != "counter-1" {
			t.Errorf("expected island ID 'counter-1', got %s", sent1[0].Island)
		}
	}

	sent2 := transport2.GetSent()
	if len(sent2) == 0 {
		t.Error("expected session 2 to receive broadcast")
	} else {
		if sent2[0].T != "message" {
			t.Errorf("expected message event, got %s", sent2[0].T)
		}
		if sent2[0].Island != "counter-2" {
			t.Errorf("expected island ID 'counter-2', got %s", sent2[0].Island)
		}
	}
}

// TestEngineConcurrentOperations tests thread safety with concurrent operations
func TestEngineConcurrentOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a test island
	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		return NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>test</div>")), nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Add and remove sessions concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sessionID := SessionID("session-" + string(rune('0'+i)))
			transport := newEngineMockTransport()
			session := NewSession(ctx, sessionID, transport)
			engine.AddSession(session)
			time.Sleep(1 * time.Millisecond)
			engine.DeleteSession(sessionID)
		}(i)
	}
	wg.Wait()

	// Verify no sessions remain
	time.Sleep(20 * time.Millisecond)
	for i := 0; i < 10; i++ {
		sessionID := SessionID("session-" + string(rune('0'+i)))
		_, ok := engine.GetSession(sessionID)
		if ok {
			t.Errorf("expected session %s to be deleted", sessionID)
		}
	}
}

// TestEngineStateRestorationOnReconnect tests that island state is restored
// when a client reconnects with a new session ID.
func TestEngineStateRestorationOnReconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a counter island
	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		island, _ := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				initialValue := props.Int("initial")
				return map[string]any{"count": initialValue}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>test</div>")), nil
			}),
		)
		island.HandleEvent("increment", func(ctx context.Context, state any, params Params) (any, error) {
			stateMap := state.(map[string]any)
			stateMap["count"] = stateMap["count"].(int) + 1
			return stateMap, nil
		})
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// --- Session 1: initial connection ---
	transport1 := newEngineMockTransport()
	session1 := NewSession(ctx, "session-1", transport1)
	engine.AddSession(session1)

	// Mount island with initial count=0
	_, err = engine.MountIsland("session-1", "counter-1", "counter", Props{"initial": 0})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Increment the counter 5 times
	for i := 0; i < 5; i++ {
		err = engine.RouteEvent("session-1", Event{
			T:      "increment",
			Island: "counter-1",
			Data:   []byte(`{}`),
		})
		if err != nil {
			t.Fatalf("failed to route event: %v", err)
		}
	}

	// Verify count is 5
	instance1, _ := session1.GetIsland("counter-1")
	state1 := instance1.State().(map[string]any)
	if state1["count"] != 5 {
		t.Fatalf("expected count 5, got %v", state1["count"])
	}

	// --- Simulate disconnect ---
	engine.DeleteSession("session-1")

	// --- Session 2: reconnection with new session ID ---
	transport2 := newEngineMockTransport()
	session2 := NewSession(ctx, "session-2", transport2)
	engine.AddSession(session2)

	// Mount the same island again (as the client re-subscribes on reconnect)
	instance2, err := engine.MountIsland("session-2", "counter-1", "counter", Props{"initial": 0})
	if err != nil {
		t.Fatalf("failed to mount island on reconnect: %v", err)
	}

	// Verify the state was restored from the previous session
	state2 := instance2.State().(map[string]any)
	if state2["count"] != 5 {
		t.Errorf("expected restored count 5, got %v", state2["count"])
	}
}

// TestEngineStateRestorationPreservesCurrentSession tests that state from
// the current session takes priority over cross-session lookup.
func TestEngineStateRestorationPreservesCurrentSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		island, _ := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>test</div>")), nil
			}),
		)
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Pre-populate state store with existing state for this session+island
	stateStore.Set("session-1", "counter-1", map[string]any{"count": 42}, 1*time.Minute)

	// Create session and mount island
	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)

	instance, err := engine.MountIsland("session-1", "counter-1", "counter", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Should restore from current session state (count=42), not mount default (count=0)
	state := instance.State().(map[string]any)
	if state["count"] != 42 {
		t.Errorf("expected restored count 42, got %v", state["count"])
	}
}

// TestEngineDeleteSessionPreservesState tests that DeleteSession keeps
// state in the store for reconnection.
func TestEngineDeleteSessionPreservesState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		return NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>test</div>")), nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create session and mount island
	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)

	_, err = engine.MountIsland("session-1", "counter-1", "counter", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Delete session (simulates disconnect)
	engine.DeleteSession("session-1")

	// State should still be in the store
	state, ok := stateStore.Get("session-1", "counter-1")
	if !ok {
		t.Fatal("expected state to be preserved after DeleteSession")
	}
	stateMap := state.(map[string]any)
	if stateMap["count"] != 0 {
		t.Errorf("expected count 0, got %v", stateMap["count"])
	}
}

// TestEngineMultipleIslandsPerSession tests multiple islands in a single session
func TestEngineMultipleIslandsPerSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register test islands
	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		return NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>counter</div>")), nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	err = registry.Register("timer", func() (*Island, error) {
		return NewIsland("timer",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"time": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>timer</div>")), nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create a session
	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	// Mount multiple islands
	_, err = engine.MountIsland("session-1", "counter-1", "counter", Props{})
	if err != nil {
		t.Fatalf("failed to mount counter: %v", err)
	}
	_, err = engine.MountIsland("session-1", "timer-1", "timer", Props{})
	if err != nil {
		t.Fatalf("failed to mount timer: %v", err)
	}

	// Verify both islands are in the session
	instances := session.ListIslands()
	if len(instances) != 2 {
		t.Fatalf("expected 2 islands, got %d", len(instances))
	}

	// Verify we can get each island
	_, ok := session.GetIsland("counter-1")
	if !ok {
		t.Error("expected to find counter island")
	}
	_, ok = session.GetIsland("timer-1")
	if !ok {
		t.Error("expected to find timer island")
	}
}

// ---------------------------------------------------------------------------
// New feature tests (RED state — will fail until engine integration is added)
// ---------------------------------------------------------------------------

// TestBroadcastToIsland tests that BroadcastToIsland sends an event to all
// sessions that contain an island with the given ID, and does not send to
// sessions whose islands have a different ID.
func TestBroadcastToIsland(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	err := registry.Register("chat", func() (*Island, error) {
		return NewIsland("chat",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>chat</div>")), nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// session-1 hosts island "chat-1"
	transport1 := newEngineMockTransport()
	session1 := NewSession(ctx, "session-1", transport1)
	engine.AddSession(session1)

	// session-2 also hosts island "chat-1" (same island ID, different session)
	transport2 := newEngineMockTransport()
	session2 := NewSession(ctx, "session-2", transport2)
	engine.AddSession(session2)

	// session-3 hosts island "chat-2" (different island ID)
	transport3 := newEngineMockTransport()
	session3 := NewSession(ctx, "session-3", transport3)
	engine.AddSession(session3)

	time.Sleep(10 * time.Millisecond)

	_, err = engine.MountIsland("session-1", "chat-1", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount chat-1 in session-1: %v", err)
	}
	_, err = engine.MountIsland("session-2", "chat-1", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount chat-1 in session-2: %v", err)
	}
	_, err = engine.MountIsland("session-3", "chat-2", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount chat-2 in session-3: %v", err)
	}

	// Clear sent events from mounting
	for _, tr := range []*engineMockTransport{transport1, transport2, transport3} {
		tr.mu.Lock()
		tr.sent = []Event{}
		tr.mu.Unlock()
	}

	// Broadcast to island ID "chat-1"
	event := Event{
		T:    "message",
		Data: []byte(`{"text":"hello chat-1"}`),
	}
	engine.BroadcastToIsland("chat-1", event)

	time.Sleep(10 * time.Millisecond)

	// session-1 (has "chat-1") should receive the event
	sent1 := transport1.GetSent()
	if len(sent1) == 0 {
		t.Error("expected session-1 (chat-1) to receive the broadcast")
	} else if sent1[0].T != "message" {
		t.Errorf("expected message event in session-1, got %q", sent1[0].T)
	}

	// session-2 (has "chat-1") should receive the event
	sent2 := transport2.GetSent()
	if len(sent2) == 0 {
		t.Error("expected session-2 (chat-1) to receive the broadcast")
	} else if sent2[0].T != "message" {
		t.Errorf("expected message event in session-2, got %q", sent2[0].T)
	}

	// session-3 (has "chat-2") should NOT receive the event
	sent3 := transport3.GetSent()
	if len(sent3) != 0 {
		t.Errorf("expected session-3 (chat-2) NOT to receive the broadcast, but got %d events", len(sent3))
	}
}

// TestSetStateTTL verifies that SetStateTTL updates the engine's internal TTL
// configuration so subsequent state saves use the new duration.
func TestSetStateTTL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		return NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return bytes.NewReader([]byte("<div>test</div>")), nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Apply a custom TTL
	customTTL := 5 * time.Minute
	engine.SetStateTTL(customTTL)

	// Verify the TTL is reflected in the engine (read the internal field under lock)
	engine.mu.RLock()
	got := engine.stateTTL
	engine.mu.RUnlock()

	if got != customTTL {
		t.Errorf("expected stateTTL %v, got %v", customTTL, got)
	}

	// Verify the TTL is used when mounting: mount an island after setting TTL and
	// confirm the state store accepted the state (no error).
	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err = engine.MountIsland("session-1", "counter-1", "counter", Props{})
	if err != nil {
		t.Fatalf("MountIsland failed after SetStateTTL: %v", err)
	}

	// State should be saved in the store
	_, ok := stateStore.Get("session-1", "counter-1")
	if !ok {
		t.Error("expected state to be saved in the store after MountIsland with custom TTL")
	}
}

// TestEngineSendSelfFromHandler tests that when an event handler calls SendSelf,
// the queued self-event is dispatched by the engine after the primary event completes.
// This test will FAIL until the engine creates an enriched context with the self-event
// queue and drains it after session.handleEvent returns.
func TestEngineSendSelfFromHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	err := registry.Register("ticker", func() (*Island, error) {
		island, _ := NewIsland("ticker",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"processed": false, "notified": false}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>ticker</div>"), nil
			}),
		)
		// "process" event calls SendSelf to enqueue a "notify" self-event
		island.HandleEvent("process", func(ctx context.Context, state any, params Params) (any, error) {
			stateMap := state.(map[string]any)
			stateMap["processed"] = true
			SendSelf(ctx, "notify", "notification-data")
			return stateMap, nil
		})
		// "notify" self handler updates state to record its execution
		island.HandleSelf("notify", func(ctx context.Context, state any, data any) (any, error) {
			stateMap := state.(map[string]any)
			stateMap["notified"] = true
			return stateMap, nil
		})
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-1", "ticker-1", "ticker", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Clear sent events from mounting
	transport.mu.Lock()
	transport.sent = []Event{}
	transport.mu.Unlock()

	// Route the "process" event - the engine should dispatch "notify" after it
	err = engine.RouteEvent("session-1", Event{
		T:      "process",
		Island: "ticker-1",
		Data:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("RouteEvent failed: %v", err)
	}

	// Verify the "notify" self handler executed (state was updated by both handlers)
	state := instance.State().(map[string]any)
	if !state["processed"].(bool) {
		t.Error("expected 'processed' to be true after process event")
	}
	if !state["notified"].(bool) {
		t.Error("expected 'notified' to be true after SendSelf dispatched 'notify' self-event; engine must drain the self-event queue after handleEvent")
	}

	// Verify at least 2 patch events were sent: one for "process", one for "notify"
	sent := transport.GetSent()
	patchCount := 0
	for _, e := range sent {
		if e.T == EventPatch {
			patchCount++
		}
	}
	if patchCount < 2 {
		t.Errorf("expected at least 2 patch events (one for 'process' and one for 'notify'), got %d", patchCount)
	}
}

// TestEngineErrorHandler tests that when an event handler returns an error, the engine
// sends an error event to the client via the default error handler.
// This test will FAIL until the engine calls island.errorHandler and sends the resulting
// event to the session transport when session.handleEvent returns an error.
func TestEngineErrorHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	err := registry.Register("failer", func() (*Island, error) {
		island, _ := NewIsland("failer",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>failer</div>"), nil
			}),
		)
		island.HandleEvent("fail", func(ctx context.Context, state any, params Params) (any, error) {
			return nil, fmt.Errorf("something went wrong")
		})
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err = engine.MountIsland("session-1", "failer-1", "failer", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Clear sent events from mounting
	transport.mu.Lock()
	transport.sent = []Event{}
	transport.mu.Unlock()

	// Route the "fail" event — the engine should return an error
	err = engine.RouteEvent("session-1", Event{
		T:      "fail",
		Island: "failer-1",
		Data:   []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected RouteEvent to return an error when the handler fails")
	}

	// Verify the transport received an error event with T == EventError
	sent := transport.GetSent()
	var errorEvent *Event
	for i := range sent {
		if sent[i].T == EventError {
			errorEvent = &sent[i]
			break
		}
	}
	if errorEvent == nil {
		t.Fatal("expected an error event to be sent to the transport; engine must call island.errorHandler and send the result")
	}

	// Verify the error event data contains {"err": "something went wrong"}
	var errData map[string]string
	if jsonErr := json.Unmarshal(errorEvent.Data, &errData); jsonErr != nil {
		t.Fatalf("failed to unmarshal error event data: %v", jsonErr)
	}
	if errData["err"] != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got %q", errData["err"])
	}
}

// TestEngineErrorHandler_Custom tests that when an island has a custom error handler,
// that handler's event is sent to the client instead of the default error event.
// This test will FAIL until the engine calls island.errorHandler (which will invoke
// the custom handler) and sends the resulting event to the session transport.
func TestEngineErrorHandler_Custom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	err := registry.Register("custom-failer", func() (*Island, error) {
		island, _ := NewIsland("custom-failer",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>custom-failer</div>"), nil
			}),
			WithErrorHandler(func(ctx context.Context, err error) Event {
				data, _ := json.Marshal(map[string]string{"custom": err.Error()})
				return Event{T: "custom-err", Data: data}
			}),
		)
		island.HandleEvent("fail", func(ctx context.Context, state any, params Params) (any, error) {
			return nil, fmt.Errorf("custom failure")
		})
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err = engine.MountIsland("session-1", "custom-failer-1", "custom-failer", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Clear sent events from mounting
	transport.mu.Lock()
	transport.sent = []Event{}
	transport.mu.Unlock()

	// Route the "fail" event
	err = engine.RouteEvent("session-1", Event{
		T:      "fail",
		Island: "custom-failer-1",
		Data:   []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected RouteEvent to return an error when the handler fails")
	}

	// Verify the transport received the custom error event
	sent := transport.GetSent()
	var customErrEvent *Event
	for i := range sent {
		if sent[i].T == "custom-err" {
			customErrEvent = &sent[i]
			break
		}
	}
	if customErrEvent == nil {
		t.Fatal("expected a 'custom-err' event to be sent to transport; engine must use island.errorHandler (the custom one)")
	}

	// Verify the custom error event data contains {"custom": "custom failure"}
	var errData map[string]string
	if jsonErr := json.Unmarshal(customErrEvent.Data, &errData); jsonErr != nil {
		t.Fatalf("failed to unmarshal custom error event data: %v", jsonErr)
	}
	if errData["custom"] != "custom failure" {
		t.Errorf("expected 'custom' field 'custom failure', got %q", errData["custom"])
	}
}

// TestEngineEventDelay tests that WithEventDelay causes the engine to re-schedule
// a self-event after the configured delay, and that the timer is cancelled on unmount.
// This test will FAIL until the engine checks island.GetEventDelay after handling
// a self-event and schedules a time.AfterFunc to re-deliver it.
func TestEngineEventDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	err := registry.Register("delayer", func() (*Island, error) {
		island, _ := NewIsland("delayer",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"tickCount": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>delayer</div>"), nil
			}),
			WithEventDelay("tick", 50*time.Millisecond),
		)
		// "tick" self handler increments tickCount
		island.HandleSelf("tick", func(ctx context.Context, state any, data any) (any, error) {
			stateMap := state.(map[string]any)
			stateMap["tickCount"] = stateMap["tickCount"].(int) + 1
			return stateMap, nil
		})
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-1", "delayer-1", "delayer", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Manually trigger the first "tick" self-event via RouteEvent with SelfData
	err = engine.RouteEvent("session-1", Event{
		T:        "tick",
		Island:   "delayer-1",
		SelfData: "tick-data",
	})
	if err != nil {
		t.Fatalf("first RouteEvent for tick failed: %v", err)
	}

	// After the first tick, the engine should schedule a re-delivery after 50ms.
	// Wait 100ms to allow at least one re-delivery to fire.
	time.Sleep(100 * time.Millisecond)

	// The "tick" handler should have executed at least twice: initial + delayed re-delivery
	state := instance.State().(map[string]any)
	tickCount := state["tickCount"].(int)
	if tickCount < 2 {
		t.Errorf("expected 'tickCount' >= 2 after 100ms (initial + at least one delayed re-delivery), got %d; engine must schedule re-delivery via time.AfterFunc after a self-event with a configured delay", tickCount)
	}

	// Unmount the island — this should cancel any pending timers
	err = engine.UnmountIsland("session-1", "delayer-1")
	if err != nil {
		t.Fatalf("failed to unmount island: %v", err)
	}

	// After unmount, verify the island is no longer in the session
	_, stillMounted := session.GetIsland("delayer-1")
	if stillMounted {
		t.Error("expected island to be unmounted from session")
	}
}

// TestEngineSendSelfFromMount tests that when the Mount handler calls SendSelf,
// the queued self-event is dispatched by the engine after MountIsland completes.
// This test will FAIL until MountIsland creates an enriched context with the
// self-event queue and drains it after instance.Mount returns.
func TestEngineSendSelfFromMount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	err := registry.Register("initter", func() (*Island, error) {
		island, _ := NewIsland("initter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				// Call SendSelf during mount to queue an "init" self-event
				SendSelf(ctx, "init", "init-data")
				return map[string]any{"initialized": false}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>initter</div>"), nil
			}),
		)
		// "init" self handler marks the island as initialized
		island.HandleSelf("init", func(ctx context.Context, state any, data any) (any, error) {
			stateMap := state.(map[string]any)
			stateMap["initialized"] = true
			return stateMap, nil
		})
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-1", "initter-1", "initter", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Verify the "init" self handler executed during MountIsland
	state := instance.State().(map[string]any)
	if !state["initialized"].(bool) {
		t.Error("expected 'initialized' to be true after MountIsland; engine must drain the self-event queue queued during the mount handler")
	}

	// Verify at least 2 patch events were sent: one for mount, one for "init" self-event
	sent := transport.GetSent()
	patchCount := 0
	for _, e := range sent {
		if e.T == EventPatch {
			patchCount++
		}
	}
	if patchCount < 2 {
		t.Errorf("expected at least 2 patch events (mount + 'init' self-event), got %d", patchCount)
	}
}

// ---------------------------------------------------------------------------
// RED tests for PatchURL, Redirect, and EventParams session routing.
// These tests reference APIs that do not yet exist and will fail to compile.
// ---------------------------------------------------------------------------

// buildParamCtx builds a context enriched with session ID and engine so that
// PatchURL and Redirect can extract the session to send via the transport.
// This helper mirrors what the engine creates during RouteEvent.
func buildParamCtx(ctx context.Context, engine *IslandEngine, sessionID SessionID, islandID IslandID) context.Context {
	ctx = contextWithSessionID(ctx, sessionID)
	ctx = contextWithIslandID(ctx, islandID)
	ctx = contextWithEngine(ctx, engine)
	return ctx
}

// setupParamTestEngine creates a minimal engine + session + island ready to receive events.
// Returns (engine, session, transport, cleanup).
func setupParamTestEngine(t *testing.T) (*IslandEngine, *Session, *engineMockTransport, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	registry := NewIslandRegistry()
	_ = registry.Register("widget", func() (*Island, error) {
		return NewIsland("widget",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>widget</div>"), nil
			}),
		)
	})

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err := engine.MountIsland("session-1", "widget-1", "widget", Props{})
	if err != nil {
		cancel()
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Clear mount events
	transport.mu.Lock()
	transport.sent = []Event{}
	transport.mu.Unlock()

	cleanup := func() {
		engine.Close()
		cancel()
	}
	return engine, session, transport, cleanup
}

// TestPatchURL verifies that PatchURL sends an EventParams event via the
// session transport with the URL-encoded values.
//
// Scenario: PatchURL sends an EventParams event via the session transport
// Scenario: The event data contains the URL-encoded values
func TestPatchURL(t *testing.T) {
	engine, _, transport, cleanup := setupParamTestEngine(t)
	defer cleanup()

	ctx := buildParamCtx(context.Background(), engine, "session-1", "widget-1")

	values := map[string]string{
		"page":   "2",
		"filter": "active",
	}
	err := PatchURL(ctx, values)
	if err != nil {
		t.Fatalf("PatchURL() error = %v", err)
	}

	// The transport should have received an EventParams event.
	sent := transport.GetSent()
	if len(sent) == 0 {
		t.Fatal("PatchURL() sent no events; expected an EventParams event on the transport")
	}

	var paramsEvent *Event
	for i := range sent {
		if sent[i].T == EventParams {
			paramsEvent = &sent[i]
			break
		}
	}
	if paramsEvent == nil {
		t.Fatalf("PatchURL() did not send an EventParams event; got events: %v", sent)
	}

	// The Data should be non-empty (URL-encoded or JSON-encoded values).
	if len(paramsEvent.Data) == 0 {
		t.Error("PatchURL() EventParams event has empty Data, want URL-encoded values")
	}
}

// TestPatchURLMissingContext verifies that PatchURL returns an error when
// the context is missing required values (session ID / engine).
//
// Scenario: PatchURL with missing context values returns error
func TestPatchURLMissingContext(t *testing.T) {
	t.Run("missing session ID returns error", func(t *testing.T) {
		// Plain context with no session/engine embedded.
		err := PatchURL(context.Background(), map[string]string{"page": "1"})
		if err == nil {
			t.Error("PatchURL() with plain context returned nil error, want non-nil")
		}
	})
}

// TestRedirect verifies that Redirect sends an EventRedirect event via the
// session transport with the URL string as data.
//
// Scenario: Redirect sends an EventRedirect event via the session transport
// Scenario: The event data contains the URL string
func TestRedirect(t *testing.T) {
	engine, _, transport, cleanup := setupParamTestEngine(t)
	defer cleanup()

	ctx := buildParamCtx(context.Background(), engine, "session-1", "widget-1")

	targetURL := "/dashboard?tab=overview"
	err := Redirect(ctx, targetURL)
	if err != nil {
		t.Fatalf("Redirect() error = %v", err)
	}

	// The transport should have received an EventRedirect event.
	sent := transport.GetSent()
	if len(sent) == 0 {
		t.Fatal("Redirect() sent no events; expected an EventRedirect event on the transport")
	}

	var redirectEvent *Event
	for i := range sent {
		if sent[i].T == EventRedirect {
			redirectEvent = &sent[i]
			break
		}
	}
	if redirectEvent == nil {
		t.Fatalf("Redirect() did not send an EventRedirect event; got events: %v", sent)
	}

	// The Data should contain the target URL.
	if len(redirectEvent.Data) == 0 {
		t.Error("Redirect() EventRedirect event has empty Data, want URL string")
	}

	// Verify the URL is in the data.
	dataStr := string(redirectEvent.Data)
	if !strings.Contains(dataStr, "/dashboard") {
		t.Errorf("Redirect() event Data = %q, expected to contain %q", dataStr, targetURL)
	}
}

// TestRedirectMissingContext verifies that Redirect returns an error when
// the context is missing required values (session ID / engine).
//
// Scenario: Redirect with missing context values returns error
func TestRedirectMissingContext(t *testing.T) {
	t.Run("missing session ID returns error", func(t *testing.T) {
		err := Redirect(context.Background(), "/some-page")
		if err == nil {
			t.Error("Redirect() with plain context returned nil error, want non-nil")
		}
	})
}

// TestSessionRouteEventParams verifies that when an EventParams event arrives
// at the session, it is routed to instance.CallParams rather than CallEvent.
//
// Scenario: When EventParams arrives, it's routed to instance.CallParams not CallEvent
func TestSessionRouteEventParams(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	paramsCalled := false
	eventCalled := false

	registry := NewIslandRegistry()
	_ = registry.Register("tracked-params", func() (*Island, error) {
		island, _ := NewIsland("tracked-params",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"params": Params{}}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>tracked</div>"), nil
			}),
		)
		// Register a regular event handler — should NOT be called for EventParams.
		_ = island.HandleEvent("params", func(ctx context.Context, state any, p Params) (any, error) {
			eventCalled = true
			return state, nil
		})
		// Register a params handler — SHOULD be called for EventParams.
		_ = island.HandleParams(func(ctx context.Context, state any, p Params) (any, error) {
			paramsCalled = true
			stateMap := state.(map[string]any)
			stateMap["params"] = p
			return stateMap, nil
		})
		return island, nil
	})

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-1", "tracked-1", "tracked-params", Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Clear mount events
	transport.mu.Lock()
	transport.sent = []Event{}
	transport.mu.Unlock()

	// Route an EventParams event with page=3.
	paramsData, _ := json.Marshal(Params{"page": "3"})
	err = engine.RouteEvent("session-1", Event{
		T:      EventParams,
		Island: "tracked-1",
		Data:   paramsData,
	})
	if err != nil {
		t.Fatalf("RouteEvent(EventParams) error = %v", err)
	}

	// The params handler should have been called.
	if !paramsCalled {
		t.Error("EventParams routing: params handler was NOT called; expected it to be called via CallParams")
	}

	// The event handler for "params" should NOT have been called.
	if eventCalled {
		t.Error("EventParams routing: event handler was called; expected only params handler to be called")
	}

	// Verify the state was updated by the params handler.
	state := instance.State().(map[string]any)
	params, ok := state["params"].(Params)
	if !ok {
		t.Fatalf("state[params] is not Params, got %T", state["params"])
	}
	if params.String("page") != "3" {
		t.Errorf("state[params][page] = %q, want %q", params.String("page"), "3")
	}
}

// TestSessionRouteEventParamsNoHandler verifies that EventParams with no
// registered params handler is silently ignored (no error).
//
// Scenario: EventParams with no handler is silently ignored
func TestSessionRouteEventParamsNoHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	_ = registry.Register("no-params-handler", func() (*Island, error) {
		return NewIsland("no-params-handler",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>no params</div>"), nil
			}),
		)
	})

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err := engine.MountIsland("session-1", "no-params-1", "no-params-handler", Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Route EventParams to an island with no params handler — should be a no-op (no error).
	paramsData, _ := json.Marshal(Params{"page": "1"})
	err = engine.RouteEvent("session-1", Event{
		T:      EventParams,
		Island: "no-params-1",
		Data:   paramsData,
	})
	if err != nil {
		t.Errorf("RouteEvent(EventParams) with no handler error = %v, want nil (silently ignored)", err)
	}
}
