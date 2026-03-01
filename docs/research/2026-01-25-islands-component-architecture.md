---
date: 2026-01-25T10:30:00+00:00
researcher: Claude
topic: "Live v2: Islands-Only Architecture (Breaking Redesign)"
tags: [research, architecture, v2, islands, breaking-change]
last_updated: 2026-01-25
last_updated_by: Claude
last_updated_note: "Updated to reflect v2 as clean break with no backward compatibility"
version: "v2.0.0"
breaking: true
---

# Research: Live v2 - Islands-Only Architecture

**Version**: v2.0.0 (Breaking Redesign)

## Research Question

How to redesign Live v2 as a pure islands architecture, where:
- Islands are the only abstraction (no full-page views)
- Islands can be included multiple times and work independently
- Islands can be nested within other islands
- Islands accept props like Angular/React components
- Each island is fully isolated with its own state
- Multiple islands share a single WebSocket connection

## v2 Goals

1. **Islands-only API**: Remove all full-page view concepts
2. **True isolation**: Each island has its own state, events, and lifecycle
3. **Component-like DX**: Props, nesting, composition patterns familiar to frontend developers
4. **Efficient networking**: Shared WebSocket with message routing
5. **Web standards**: Built on custom elements for natural browser integration
6. **Minimal boilerplate**: Simple registration and usage patterns
7. **Static-friendly**: Islands embedded in any HTML (static sites, SSR, etc.)

## Summary

**This research documents the v2 architecture - a complete rewrite with no backward compatibility.**

The Live v1 library operates as a **full-page live view system** where a single WebSocket connection manages the entire page state. The architecture follows Phoenix LiveView's model: one Handler per route, one Socket per connection, and a single render tree for the entire page.

Live v2 will replace this with an **islands-only architecture**:

**v1 (Current):**
- Single Handler per page route
- Single WebSocket per page
- Full-page render cycle
- Components share resources

**v2 (Target):**
- No concept of "pages" - only islands
- Shared WebSocket with island routing
- Islands render independently
- Fully isolated island state
- Custom element-based embedding (`<live-island>`)
- True component nesting
- Props-based configuration

This is a clean architectural break that eliminates full-page patterns entirely.

## Detailed Findings

### v1 Architecture Analysis (For Comparison)

The following documents v1's full-page architecture to understand what v2 is replacing:

#### Handler Structure (`handler.go:43-62`)

The Handler struct is designed as a single controller for an entire page:

```go
type Handler struct {
    MountHandler    MountHandler           // Called on GET and WS connect
    UnmountHandler  UnmountHandler         // Called on WS disconnect
    RenderHandler   RenderHandler          // Renders entire page
    ErrorHandler    ErrorHandler           // Handles errors
    eventHandlers   map[string]EventHandler // ALL event handlers for page
    selfHandlers    map[string]SelfHandler  // ALL self handlers for page
    paramsHandlers  []EventHandler          // URL param handlers
}
```

All event handlers are registered in a single flat map. When a component registers an event, it uses a scoped name (`componentID--eventName`) but the handler still belongs to the single Handler instance.

#### Engine and Socket Relationship (`engine.go:54-86`)

The Engine manages all connected sockets for a single Handler:

```go
type Engine struct {
    Handler *Handler                    // Single handler for all sockets
    addSocketC      chan engineAddSocket
    getSocketC      chan engineGetSocket
    deleteSocketC   chan engineDeleteSocket
    iterateSocketsC chan engineIterateSockets
    socketStateStore SocketStateStore   // Shared state store
}
```

Key architectural constraints:
- One Engine per Handler
- Engine tracks all connected sockets in a single map (`engine.go:109-135`)
- Broadcasts go to ALL sockets on the engine (`engine.go:166-173`)
- State store is shared across all sockets

#### WebSocket Connection (`socket.ts:46-56`)

The client creates a single WebSocket per page:

```typescript
this.conn = new WebSocket(
    `${location.protocol === "https:" ? "wss" : "ws"}://${
        location.host
    }${location.pathname}${location.search}${location.hash}`
);
```

The WebSocket URL is derived from the current page URL, meaning one connection per page load.

#### Rendering Pipeline (`render.go:21-51`, `engine.go:200-209`)

Every state change triggers a full render cycle:

```go
func RenderSocket(ctx context.Context, e *Engine, s *Socket) (*html.Node, error) {
    rc := &RenderContext{
        Socket:  s,
        Uploads: s.Uploads(),
        Assigns: s.Assigns(),  // Gets ALL socket state
    }
    output, err := e.Handler.RenderHandler(ctx, rc)
    // ... parse, diff, patch
}
```

The RenderHandler receives the entire socket state and renders the complete page. Diffing then calculates patches, but the full template is always executed.

### Existing Component System (`page/component.go`)

The `page` package provides a component abstraction that partially addresses the requirements:

#### Component Structure (`page/component.go:39-66`)

```go
type Component struct {
    ID       string           // Unique stable ID
    Handler  *live.Handler    // Reference to HOST handler (shared)
    Socket   *live.Socket     // Reference to socket (shared)
    Register RegisterHandler  // Setup event handlers
    Mount    MountHandler     // Initialize state
    Render   RenderHandler    // Render this component
    State    any              // Component-specific state
    Uploads  live.UploadContext
}
```

**Current Capabilities:**
- Components have their own ID and State
- Components can render themselves independently
- Event scoping via `c.Event(name)` returns `"componentID--name"` (`page/component.go:146-148`)

**Current Limitations:**
- Components share the same Handler (event handlers are registered globally)
- Components share the same Socket (single WebSocket connection)
- All components re-render on any state change (no isolation)

#### Event Scoping (`page/component.go:121-130`)

```go
func (c *Component) HandleEvent(event string, handler EventHandler) {
    c.Handler.HandleEvent(c.Event(event), func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
        state, err := handler(ctx, p)
        c.State = state
        return s.Assigns(), nil  // Returns FULL socket assigns
    })
}
```

While events are scoped by name, the handler:
1. Registers on the shared Handler
2. Returns the full socket assigns, triggering full-page re-render

#### Component Initialization and Nesting (`page/component.go:88-100`)

```go
func Init(ctx context.Context, construct func() (*Component, error)) (*Component, error) {
    comp, err := construct()
    if err := comp.Register(comp); err != nil { ... }
    if err := comp.Mount(ctx, comp); err != nil { ... }
    return comp, nil
}
```

Nesting is demonstrated in `examples/components/page.go:70-85`:

```go
clock, err := page.Init(context.Background(), func() (*page.Component, error) {
    return NewClock(
        fmt.Sprintf("clock-%d", len(state.Clocks)+1),
        c.Handler,  // Parent's handler passed to child
        c.Socket,   // Parent's socket passed to child
        tz,
    )
})
state.Clocks = append(state.Clocks, clock)
```

Children receive the parent's Handler and Socket, creating tight coupling.

### Client-Side Architecture

#### Single Socket Connection (`socket.ts`)

The client maintains one global WebSocket:
- Socket ID stored in cookie (`_psid`)
- Connection derived from page URL
- All events sent through single connection
- All patches received on single connection

#### Event Wiring (`events.ts`)

Events are wired by scanning DOM for `live-*` attributes:

```typescript
document.querySelectorAll(`*[${this.attribute}]`).forEach((element) => {
    element.addEventListener(this.event, (e) => {
        Socket.sendAndTrack(new LiveEvent(t, params, LiveEvent.GetID()), element);
    });
});
```

Events are sent with the scoped event name but through the single WebSocket.

#### Patch Application (`patch.ts`)

Patches target elements by anchor attribute:

```typescript
static applyPatch(e: PatchEvent) {
    const target = document.querySelector(`*[${e.Anchor}]`);
    target.outerHTML = e.HTML;
}
```

Anchors are hierarchical (`_l_0_1_0`) and generated during full-tree diffing.

### Examples Analysis

#### Simple Handler Pattern (`examples/buttons/main.go`)

Full-page handler with global event handlers:

```go
h := live.NewHandler(live.WithTemplateRenderer(t))
h.MountHandler = func(ctx context.Context, s *live.Socket) (any, error) {
    return newCounter(s), nil
}
h.HandleEvent(inc, func(ctx context.Context, s *live.Socket, _ live.Params) (any, error) {
    c := newCounter(s)
    c.Value += 1
    return c, nil
})
```

#### Component Pattern (`examples/clocks/main.go`)

Uses page components but still creates single handler:

```go
h := live.NewHandler(
    page.WithComponentMount(func(ctx context.Context, h *live.Handler, s *live.Socket) (*page.Component, error) {
        return components.NewPage("app", h, s, "Clocks")
    }),
    page.WithComponentRenderer(),
)
http.Handle("/", live.NewHttpHandler(context.Background(), h))
```

The page component owns child clock components, but all share the single Handler/Socket.

#### Nested Components (`examples/components/`)

Parent page creates clock children dynamically:

```go
// Parent stores child references
type PageState struct {
    Clocks []*page.Component
}

// Child created with parent's Handler/Socket
clock, _ := page.Init(ctx, func() (*page.Component, error) {
    return NewClock("clock-1", c.Handler, c.Socket, "Europe/London")
})
```

## Architecture Documentation

### Current Data Flow

```
[Browser]
    |
    | HTTP GET /
    v
[Engine.get()] --> [Handler.MountHandler] --> [RenderSocket] --> HTML Response
    |
    | WebSocket Upgrade
    v
[Engine._serveWS()] --> [AddSocket] --> [MountHandler again] --> [RenderSocket]
    |
    | Client Event (e.g., "clock-1--tick")
    v
[Engine.CallEvent()] --> [Handler.eventHandlers["clock-1--tick"]]
    |                              |
    |                              v
    |                      [handler updates state]
    |                              |
    v                              v
[RenderSocket()] <-------- [sock.Assign(data)]
    |
    | Full page re-render
    v
[Diff()] --> [Patches] --> [sock.Send(EventPatch, patches)]
    |
    v
[Browser: Patch.applyPatch()]
```

### Key Coupling Points

1. **Engine ↔ Handler**: 1:1 relationship, Engine processes all events through single Handler
2. **Handler ↔ Events**: Single flat map holds all event handlers for entire page
3. **Socket ↔ State**: Socket's Assigns() returns all state, not component-scoped state
4. **Render ↔ State**: RenderContext receives full socket state, renders full page
5. **Client ↔ Server**: Single WebSocket, single socket ID cookie

### DOM Anchoring System (`diff.go`)

The diffing system uses hierarchical anchors:

```go
func (n anchorGenerator) String() string {
    out := "_l"  // liveAnchorPrefix
    for _, i := range n.idx {
        out += fmt.Sprintf("_%d", i)
    }
    return out  // e.g., "_l_0_1_0"
}
```

Anchors are generated by tree position, not by component ID. This means:
- Moving a component in the DOM changes its anchors
- Component boundaries are not preserved in anchoring
- Patches are position-based, not component-based

## External Context

### Phoenix LiveView Components

Phoenix LiveView's component architecture provides relevant patterns:

1. **LiveComponent** - Components run in parent process but maintain separate socket state
2. **Stateful vs Stateless** - Components can be stateless (pure functions) or stateful (maintain state)
3. **Communication Patterns**:
   - Parent-to-child: Explicit assign passing
   - Child-to-parent: `send(self(), {:event, data})`
   - Cross-component: `send_update/3` for targeted updates

4. **Two Competing Patterns**:
   - "LiveView as Source of Truth": Parent owns state, components request updates
   - "Component as Source of Truth": Components manage own state, parent passes identifiers

### Astro Islands Architecture

Astro's islands pattern represents the target architecture:

1. **Static by default** - Most content is static HTML
2. **Selective hydration** - Only interactive regions get JavaScript
3. **Complete isolation** - Islands don't share state unless explicitly connected
4. **Independent loading** - Each island hydrates separately
5. **Framework agnostic** - Different frameworks per island

**Key patterns from Astro:**
- Hydration directives: `client:load`, `client:idle`, `client:visible`
- No global state by default
- State sharing via stores or custom events
- Parallel loading with no blocking

### Go LiveView Implementations

Other Go LiveView implementations show similar patterns:

- **go-live-view**: Uses stateful structs per connection with dynamic components for optimization
- **jfyne/live**: Current implementation with component abstraction

## Code References

### Core Architecture
- `handler.go:43-62` - Handler struct with single event handler maps
- `engine.go:54-86` - Engine struct coupling Handler and socket management
- `engine.go:109-135` - Socket map management in single goroutine
- `engine.go:256-274` - CallEvent routing to single Handler's event map
- `socket.go:31-44` - Socket struct with single engine reference
- `socket.go:121-136` - Assigns/Assign operating on shared state store

### Component System
- `page/component.go:39-66` - Component struct with shared Handler/Socket references
- `page/component.go:88-100` - Init function for component lifecycle
- `page/component.go:121-130` - HandleEvent registering on shared Handler
- `page/component.go:146-148` - Event name scoping via ID prefix

### Rendering Pipeline
- `render.go:21-51` - RenderSocket full-page rendering
- `diff.go:33-68` - anchorGenerator hierarchical anchoring
- `diff.go:143-155` - anchorTree applying anchors by position

### Client Code
- `web/src/socket.ts:46-56` - Single WebSocket connection per page
- `web/src/live.ts:9-25` - Single initialization per page
- `web/src/events.ts` - DOM-wide event scanning and wiring
- `web/src/patch.ts:23-57` - Position-based patch application

### Examples
- `examples/clocks/main.go:13-20` - Full-page handler with component mount
- `examples/components/page.go:70-85` - Child component creation with shared resources
- `examples/components/clock.go:102-108` - Component configuration pattern

## Historical Context (from docs/)

- `docs/knowledge/project.md` - Describes Live as "alternative to React, Vue, Angular" with server-rendered HTML and WebSocket updates
- `docs/knowledge/tech-stack.md` - Minimal dependencies, stdlib-focused design
- `docs/knowledge/guidelines.md` - Preference for small interfaces, explicit error handling

## Related Research

No existing research documents found in `docs/research/`.

## Design Decisions (Resolved)

Based on discussion with project owner, the following architectural decisions have been made:

### 1. Connection Model: Shared Connection with Routing
- **Decision**: Single connection per page (transport-agnostic), with message routing to individual islands
- **Rationale**: More efficient than separate connections per island, reduces server load
- **Implications**:
  - Need to add island ID to event protocol (`{t: "event", i: 123, island: "counter-1", d: {...}}`)
  - Server needs routing layer to dispatch events to correct island handler
  - Client needs to scope patches by island container
  - **Transport abstraction**: Support WebSocket, SSE, and potentially long-polling

### 2. State Isolation: Fully Isolated
- **Decision**: Each island has its own state store, cannot access other islands' state
- **Rationale**: True component isolation, prevents accidental coupling between islands
- **Implications**:
  - Replace `Socket.Assigns()` with island-scoped state access
  - Each island gets its own state key in the store (e.g., `socketID:islandID`)
  - No shared state between islands by default
  - Cross-island communication via explicit events only

### 3. Nesting Model: True Nesting (React/Angular Style)
- **Decision**: Islands can contain other islands, forming hierarchies
- **Rationale**: Matches developer expectations from React/Angular/Vue
- **Implications**:
  - Need parent-child relationship tracking
  - Event bubbling or explicit parent communication
  - Nested islands share the same WebSocket but have isolated state
  - Child islands scoped within parent's DOM container

### 4. Embedding: Custom Element
- **Decision**: Use `<live-island id="counter">` custom elements
- **Rationale**: Clean, declarative, works with any templating system
- **Implications**:
  - Define `LiveIsland` custom element class in client
  - Custom element handles lifecycle: connectedCallback, disconnectedCallback
  - Attributes become island props: `<live-island id="clock" timezone="Europe/London">`
  - Shadow DOM optional (can use light DOM for easier styling)

### 5. Server Registration: Central Registry
- **Decision**: Register all islands at startup via `live.RegisterIsland("counter", counterHandler)`
- **Rationale**: Explicit, predictable, easy to understand and debug
- **Implications**:
  - Global island registry in the `live` package
  - Single endpoint handles all islands (e.g., `/_live/ws`)
  - Island ID in initial connection identifies which handler to use
  - Hot reload could re-register islands without server restart

### 6. Transport Methods: Pluggable Transports
- **Decision**: Support multiple transport methods (WebSocket, SSE, long-polling)
- **Rationale**: Different environments have different requirements (proxies, firewalls, read-heavy vs interactive)
- **Transports**:
  - **WebSocket**: Default for full-duplex, low-latency scenarios
  - **SSE (Server-Sent Events)**: Read-heavy updates, better proxy compatibility, HTTP POST for client events
  - **Long-polling**: Fallback for restrictive environments
- **Implications**:
  - Transport abstraction interface: `Transport.Send()`, `Transport.Receive()`
  - Client auto-negotiates transport (try WebSocket, fallback to SSE, fallback to polling)
  - Server implements same island protocol over any transport
  - Different endpoints per transport: `/_live/ws`, `/_live/sse`, `/_live/poll`

## v2 Breaking Changes

The following v1 concepts are **completely removed** in v2:

### Removed Types
- `Handler` struct - No page-level handlers
- `Engine.Handler` field - No 1:1 Handler coupling
- `page.Component` - Replaced by `Island`
- `MountHandler` (page-level) - Islands have their own mount
- `RenderHandler` (page-level) - Islands render themselves
- `Socket.Assigns()` returning full page state - Islands get scoped state only

### Removed Patterns
- Full-page render cycle
- Single Handler per route
- `http.Handle("/", live.NewHttpHandler(h))` pattern
- Template-based full-page rendering
- `page.WithComponentMount()` configuration
- Position-based anchoring (`_l_0_1_0`)
- Hardcoded WebSocket transport (now abstracted)

### New in v2
- Transport abstraction (WebSocket, SSE, long-polling)
- Island-scoped anchoring
- Custom element client API
- Props-based configuration
- Central island registry
- Shared connection with message routing

### Removed Files (v1)
- `page/component.go` - v1 component abstraction
- `page/configuration.go` - v1 component configs
- `page/render.go` - v1 component rendering helpers
- Examples using full-page pattern

## v2 Architecture

Based on the design decisions, here is the v2 target architecture:

### Server-Side Components

```
                                   ┌─────────────────┐
                                   │  Island Registry │
                                   │  (global map)    │
                                   └────────┬────────┘
                                            │
         ┌──────────────────────────────────┼──────────────────────────────────┐
         │                                  │                                  │
         ▼                                  ▼                                  ▼
┌─────────────────┐              ┌─────────────────┐              ┌─────────────────┐
│ Island: counter │              │ Island: clock   │              │ Island: chat    │
│ ┌─────────────┐ │              │ ┌─────────────┐ │              │ ┌─────────────┐ │
│ │  Handler    │ │              │ │  Handler    │ │              │ │  Handler    │ │
│ │  (own copy) │ │              │ │  (own copy) │ │              │ │  (own copy) │ │
│ └─────────────┘ │              │ └─────────────┘ │              │ └─────────────┘ │
└─────────────────┘              └─────────────────┘              └─────────────────┘

                    ┌────────────────────────────────┐
                    │         Island Engine          │
                    │  (transport-agnostic manager)  │
                    │  - Routes events by island ID   │
                    │  - Manages island state scopes  │
                    │  - Handles multiplexed patches  │
                    └────────┬───────────────────────┘
                             │
                    ┌────────┴────────┐
                    │    Transport    │
                    │   Abstraction   │
                    └────────┬────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
  ┌───────────┐      ┌───────────┐      ┌───────────┐
  │ WebSocket │      │    SSE    │      │  Polling  │
  │ Transport │      │ Transport │      │ Transport │
  └───────────┘      └───────────┘      └───────────┘
         │                   │                   │
         └───────────────────┼───────────────────┘
                             │
                    ┌────────▼────────────────────────┐
                    │    Island State Store           │
                    │  key: sessionID:islandID        │
                    │  value: island-specific state   │
                    └─────────────────────────────────┘
```

### Client-Side Components

```html
<!-- Static HTML page with island elements -->
<html>
  <body>
    <h1>My App</h1>

    <live-island id="counter-1" type="counter" initial-value="0">
      <!-- Server renders initial HTML here -->
      <button live-click="inc">+</button>
      <span>0</span>
      <button live-click="dec">-</button>
    </live-island>

    <live-island id="clock-1" type="clock" timezone="Europe/London">
      <time>12:00:00</time>
    </live-island>

    <!-- Nested islands -->
    <live-island id="dashboard" type="dashboard">
      <div class="widgets">
        <live-island id="chart-1" type="chart" data-source="/api/sales">
          <!-- Chart renders here -->
        </live-island>
      </div>
    </live-island>

    <script src="/live.js"></script>
  </body>
</html>
```

### Event Protocol

Current protocol:
```json
{"t": "click-event", "i": 1, "d": {"key": "value"}}
```

Proposed protocol with island scoping:
```json
{
  "t": "click-event",
  "i": 1,
  "island": "counter-1",
  "d": {"key": "value"}
}
```

### Patch Protocol

Current patch targeting by position anchor:
```json
{"Anchor": "_l_0_1_0", "Action": 1, "HTML": "<span>5</span>"}
```

Proposed patch with island scoping:
```json
{
  "island": "counter-1",
  "patches": [
    {"Anchor": "_l_0", "Action": 1, "HTML": "<span>5</span>"}
  ]
}
```

### v2 Server API

**Island Registration:**
```go
// Register islands at startup
live.RegisterIsland("counter", func() *Island {
    return &Island{
        Mount: counterMount,
        Render: counterRender,
        Events: map[string]EventHandler{
            "inc": incrementHandler,
            "dec": decrementHandler,
        },
    }
})

// Create engine with transport options
engine := live.NewEngine(
    live.WithWebSocket(),     // Enable WebSocket transport
    live.WithSSE(),           // Enable SSE transport
    live.WithPolling(),       // Enable long-polling fallback
)

// Serve transport endpoints
http.Handle("/_live/ws", engine.WebSocketHandler())
http.Handle("/_live/sse", engine.SSEHandler())
http.Handle("/_live/poll", engine.PollingHandler())
http.Handle("/live.js", live.Javascript{})
```

**Island Definition:**
```go
type Island struct {
    // Mount called when island connects
    Mount func(ctx context.Context, props Props) (State, error)

    // Render called to generate island HTML
    Render func(state State) (io.Reader, error)

    // Events map of event handlers
    Events map[string]EventHandler

    // Children for nested island support
    Children map[string]*Island
}

type EventHandler func(ctx context.Context, state State, params Params) (State, error)

// Transport abstraction
type Transport interface {
    // Send patches to client
    Send(ctx context.Context, sessionID string, patches []Patch) error

    // Receive events from client (channel-based)
    Events() <-chan Event

    // Close the transport
    Close() error
}

// Specific transport implementations
type WebSocketTransport struct { /* ... */ }
type SSETransport struct { /* ... */ }
type PollingTransport struct { /* ... */ }
```

**Client Custom Element:**
```typescript
class LiveIsland extends HTMLElement {
    private transport: Transport;
    private islandId: string;
    private islandType: string;

    async connectedCallback() {
        this.islandId = this.getAttribute('id');
        this.islandType = this.getAttribute('type');

        // Negotiate transport (try WebSocket, fallback to SSE, then polling)
        this.transport = await TransportNegotiator.connect({
            prefer: 'websocket',
            fallback: ['sse', 'polling']
        });

        this.transport.subscribe(this.islandId, this.handlePatch.bind(this));
        this.wireEvents();
    }

    disconnectedCallback() {
        this.transport.unsubscribe(this.islandId);
    }

    private wireEvents() {
        // Only wire events within this island's DOM
        this.querySelectorAll('[live-click]').forEach(el => {
            el.addEventListener('click', (e) => {
                this.transport.send({
                    t: el.getAttribute('live-click'),
                    island: this.islandId,
                    d: this.collectParams(el)
                });
            });
        });
    }

    private handlePatch(patches: Patch[]) {
        // Apply patches only within this island
        patches.forEach(p => {
            const target = this.querySelector(`[${p.Anchor}]`);
            if (target) target.outerHTML = p.HTML;
        });
        this.wireEvents(); // Re-wire after patch
    }
}

customElements.define('live-island', LiveIsland);

// Transport negotiation
class TransportNegotiator {
    static async connect(options: TransportOptions): Promise<Transport> {
        // Try WebSocket first
        if (options.prefer === 'websocket' || options.fallback.includes('websocket')) {
            try {
                return await WebSocketTransport.connect();
            } catch (e) {
                console.warn('WebSocket unavailable, falling back');
            }
        }

        // Try SSE
        if (options.fallback.includes('sse')) {
            try {
                return await SSETransport.connect();
            } catch (e) {
                console.warn('SSE unavailable, falling back');
            }
        }

        // Fallback to polling
        return await PollingTransport.connect();
    }
}
```

### Nesting Implementation

For nested islands, the parent island renders child island elements:

```go
func dashboardRender(state DashboardState) (io.Reader, error) {
    return template.Execute(`
        <div class="dashboard">
            <h2>{{.Title}}</h2>
            {{range .Charts}}
            <live-island id="{{.ID}}" type="chart" data-source="{{.DataSource}}">
                <!-- Child island renders here -->
            </live-island>
            {{end}}
        </div>
    `, state)
}
```

The client handles nesting naturally:
1. Parent `<live-island>` initializes
2. During parent render, child `<live-island>` elements are added to DOM
3. Browser's custom element lifecycle calls `connectedCallback` on children
4. Each child subscribes independently to the shared socket

## What v2 Enables (That v1 Cannot)

1. **Embed islands in static sites**: Drop `<live-island>` into any HTML without full-page takeover
2. **Mix frameworks**: Use Live islands alongside React, Vue, or vanilla JS on the same page
3. **Selective interactivity**: Static content stays static, only islands are interactive
4. **True component reuse**: Same island type multiple times with different props
5. **Progressive enhancement**: Islands can enhance existing server-rendered HTML
6. **Smaller payload**: Only load island code that's actually used on the page
7. **Independent updates**: One island can update without touching others
8. **Easier testing**: Test islands in isolation without page context
9. **Transport flexibility**: Choose WebSocket, SSE, or polling based on environment
10. **Better proxy compatibility**: SSE works through proxies that block WebSockets
11. **Read-heavy optimization**: SSE is more efficient for server-to-client only updates
12. **Graceful degradation**: Auto-fallback to compatible transport

### v2 Implementation Approach

Clean rewrite removing all v1 concepts:

1. **Remove full-page concepts entirely**
   - Delete `Handler.MountHandler`, `Handler.RenderHandler` (page-level handlers)
   - Delete `Engine` as page-level connection manager
   - Remove `page/component.go` (v1 component abstraction)
   - Remove hardcoded WebSocket implementation

2. **New core types**
   - `Island` - Self-contained interactive component
   - `IslandRegistry` - Global registry mapping island types to constructors
   - `IslandEngine` - Manages shared connection and routes to islands
   - `Transport` - Interface for WebSocket/SSE/polling implementations
   - `Session` - Scoped session per client (transport-agnostic)

3. **Protocol changes**
   - Add `island` field to all events
   - Scope patches by island ID
   - Island-scoped anchors (not position-based)
   - Transport-agnostic message format

4. **Transport layer**
   - Implement `Transport` interface
   - WebSocket transport (primary)
   - SSE transport with HTTP POST for events
   - Optional polling transport
   - Client auto-negotiation with fallback

5. **Client rewrite**
   - Custom element as primary API
   - Transport abstraction with auto-negotiation
   - Island-scoped event wiring and patching
   - Shared connection with island subscriptions

6. **Examples as v2 showcase**
   - All examples use island pattern
   - Show composition, nesting, props
   - Demonstrate state isolation
   - Examples using different transports

## v2 Implementation Questions

The following questions remain for v2 implementation planning:

### 1. Anchor Generation Strategy
**Question**: How should anchors be scoped in v2?

**Options**:
- Island-scoped: `_i_counter-1_0_1` (scoped to island instance)
- Type-scoped: `_t_counter_0_1` (same across all instances of island type)
- Hybrid: Island ID prefix + relative path

**Recommendation**: Island-scoped (`_i_counter-1_0_1`) for stability and isolation

### 2. State Storage Schema
**Question**: How should island state be keyed in the store?

**Options**:
- Composite key: `socketID:islandID`
- Nested structure: `{socketID: {islands: {islandID: state}}}`
- Separate stores per island type

**Recommendation**: Composite key for simplicity, single SocketStateStore

### 3. Event Protocol Format
**Question**: Should island ID be in event payload or as WebSocket subprotocol?

**Options**:
- In JSON payload: `{t: "inc", island: "counter-1", d: {}}`
- As message envelope: `{island: "counter-1", event: {t: "inc", d: {}}}`
- URL-based routing: `/ws/counter-1` (separate connections)

**Recommendation**: JSON payload for single connection multiplexing

### 4. Server-Side Rendering
**Question**: How should islands render their initial HTML?

**Options**:
- Server renders at page load, custom element hydrates
- Empty custom element, island renders after connect
- Hybrid: static shell + live hydration

**Recommendation**: Server renders initial HTML, custom element connects for interactivity

### 5. Nested Island Communication
**Question**: How should parent-child islands communicate?

**Options**:
- Direct method calls (parent has child reference)
- Event bubbling through DOM
- Explicit events via shared socket
- Props down, events up (React pattern)

**Recommendation**: Props down + explicit events up for predictability

### 6. Island Lifecycle Hooks
**Question**: What lifecycle hooks should islands expose?

**Proposed**:
- `Mount(ctx, props)` - Initialize state
- `Update(ctx, state, props)` - Props changed
- `HandleEvent(ctx, state, event, params)` - User event
- `HandleSelf(ctx, state, event, data)` - Server event
- `Unmount(ctx, state)` - Cleanup

### 7. TypeScript Client API
**Question**: Should client provide imperative API beyond custom elements?

**Options**:
- Custom element only (declarative)
- `new LiveIsland({type, props})` (imperative)
- Both (use case dependent)

**Recommendation**: Custom element primary, imperative for programmatic use

### 8. Hot Reload Support
**Question**: How should v2 support hot reload during development?

**Options**:
- Re-register island on file change, reconnect all instances
- Send reload event, islands re-mount
- Full page reload (simplest)

**Recommendation**: Island-level re-mount for fast iteration

### 9. Transport Implementation Details
**Question**: How should different transports be implemented?

**WebSocket Implementation**:
```go
// Full-duplex, bidirectional
type WebSocketTransport struct {
    conn *websocket.Conn
    send chan Message
    recv chan Event
}

// Server to client: patches via ws.Write()
// Client to server: events via ws.Read()
```

**SSE Implementation**:
```go
// Server to client: SSE stream
// Client to server: HTTP POST
type SSETransport struct {
    writer http.ResponseWriter
    flusher http.Flusher
    eventEndpoint string  // POST endpoint for client events
}

// Server to client: SSE events
// Client to server: fetch('/_live/sse/event', {method: 'POST', body: event})
```

**Polling Implementation**:
```go
// Client polls for updates
type PollingTransport struct {
    pollInterval time.Duration
    lastEventID  string
}

// Server: Long-poll endpoint returns patches when available
// Client: Poll /_live/poll?lastEventId=123, POST events to /_live/poll/event
```

**Transport Selection Algorithm**:
1. Client attempts WebSocket connection
2. If fails (timeout, 4xx/5xx), try SSE
3. If SSE fails, fallback to polling
4. Once connected, stick with transport (don't switch mid-session)

**Transport Use Cases**:
- **WebSocket**: Interactive islands with frequent bidirectional updates (chat, collaborative editing)
- **SSE**: Read-heavy islands with server-driven updates (dashboards, live feeds, notifications)
- **Polling**: Fallback for restrictive networks or legacy browser support

**SSE Benefits**:
- Automatic reconnection built into EventSource API
- Works through most HTTP proxies and firewalls
- Simpler server implementation (standard HTTP response)
- Lower overhead for server-to-client only scenarios
- Native browser retry with exponential backoff

**Recommendation**: Implement WebSocket first, SSE second (high priority), polling as optional fallback

## Conclusion

Live v2 represents a fundamental architectural shift from full-page live views to a pure islands architecture. This change aligns the library with modern web development patterns (Astro Islands, React Server Components, etc.) while maintaining Go's server-side strengths.

### Key v2 Principles

1. **Islands are the primitive**: No concept of pages, only reusable islands
2. **Isolation by default**: Each island is a self-contained unit
3. **Shared infrastructure**: Single connection efficiently multiplexes island messages
4. **Transport flexibility**: WebSocket, SSE, or polling based on environment
5. **Web standards**: Custom elements provide natural browser integration
6. **Developer familiarity**: Props, nesting, and composition match React/Vue patterns

### Next Steps

1. ✅ Create v2 branch (already done)
2. Define Transport interface
3. Implement WebSocket transport
4. Implement core Island type and registry
5. Build IslandEngine with message routing and transport abstraction
6. Develop custom element client with transport negotiation
7. Implement SSE transport
8. Port one example (counter) to v2 pattern
9. Add examples using different transports
10. Iterate on API based on practical usage
11. Document migration guide for v1 users
12. Release v2.0.0 as breaking major version

**Implementation Priority**:
- **Phase 1**: WebSocket transport + basic island system (MVP)
- **Phase 2**: SSE transport (production-ready)
- **Phase 3**: Polling transport (optional, for edge cases)

### Breaking Change Philosophy

v2 is unapologetically incompatible with v1. This clean break enables:
- Better architecture without legacy constraints
- Simpler codebase without backward compatibility code
- Clear mental model (islands only)
- Modern patterns aligned with ecosystem trends

Users who need v1 can pin to `v1.x` tags. v2 is the future.
