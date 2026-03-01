package live

import (
	"context"
	"net/http"
	"time"
)

// Transport defines the interface for bidirectional communication between
// the server and a client. Implementations include WebSocket, SSE, and polling.
//
// A Transport is responsible for:
// - Sending events to the client via Send()
// - Receiving events from the client via the Events() channel
// - Lifecycle management via Close()
//
// Transport implementations must be safe for concurrent use.
type Transport interface {
	// Send transmits an event to the client.
	// Returns an error if the transport is closed or the send fails.
	Send(Event) error

	// Events returns a receive-only channel of events from the client.
	// The channel is closed when the transport is closed.
	// Consumers should read from this channel until it's closed.
	Events() <-chan Event

	// Close terminates the transport connection and releases resources.
	// After Close is called:
	// - The Events() channel will be closed
	// - Send() will return an error
	// - All goroutines should be cleaned up
	//
	// Close must be safe to call multiple times.
	Close() error
}

// TransportFactory creates Transport instances from HTTP requests.
// Different factories handle different protocols (WebSocket, SSE, polling).
type TransportFactory interface {
	// Upgrade converts an HTTP request/response pair into a Transport.
	// This typically involves protocol negotiation and connection upgrade.
	//
	// The context can be used to cancel the upgrade process or manage
	// the transport's lifecycle.
	//
	// Returns an error if the upgrade fails (e.g., unsupported protocol,
	// handshake failure).
	Upgrade(ctx context.Context, w http.ResponseWriter, r *http.Request) (Transport, error)
}

// TransportConfig holds configuration options for transport connections.
type TransportConfig struct {
	// WriteTimeout is the maximum duration for writing a message.
	// Zero means no timeout.
	WriteTimeout time.Duration

	// ReadTimeout is the maximum duration for reading a message.
	// Zero means no timeout.
	ReadTimeout time.Duration

	// PingInterval is how often to send ping messages to keep the
	// connection alive. Zero disables pings.
	PingInterval time.Duration

	// PongTimeout is how long to wait for a pong response before
	// considering the connection dead.
	PongTimeout time.Duration

	// MaxMessageSize is the maximum size in bytes for incoming messages.
	// Zero means no limit.
	MaxMessageSize int64

	// EventBufferSize is the size of the Events() channel buffer.
	// A larger buffer reduces backpressure but increases memory usage.
	// Zero uses a default buffer size.
	EventBufferSize int
}

// DefaultTransportConfig returns sensible default transport configuration.
func DefaultTransportConfig() TransportConfig {
	return TransportConfig{
		WriteTimeout:    10 * time.Second,
		ReadTimeout:     60 * time.Second,
		PingInterval:    30 * time.Second,
		PongTimeout:     10 * time.Second,
		MaxMessageSize:  1024 * 1024, // 1MB
		EventBufferSize: 256,
	}
}

// ConnectionMetadata contains information about a transport connection.
type ConnectionMetadata struct {
	// SessionID uniquely identifies the session for this connection.
	SessionID SessionID

	// RemoteAddr is the client's remote address.
	RemoteAddr string

	// UserAgent is the client's user agent string.
	UserAgent string

	// Protocol is the transport protocol being used (e.g., "websocket", "sse").
	Protocol string

	// ConnectedAt is when the connection was established.
	ConnectedAt time.Time

	// Headers contains relevant HTTP headers from the upgrade request.
	Headers http.Header
}
