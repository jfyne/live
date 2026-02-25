package live

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// registerChatIsland registers a "chat" island type with a self-handler for
// "new-message" that appends the message string to the messages slice in state.
// This helper is shared by multiple broadcast tests.
func registerChatIsland(t *testing.T, registry *IslandRegistry) {
	t.Helper()
	err := registry.Register("chat", func() (*Island, error) {
		island, err := NewIsland("chat",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"messages": []string{}}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>chat</div>"), nil
			}),
		)
		if err != nil {
			return nil, err
		}
		island.HandleSelf("new-message", func(ctx context.Context, state any, data any) (any, error) {
			stateMap := state.(map[string]any)
			messages := stateMap["messages"].([]string)
			stateMap["messages"] = append(messages, data.(string))
			return stateMap, nil
		})
		return island, nil
	})
	if err != nil {
		t.Fatalf("failed to register chat island: %v", err)
	}
}

// setupEngineWithChatIsland creates a new engine with a "chat" island mounted
// in a single session. Returns the engine, session, and transport for assertions.
func setupEngineWithChatIsland(t *testing.T, ctx context.Context, sessionID SessionID, islandID IslandID) (*IslandEngine, *Session, *engineMockTransport) {
	t.Helper()

	registry := NewIslandRegistry()
	registerChatIsland(t, registry)

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	t.Cleanup(func() { engine.Close() })

	transport := newEngineMockTransport()
	session := NewSession(ctx, sessionID, transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err := engine.MountIsland(sessionID, islandID, "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount chat island: %v", err)
	}

	// Clear mounting events before test assertions.
	transport.mu.Lock()
	transport.sent = []Event{}
	transport.mu.Unlock()

	return engine, session, transport
}

// ---------------------------------------------------------------------------
// Regression: existing BroadcastToIslandType continues to work after adding
// the new BroadcastSelfToIslandType. These tests verify the old behaviour is
// preserved and should PASS.
// ---------------------------------------------------------------------------

// TestBroadcastRegression_BroadcastToIslandType verifies that the pre-existing
// BroadcastToIslandType method still sends raw events (not self-events) to all
// islands of the given type. This is a regression guard — it must PASS.
func TestBroadcastRegression_BroadcastToIslandType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	registerChatIsland(t, registry)

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err := engine.MountIsland("session-1", "chat-1", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Clear mount events.
	transport.mu.Lock()
	transport.sent = []Event{}
	transport.mu.Unlock()

	// Broadcast a raw event to all "chat" islands.
	broadcastEvent := Event{
		T:    "update",
		Data: []byte(`{"text":"hello"}`),
	}
	engine.BroadcastToIslandType("chat", broadcastEvent)
	time.Sleep(10 * time.Millisecond)

	sent := transport.GetSent()
	if len(sent) == 0 {
		t.Fatal("expected at least one event to be sent via BroadcastToIslandType")
	}
	if sent[0].T != "update" {
		t.Errorf("expected event type 'update', got %q", sent[0].T)
	}
	if sent[0].Island != "chat-1" {
		t.Errorf("expected island 'chat-1', got %q", sent[0].Island)
	}
}

// ---------------------------------------------------------------------------
// New feature tests (RED state — will fail until broadcast.go is implemented)
// ---------------------------------------------------------------------------

// TestBroadcastSelfToIslandType verifies that calling BroadcastSelfToIslandType
// on the engine iterates all sessions, finds islands of the given type, and
// routes a self-event to each one via RouteEvent with SelfData set. This causes
// the island's self-handler to fire and its state to be updated.
//
// Scenario (from plan):
//
//	Given an engine with two sessions, each containing a "chat" island
//	When BroadcastSelfToIslandType("chat", event) is called
//	Then the self-handler on both island instances is invoked
//	And both islands are re-rendered with updated state
func TestBroadcastSelfToIslandType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	registerChatIsland(t, registry)

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Set up two sessions, each with a "chat" island.
	transport1 := newEngineMockTransport()
	session1 := NewSession(ctx, "session-1", transport1)
	engine.AddSession(session1)

	transport2 := newEngineMockTransport()
	session2 := NewSession(ctx, "session-2", transport2)
	engine.AddSession(session2)

	time.Sleep(10 * time.Millisecond)

	instance1, err := engine.MountIsland("session-1", "chat-1", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island in session 1: %v", err)
	}

	instance2, err := engine.MountIsland("session-2", "chat-2", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island in session 2: %v", err)
	}

	// Clear mount events.
	transport1.mu.Lock()
	transport1.sent = []Event{}
	transport1.mu.Unlock()
	transport2.mu.Lock()
	transport2.sent = []Event{}
	transport2.mu.Unlock()

	// Call the new method: BroadcastSelfToIslandType routes a self-event to
	// all matching island instances across all sessions.
	selfEvent := Event{
		T:        "new-message",
		SelfData: "hello world",
	}
	engine.BroadcastSelfToIslandType("chat", selfEvent)

	// Give any async work time to complete.
	time.Sleep(20 * time.Millisecond)

	// Verify instance1's self-handler fired and state was updated.
	state1 := instance1.State().(map[string]any)
	messages1 := state1["messages"].([]string)
	if len(messages1) != 1 {
		t.Errorf("session-1 chat island: expected 1 message, got %d", len(messages1))
	} else if messages1[0] != "hello world" {
		t.Errorf("session-1 chat island: expected message 'hello world', got %q", messages1[0])
	}

	// Verify instance2's self-handler fired and state was updated.
	state2 := instance2.State().(map[string]any)
	messages2 := state2["messages"].([]string)
	if len(messages2) != 1 {
		t.Errorf("session-2 chat island: expected 1 message, got %d", len(messages2))
	} else if messages2[0] != "hello world" {
		t.Errorf("session-2 chat island: expected message 'hello world', got %q", messages2[0])
	}

	// Verify patch events were sent to both transports (re-render after self-event).
	sent1 := transport1.GetSent()
	hasPatch1 := false
	for _, e := range sent1 {
		if e.T == EventPatch {
			hasPatch1 = true
			break
		}
	}
	if !hasPatch1 {
		t.Error("expected a patch event to be sent to session-1 transport after BroadcastSelfToIslandType")
	}

	sent2 := transport2.GetSent()
	hasPatch2 := false
	for _, e := range sent2 {
		if e.T == EventPatch {
			hasPatch2 = true
			break
		}
	}
	if !hasPatch2 {
		t.Error("expected a patch event to be sent to session-2 transport after BroadcastSelfToIslandType")
	}
}

// TestBroadcast_SubscribeAndReceive verifies that subscribing an engine to a
// topic and then calling Receive directly routes a self-event to the matching
// islands in that engine.
//
// This tests the Broadcast struct's Receive method in isolation without going
// through the transport layer.
func TestBroadcast_SubscribeAndReceive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a LocalTransport (even though we call Receive directly, the Broadcast
	// still needs a transport to be constructed).
	lt := NewLocalTransport()
	b := NewBroadcast(ctx, lt)

	// Set up an engine with a "chat" island.
	registry := NewIslandRegistry()
	registerChatIsland(t, registry)

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-1", "chat-1", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Subscribe the engine to the "chat" topic for island type "chat".
	b.Subscribe("chat", "chat", engine)

	// Call Receive directly to simulate a message arriving from the transport.
	msg := Event{
		T:        "new-message",
		SelfData: "subscribe-test message",
	}
	b.Receive("chat", msg)

	// Give async work time to complete.
	time.Sleep(20 * time.Millisecond)

	// Verify the self-handler on the island instance was invoked.
	state := instance.State().(map[string]any)
	messages := state["messages"].([]string)
	if len(messages) != 1 {
		t.Errorf("expected 1 message after Receive, got %d", len(messages))
	} else if messages[0] != "subscribe-test message" {
		t.Errorf("expected message 'subscribe-test message', got %q", messages[0])
	}
}

// TestBroadcast_PublishThroughLocalTransport verifies the end-to-end flow:
// a message published via Broadcast.Publish is delivered through LocalTransport
// to the subscribed engine, and the island's self-handler fires.
//
// Scenario (from plan):
//
//	Given a Broadcast with LocalTransport
//	And engine A subscribed to topic "chat" for island type "chat"
//	And engine A has a session with a "chat" island mounted
//	When a message is published to topic "chat"
//	Then the "chat" island's self-handler receives the message
//	And the island state is updated and re-rendered
func TestBroadcast_PublishThroughLocalTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lt := NewLocalTransport()
	b := NewBroadcast(ctx, lt)

	// Set up engine with a mounted "chat" island.
	registry := NewIslandRegistry()
	registerChatIsland(t, registry)

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-1", "chat-1", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Clear mount events.
	transport.mu.Lock()
	transport.sent = []Event{}
	transport.mu.Unlock()

	// Subscribe engine to the topic.
	b.Subscribe("chat", "chat", engine)

	// Publish a message via Broadcast.
	publishedMsg := Event{
		T:        "new-message",
		SelfData: "broadcasted message",
	}
	if err := b.Publish(ctx, "chat", publishedMsg); err != nil {
		t.Fatalf("Publish returned an error: %v", err)
	}

	// The LocalTransport delivers asynchronously; give it time.
	time.Sleep(50 * time.Millisecond)

	// Verify the island's self-handler was invoked.
	state := instance.State().(map[string]any)
	messages := state["messages"].([]string)
	if len(messages) != 1 {
		t.Errorf("expected 1 message after Publish, got %d", len(messages))
	} else if messages[0] != "broadcasted message" {
		t.Errorf("expected message 'broadcasted message', got %q", messages[0])
	}

	// Verify a patch event was sent (island re-rendered).
	sent := transport.GetSent()
	hasPatch := false
	for _, e := range sent {
		if e.T == EventPatch {
			hasPatch = true
			break
		}
	}
	if !hasPatch {
		t.Error("expected a patch event after Publish — island should be re-rendered")
	}
}

// TestBroadcast_MultipleEngines verifies that two different engines both
// receive a broadcast when both are subscribed to the same topic.
//
// Scenario (from plan):
//
//	Given a Broadcast with LocalTransport
//	And engine A and engine B are both subscribed to topic "chat" for island type "chat"
//	When a message is published to topic "chat"
//	Then both engines receive the message
//	And all matching islands in both engines have their self-handlers invoked
func TestBroadcast_MultipleEngines(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lt := NewLocalTransport()
	b := NewBroadcast(ctx, lt)

	// Engine A
	registryA := NewIslandRegistry()
	registerChatIsland(t, registryA)
	storeA := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engineA := NewIslandEngine(ctx, registryA, storeA)
	defer engineA.Close()

	transportA := newEngineMockTransport()
	sessionA := NewSession(ctx, "session-a", transportA)
	engineA.AddSession(sessionA)
	time.Sleep(10 * time.Millisecond)

	instanceA, err := engineA.MountIsland("session-a", "chat-a", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island in engine A: %v", err)
	}

	// Engine B
	registryB := NewIslandRegistry()
	registerChatIsland(t, registryB)
	storeB := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engineB := NewIslandEngine(ctx, registryB, storeB)
	defer engineB.Close()

	transportB := newEngineMockTransport()
	sessionB := NewSession(ctx, "session-b", transportB)
	engineB.AddSession(sessionB)
	time.Sleep(10 * time.Millisecond)

	instanceB, err := engineB.MountIsland("session-b", "chat-b", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island in engine B: %v", err)
	}

	// Subscribe both engines to the same topic.
	b.Subscribe("chat", "chat", engineA)
	b.Subscribe("chat", "chat", engineB)

	// Publish a message.
	publishedMsg := Event{
		T:        "new-message",
		SelfData: "multi-engine message",
	}
	if err := b.Publish(ctx, "chat", publishedMsg); err != nil {
		t.Fatalf("Publish returned an error: %v", err)
	}

	// Allow delivery time.
	time.Sleep(50 * time.Millisecond)

	// Verify engine A's island received the message.
	stateA := instanceA.State().(map[string]any)
	messagesA := stateA["messages"].([]string)
	if len(messagesA) != 1 {
		t.Errorf("engine A chat island: expected 1 message, got %d", len(messagesA))
	} else if messagesA[0] != "multi-engine message" {
		t.Errorf("engine A chat island: expected 'multi-engine message', got %q", messagesA[0])
	}

	// Verify engine B's island received the message.
	stateB := instanceB.State().(map[string]any)
	messagesB := stateB["messages"].([]string)
	if len(messagesB) != 1 {
		t.Errorf("engine B chat island: expected 1 message, got %d", len(messagesB))
	} else if messagesB[0] != "multi-engine message" {
		t.Errorf("engine B chat island: expected 'multi-engine message', got %q", messagesB[0])
	}
}

// TestLocalTransport_RoundTrip verifies that a message published via
// LocalTransport.Publish is received by the LocalTransport's Listen goroutine
// and ultimately delivered to a subscribed engine's island self-handler.
//
// This is the core transport-layer round-trip test: Publish → channel → Listen
// → Broadcast.Receive → BroadcastSelfToIslandType → island self-handler.
func TestLocalTransport_RoundTrip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lt := NewLocalTransport()

	// Set up a real engine with a mounted "chat" island.
	registry := NewIslandRegistry()
	registerChatIsland(t, registry)

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-1", "chat-1", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// NewBroadcast starts lt.Listen in a background goroutine.
	b := NewBroadcast(ctx, lt)
	b.Subscribe("chat", "chat", engine)

	// Publish a message directly on the LocalTransport.
	msg := Event{
		T:        "new-message",
		SelfData: "round-trip message",
	}
	if err := lt.Publish(ctx, "chat", msg); err != nil {
		t.Fatalf("LocalTransport.Publish failed: %v", err)
	}

	// Allow the Listen goroutine to process the message and route it.
	time.Sleep(50 * time.Millisecond)

	// Verify the self-handler on the island was invoked.
	state := instance.State().(map[string]any)
	messages := state["messages"].([]string)
	if len(messages) != 1 {
		t.Errorf("expected 1 message after LocalTransport round-trip, got %d", len(messages))
	} else if messages[0] != "round-trip message" {
		t.Errorf("expected message 'round-trip message', got %q", messages[0])
	}
}

// TestBroadcastSelfToIslandType_NonMatchingType verifies that
// BroadcastSelfToIslandType does NOT invoke self-handlers on islands of a
// different type. This is a correctness guard for the new engine method.
func TestBroadcastSelfToIslandType_NonMatchingType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := NewIslandRegistry()
	registerChatIsland(t, registry)

	// Also register a "counter" island with a self-handler.
	counterSelfCalled := false
	err := registry.Register("counter", func() (*Island, error) {
		island, err := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]any{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>counter</div>"), nil
			}),
		)
		if err != nil {
			return nil, err
		}
		island.HandleSelf("increment", func(ctx context.Context, state any, data any) (any, error) {
			counterSelfCalled = true
			stateMap := state.(map[string]any)
			stateMap["count"] = stateMap["count"].(int) + 1
			return stateMap, nil
		})
		return island, nil
	})
	if err != nil {
		t.Fatalf("failed to register counter island: %v", err)
	}

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newEngineMockTransport()
	session := NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	// Mount a counter island AND a chat island.
	_, err = engine.MountIsland("session-1", "counter-1", "counter", Props{})
	if err != nil {
		t.Fatalf("failed to mount counter island: %v", err)
	}

	chatInstance, err := engine.MountIsland("session-1", "chat-1", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount chat island: %v", err)
	}

	// Broadcast only to "chat" islands.
	selfEvent := Event{
		T:        "new-message",
		SelfData: "only for chat",
	}
	engine.BroadcastSelfToIslandType("chat", selfEvent)

	time.Sleep(20 * time.Millisecond)

	// Chat island should have received the message.
	chatState := chatInstance.State().(map[string]any)
	chatMessages := chatState["messages"].([]string)
	if len(chatMessages) != 1 {
		t.Errorf("chat island: expected 1 message, got %d", len(chatMessages))
	}

	// Counter island should NOT have been called.
	if counterSelfCalled {
		t.Error("counter island self-handler should NOT have been called when broadcasting to 'chat' type")
	}
}

// TestBroadcast_ReceiveCallsBroadcastSelfToIslandType verifies that calling
// Broadcast.Receive directly triggers BroadcastSelfToIslandType on all
// subscribed engines for the given topic.
func TestBroadcast_ReceiveCallsBroadcastSelfToIslandType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lt := NewLocalTransport()
	b := NewBroadcast(ctx, lt)

	// Engine with two sessions, each having a "chat" island.
	registry := NewIslandRegistry()
	registerChatIsland(t, registry)

	stateStore := NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport1 := newEngineMockTransport()
	session1 := NewSession(ctx, "session-1", transport1)
	engine.AddSession(session1)

	transport2 := newEngineMockTransport()
	session2 := NewSession(ctx, "session-2", transport2)
	engine.AddSession(session2)

	time.Sleep(10 * time.Millisecond)

	instance1, err := engine.MountIsland("session-1", "chat-1", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island in session-1: %v", err)
	}

	instance2, err := engine.MountIsland("session-2", "chat-2", "chat", Props{})
	if err != nil {
		t.Fatalf("failed to mount island in session-2: %v", err)
	}

	b.Subscribe("chat", "chat", engine)

	msg := Event{
		T:        "new-message",
		SelfData: "receive test",
	}
	b.Receive("chat", msg)

	time.Sleep(20 * time.Millisecond)

	// Both island instances should have received the message.
	state1 := instance1.State().(map[string]any)
	messages1 := state1["messages"].([]string)
	if len(messages1) != 1 {
		t.Errorf("session-1 island: expected 1 message, got %d", len(messages1))
	} else if messages1[0] != "receive test" {
		t.Errorf("session-1 island: expected 'receive test', got %q", messages1[0])
	}

	state2 := instance2.State().(map[string]any)
	messages2 := state2["messages"].([]string)
	if len(messages2) != 1 {
		t.Errorf("session-2 island: expected 1 message, got %d", len(messages2))
	} else if messages2[0] != "receive test" {
		t.Errorf("session-2 island: expected 'receive test', got %q", messages2[0])
	}
}
