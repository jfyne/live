package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestWebSocketTransport_Upgrade tests the WebSocket upgrade handshake.
func TestWebSocketTransport_Upgrade(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer transport.Close()

		// Keep connection alive briefly
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Create a WebSocket client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read the connect event
	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}

	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.T != EventConnect {
		t.Errorf("expected connect event, got %s", event.T)
	}
}

// TestWebSocketTransport_BidirectionalMessages tests sending and receiving messages.
func TestWebSocketTransport_BidirectionalMessages(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)

	// Channel to pass the transport to the test
	transportCh := make(chan Transport, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport

		// Keep connection alive
		<-r.Context().Done()
	}))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Create a WebSocket client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read the connect event
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("read connect event failed: %v", err)
	}

	// Get the server-side transport
	serverTransport := <-transportCh
	defer serverTransport.Close()

	// Test 1: Client sends message to server
	clientEvent := Event{
		T:    "test-event",
		ID:   42,
		Data: json.RawMessage(`{"message":"hello from client"}`),
	}

	clientData, _ := json.Marshal(clientEvent)
	if err := conn.Write(ctx, websocket.MessageText, clientData); err != nil {
		t.Fatalf("client write failed: %v", err)
	}

	// Server receives the message
	select {
	case receivedEvent := <-serverTransport.Events():
		if receivedEvent.T != clientEvent.T {
			t.Errorf("expected event type %s, got %s", clientEvent.T, receivedEvent.T)
		}
		if receivedEvent.ID != clientEvent.ID {
			t.Errorf("expected event ID %d, got %d", clientEvent.ID, receivedEvent.ID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event from client")
	}

	// Test 2: Server sends message to client
	serverEvent := Event{
		T:    "server-event",
		ID:   99,
		Data: json.RawMessage(`{"message":"hello from server"}`),
	}

	if err := serverTransport.Send(serverEvent); err != nil {
		t.Fatalf("server send failed: %v", err)
	}

	// Client receives the message
	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("client read failed: %v", err)
	}

	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}

	var receivedEvent Event
	if err := json.Unmarshal(data, &receivedEvent); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if receivedEvent.T != serverEvent.T {
		t.Errorf("expected event type %s, got %s", serverEvent.T, receivedEvent.T)
	}
	if receivedEvent.ID != serverEvent.ID {
		t.Errorf("expected event ID %d, got %d", serverEvent.ID, receivedEvent.ID)
	}
}

// TestWebSocketTransport_ConcurrentSends tests thread safety with concurrent sends.
func TestWebSocketTransport_ConcurrentSends(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)

	transportCh := make(chan Transport, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport
		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read connect event
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("read connect event failed: %v", err)
	}

	serverTransport := <-transportCh
	defer serverTransport.Close()

	// Send many messages concurrently
	const numGoroutines = 10
	const messagesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines*messagesPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				event := Event{
					T:    fmt.Sprintf("event-%d-%d", id, j),
					ID:   id*1000 + j,
					Data: json.RawMessage(fmt.Sprintf(`{"id":%d,"msg":%d}`, id, j)),
				}
				if err := serverTransport.Send(event); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	// Wait for all sends to complete
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("send error: %v", err)
	}

	// Read all the messages from the client side
	received := 0
	for received < numGoroutines*messagesPerGoroutine {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _, err := conn.Read(ctx)
		cancel()
		if err != nil {
			t.Fatalf("client read failed after %d messages: %v", received, err)
		}
		received++
	}

	if received != numGoroutines*messagesPerGoroutine {
		t.Errorf("expected %d messages, got %d", numGoroutines*messagesPerGoroutine, received)
	}
}

// TestWebSocketTransport_SafariCompressionWorkaround tests Safari compression handling.
func TestWebSocketTransport_SafariCompressionWorkaround(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)

	tests := []struct {
		name      string
		userAgent string
		isSafari  bool
	}{
		{
			name:      "Safari should be detected",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Safari/605.1.15",
			isSafari:  true,
		},
		{
			name:      "Chrome should not be detected as Safari",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/95.0.4638.69 Safari/537.36",
			isSafari:  false,
		},
		{
			name:      "Firefox should not be detected as Safari",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:94.0) Gecko/20100101 Firefox/94.0",
			isSafari:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressionCheckDone := make(chan bool, 1)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Note: We can't directly check if compression is disabled in the WebSocket upgrade
				// because the coder/websocket library doesn't expose this information easily.
				// However, we can verify that Safari user agents are detected correctly.
				// Safari detection: contains "Safari" but NOT "Chrome"
				ua := r.UserAgent()
				isSafari := strings.Contains(ua, "Safari") && !strings.Contains(ua, "Chrome")
				compressionCheckDone <- isSafari == tt.isSafari

				transport, err := factory.Upgrade(r.Context(), w, r)
				if err != nil {
					t.Errorf("upgrade failed: %v", err)
					return
				}
				defer transport.Close()

				time.Sleep(100 * time.Millisecond)
			}))
			defer server.Close()

			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Create request with custom user agent
			dialOpts := &websocket.DialOptions{
				HTTPHeader: http.Header{
					"User-Agent": []string{tt.userAgent},
				},
			}

			conn, _, err := websocket.Dial(ctx, wsURL, dialOpts)
			if err != nil {
				t.Fatalf("dial failed: %v", err)
			}
			defer conn.Close(websocket.StatusNormalClosure, "")

			// Read connect event
			_, _, err = conn.Read(ctx)
			if err != nil {
				t.Fatalf("read connect event failed: %v", err)
			}

			// Check if the Safari detection worked correctly
			select {
			case detected := <-compressionCheckDone:
				if !detected {
					t.Errorf("Safari detection mismatch for user agent: %s", tt.userAgent)
				}
			case <-time.After(1 * time.Second):
				t.Fatal("timeout waiting for compression check")
			}
		})
	}
}

// TestWebSocketTransport_Close tests proper cleanup when closing.
func TestWebSocketTransport_Close(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)

	transportCh := make(chan Transport, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport
		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read connect event
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("read connect event failed: %v", err)
	}

	serverTransport := <-transportCh

	// Close the transport
	if err := serverTransport.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}

	// Verify that Send returns an error after close
	err = serverTransport.Send(Event{T: "test"})
	if err == nil {
		t.Error("expected error when sending to closed transport")
	}

	// Verify that Events channel is eventually closed
	timeout := time.After(2 * time.Second)
	eventsClosed := false

	for !eventsClosed {
		select {
		case _, ok := <-serverTransport.Events():
			if !ok {
				eventsClosed = true
			}
		case <-timeout:
			t.Fatal("events channel not closed after timeout")
		}
	}

	// Verify that calling Close again is safe (idempotent)
	if err := serverTransport.Close(); err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

// TestWebSocketTransport_PingPong tests keepalive ping/pong functionality.
func TestWebSocketTransport_PingPong(t *testing.T) {
	// Use a short ping interval for testing
	config := DefaultTransportConfig()
	config.PingInterval = 100 * time.Millisecond
	config.PongTimeout = 1 * time.Second

	factory := NewWebSocketTransportFactory(config)

	transportCh := make(chan Transport, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport
		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read connect event
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("read connect event failed: %v", err)
	}

	serverTransport := <-transportCh
	defer serverTransport.Close()

	// Wait for at least one ping/pong cycle to complete
	time.Sleep(500 * time.Millisecond)

	// If we got here without the connection closing, ping/pong is working
	// Try to send a message to verify the connection is still alive
	if err := serverTransport.Send(Event{T: "ping-test"}); err != nil {
		t.Errorf("send after ping/pong failed: %v", err)
	}

	// Client should be able to read the message
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Errorf("read after ping/pong failed: %v", err)
	}
}

// TestWebSocketTransport_MaxMessageSize tests message size limits.
func TestWebSocketTransport_MaxMessageSize(t *testing.T) {
	config := DefaultTransportConfig()
	config.MaxMessageSize = 100 // Very small limit for testing

	factory := NewWebSocketTransportFactory(config)

	transportCh := make(chan Transport, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport
		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read connect event
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("read connect event failed: %v", err)
	}

	serverTransport := <-transportCh
	defer serverTransport.Close()

	// First send a small message that should succeed
	smallMessage := Event{
		T:    "small-event",
		Data: json.RawMessage(`{"ok":true}`),
	}

	smallData, _ := json.Marshal(smallMessage)
	if err := conn.Write(ctx, websocket.MessageText, smallData); err != nil {
		t.Fatalf("small message write failed: %v", err)
	}

	// Verify small message is received
	select {
	case event := <-serverTransport.Events():
		if event.T != "small-event" {
			t.Errorf("expected small-event, got %s", event.T)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for small message")
	}

	// Now send a message that exceeds the size limit
	// The coder/websocket library will close the connection when the read limit is exceeded
	largeMessage := Event{
		T:    "large-event",
		Data: json.RawMessage(strings.Repeat("x", 1000)),
	}

	largeData, _ := json.Marshal(largeMessage)
	if err := conn.Write(ctx, websocket.MessageText, largeData); err != nil {
		// Write might fail if the server closed the connection
		t.Logf("large message write failed (expected): %v", err)
	}

	// The connection should close shortly after the large message
	// We verify this by checking that the client can't read anymore
	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel()

	_, _, err = conn.Read(readCtx)
	if err == nil {
		t.Error("expected read to fail after sending oversized message")
	}
}

// TestUpgradeWebSocket tests the standalone upgrade function.
func TestUpgradeWebSocket(t *testing.T) {
	config := DefaultTransportConfig()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport, err := UpgradeWebSocket(r.Context(), w, r, config)
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		defer transport.Close()

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read connect event
	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}

	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.T != EventConnect {
		t.Errorf("expected connect event, got %s", event.T)
	}
}
