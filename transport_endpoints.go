package live

import (
	"context"
	"net/http"
)

// WebSocketHandler returns an HTTP handler that upgrades connections to WebSocket transports.
// It uses the provided configuration for transport settings.
//
// Example usage:
//
//	config := live.DefaultTransportConfig()
//	http.Handle("/ws", live.WebSocketHandler(config))
func WebSocketHandler(config TransportConfig) http.HandlerFunc {
	factory := NewWebSocketTransportFactory(config)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		transport, err := factory.Upgrade(ctx, w, r)
		if err != nil {
			http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
			return
		}

		// Note: The actual handling of the transport (reading events, sending responses)
		// should be done by the caller or by a session manager. This handler just
		// performs the upgrade and returns the transport.
		//
		// In practice, you would typically pass the transport to a session manager
		// or connection handler. For now, this is a basic example that just upgrades
		// and closes.
		defer transport.Close()

		// Keep the connection alive until the context is done
		<-ctx.Done()
	}
}

// WebSocketHandlerWithFactory returns an HTTP handler that upgrades connections using a custom factory.
// This allows for more control over the upgrade process and configuration.
//
// Example usage:
//
//	factory := live.NewWebSocketTransportFactory(config)
//	http.Handle("/ws", live.WebSocketHandlerWithFactory(factory))
func WebSocketHandlerWithFactory(factory *WebSocketTransportFactory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		transport, err := factory.Upgrade(ctx, w, r)
		if err != nil {
			http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
			return
		}

		defer transport.Close()

		// Keep the connection alive until the context is done
		<-ctx.Done()
	}
}

// UpgradeWebSocket performs a WebSocket upgrade on the given HTTP request/response pair.
// This is a lower-level function that gives you direct access to the Transport.
//
// Unlike the handler functions above, this function does not manage the connection lifecycle.
// The caller is responsible for using the returned Transport and closing it when done.
//
// Example usage:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//		config := live.DefaultTransportConfig()
//		transport, err := live.UpgradeWebSocket(r.Context(), w, r, config)
//		if err != nil {
//			http.Error(w, "upgrade failed", http.StatusBadRequest)
//			return
//		}
//		defer transport.Close()
//
//		// Use the transport...
//		for event := range transport.Events() {
//			// Handle event...
//		}
//	}
func UpgradeWebSocket(ctx context.Context, w http.ResponseWriter, r *http.Request, config TransportConfig) (Transport, error) {
	factory := NewWebSocketTransportFactory(config)
	return factory.Upgrade(ctx, w, r)
}

// SSEHandler returns an HTTP handler that upgrades connections to SSE transports.
// It uses the provided configuration for transport settings.
//
// Note: SSE requires two endpoints - one for the SSE stream (this handler) and one
// for client-to-server POST requests. Use SSEHandlerWithFactory for more control.
//
// Example usage:
//
//	config := live.DefaultTransportConfig()
//	http.Handle("/sse", live.SSEHandler(config))
func SSEHandler(config TransportConfig) http.HandlerFunc {
	factory := NewSSETransportFactory(config)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		transport, err := factory.Upgrade(ctx, w, r)
		if err != nil {
			http.Error(w, "SSE upgrade failed", http.StatusBadRequest)
			return
		}

		// Note: The actual handling of the transport (reading events, sending responses)
		// should be done by the caller or by a session manager. This handler just
		// performs the upgrade and returns the transport.
		//
		// In practice, you would typically pass the transport to a session manager
		// or connection handler. For now, this is a basic example that just upgrades
		// and closes.
		defer transport.Close()

		// Keep the connection alive until the context is done
		<-ctx.Done()
	}
}

// SSEHandlerWithFactory returns SSE stream and POST handlers using a shared factory.
// This ensures both endpoints can coordinate on session management.
//
// Returns two handlers: (sseHandler, postHandler)
//
// Example usage:
//
//	factory := live.NewSSETransportFactory(config)
//	sseHandler, postHandler := live.SSEHandlerWithFactory(factory)
//	http.Handle("/sse", sseHandler)
//	http.Handle("/sse/post", postHandler)
func SSEHandlerWithFactory(factory *SSETransportFactory) (http.HandlerFunc, http.HandlerFunc) {
	sseHandler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		transport, err := factory.Upgrade(ctx, w, r)
		if err != nil {
			http.Error(w, "SSE upgrade failed", http.StatusBadRequest)
			return
		}

		defer transport.Close()

		// Keep the connection alive until the context is done
		<-ctx.Done()
	}

	postHandler := func(w http.ResponseWriter, r *http.Request) {
		factory.HandlePost(w, r)
	}

	return sseHandler, postHandler
}

// UpgradeSSE performs an SSE upgrade on the given HTTP request/response pair.
// This is a lower-level function that gives you direct access to the Transport.
//
// Unlike the handler functions above, this function does not manage the connection lifecycle.
// The caller is responsible for using the returned Transport and closing it when done.
//
// Note: For SSE to work properly, you also need to handle POST requests for client-to-server
// events. See SSETransportFactory.HandlePost for the POST handler.
//
// Example usage:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//		config := live.DefaultTransportConfig()
//		transport, err := live.UpgradeSSE(r.Context(), w, r, config)
//		if err != nil {
//			http.Error(w, "upgrade failed", http.StatusBadRequest)
//			return
//		}
//		defer transport.Close()
//
//		// Use the transport...
//		for event := range transport.Events() {
//			// Handle event...
//		}
//	}
func UpgradeSSE(ctx context.Context, w http.ResponseWriter, r *http.Request, config TransportConfig) (Transport, error) {
	factory := NewSSETransportFactory(config)
	return factory.Upgrade(ctx, w, r)
}
