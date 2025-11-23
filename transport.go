package live

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/coder/websocket"
)

// Transport represents a connection transport (WebSocket, SSE, etc.)
type Transport interface {
	// ReadMessage reads a message from the client.
	ReadMessage(ctx context.Context) (Event, error)
	// WriteMessage writes a message to the client.
	WriteMessage(ctx context.Context, msg Event) error
	// Close closes the transport.
	Close(ctx context.Context, reason string) error
}

// WebSocketTransport implements Transport using a WebSocket connection.
type WebSocketTransport struct {
	Conn           *websocket.Conn
	MaxMessageSize int64
}

func (t *WebSocketTransport) ReadMessage(ctx context.Context) (Event, error) {
	for {
		typ, data, err := t.Conn.Read(ctx)
		if err != nil {
			return Event{}, err
		}
		if typ == websocket.MessageBinary {
			slog.Warn("binary messages unhandled")
			continue
		}
		var msg Event
		if err := json.Unmarshal(data, &msg); err != nil {
			return Event{}, err
		}
		return msg, nil
	}
}

func (t *WebSocketTransport) WriteMessage(ctx context.Context, msg Event) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	data, err := json.Marshal(&msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return t.Conn.Write(ctx, websocket.MessageText, data)
}

func (t *WebSocketTransport) Close(ctx context.Context, reason string) error {
	return t.Conn.Close(websocket.StatusPolicyViolation, reason)
}
