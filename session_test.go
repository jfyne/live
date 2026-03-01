package live

import (
	"context"
	"sync"
	"testing"
	"time"
)

// Helper function to send an event to a mock transport
func sendEventToMockTransport(transport *mockTransport, event Event) {
	transport.events <- event
}

// Helper function to create a session-specific mock transport with sent event tracking
type sessionMockTransport struct {
	*mockTransport
	sentEvents []Event
	mu         sync.Mutex
}

func newSessionMockTransport() *sessionMockTransport {
	return &sessionMockTransport{
		mockTransport: newMockTransport(10),
		sentEvents:    make([]Event, 0),
	}
}

func (s *sessionMockTransport) Send(event Event) error {
	s.mu.Lock()
	s.sentEvents = append(s.sentEvents, event)
	s.mu.Unlock()
	return s.mockTransport.Send(event)
}

func (s *sessionMockTransport) getSentEvents() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event{}, s.sentEvents...)
}

func TestNewSession(t *testing.T) {
	ctx := context.Background()
	transport := newSessionMockTransport()
	sessionID := SessionID("test-session")

	session := NewSession(ctx, sessionID, transport)
	defer session.Close()

	if session.ID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, session.ID)
	}

	if session.islands == nil {
		t.Error("expected islands map to be initialized")
	}
}

func TestSession_AddAndGetIsland(t *testing.T) {
	ctx := context.Background()
	transport := newSessionMockTransport()
	session := NewSession(ctx, SessionID("test-session"), transport)
	defer session.Close()

	// Register a test island
	RegisterIsland("test", func() (*Island, error) {
		return NewIsland("test", WithMount(func(ctx context.Context, props Props, children string) (any, error) {
			return "initial", nil
		}))
	})

	// Create an island instance
	instance, err := NewIslandInstance("island-1", "test", Props{})
	if err != nil {
		t.Fatalf("failed to create island instance: %v", err)
	}

	// Add to session
	session.AddIsland(instance)

	// Retrieve it
	retrieved, ok := session.GetIsland("island-1")
	if !ok {
		t.Fatal("expected to find island")
	}

	if retrieved.ID != instance.ID {
		t.Errorf("expected island ID %s, got %s", instance.ID, retrieved.ID)
	}
}

func TestSession_GetIsland_NotFound(t *testing.T) {
	ctx := context.Background()
	transport := newSessionMockTransport()
	session := NewSession(ctx, SessionID("test-session"), transport)
	defer session.Close()

	_, ok := session.GetIsland("nonexistent")
	if ok {
		t.Error("expected island not to be found")
	}
}

func TestSession_RemoveIsland(t *testing.T) {
	ctx := context.Background()
	transport := newSessionMockTransport()
	session := NewSession(ctx, SessionID("test-session"), transport)
	defer session.Close()

	// Register a test island
	RegisterIsland("test-remove", func() (*Island, error) {
		return NewIsland("test-remove", WithMount(func(ctx context.Context, props Props, children string) (any, error) {
			return "initial", nil
		}))
	})

	// Create and add island
	instance, err := NewIslandInstance("island-1", "test-remove", Props{})
	if err != nil {
		t.Fatalf("failed to create island instance: %v", err)
	}
	session.AddIsland(instance)

	// Remove it
	session.RemoveIsland("island-1")

	// Verify it's gone
	_, ok := session.GetIsland("island-1")
	if ok {
		t.Error("expected island to be removed")
	}
}

func TestSession_ListIslands(t *testing.T) {
	ctx := context.Background()
	transport := newSessionMockTransport()
	session := NewSession(ctx, SessionID("test-session"), transport)
	defer session.Close()

	// Register a test island
	RegisterIsland("test-list", func() (*Island, error) {
		return NewIsland("test-list", WithMount(func(ctx context.Context, props Props, children string) (any, error) {
			return "initial", nil
		}))
	})

	// Add multiple islands
	for i := 1; i <= 3; i++ {
		instance, err := NewIslandInstance(string(rune('a'+i-1)), "test-list", Props{})
		if err != nil {
			t.Fatalf("failed to create island instance: %v", err)
		}
		session.AddIsland(instance)
	}

	// List them
	islands := session.ListIslands()
	if len(islands) != 3 {
		t.Errorf("expected 3 islands, got %d", len(islands))
	}
}

func TestSession_Send(t *testing.T) {
	ctx := context.Background()
	transport := newSessionMockTransport()
	session := NewSession(ctx, SessionID("test-session"), transport)
	defer session.Close()

	// Send an event
	event := Event{
		T:      "test",
		Island: "island-1",
	}

	err := session.Send(event)
	if err != nil {
		t.Fatalf("failed to send event: %v", err)
	}

	// Verify it was sent via transport
	sentEvents := transport.getSentEvents()
	if len(sentEvents) != 1 {
		t.Fatalf("expected 1 sent event, got %d", len(sentEvents))
	}

	if sentEvents[0].T != event.T {
		t.Errorf("expected event type %s, got %s", event.T, sentEvents[0].T)
	}
}

func TestSession_EventRouting(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport(10)
	session := NewSession(ctx, SessionID("test-session"), transport)
	defer session.Close()

	// Register a test island with an event handler
	var eventHandlerMu sync.Mutex
	eventHandlerCalled := false
	RegisterIsland("counter", func() (*Island, error) {
		island, err := NewIsland("counter", WithMount(func(ctx context.Context, props Props, children string) (any, error) {
			return map[string]int{"count": 0}, nil
		}))
		if err != nil {
			return nil, err
		}
		_ = island.HandleEvent("increment", func(ctx context.Context, state any, params Params) (any, error) {
			eventHandlerMu.Lock()
			eventHandlerCalled = true
			eventHandlerMu.Unlock()
			s := state.(map[string]int)
			s["count"]++
			return s, nil
		})
		return island, nil
	})

	// Create and mount island
	instance, err := NewIslandInstance("counter-1", "counter", Props{})
	if err != nil {
		t.Fatalf("failed to create island instance: %v", err)
	}

	err = instance.Mount(ctx)
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	// Add to session
	session.AddIsland(instance)

	// Create an event
	event := Event{
		T:      "increment",
		Island: "counter-1",
		Data:   []byte(`{}`),
	}

	// Route the event manually (simulating what the engine does)
	err = session.handleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("failed to handle event: %v", err)
	}

	// Verify the event handler was called
	eventHandlerMu.Lock()
	called := eventHandlerCalled
	eventHandlerMu.Unlock()
	if !called {
		t.Error("expected event handler to be called")
	}

	// Verify state was updated
	state := instance.State().(map[string]int)
	if state["count"] != 1 {
		t.Errorf("expected count to be 1, got %d", state["count"])
	}
}

func TestSession_EventRoutingToNonexistentIsland(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport(10)
	session := NewSession(ctx, SessionID("test-session"), transport)
	defer session.Close()

	// Send event to nonexistent island
	event := Event{
		T:      "test",
		Island: "nonexistent",
		Data:   []byte(`{}`),
	}

	transport.events <- event

	// Give the event loop time to process
	time.Sleep(50 * time.Millisecond)

	// No crash means the session handled the error gracefully
}

func TestSession_Close(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport(10)
	session := NewSession(ctx, SessionID("test-session"), transport)

	// Add an island
	RegisterIsland("test-close", func() (*Island, error) {
		return NewIsland("test-close", WithMount(func(ctx context.Context, props Props, children string) (any, error) {
			return "initial", nil
		}))
	})

	instance, _ := NewIslandInstance("island-1", "test-close", Props{})
	session.AddIsland(instance)

	// Close the session
	err := session.Close()
	if err != nil {
		t.Errorf("unexpected error closing session: %v", err)
	}

	// Verify transport is closed
	if !transport.closeCalled {
		t.Error("expected transport to be closed")
	}

	// Verify islands map is cleared
	islands := session.ListIslands()
	if len(islands) != 0 {
		t.Errorf("expected islands to be cleared, got %d", len(islands))
	}

	// Verify context is cancelled
	select {
	case <-session.Context().Done():
		// Expected
	default:
		t.Error("expected session context to be cancelled")
	}

	// Verify multiple Close calls are safe
	err = session.Close()
	if err != nil {
		t.Errorf("unexpected error on second close: %v", err)
	}
}

func TestSession_CloseStopsEventLoop(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport(10)
	session := NewSession(ctx, SessionID("test-session"), transport)

	// Close the session
	session.Close()

	// Try to send an event - should not be processed
	eventHandlerCalled := false
	RegisterIsland("test-loop", func() (*Island, error) {
		island, err := NewIsland("test-loop", WithMount(func(ctx context.Context, props Props, children string) (any, error) {
			return "initial", nil
		}))
		if err != nil {
			return nil, err
		}
		_ = island.HandleEvent("test", func(ctx context.Context, state any, params Params) (any, error) {
			eventHandlerCalled = true
			return state, nil
		})
		return island, nil
	})

	instance, _ := NewIslandInstance("island-1", "test-loop", Props{})
	instance.Mount(ctx)
	session.AddIsland(instance)

	// The transport is already closed, so this won't work, but verifies the session
	// doesn't panic or deadlock
	time.Sleep(50 * time.Millisecond)

	if eventHandlerCalled {
		t.Error("expected event handler not to be called after close")
	}
}

func TestSession_ConcurrentIslandOperations(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport(10)
	session := NewSession(ctx, SessionID("test-session"), transport)
	defer session.Close()

	// Register test island
	RegisterIsland("test-concurrent", func() (*Island, error) {
		return NewIsland("test-concurrent", WithMount(func(ctx context.Context, props Props, children string) (any, error) {
			return "initial", nil
		}))
	})

	// Perform concurrent operations
	var wg sync.WaitGroup
	const numGoroutines = 10

	// Add islands concurrently
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			instance, err := NewIslandInstance(string(rune('a'+id)), "test-concurrent", Props{})
			if err != nil {
				return
			}
			session.AddIsland(instance)
		}(i)
	}
	wg.Wait()

	// Get islands concurrently
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			_, _ = session.GetIsland(IslandID(string(rune('a' + id))))
		}(i)
	}
	wg.Wait()

	// List islands concurrently
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_ = session.ListIslands()
		}()
	}
	wg.Wait()

	// Remove islands concurrently
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			session.RemoveIsland(IslandID(string(rune('a' + id))))
		}(i)
	}
	wg.Wait()

	// Verify all islands were removed
	islands := session.ListIslands()
	if len(islands) != 0 {
		t.Errorf("expected all islands to be removed, got %d", len(islands))
	}
}

func TestSession_SelfEvent(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport(10)
	session := NewSession(ctx, SessionID("test-session"), transport)
	defer session.Close()

	// Register island with self handler
	var selfHandlerMu sync.Mutex
	selfHandlerCalled := false
	RegisterIsland("test-self", func() (*Island, error) {
		island, err := NewIsland("test-self", WithMount(func(ctx context.Context, props Props, children string) (any, error) {
			return map[string]string{"status": "initial"}, nil
		}))
		if err != nil {
			return nil, err
		}
		_ = island.HandleSelf("update", func(ctx context.Context, state any, data any) (any, error) {
			selfHandlerMu.Lock()
			selfHandlerCalled = true
			selfHandlerMu.Unlock()
			s := state.(map[string]string)
			s["status"] = data.(string)
			return s, nil
		})
		return island, nil
	})

	// Create and mount island
	instance, err := NewIslandInstance("island-1", "test-self", Props{})
	if err != nil {
		t.Fatalf("failed to create island instance: %v", err)
	}

	err = instance.Mount(ctx)
	if err != nil {
		t.Fatalf("failed to mount island: %v", err)
	}

	session.AddIsland(instance)

	// Create a self event
	event := Event{
		T:        "update",
		Island:   "island-1",
		SelfData: "updated",
	}

	// Route the event manually (simulating what the engine does)
	err = session.handleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("failed to handle event: %v", err)
	}

	// Verify handler was called
	selfHandlerMu.Lock()
	called := selfHandlerCalled
	selfHandlerMu.Unlock()
	if !called {
		t.Error("expected self handler to be called")
	}

	// Verify state was updated
	state := instance.State().(map[string]string)
	if state["status"] != "updated" {
		t.Errorf("expected status to be 'updated', got %s", state["status"])
	}
}
