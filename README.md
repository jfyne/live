# ⚡ live

[![Go Reference](https://pkg.go.dev/badge/github.com/jfyne/live#.svg)](https://pkg.go.dev/github.com/jfyne/live#)

Real-time interactive UI components with server-rendered HTML in Go. Build dynamic web applications using only Go and HTML templates.

Live v2 introduces a pure **islands architecture** where independent, interactive components (islands) share a single transport connection. Each island maintains isolated state and lifecycle, enabling granular interactivity without full-page reloads.

## Version 2 Breaking Changes

**This is version 2 of Live**, a complete rewrite with breaking changes from v1. v2 adopts a pure islands architecture, replacing the full-page LiveView pattern.

### Major Changes from v1

- **Islands-only architecture**: No more full-page `Handler` - use `Island` components instead
- **New API**: `Island`, `IslandEngine`, `Session`, and `Transport` replace v1's `Handler`, `Engine`, and `Socket`
- **Custom elements**: Client uses `<live-island>` custom elements instead of document-wide initialization
- **Transport abstraction**: WebSocket, SSE, or polling (v1 was WebSocket-only)
- **State isolation**: Each island instance has completely isolated state
- **Multiple islands per page**: Share a single connection with message routing

If you're using v1, see the [Migration Guide](#migration-from-v1) below.

## Table of Contents

- [Community](#community)
- [Getting Started](#getting-started)
  - [Installation](#installation)
  - [Quick Example](#quick-example)
- [Core Concepts](#core-concepts)
  - [Islands](#islands)
  - [Props and State](#props-and-state)
  - [Event Handling](#event-handling)
  - [Transport Layer](#transport-layer)
- [Server-Side API](#server-side-api)
  - [Creating an Island](#creating-an-island)
  - [Registering Islands](#registering-islands)
  - [Setting Up the Engine](#setting-up-the-engine)
  - [Transport Endpoints](#transport-endpoints)
- [Client-Side Usage](#client-side-usage)
  - [The `<live-island>` Element](#the-live-island-element)
  - [Passing Props](#passing-props)
  - [Event Attributes](#event-attributes)
- [Examples](#examples)
- [Migration from v1](#migration-from-v1)
- [Advanced Topics](#advanced-topics)
- [API Reference](#api-reference)

## Community

For bugs, please use GitHub issues. For questions about design or adding features, use the discussions tab.

Discord server: [https://discord.gg/TuMNaXJMUG](https://discord.gg/TuMNaXJMUG)

## Getting Started

### Installation

```bash
go get github.com/jfyne/live@v2
```

**Note**: Make sure to use the `v2` tag or the `v2` branch to get the islands architecture.

### Quick Example

Here's a simple counter island to demonstrate the v2 API:

**Server (Go):**

```go
package main

import (
    "bytes"
    "context"
    "html/template"
    "io"
    "log"
    "net/http"
    "time"

    "github.com/jfyne/live"
)

// CounterState holds the state for a counter island
type CounterState struct {
    Count int
}

// NewCounterIsland creates a counter island definition
func NewCounterIsland() (*live.Island, error) {
    island, err := live.NewIsland(
        "counter",
        live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
            // Initialize state from props
            initialValue := props.Int("initial-value")
            return &CounterState{Count: initialValue}, nil
        }),
        live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
            state := rc.State.(*CounterState)

            tmpl := `
                <div>
                    <div class="count">{{.Count}}</div>
                    <button live-click="dec">-</button>
                    <button live-click="inc">+</button>
                </div>
            `

            t, _ := template.New("counter").Parse(tmpl)
            var buf bytes.Buffer
            t.Execute(&buf, state)
            return &buf, nil
        }),
    )
    if err != nil {
        return nil, err
    }

    // Register event handlers
    island.HandleEvent("inc", func(ctx context.Context, state any, params live.Params) (any, error) {
        s := state.(*CounterState)
        s.Count++
        return s, nil
    })

    island.HandleEvent("dec", func(ctx context.Context, state any, params live.Params) (any, error) {
        s := state.(*CounterState)
        s.Count--
        return s, nil
    })

    return island, nil
}

func main() {
    // Register the island
    live.RegisterIsland("counter", NewCounterIsland)

    ctx := context.Background()
    stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
    engine := live.NewIslandEngine(ctx, live.DefaultRegistry(), stateStore)

    // Set up WebSocket endpoint
    wsConfig := live.DefaultTransportConfig()
    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        transport, _ := live.UpgradeWebSocket(r.Context(), w, r, wsConfig)
        sessionID := live.SessionID("session-123")
        session := live.NewSession(r.Context(), sessionID, transport)

        engine.AddSession(session)
        defer engine.DeleteSession(sessionID)

        // Handle events
        for event := range transport.Events() {
            if event.T == "subscribe" {
                params, _ := event.Params()
                islandType := params.String("type")
                props := make(live.Props)
                for k, v := range params {
                    if k != "type" && k != "id" {
                        props[k] = v
                    }
                }
                engine.MountIsland(sessionID, live.IslandID(event.Island), islandType, props)
            } else {
                engine.RouteEvent(sessionID, event)
            }
        }
    })

    // Serve HTML
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(indexHTML))
    })

    log.Println("Server running on :8080")
    http.ListenAndServe(":8080", nil)
}

const indexHTML = `<!DOCTYPE html>
<html>
<head><title>Counter</title></head>
<body>
    <h1>Live v2 Counter</h1>
    <live-island type="counter" id="counter-1" data-initial-value="0">
        <div><div class="count">0</div></div>
    </live-island>
    <script src="/custom-island.js"></script>
</body>
</html>`
```

**Client (HTML):**

```html
<!DOCTYPE html>
<html>
<head>
    <title>Counter Example</title>
</head>
<body>
    <h1>Live v2 Counter</h1>

    <!-- Counter island with initial value of 0 -->
    <live-island type="counter" id="counter-1" data-initial-value="0">
        <div>
            <div class="count">0</div>
            <button live-click="dec">-</button>
            <button live-click="inc">+</button>
        </div>
    </live-island>

    <!-- Counter island with initial value of 5 -->
    <live-island type="counter" id="counter-2" data-initial-value="5">
        <div>
            <div class="count">5</div>
            <button live-click="dec">-</button>
            <button live-click="inc">+</button>
        </div>
    </live-island>

    <!-- Load the live island client library -->
    <script src="/custom-island.js"></script>
</body>
</html>
```

The content inside `<live-island>` is the initial server-rendered HTML, replaced by the server once the island mounts.

## Core Concepts

### Islands

An **island** is an independent, interactive component with its own:
- **State**: Isolated state that persists across events
- **Lifecycle**: Mount, render, unmount handlers
- **Events**: User interactions (clicks, form submissions, etc.)
- **Props**: Initial configuration passed from HTML attributes

Islands are defined once via `NewIsland()` and registered globally. Multiple instances can be created from a single island definition.

### Props and State

**Props** are passed to an island from its HTML attributes and are read-only:

```html
<live-island type="counter" id="counter-1" data-initial-value="10">
```

```go
live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
    initialValue := props.Int("initial-value") // Reads data-initial-value
    return &CounterState{Count: initialValue}, nil
})
```

**State** is the internal data of an island instance, updated by event handlers:

```go
island.HandleEvent("inc", func(ctx context.Context, state any, params live.Params) (any, error) {
    s := state.(*CounterState)
    s.Count++ // Mutate state
    return s, nil // Return updated state
})
```

After an event handler returns, Live re-renders the island with the new state and sends a patch to the client.

### Event Handling

Islands handle user interactions via event handlers registered with `HandleEvent()`:

```go
island.HandleEvent("inc", func(ctx context.Context, state any, params live.Params) (any, error) {
    // Handle the event, update state
    return newState, nil
})
```

Events are triggered by `live-*` attributes in the HTML:

```html
<button live-click="inc">Increment</button>
<form live-submit="save">...</form>
<input live-change="update" />
```

When a user clicks the button, the client sends an event with `t: "inc"` to the server, which routes it to the correct island instance.

### Transport Layer

Live v2 abstracts the transport layer, supporting multiple protocols:

- **WebSocket** (default): Bidirectional, low-latency
- **SSE (Server-Sent Events)**: Server-to-client streaming with HTTP POST for client events
- **Polling** (future): Fallback for restricted environments

The client automatically negotiates the best available transport. All islands on a page share a single connection.

## Server-Side API

### Creating an Island

Use `NewIsland()` with functional options to define an island:

```go
func NewCounterIsland() (*live.Island, error) {
    island, err := live.NewIsland(
        "counter", // Island type name
        live.WithMount(mountHandler),
        live.WithRender(renderHandler),
        live.WithUnmount(unmountHandler),
    )
    if err != nil {
        return nil, err
    }

    // Register event handlers
    island.HandleEvent("inc", incHandler)
    island.HandleEvent("dec", decHandler)

    return island, nil
}
```

**Handler signatures:**

```go
// Called when island is mounted (created)
func mountHandler(ctx context.Context, props live.Props, children string) (any, error) {
    // Return initial state
    return &MyState{}, nil
}

// Called to render the island's current state
func renderHandler(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
    state := rc.State.(*MyState)
    // Render state to HTML
    return reader, nil
}

// Called when island is unmounted (destroyed)
func unmountHandler(ctx context.Context, state any) error {
    // Cleanup resources
    return nil
}

// Called when an event occurs
func eventHandler(ctx context.Context, state any, params live.Params) (any, error) {
    // Update and return state
    return newState, nil
}
```

### Registering Islands

Register islands with the global registry so they can be instantiated by the engine:

```go
func main() {
    err := live.RegisterIsland("counter", NewCounterIsland)
    if err != nil {
        log.Fatal(err)
    }
}
```

The first argument is the island type name (must match the `type` attribute in HTML).

### Setting Up the Engine

Create an `IslandEngine` to manage sessions and island instances:

```go
ctx := context.Background()

// Create a state store for island state persistence
stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)

// Create the engine
registry := live.DefaultRegistry()
engine := live.NewIslandEngine(ctx, registry, stateStore)
defer engine.Close()
```

The engine:
- Manages sessions (one per connected client)
- Routes events to the correct island instance
- Coordinates rendering and patching

### Transport Endpoints

Set up HTTP endpoints for WebSocket or SSE transports.

**WebSocket endpoint:**

```go
http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
    wsConfig := live.DefaultTransportConfig()
    transport, err := live.UpgradeWebSocket(r.Context(), w, r, wsConfig)
    if err != nil {
        http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
        return
    }

    sessionID := live.SessionID("unique-session-id")
    session := live.NewSession(r.Context(), sessionID, transport)
    engine.AddSession(session)
    defer engine.DeleteSession(sessionID)

    // Process events
    for event := range transport.Events() {
        if event.T == "subscribe" && event.Island != "" {
            // Island mount request
            params, _ := event.Params()
            islandType := params.String("type")
            props := extractProps(params)

            engine.MountIsland(
                sessionID,
                live.IslandID(event.Island),
                islandType,
                props,
            )
        } else {
            // Regular event
            engine.RouteEvent(sessionID, event)
        }
    }
})
```

**SSE endpoints:**

```go
sseConfig := live.DefaultTransportConfig()
sseFactory := live.NewSSETransportFactory(sseConfig)

// SSE stream endpoint
http.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
    transport, err := sseFactory.Upgrade(r.Context(), w, r)
    if err != nil {
        http.Error(w, "SSE upgrade failed", http.StatusBadRequest)
        return
    }

    sessionID := live.SessionID("unique-session-id")
    session := live.NewSession(r.Context(), sessionID, transport)
    engine.AddSession(session)
    defer engine.DeleteSession(sessionID)

    // Process events (same as WebSocket)
    for event := range transport.Events() {
        // ... handle events
    }
})

// SSE POST endpoint for client events
http.HandleFunc("/sse/post", sseFactory.HandlePost)
```

## Client-Side Usage

### The `<live-island>` Element

Use the `<live-island>` custom element to define interactive islands in your HTML:

```html
<live-island type="counter" id="counter-1" data-initial-value="0">
    <!-- Initial server-rendered content -->
    <div>Count: 0</div>
</live-island>
```

**Required attributes:**
- `type`: Island type name (must match registered island)
- `id`: Unique identifier for this island instance

**Optional attributes:**
- `data-*`: Props passed to the island's mount handler

### Passing Props

Props are extracted from `data-*` attributes:

```html
<live-island
    type="user-profile"
    id="profile-1"
    data-user-id="123"
    data-editable="true">
</live-island>
```

Access in Go:

```go
live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
    userID := props.String("user-id")    // "123"
    editable := props.Bool("editable")    // true
    return &ProfileState{UserID: userID, Editable: editable}, nil
})
```

### Event Attributes

Wire up event handlers with `live-*` attributes:

**Click events:**
```html
<button live-click="save">Save</button>
<button live-click="delete" live-value-id="123">Delete</button>
```

**Form events:**
```html
<form live-submit="create">
    <input name="title" />
    <button type="submit">Create</button>
</form>

<input live-change="validate" />
```

**Key events:**
```html
<input live-keydown="check" live-key="Enter" />
```

**Focus events:**
```html
<input live-focus="focus-handler" live-blur="blur-handler" />
```

**Rate limiting:**
```html
<input live-change="search" live-debounce="300" />
<button live-click="action" live-throttle="1000">Click</button>
```

**DOM patching control:**
```html
<div live-update="append"><!-- Append patches instead of replace --></div>
<div live-update="ignore"><!-- Never update this element --></div>
```

## Examples

Complete examples are in the `examples/` directory:

- **[Counter](examples/counter/)**: Basic counter demonstrating islands, props, and events

More examples coming soon:
- Chat application with multiple islands
- Form validation and submission
- Real-time collaboration
- Server-pushed updates

## Migration from v1

v2 is a complete rewrite. Here's how to migrate from v1 to v2:

### Conceptual Changes

| v1 Concept | v2 Equivalent | Notes |
|------------|---------------|-------|
| Full-page `Handler` | `Island` components | Islands replace full-page LiveViews |
| `Handler.MountHandler` | `Island.Mount` | Called per island instance, not per page |
| `Handler.RenderHandler` | `Island.Render` | Renders island HTML, not full page |
| `Socket` | `Session` | Transport-agnostic connection |
| `Engine` (1:1 with Handler) | `IslandEngine` | Manages multiple islands |
| `page.Component` | `Island` | Islands replace the component abstraction |
| Document-wide events | Island-scoped events | Events wired within island boundary |
| WebSocket only | `Transport` interface | WebSocket, SSE, or polling |

### Code Migration

**v1 full-page handler:**

```go
// v1
h := live.NewHandler()
h.MountHandler = func(ctx context.Context, s *live.Socket) (any, error) {
    return &PageModel{}, nil
}
h.RenderHandler = func(ctx context.Context, rc *live.RenderContext) (io.Reader, error) {
    // Render entire page
    return renderPage(rc.Assigns), nil
}
h.HandleEvent("click", clickHandler)
http.Handle("/live", live.NewHttpHandler(ctx, h))
```

**v2 island:**

```go
// v2
func NewMyIsland() (*live.Island, error) {
    island, _ := live.NewIsland(
        "my-island",
        live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
            return &IslandState{}, nil
        }),
        live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
            // Render only this island
            return renderIsland(rc.State), nil
        }),
    )
    island.HandleEvent("click", clickHandler)
    return island, nil
}

live.RegisterIsland("my-island", NewMyIsland)
```

**v1 HTML:**

```html
<!-- v1: Full page rendered by Live -->
<div id="live">
    <button live-click="click">Click</button>
</div>
<script src="/live.js"></script>
```

**v2 HTML:**

```html
<!-- v2: Islands within static page -->
<h1>My Page</h1>
<live-island type="my-island" id="island-1">
    <button live-click="click">Click</button>
</live-island>
<script src="/custom-island.js"></script>
```

### Key Differences

1. **No full-page LiveViews**: v2 only supports islands. If you need interactivity across the whole page, create a root island.

2. **Multiple islands per page**: v2 allows multiple independent islands on one page, each with isolated state.

3. **Props instead of assigns**: v1 used `Socket.Assigns()` for state. v2 uses `Props` (read-only from HTML) and island state (mutable).

4. **Session management**: You must manage session IDs and transport endpoints. v1 handled this automatically.

5. **Client initialization**: v1 auto-initialized on page load. v2 uses custom elements that initialize when added to the DOM.

6. **Event routing**: v2 requires explicit event routing via `engine.RouteEvent()`. v1 routed automatically.

### Breaking Changes Summary

- Removed `Handler`, `NewHandler()`, `NewHttpHandler()`
- Removed `Socket.Assigns()` - use island state instead
- Removed `page.Component` - use `Island` instead
- Removed `RenderSocket()` - use `Island.Render()`
- Removed `WithTemplateRenderer()` - implement render handler directly
- Removed automatic WebSocket endpoint - you must set up transports
- Client library rewritten - `<live-island>` custom element replaces `Live.init()`

## Advanced Topics

### Server-Pushed Updates

Islands can receive server-pushed updates via self-handlers:

```go
island.HandleSelf("refresh", func(ctx context.Context, state any, data any) (any, error) {
    // Update state based on server push
    return updatedState, nil
})

// Trigger from server
engine.BroadcastToIsland(sessionID, islandID, "refresh", data)
```

### Broadcasting

Broadcast events to multiple islands:

```go
// Broadcast to all instances of an island type
engine.BroadcastToIslandType(islandType, "update", data)

// Broadcast to a specific island instance
engine.BroadcastToIsland(sessionID, islandID, "update", data)
```

### Custom State Stores

Implement `IslandStateStore` interface for custom state persistence (Redis, database, etc.):

```go
type IslandStateStore interface {
    Get(sessionID SessionID, islandID IslandID) (any, error)
    Set(sessionID SessionID, islandID IslandID, state any, ttl time.Duration) error
    Delete(sessionID SessionID, islandID IslandID) error
}
```

### Nested Islands

Islands can contain other islands. Each maintains its own state:

```html
<live-island type="parent" id="parent-1">
    <h2>Parent Island</h2>
    <live-island type="child" id="child-1">
        <p>Child Island</p>
    </live-island>
</live-island>
```

Islands are treated as opaque boundaries - parent renders don't diff into child content.

## API Reference

### Core Types

```go
type Island struct {
    Name   string
    Mount  IslandMountHandler
    Render IslandRenderHandler
    Unmount IslandUnmountHandler
}

type IslandInstance struct {
    ID   string
    Type string
}

type Props map[string]any

type Session struct {
    ID SessionID
}

type IslandEngine struct {
    // Manages sessions and islands
}

type Transport interface {
    Send(Event) error
    Events() <-chan Event
    Close() error
}
```

### Functions

```go
// Island creation
func NewIsland(name string, configs ...IslandConfig) (*Island, error)
func WithMount(handler IslandMountHandler) IslandConfig
func WithRender(handler IslandRenderHandler) IslandConfig
func WithUnmount(handler IslandUnmountHandler) IslandConfig

// Registration
func RegisterIsland(name string, constructor IslandConstructor) error
func GetIsland(name string) (IslandConstructor, error)
func ListIslands() []string

// Engine
func NewIslandEngine(ctx context.Context, registry *IslandRegistry, stateStore IslandStateStore) *IslandEngine
func (e *IslandEngine) AddSession(session *Session)
func (e *IslandEngine) DeleteSession(sessionID SessionID)
func (e *IslandEngine) MountIsland(sessionID SessionID, islandID IslandID, islandType string, props Props) (*IslandInstance, error)
func (e *IslandEngine) RouteEvent(sessionID SessionID, event Event) error

// Transport
func UpgradeWebSocket(ctx context.Context, w http.ResponseWriter, r *http.Request, config *TransportConfig) (Transport, error)
func NewSSETransportFactory(config *TransportConfig) *SSETransportFactory
```

See [pkg.go.dev](https://pkg.go.dev/github.com/jfyne/live) for complete API documentation.

---

Built with ⚡ by [@jfyne](https://github.com/jfyne)
