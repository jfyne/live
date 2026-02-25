package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// mockTransport is a transport implementation for hooks example testing
// that tracks sent events.
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

// TestHooksIsland_ProblemReturnsError verifies that the "problem" event
// handler on HooksIsland returns an error with message "something went wrong".
// Gherkin: Error event received by client hook (server-side: handler returns error)
func TestHooksIsland_ProblemReturnsError(t *testing.T) {
	island, err := NewHooksIsland()
	if err != nil {
		t.Fatalf("NewHooksIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("problem")
	if err != nil {
		t.Fatalf("expected 'problem' event handler to be registered, got error: %v", err)
	}

	ctx := context.Background()
	_, handlerErr := handler(ctx, &HooksState{}, live.Params{})
	if handlerErr == nil {
		t.Fatal("expected 'problem' event handler to return an error, got nil")
	}
	if handlerErr.Error() != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got %q", handlerErr.Error())
	}
}

// TestHooksIsland_ErrorEventSentToTransport verifies that when a "problem"
// event is routed through the engine, the mock transport receives an error
// event with T: "err" and data containing {"err": "something went wrong"}.
// Gherkin: Error event received by client hook (server sends error event to transport)
// Gherkin: Error does not crash the server (multiple problem events work)
func TestHooksIsland_ErrorEventSentToTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register the hooks island with the engine registry
	registry := live.NewIslandRegistry()
	err := registry.Register("hooks", func() (*live.Island, error) {
		return NewHooksIsland()
	})
	if err != nil {
		t.Fatalf("failed to register hooks island: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Create a session with a mock transport
	transport := newMockTransport()
	defer transport.Close()
	session := live.NewSession(ctx, "session-1", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	// Mount the hooks island
	_, err = engine.MountIsland("session-1", "hooks-1", "hooks", live.Props{})
	if err != nil {
		t.Fatalf("failed to mount hooks island: %v", err)
	}

	// Clear sent events from mounting
	transport.mu.Lock()
	transport.sent = []live.Event{}
	transport.mu.Unlock()

	// Route the "problem" event — the engine should return an error
	routeErr := engine.RouteEvent("session-1", live.Event{
		T:      "problem",
		Island: "hooks-1",
		Data:   []byte(`{}`),
	})
	if routeErr == nil {
		t.Fatal("expected RouteEvent to return an error when the 'problem' handler returns an error")
	}

	// Verify the transport received an error event with T == "err"
	sent := transport.GetSent()
	var errorEvent *live.Event
	for i := range sent {
		if sent[i].T == "err" {
			errorEvent = &sent[i]
			break
		}
	}
	if errorEvent == nil {
		t.Fatalf("expected an error event (T: \"err\") to be sent to the transport; got %d events: %+v", len(sent), sent)
	}

	// Verify the error event data contains {"err": "something went wrong"}
	var errData map[string]string
	if jsonErr := json.Unmarshal(errorEvent.Data, &errData); jsonErr != nil {
		t.Fatalf("failed to unmarshal error event data: %v", jsonErr)
	}
	if errData["err"] != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got %q", errData["err"])
	}

	// Verify the server does not crash when multiple "problem" events are sent
	// (Gherkin: Error does not crash the server)
	for i := 0; i < 3; i++ {
		err := engine.RouteEvent("session-1", live.Event{
			T:      "problem",
			Island: "hooks-1",
			Data:   []byte(`{}`),
		})
		if err == nil {
			t.Errorf("expected RouteEvent to return an error on repeated call %d", i+1)
		}
	}

	// Verify each additional call sent another error event
	allSent := transport.GetSent()
	errCount := 0
	for _, e := range allSent {
		if e.T == "err" {
			errCount++
		}
	}
	if errCount < 4 {
		t.Errorf("expected at least 4 error events (1 original + 3 repeated), got %d", errCount)
	}
}
