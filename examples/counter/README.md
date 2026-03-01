# Live v2 Counter Example

This example demonstrates the v2 islands architecture for the Live framework.

## Features Demonstrated

- **Islands Architecture**: Each `<live-island>` element is an independent component
- **Props Passing**: `data-initial-value` attribute sets the starting count
- **State Isolation**: Each counter maintains its own isolated state
- **Shared Transport**: All islands share a single WebSocket connection
- **Event Handling**: Click events are routed to the correct island instance

## Running the Example

```bash
cd examples/counter
go run main.go
```

Then visit http://localhost:8080 in your browser.

## Files

- `main.go` - Go server implementing the counter island and transport endpoints
- `index.html` - Main HTML page with multiple counter islands
- `counter.html` - Template for rendering counter island state
- `custom-island.js` - Client-side JavaScript for island hydration (temporary implementation)

## How It Works

### Server Side

1. **Island Definition** (`NewCounterIsland()`):
   - Defines mount handler that extracts `initial-value` prop
   - Defines render handler that uses `counter.html` template
   - Registers event handlers for `inc` and `dec` events

2. **Island Registration**:
   - Island is registered with `live.RegisterIsland("counter", NewCounterIsland)`

3. **Transport Setup**:
   - WebSocket endpoint at `/ws`
   - SSE endpoint at `/sse` with POST handler at `/sse/post`

4. **Event Handling**:
   - `subscribe` events trigger `engine.MountIsland()` to create island instances
   - `inc`/`dec` events are routed via `engine.RouteEvent()` to update state
   - After each state change, island is re-rendered and patch sent to client

### Client Side

The `custom-island.js` file provides a temporary client implementation that:

1. Defines a `<live-island>` custom element
2. Extracts props from element attributes
3. Sends `subscribe` message with props when WebSocket connects
4. Handles `patch` messages to update island HTML
5. Sends event messages when buttons are clicked

Note: This is a simplified client for demonstration. The full v2 client (auto.js)
will handle this automatically once the client-server protocol is finalized.

## Testing Checklist

- [ ] Load example in browser (http://localhost:8080)
- [ ] Verify all three counters show correct initial values (0, 5, 10)
- [ ] Click increment on Counter 1, verify it increases
- [ ] Click decrement on Counter 1, verify it decreases
- [ ] Click increment on Counter 2, verify only Counter 2 changes
- [ ] Click increment on Counter 3, verify only Counter 3 changes
- [ ] Verify counters maintain independent state
- [ ] Open browser dev tools console, verify WebSocket connection
- [ ] Check for JavaScript errors in console

## Known Issues

The current implementation uses a simplified custom client (`custom-island.js`) because
the v2 client library's island subscription protocol needs to include props with the
subscribe message. This will be addressed in a future update to the v2 client library.
