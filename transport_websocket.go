package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// WebSocketTransport implements Transport interface using WebSocket protocol.
// It provides bidirectional communication with automatic ping/pong keepalive.
//
// Thread-safety: All public methods are safe for concurrent use.
type WebSocketTransport struct {
	conn   *websocket.Conn
	config TransportConfig
	meta   ConnectionMetadata

	// Channel for incoming events from the client
	events chan Event

	// Context and cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Synchronization
	closeOnce sync.Once
	closed    chan struct{}

	// Write mutex protects concurrent writes to the WebSocket
	writeMu sync.Mutex
}

// NewWebSocketTransport creates a new WebSocket transport from an established connection.
func NewWebSocketTransport(ctx context.Context, conn *websocket.Conn, config TransportConfig, meta ConnectionMetadata) *WebSocketTransport {
	transportCtx, cancel := context.WithCancel(ctx)

	bufferSize := config.EventBufferSize
	if bufferSize == 0 {
		bufferSize = DefaultTransportConfig().EventBufferSize
	}

	t := &WebSocketTransport{
		conn:   conn,
		config: config,
		meta:   meta,
		events: make(chan Event, bufferSize),
		ctx:    transportCtx,
		cancel: cancel,
		closed: make(chan struct{}),
	}

	// Set read limit if configured
	if config.MaxMessageSize > 0 {
		conn.SetReadLimit(config.MaxMessageSize)
	}

	// Start the read pump to receive messages from client
	go t.readPump()

	// Start the ping/pong keepalive if configured
	if config.PingInterval > 0 {
		go t.pingPump()
	}

	return t
}

// Send transmits an event to the client over the WebSocket connection.
// This method is thread-safe and can be called concurrently.
func (t *WebSocketTransport) Send(event Event) error {
	select {
	case <-t.closed:
		return fmt.Errorf("transport closed")
	default:
	}

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	ctx := t.ctx
	if t.config.WriteTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.config.WriteTimeout)
		defer cancel()
	}

	data, err := json.Marshal(&event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if err := t.conn.Write(ctx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	return nil
}

// Events returns a receive-only channel of events from the client.
// The channel is closed when the transport is closed.
func (t *WebSocketTransport) Events() <-chan Event {
	return t.events
}

// Close terminates the WebSocket connection and cleans up resources.
// It is safe to call Close multiple times.
func (t *WebSocketTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		// Cancel the context to signal all goroutines to stop
		t.cancel()

		// Close the closed channel to signal that we're shutting down
		close(t.closed)

		// Close the WebSocket connection with normal closure status
		closeErr := t.conn.Close(websocket.StatusNormalClosure, "")
		if closeErr != nil {
			err = fmt.Errorf("close websocket: %w", closeErr)
		}

		// Note: t.events is closed by readPump via defer to avoid race conditions
	})
	return err
}

// readPump continuously reads messages from the WebSocket connection
// and forwards them to the events channel.
func (t *WebSocketTransport) readPump() {
	defer close(t.events)
	defer func() {
		// Ensure we close the transport if the read loop exits
		t.Close()
	}()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.closed:
			return
		default:
		}

		ctx := t.ctx
		var cancel context.CancelFunc
		if t.config.ReadTimeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, t.config.ReadTimeout)
		}

		msgType, data, err := t.conn.Read(ctx)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			// Check if this is a normal closure
			if errors.Is(err, context.Canceled) {
				return
			}

			closeStatus := websocket.CloseStatus(err)
			switch closeStatus {
			case websocket.StatusNormalClosure, websocket.StatusGoingAway:
				// Normal closure, exit cleanly
				return
			case -1:
				// No close status, likely a network error
				slog.Debug("websocket read error", "err", err)
				return
			default:
				// Abnormal closure
				slog.Error("websocket closed abnormally",
					"status", closeStatus,
					"err", err)
				return
			}
		}

		switch msgType {
		case websocket.MessageText:
			var event Event
			if err := json.Unmarshal(data, &event); err != nil {
				slog.Error("failed to unmarshal event",
					"err", err,
					"data", string(data))
				continue
			}

			// Send the event to the events channel
			select {
			case t.events <- event:
				// Event sent successfully
			case <-t.ctx.Done():
				return
			case <-t.closed:
				return
			default:
				slog.Error("websocket read pump: events channel full, event dropped", "event_t", event.T)
			}

		case websocket.MessageBinary:
			slog.Warn("binary messages not supported, ignoring")

		default:
			slog.Warn("unknown message type", "type", msgType)
		}
	}
}

// pingPump sends periodic ping messages to keep the connection alive
// and detect dead connections.
func (t *WebSocketTransport) pingPump() {
	ticker := time.NewTicker(t.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx := t.ctx
			var cancel context.CancelFunc
			if t.config.PongTimeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, t.config.PongTimeout)
			}

			err := t.conn.Ping(ctx)
			if cancel != nil {
				cancel()
			}

			if err != nil {
				slog.Debug("ping failed, closing connection", "err", err)
				t.Close()
				return
			}

		case <-t.ctx.Done():
			return
		case <-t.closed:
			return
		}
	}
}

// WebSocketTransportFactory creates WebSocket transports by upgrading HTTP connections.
type WebSocketTransportFactory struct {
	config        TransportConfig
	acceptOptions *websocket.AcceptOptions
}

// NewWebSocketTransportFactory creates a new WebSocket transport factory with the given configuration.
func NewWebSocketTransportFactory(config TransportConfig) *WebSocketTransportFactory {
	return &WebSocketTransportFactory{
		config:        config,
		acceptOptions: &websocket.AcceptOptions{},
	}
}

// Upgrade converts an HTTP request into a WebSocket transport.
// It handles the WebSocket upgrade handshake and Safari compression workaround.
func (f *WebSocketTransportFactory) Upgrade(ctx context.Context, w http.ResponseWriter, r *http.Request) (Transport, error) {
	// Safari has issues with WebSocket compression, so we disable it
	// for Safari user agents to ensure compatibility.
	// Note: Chrome's user agent also contains "Safari" for compatibility,
	// so we need to check that it's NOT Chrome.
	acceptOpts := f.acceptOptions
	ua := r.UserAgent()
	if strings.Contains(ua, "Safari") && !strings.Contains(ua, "Chrome") {
		opts := *f.acceptOptions // Copy to avoid mutation
		opts.CompressionMode = websocket.CompressionDisabled
		acceptOpts = &opts
	}

	// Perform the WebSocket upgrade handshake
	conn, err := websocket.Accept(w, r, acceptOpts)
	if err != nil {
		return nil, fmt.Errorf("websocket upgrade failed: %w", err)
	}

	// Build connection metadata
	meta := ConnectionMetadata{
		RemoteAddr:  r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Protocol:    "websocket",
		ConnectedAt: time.Now(),
		Headers:     r.Header,
	}

	// Create the transport
	transport := NewWebSocketTransport(ctx, conn, f.config, meta)

	// Send the initial connect event to the client
	if err := transport.Send(Event{T: EventConnect}); err != nil {
		conn.Close(websocket.StatusInternalError, "failed to send connect event")
		return nil, fmt.Errorf("send connect event: %w", err)
	}

	return transport, nil
}
