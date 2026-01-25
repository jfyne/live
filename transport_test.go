package live

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// mockTransport is a test implementation of the Transport interface.
type mockTransport struct {
	events      chan Event
	sendErr     error
	closeErr    error
	closeCalled bool
	mu          sync.Mutex
}

// newMockTransport creates a new mock transport with a buffered event channel.
func newMockTransport(bufferSize int) *mockTransport {
	return &mockTransport{
		events: make(chan Event, bufferSize),
	}
}

func (m *mockTransport) Send(e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closeCalled {
		return ErrTransportClosed
	}
	if m.sendErr != nil {
		return m.sendErr
	}

	// In a real transport, this would send to the client.
	// For testing, we just verify the method contract.
	return nil
}

func (m *mockTransport) Events() <-chan Event {
	return m.events
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closeCalled {
		return nil // Close is idempotent
	}

	m.closeCalled = true
	close(m.events)

	if m.closeErr != nil {
		return m.closeErr
	}
	return nil
}

// mockTransportFactory creates mock transports for testing.
type mockTransportFactory struct {
	transport   *mockTransport
	upgradeErr  error
	upgradeFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request) (Transport, error)
}

func (f *mockTransportFactory) Upgrade(ctx context.Context, w http.ResponseWriter, r *http.Request) (Transport, error) {
	if f.upgradeFunc != nil {
		return f.upgradeFunc(ctx, w, r)
	}
	if f.upgradeErr != nil {
		return nil, f.upgradeErr
	}
	return f.transport, nil
}

// TestTransportSendReceiveContract verifies the basic send/receive contract.
func TestTransportSendReceiveContract(t *testing.T) {
	transport := newMockTransport(10)
	defer transport.Close()

	// Test sending
	event := Event{
		T:      "test",
		Island: "island-1",
		Data:   json.RawMessage(`{"key":"value"}`),
	}

	err := transport.Send(event)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Test receiving via channel
	go func() {
		receiveEvent := Event{
			T:      "click",
			Island: "island-1",
			Data:   json.RawMessage(`{"button":"submit"}`),
		}
		transport.events <- receiveEvent
	}()

	select {
	case received := <-transport.Events():
		if received.T != "click" {
			t.Errorf("Expected event type 'click', got '%s'", received.T)
		}
		if received.Island != "island-1" {
			t.Errorf("Expected island 'island-1', got '%s'", received.Island)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for event")
	}
}

// TestTransportCloseCleanup verifies that Close properly cleans up resources.
func TestTransportCloseCleanup(t *testing.T) {
	transport := newMockTransport(10)

	// Close the transport
	err := transport.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify channel is closed
	select {
	case _, ok := <-transport.Events():
		if ok {
			t.Error("Events channel should be closed after Close()")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Events channel not closed")
	}

	// Verify Send returns error after close
	err = transport.Send(Event{T: "test"})
	if err != ErrTransportClosed {
		t.Errorf("Expected ErrTransportClosed after Close(), got: %v", err)
	}

	// Verify Close is idempotent
	err = transport.Close()
	if err != nil {
		t.Errorf("Second Close() should not return error, got: %v", err)
	}
}

// TestTransportContextCancellation verifies proper handling of context cancellation.
func TestTransportContextCancellation(t *testing.T) {
	transport := newMockTransport(10)
	defer transport.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Simulate a goroutine reading from the transport with context
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-ctx.Done():
				done <- true
				return
			case _, ok := <-transport.Events():
				if !ok {
					done <- true
					return
				}
			}
		}
	}()

	// Cancel the context
	cancel()

	// Verify goroutine exits
	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Goroutine did not exit after context cancellation")
	}
}

// TestTransportConcurrentSends verifies thread safety of Send.
func TestTransportConcurrentSends(t *testing.T) {
	transport := newMockTransport(10)
	defer transport.Close()

	const numGoroutines = 100
	const sendsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines*sendsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < sendsPerGoroutine; j++ {
				event := Event{
					T:      "concurrent",
					Island: "test",
					ID:     id*sendsPerGoroutine + j,
				}
				if err := transport.Send(event); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent send failed: %v", err)
	}
}

// TestTransportFactoryUpgrade tests the TransportFactory interface.
func TestTransportFactoryUpgrade(t *testing.T) {
	mockTransport := newMockTransport(10)
	factory := &mockTransportFactory{
		transport: mockTransport,
	}

	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()
	ctx := context.Background()

	transport, err := factory.Upgrade(ctx, w, req)
	if err != nil {
		t.Fatalf("Upgrade failed: %v", err)
	}

	if transport != mockTransport {
		t.Error("Expected factory to return mock transport")
	}
}

// TestTransportFactoryUpgradeError tests error handling in factory.
func TestTransportFactoryUpgradeError(t *testing.T) {
	expectedErr := ErrNoEventHandler
	factory := &mockTransportFactory{
		upgradeErr: expectedErr,
	}

	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()
	ctx := context.Background()

	_, err := factory.Upgrade(ctx, w, req)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// TestTransportEventBuffering tests that events are properly buffered.
func TestTransportEventBuffering(t *testing.T) {
	bufferSize := 5
	transport := newMockTransport(bufferSize)
	defer transport.Close()

	// Fill the buffer
	for i := 0; i < bufferSize; i++ {
		transport.events <- Event{T: "buffered", ID: i}
	}

	// Read all buffered events
	for i := 0; i < bufferSize; i++ {
		select {
		case event := <-transport.Events():
			if event.ID != i {
				t.Errorf("Expected event ID %d, got %d", i, event.ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Timeout reading buffered event %d", i)
		}
	}

	// Verify no more events
	select {
	case <-transport.Events():
		t.Error("Unexpected event in buffer")
	case <-time.After(10 * time.Millisecond):
		// Expected - buffer is empty
	}
}

// TestDefaultTransportConfig verifies the default configuration is sensible.
func TestDefaultTransportConfig(t *testing.T) {
	config := DefaultTransportConfig()

	if config.WriteTimeout == 0 {
		t.Error("WriteTimeout should have a default value")
	}
	if config.ReadTimeout == 0 {
		t.Error("ReadTimeout should have a default value")
	}
	if config.PingInterval == 0 {
		t.Error("PingInterval should have a default value")
	}
	if config.PongTimeout == 0 {
		t.Error("PongTimeout should have a default value")
	}
	if config.MaxMessageSize == 0 {
		t.Error("MaxMessageSize should have a default value")
	}
	if config.EventBufferSize == 0 {
		t.Error("EventBufferSize should have a default value")
	}

	// Verify sensible relationships
	if config.PongTimeout >= config.ReadTimeout {
		t.Error("PongTimeout should be less than ReadTimeout")
	}
	if config.PingInterval >= config.ReadTimeout {
		t.Error("PingInterval should be less than ReadTimeout")
	}
}

// TestConnectionMetadata verifies the metadata structure.
func TestConnectionMetadata(t *testing.T) {
	now := time.Now()
	headers := http.Header{}
	headers.Set("User-Agent", "TestAgent/1.0")

	meta := ConnectionMetadata{
		SessionID:   SessionID("session-123"),
		RemoteAddr:  "192.168.1.1:12345",
		UserAgent:   "TestAgent/1.0",
		Protocol:    "websocket",
		ConnectedAt: now,
		Headers:     headers,
	}

	if meta.SessionID != "session-123" {
		t.Errorf("Expected SessionID 'session-123', got '%s'", meta.SessionID)
	}
	if meta.Protocol != "websocket" {
		t.Errorf("Expected Protocol 'websocket', got '%s'", meta.Protocol)
	}
	if meta.ConnectedAt != now {
		t.Error("ConnectedAt should match the provided time")
	}
	if meta.Headers.Get("User-Agent") != "TestAgent/1.0" {
		t.Error("Headers not preserved correctly")
	}
}

// TestTransportMultipleReaders tests that multiple goroutines can safely
// read from the Events channel (though typically there's only one reader).
func TestTransportMultipleReaders(t *testing.T) {
	transport := newMockTransport(100)
	defer transport.Close()

	const numReaders = 5
	const numEvents = 50

	received := make([][]Event, numReaders)
	for i := range received {
		received[i] = make([]Event, 0, numEvents)
	}

	var wg sync.WaitGroup
	wg.Add(numReaders)

	// Start multiple readers
	for i := 0; i < numReaders; i++ {
		go func(readerID int) {
			defer wg.Done()
			for event := range transport.Events() {
				received[readerID] = append(received[readerID], event)
			}
		}(i)
	}

	// Send events
	go func() {
		for i := 0; i < numEvents; i++ {
			transport.events <- Event{T: "multi", ID: i}
		}
		transport.Close()
	}()

	wg.Wait()

	// Verify all events were received (across all readers)
	totalReceived := 0
	for i := 0; i < numReaders; i++ {
		totalReceived += len(received[i])
	}

	if totalReceived != numEvents {
		t.Errorf("Expected %d total events received, got %d", numEvents, totalReceived)
	}
}

// TestTransportFactoryContextCancellation tests upgrade cancellation.
func TestTransportFactoryContextCancellation(t *testing.T) {
	factory := &mockTransportFactory{
		upgradeFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request) (Transport, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return newMockTransport(10), nil
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()

	_, err := factory.Upgrade(ctx, w, req)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}
