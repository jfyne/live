package live

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestWebSocketHandlerReturnsHandler verifies that WebSocketHandler returns a
// valid http.HandlerFunc that can serve HTTP traffic without panicking.
func TestWebSocketHandlerReturnsHandler(t *testing.T) {
	config := DefaultTransportConfig()
	handler := WebSocketHandler(config)

	if handler == nil {
		t.Fatal("WebSocketHandler returned nil")
	}

	// Verify that the returned value implements http.Handler.
	var _ http.Handler = handler
}

// TestWebSocketHandlerRespondsToRequest verifies that the WebSocketHandler
// responds to a real WebSocket upgrade request without panicking.
// The handler will return a 400 on a plain HTTP request (not a WS upgrade).
func TestWebSocketHandlerRespondsToRequest(t *testing.T) {
	config := DefaultTransportConfig()
	handler := WebSocketHandler(config)

	server := httptest.NewServer(handler)
	defer server.Close()

	// A plain GET request (not a WS upgrade) should result in a 4xx response,
	// not a panic.
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error making request: %v", err)
	}
	defer resp.Body.Close()

	// The handler should return a bad-request (400) for a non-WebSocket request.
	if resp.StatusCode == http.StatusInternalServerError {
		t.Errorf("expected non-500 response for plain GET, got %d", resp.StatusCode)
	}
}

// TestWebSocketHandlerWithFactoryReturnsHandler verifies that
// WebSocketHandlerWithFactory returns a valid http.HandlerFunc.
func TestWebSocketHandlerWithFactoryReturnsHandler(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)
	handler := WebSocketHandlerWithFactory(factory)

	if handler == nil {
		t.Fatal("WebSocketHandlerWithFactory returned nil")
	}

	var _ http.Handler = handler
}

// TestWebSocketHandlerWithFactoryServesWebSocket verifies that the handler
// produced by WebSocketHandlerWithFactory successfully upgrades a WebSocket
// connection.
func TestWebSocketHandlerWithFactoryServesWebSocket(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewWebSocketTransportFactory(config)
	handler := WebSocketHandlerWithFactory(factory)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")
}

// TestSSEHandlerReturnsHandler verifies that SSEHandler returns a valid
// http.HandlerFunc that can be registered with a mux.
func TestSSEHandlerReturnsHandler(t *testing.T) {
	config := DefaultTransportConfig()
	handler := SSEHandler(config)

	if handler == nil {
		t.Fatal("SSEHandler returned nil")
	}

	var _ http.Handler = handler
}

// TestSSEHandlerRespondsToRequest verifies that SSEHandler does not panic when
// it receives a plain HTTP GET (non-SSE) request.
func TestSSEHandlerRespondsToRequest(t *testing.T) {
	config := DefaultTransportConfig()
	handler := SSEHandler(config)

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusInternalServerError {
		t.Errorf("expected non-500 response, got %d", resp.StatusCode)
	}
}

// TestSSEHandlerWithFactoryReturnsTwoHandlers verifies that
// SSEHandlerWithFactory returns exactly two non-nil handlers: the SSE stream
// handler and the POST handler.
func TestSSEHandlerWithFactoryReturnsTwoHandlers(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewSSETransportFactory(config)

	sseHandler, postHandler := SSEHandlerWithFactory(factory)

	if sseHandler == nil {
		t.Error("SSEHandlerWithFactory returned nil SSE handler")
	}
	if postHandler == nil {
		t.Error("SSEHandlerWithFactory returned nil POST handler")
	}

	// Both values should implement http.Handler.
	var _ http.Handler = sseHandler
	var _ http.Handler = postHandler
}

// TestSSEHandlerWithFactoryPostHandlerResponds verifies that the POST handler
// returned by SSEHandlerWithFactory can handle an HTTP request without panicking.
func TestSSEHandlerWithFactoryPostHandlerResponds(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewSSETransportFactory(config)

	_, postHandler := SSEHandlerWithFactory(factory)

	server := httptest.NewServer(postHandler)
	defer server.Close()

	resp, err := http.Post(server.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Any response other than a 5xx is acceptable; the handler should not panic.
	if resp.StatusCode >= http.StatusInternalServerError {
		t.Errorf("expected non-5xx response from POST handler, got %d", resp.StatusCode)
	}
}
