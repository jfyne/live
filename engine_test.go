package live

import (
	"bytes"
	"context"
	"html/template"
	"io"
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
