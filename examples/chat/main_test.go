package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// mockTransport is a transport implementation for chat example testing.
// It tracks sent events and provides a channel for injecting inbound events.
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
// Regression tests (must PASS with existing framework before chat is created)
// ---------------------------------------------------------------------------

// TestFramework_NewIsland_Chat verifies that the live package NewIsland
// constructor works correctly — a basic regression of core framework behaviour.
func TestFramework_NewIsland_Chat(t *testing.T) {
	island, err := live.NewIsland("chat-test",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return map[string]any{"messages": []string{}}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewIsland failed: %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}
}

// TestFramework_MountIsland_Chat verifies that the engine mounts an island,
// calls the mount handler, and returns a valid instance — regression for
// the engine mount path.
func TestFramework_MountIsland_Chat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("basic-chat", func() (*live.Island, error) {
		return live.NewIsland("basic-chat",
			live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
				return map[string]any{"ready": true}, nil
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

	instance, err := engine.MountIsland("session-reg", "basic-chat-1", "basic-chat", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}
	state := instance.State().(map[string]any)
	if state["ready"] != true {
		t.Errorf("expected state ready=true, got %v", state["ready"])
	}
}

// TestFramework_HandleSelf_Chat verifies that HandleSelf and GetSelfHandler
// work correctly — regression for self handler registration.
func TestFramework_HandleSelf_Chat(t *testing.T) {
	island, err := live.NewIsland("selftest")
	if err != nil {
		t.Fatalf("NewIsland failed: %v", err)
	}

	handlerCalled := false
	err = island.HandleSelf("myevent", func(ctx context.Context, state any, data any) (any, error) {
		handlerCalled = true
		return state, nil
	})
	if err != nil {
		t.Fatalf("HandleSelf failed: %v", err)
	}

	handler, err := island.GetSelfHandler("myevent")
	if err != nil {
		t.Fatalf("GetSelfHandler failed: %v", err)
	}

	_, _ = handler(context.Background(), nil, nil)
	if !handlerCalled {
		t.Error("expected self handler to be called")
	}
}

// ---------------------------------------------------------------------------
// New feature tests (RED — fail to compile until chat/main.go is created)
// ---------------------------------------------------------------------------

// TestChatIsland_NewMessage verifies that calling the "newmessage" self handler
// with a Message struct results in state.Messages containing that message.
//
// Scenario: Send a message within same server
func TestChatIsland_NewMessage(t *testing.T) {
	island, err := NewChatIsland()
	if err != nil {
		t.Fatalf("NewChatIsland() failed: %v", err)
	}

	// Get the "newmessage" self handler
	newMessageHandler, err := island.GetSelfHandler("newmessage")
	if err != nil {
		t.Fatalf("GetSelfHandler('newmessage') failed: %v", err)
	}

	// Set up initial ChatState (as would be returned by mount)
	initialState := &ChatState{
		Messages: []Message{},
	}

	msg := Message{
		ID:   "msg-1",
		User: "alice",
		Msg:  "Hello",
	}

	ctx := context.Background()
	newState, err := newMessageHandler(ctx, initialState, msg)
	if err != nil {
		t.Fatalf("newmessage handler returned error: %v", err)
	}

	chatState, ok := newState.(*ChatState)
	if !ok {
		t.Fatalf("expected *ChatState from newmessage handler, got %T", newState)
	}

	if len(chatState.Messages) != 1 {
		t.Fatalf("expected 1 message in state, got %d", len(chatState.Messages))
	}

	got := chatState.Messages[0]
	if got.ID != "msg-1" {
		t.Errorf("expected message ID 'msg-1', got %q", got.ID)
	}
	if got.User != "alice" {
		t.Errorf("expected message user 'alice', got %q", got.User)
	}
	if got.Msg != "Hello" {
		t.Errorf("expected message text 'Hello', got %q", got.Msg)
	}
}

// TestChatIsland_NewMessageAppend verifies that calling "newmessage" twice
// results in state.Messages containing only the latest single message.
// This is intentional for live-update="append" mode: the handler sets
// state.Messages = []Message{msg} so the re-rendered HTML contains only
// the new message, which the diff engine appends to the existing DOM.
//
// Scenario: Messages append without replacing
func TestChatIsland_NewMessageAppend(t *testing.T) {
	island, err := NewChatIsland()
	if err != nil {
		t.Fatalf("NewChatIsland() failed: %v", err)
	}

	newMessageHandler, err := island.GetSelfHandler("newmessage")
	if err != nil {
		t.Fatalf("GetSelfHandler('newmessage') failed: %v", err)
	}

	ctx := context.Background()

	// First call
	initialState := &ChatState{
		Messages: []Message{},
	}

	msg1 := Message{
		ID:   "msg-1",
		User: "alice",
		Msg:  "Hello",
	}

	state1, err := newMessageHandler(ctx, initialState, msg1)
	if err != nil {
		t.Fatalf("first newmessage handler call failed: %v", err)
	}

	chatState1, ok := state1.(*ChatState)
	if !ok {
		t.Fatalf("expected *ChatState after first call, got %T", state1)
	}

	// Second call — simulates a second broadcast arriving
	msg2 := Message{
		ID:   "msg-2",
		User: "bob",
		Msg:  "World",
	}

	state2, err := newMessageHandler(ctx, chatState1, msg2)
	if err != nil {
		t.Fatalf("second newmessage handler call failed: %v", err)
	}

	chatState2, ok := state2.(*ChatState)
	if !ok {
		t.Fatalf("expected *ChatState after second call, got %T", state2)
	}

	// For live-update="append" mode the handler stores only the latest message
	// so the re-rendered fragment contains just that message (which gets appended
	// to existing DOM). State should have exactly one message — the latest.
	if len(chatState2.Messages) != 1 {
		t.Fatalf("expected 1 message after second newmessage call (append mode), got %d", len(chatState2.Messages))
	}

	got := chatState2.Messages[0]
	if got.ID != "msg-2" {
		t.Errorf("expected latest message ID 'msg-2', got %q", got.ID)
	}
	if got.User != "bob" {
		t.Errorf("expected latest message user 'bob', got %q", got.User)
	}
	if got.Msg != "World" {
		t.Errorf("expected latest message text 'World', got %q", got.Msg)
	}
}

// TestChatIsland_Mount verifies that mounting the chat island with
// Props{"user": "session-1"} results in an initial state that contains
// a welcome message.
//
// Scenario: Send a message within same server (setup: island mounts with user info)
func TestChatIsland_Mount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewChatIsland()
	if err != nil {
		t.Fatalf("NewChatIsland() failed: %v", err)
	}

	// Call the mount handler directly with user prop
	state, err := island.Mount(ctx, live.Props{
		"user": "session-1",
	}, "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	chatState, ok := state.(*ChatState)
	if !ok {
		t.Fatalf("expected *ChatState from mount, got %T", state)
	}

	// The mount handler should return a welcome message
	if len(chatState.Messages) == 0 {
		t.Fatal("expected at least one message (welcome message) after mount, got 0")
	}
}
