package live

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestFullLifecycleIntegration tests the complete flow:
// mount → event → render → patch with real WebSocket transport.
func TestFullLifecycleIntegration(t *testing.T) {
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
				state := rc.State.(map[string]any)
				count := state["count"].(int)
				tmpl := template.Must(template.New("counter").Parse(
					`<div id="count-display">Count: {{.}}</div><button live-click="increment">+</button>`,
				))
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

	// Create engine with state store
	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create HTTP test server with WebSocket endpoint
	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Upgrade to WebSocket
		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		defer transport.Close()

		// Create a custom transport wrapper that intercepts events
		// and routes them through the engine instead of the session
		sessionID := SessionID("test-session-1")
		engineRoutedTransport := newEngineRoutedTransport(transport, engine, sessionID)

		// Create session with the wrapper transport
		session := NewSession(r.Context(), sessionID, engineRoutedTransport)
		engine.AddSession(session)
		defer engine.DeleteSession(sessionID)

		// Wait for session to be added
		time.Sleep(10 * time.Millisecond)

		// Mount the counter island
		_, err = engine.MountIsland(sessionID, "counter-1", "counter", Props{"initial": 10})
		if err != nil {
			t.Logf("mount error: %v", err)
			return
		}

		// Keep connection alive
		<-r.Context().Done()
	}))
	defer server.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer clientCancel()

	conn, _, err := websocket.Dial(clientCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read connect event
	_, _, err = conn.Read(clientCtx)
	if err != nil {
		t.Fatalf("read connect event failed: %v", err)
	}

	// Read initial mount patch event
	msgType, data, err := conn.Read(clientCtx)
	if err != nil {
		t.Fatalf("read mount patch failed: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Fatalf("expected text message, got %v", msgType)
	}

	var mountPatch Event
	if err := json.Unmarshal(data, &mountPatch); err != nil {
		t.Fatalf("unmarshal mount patch failed: %v", err)
	}

	if mountPatch.T != EventPatch {
		t.Errorf("expected patch event, got %s", mountPatch.T)
	}
	if mountPatch.Island != "counter-1" {
		t.Errorf("expected island 'counter-1', got %s", mountPatch.Island)
	}

	// Verify the initial render contains "Count: 10"
	var patches []Patch
	if err := json.Unmarshal(mountPatch.Data, &patches); err != nil {
		t.Fatalf("unmarshal patch data failed: %v", err)
	}
	if len(patches) == 0 {
		t.Fatal("expected at least one patch")
	}
	// Check if any patch contains "Count: 10"
	foundCount := false
	for _, patch := range patches {
		if strings.Contains(patch.HTML, "Count: 10") {
			foundCount = true
			break
		}
	}
	if !foundCount {
		t.Errorf("expected initial render to contain 'Count: 10' in patches: %+v", patches)
	}

	// Send increment event
	incrementEvent := Event{
		T:      "increment",
		Island: "counter-1",
		Data:   json.RawMessage(`{}`),
	}
	incrementData, _ := json.Marshal(incrementEvent)
	if err := conn.Write(clientCtx, websocket.MessageText, incrementData); err != nil {
		t.Fatalf("send increment event failed: %v", err)
	}

	// Read patch event after increment
	msgType, data, err = conn.Read(clientCtx)
	if err != nil {
		t.Fatalf("read increment patch failed: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Fatalf("expected text message, got %v", msgType)
	}

	var incrementPatch Event
	if err := json.Unmarshal(data, &incrementPatch); err != nil {
		t.Fatalf("unmarshal increment patch failed: %v", err)
	}

	if incrementPatch.T != EventPatch {
		t.Errorf("expected patch event, got %s", incrementPatch.T)
	}

	// Verify the updated render contains "Count: 11"
	if err := json.Unmarshal(incrementPatch.Data, &patches); err != nil {
		t.Fatalf("unmarshal increment patch data failed: %v", err)
	}
	foundCount = false
	for _, patch := range patches {
		if strings.Contains(patch.HTML, "Count: 11") {
			foundCount = true
			break
		}
	}
	if !foundCount {
		t.Errorf("expected updated render to contain 'Count: 11' in patches: %+v", patches)
	}

	// Verify state was persisted in state store
	savedState, ok := stateStore.Get("test-session-1", "counter-1")
	if !ok {
		t.Fatal("expected state to be saved in state store")
	}
	savedStateMap := savedState.(map[string]any)
	if savedStateMap["count"] != 11 {
		t.Errorf("expected saved count to be 11, got %v", savedStateMap["count"])
	}
}

// TestMultipleIslandsOnSameSession tests multiple islands in one session
// with proper event routing and state isolation.
func TestMultipleIslandsOnSameSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register counter island
	registry := NewIslandRegistry()
	err := registry.Register("counter", func() (*Island, error) {
		island, _ := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(map[string]any)
				count := state["count"].(int)
				return bytes.NewReader([]byte(fmt.Sprintf("<div>Counter: %d</div>", count))), nil
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

	// Register timer island
	err = registry.Register("timer", func() (*Island, error) {
		island, _ := NewIsland("timer",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"seconds": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(map[string]any)
				seconds := state["seconds"].(int)
				return bytes.NewReader([]byte(fmt.Sprintf("<div>Timer: %d</div>", seconds))), nil
			}),
		)
		island.HandleEvent("tick", func(ctx context.Context, state any, params Params) (any, error) {
			stateMap := state.(map[string]any)
			stateMap["seconds"] = stateMap["seconds"].(int) + 1
			return stateMap, nil
		})
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create engine
	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create server
	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)

	sessionID := SessionID("test-session-multi")
	var serverSession *Session

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		defer transport.Close()

		// Create transport wrapper that routes through engine
		engineRoutedTransport := newEngineRoutedTransport(transport, engine, sessionID)

		serverSession = NewSession(r.Context(), sessionID, engineRoutedTransport)
		engine.AddSession(serverSession)
		defer engine.DeleteSession(sessionID)

		time.Sleep(10 * time.Millisecond)

		// Mount two counter islands and one timer island
		_, err = engine.MountIsland(sessionID, "counter-a", "counter", Props{})
		if err != nil {
			t.Logf("mount counter-a error: %v", err)
			return
		}
		_, err = engine.MountIsland(sessionID, "counter-b", "counter", Props{})
		if err != nil {
			t.Logf("mount counter-b error: %v", err)
			return
		}
		_, err = engine.MountIsland(sessionID, "timer-1", "timer", Props{})
		if err != nil {
			t.Logf("mount timer-1 error: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	// Connect client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer clientCancel()

	conn, _, err := websocket.Dial(clientCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read connect event
	_, _, _ = conn.Read(clientCtx)

	// Read 3 mount patches (counter-a, counter-b, timer-1)
	for i := 0; i < 3; i++ {
		_, _, err := conn.Read(clientCtx)
		if err != nil {
			t.Fatalf("read mount patch %d failed: %v", i, err)
		}
	}

	// Wait for session setup
	time.Sleep(20 * time.Millisecond)

	// Send increment to counter-a
	counterAEvent := Event{
		T:      "increment",
		Island: "counter-a",
		Data:   json.RawMessage(`{}`),
	}
	data, _ := json.Marshal(counterAEvent)
	if err := conn.Write(clientCtx, websocket.MessageText, data); err != nil {
		t.Fatalf("send counter-a event failed: %v", err)
	}

	// Read patch for counter-a
	msgType, patchData, err := conn.Read(clientCtx)
	if err != nil {
		t.Fatalf("read counter-a patch failed: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Fatalf("expected text message")
	}

	var patch Event
	if err := json.Unmarshal(patchData, &patch); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if patch.Island != "counter-a" {
		t.Errorf("expected patch for counter-a, got %s", patch.Island)
	}

	// Send tick to timer-1
	timerEvent := Event{
		T:      "tick",
		Island: "timer-1",
		Data:   json.RawMessage(`{}`),
	}
	data, _ = json.Marshal(timerEvent)
	if err := conn.Write(clientCtx, websocket.MessageText, data); err != nil {
		t.Fatalf("send timer event failed: %v", err)
	}

	// Read patch for timer-1
	msgType, patchData, err = conn.Read(clientCtx)
	if err != nil {
		t.Fatalf("read timer patch failed: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Fatalf("expected text message")
	}

	if err := json.Unmarshal(patchData, &patch); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if patch.Island != "timer-1" {
		t.Errorf("expected patch for timer-1, got %s", patch.Island)
	}

	// Verify state isolation: counter-a should be 1, counter-b should be 0, timer should be 1
	time.Sleep(10 * time.Millisecond)

	counterAState, ok := stateStore.Get(sessionID, "counter-a")
	if !ok {
		t.Fatal("counter-a state not found")
	}
	if counterAState.(map[string]any)["count"] != 1 {
		t.Errorf("expected counter-a count to be 1, got %v", counterAState.(map[string]any)["count"])
	}

	counterBState, ok := stateStore.Get(sessionID, "counter-b")
	if !ok {
		t.Fatal("counter-b state not found")
	}
	if counterBState.(map[string]any)["count"] != 0 {
		t.Errorf("expected counter-b count to be 0, got %v", counterBState.(map[string]any)["count"])
	}

	timerState, ok := stateStore.Get(sessionID, "timer-1")
	if !ok {
		t.Fatal("timer-1 state not found")
	}
	if timerState.(map[string]any)["seconds"] != 1 {
		t.Errorf("expected timer seconds to be 1, got %v", timerState.(map[string]any)["seconds"])
	}
}

// TestTransportReconnectionWithStatePreservation tests that island state
// is preserved across transport reconnections using the state store.
func TestTransportReconnectionWithStatePreservation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a stateful island
	registry := NewIslandRegistry()
	err := registry.Register("stateful", func() (*Island, error) {
		island, _ := NewIsland("stateful",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"value": "initial"}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(map[string]any)
				value := state["value"].(string)
				return bytes.NewReader([]byte(fmt.Sprintf("<div>Value: %s</div>", value))), nil
			}),
		)
		island.HandleEvent("update", func(ctx context.Context, state any, params Params) (any, error) {
			stateMap := state.(map[string]any)
			stateMap["value"] = params.String("newValue")
			return stateMap, nil
		})
		return island, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create engine with state store
	stateStore := NewMemoryIslandStateStore(ctx, 10*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)

	sessionID := SessionID("test-session-reconnect")
	serverMu := sync.Mutex{}
	var currentSession *Session

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		defer transport.Close()

		serverMu.Lock()
		// Check if we already have a session - if so, this is a reconnect
		existingSession, exists := engine.GetSession(sessionID)
		if exists {
			// Close the old session's transport (but don't delete from engine yet)
			// This preserves the state in the state store
			existingSession.Close()
			time.Sleep(20 * time.Millisecond)

			// Manually remove from sessions map without calling DeleteSession
			// which would delete the state from the store
			engine.mu.Lock()
			delete(engine.sessions, sessionID)
			engine.mu.Unlock()
		}

		// Create transport wrapper that routes through engine
		engineRoutedTransport := newEngineRoutedTransport(transport, engine, sessionID)

		currentSession = NewSession(r.Context(), sessionID, engineRoutedTransport)
		engine.AddSession(currentSession)
		serverMu.Unlock()

		time.Sleep(10 * time.Millisecond)

		// On first connect, mount the island
		if !exists {
			_, err = engine.MountIsland(sessionID, "stateful-1", "stateful", Props{})
			if err != nil {
				t.Logf("mount error: %v", err)
				return
			}
		} else {
			// On reconnect, restore island from state store
			savedState, ok := stateStore.Get(sessionID, "stateful-1")
			if ok {
				// Recreate the island instance with saved state
				instance, err := NewIslandInstanceFromRegistry("stateful-1", "stateful", Props{}, registry)
				if err != nil {
					t.Logf("recreate instance error: %v", err)
					return
				}
				instance.SetState(savedState)
				currentSession.AddIsland(instance)

				// Give the session time to be added to the engine
				time.Sleep(20 * time.Millisecond)

				// Send current state to client
				if err := engine.renderAndSendIsland(currentSession, instance); err != nil {
					t.Logf("render error: %v", err)
				}
			}
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// First connection
	clientCtx1, clientCancel1 := context.WithTimeout(context.Background(), 10*time.Second)
	defer clientCancel1()

	conn1, _, err := websocket.Dial(clientCtx1, wsURL, nil)
	if err != nil {
		t.Fatalf("first dial failed: %v", err)
	}

	// Read connect event
	_, _, _ = conn1.Read(clientCtx1)

	// Read initial mount patch
	_, _, err = conn1.Read(clientCtx1)
	if err != nil {
		t.Fatalf("read mount patch failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Update state
	updateEvent := Event{
		T:      "update",
		Island: "stateful-1",
		Data:   json.RawMessage(`{"newValue":"updated"}`),
	}
	data, _ := json.Marshal(updateEvent)
	if err := conn1.Write(clientCtx1, websocket.MessageText, data); err != nil {
		t.Fatalf("send update event failed: %v", err)
	}

	// Read patch after update
	_, _, err = conn1.Read(clientCtx1)
	if err != nil {
		t.Fatalf("read update patch failed: %v", err)
	}

	// Verify state was saved
	time.Sleep(10 * time.Millisecond)
	savedState, ok := stateStore.Get(sessionID, "stateful-1")
	if !ok {
		t.Fatal("state not saved before disconnect")
	}
	savedStateMap := savedState.(map[string]any)
	if savedStateMap["value"] != "updated" {
		t.Errorf("expected saved value 'updated', got %v", savedStateMap["value"])
	}

	// Close first connection (simulate disconnect)
	conn1.Close(websocket.StatusNormalClosure, "")
	time.Sleep(50 * time.Millisecond)

	// Second connection (reconnection)
	clientCtx2, clientCancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer clientCancel2()

	conn2, _, err := websocket.Dial(clientCtx2, wsURL, nil)
	if err != nil {
		t.Fatalf("second dial failed: %v", err)
	}
	defer conn2.Close(websocket.StatusNormalClosure, "")

	// Read connect event
	_, _, _ = conn2.Read(clientCtx2)

	// Read state restoration patch
	msgType, patchData, err := conn2.Read(clientCtx2)
	if err != nil {
		t.Fatalf("read restoration patch failed: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Fatalf("expected text message")
	}

	var restorationPatch Event
	if err := json.Unmarshal(patchData, &restorationPatch); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restorationPatch.T != EventPatch {
		t.Errorf("expected patch event, got %s", restorationPatch.T)
	}
	if restorationPatch.Island != "stateful-1" {
		t.Errorf("expected island 'stateful-1', got %s", restorationPatch.Island)
	}

	// Verify the restored render contains "Value: updated"
	var restoredPatches []Patch
	if err := json.Unmarshal(restorationPatch.Data, &restoredPatches); err != nil {
		t.Fatalf("unmarshal patch data failed: %v", err)
	}
	foundValue := false
	for _, patch := range restoredPatches {
		if strings.Contains(patch.HTML, "Value: updated") {
			foundValue = true
			break
		}
	}
	if !foundValue {
		t.Errorf("expected restored render to contain 'Value: updated' in patches: %+v", restoredPatches)
	}

	// Verify the island can still process events after reconnection
	time.Sleep(10 * time.Millisecond)

	updateEvent2 := Event{
		T:      "update",
		Island: "stateful-1",
		Data:   json.RawMessage(`{"newValue":"reconnected"}`),
	}
	data2, _ := json.Marshal(updateEvent2)
	if err := conn2.Write(clientCtx2, websocket.MessageText, data2); err != nil {
		t.Fatalf("send event after reconnect failed: %v", err)
	}

	// Read patch after post-reconnection update
	msgType, patchData, err = conn2.Read(clientCtx2)
	if err != nil {
		t.Fatalf("read post-reconnect patch failed: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Fatalf("expected text message")
	}

	var postReconnectPatch Event
	if err := json.Unmarshal(patchData, &postReconnectPatch); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	var postReconnectPatches []Patch
	if err := json.Unmarshal(postReconnectPatch.Data, &postReconnectPatches); err != nil {
		t.Fatalf("unmarshal patch data failed: %v", err)
	}
	foundReconnected := false
	for _, patch := range postReconnectPatches {
		if strings.Contains(patch.HTML, "Value: reconnected") {
			foundReconnected = true
			break
		}
	}
	if !foundReconnected {
		t.Errorf("expected final render to contain 'Value: reconnected' in patches: %+v", postReconnectPatches)
	}
}

// TestConcurrentIslandOperations tests thread safety with concurrent
// island operations across multiple sessions.
func TestConcurrentIslandOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a simple island
	registry := NewIslandRegistry()
	err := registry.Register("simple", func() (*Island, error) {
		return NewIsland("simple",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"id": props.String("id")}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(map[string]any)
				id := state["id"].(string)
				return bytes.NewReader([]byte(fmt.Sprintf("<div>%s</div>", id))), nil
			}),
		)
	})
	if err != nil {
		t.Fatal(err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Launch multiple goroutines that create sessions and mount islands
	var wg sync.WaitGroup
	numGoroutines := 10
	islandsPerGoroutine := 5

	errors := make(chan error, numGoroutines*islandsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create a mock transport
			transport := newIntegrationMockTransport()
			sessionID := SessionID(fmt.Sprintf("session-%d", id))
			session := NewSession(ctx, sessionID, transport)
			engine.AddSession(session)
			time.Sleep(5 * time.Millisecond)

			// Mount multiple islands
			for j := 0; j < islandsPerGoroutine; j++ {
				islandID := IslandID(fmt.Sprintf("island-%d-%d", id, j))
				_, err := engine.MountIsland(sessionID, islandID, "simple", Props{"id": fmt.Sprintf("%d-%d", id, j)})
				if err != nil {
					errors <- fmt.Errorf("mount error [%d-%d]: %w", id, j, err)
				}
			}

			// Clean up
			engine.DeleteSession(sessionID)
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}

// integrationMockTransport is a mock transport for integration tests
type integrationMockTransport struct {
	events chan Event
	sent   []Event
	mu     sync.Mutex
	closed bool
}

func newIntegrationMockTransport() *integrationMockTransport {
	return &integrationMockTransport{
		events: make(chan Event, 16),
		sent:   []Event{},
	}
}

func (m *integrationMockTransport) Send(e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrTransportClosed
	}

	m.sent = append(m.sent, e)
	return nil
}

func (m *integrationMockTransport) Events() <-chan Event {
	return m.events
}

func (m *integrationMockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		close(m.events)
		m.closed = true
	}
	return nil
}

func (m *integrationMockTransport) GetSent() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Event{}, m.sent...)
}

// engineRoutedTransport wraps a transport and routes incoming events through
// the engine instead of letting the session handle them directly.
// This ensures proper state updates and patch generation in integration tests.
type engineRoutedTransport struct {
	underlying Transport
	engine     *IslandEngine
	sessionID  SessionID
	events     chan Event
	done       chan struct{}
	closed     chan struct{}
	closeOnce  sync.Once
}

func newEngineRoutedTransport(underlying Transport, engine *IslandEngine, sessionID SessionID) *engineRoutedTransport {
	t := &engineRoutedTransport{
		underlying: underlying,
		engine:     engine,
		sessionID:  sessionID,
		events:     make(chan Event, 16),
		done:       make(chan struct{}),
		closed:     make(chan struct{}),
	}

	// Start goroutine to read from underlying transport and route through engine
	go t.routeEvents()

	return t
}

func (t *engineRoutedTransport) routeEvents() {
	defer close(t.events)
	defer close(t.done)

	for {
		select {
		case event, ok := <-t.underlying.Events():
			if !ok {
				return
			}
			// Route through engine which updates state and sends patches
			if err := t.engine.RouteEvent(t.sessionID, event); err != nil {
				// Log error but continue
				_ = err
			}
		case <-t.closed:
			return
		}
	}
}

func (t *engineRoutedTransport) Send(event Event) error {
	return t.underlying.Send(event)
}

func (t *engineRoutedTransport) Events() <-chan Event {
	// Return our own channel which will never receive events
	// (events are routed through engine instead)
	return t.events
}

func (t *engineRoutedTransport) Close() error {
	t.closeOnce.Do(func() {
		close(t.closed)
	})
	return t.underlying.Close()
}
