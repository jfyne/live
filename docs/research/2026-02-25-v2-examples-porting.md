---
date: 2026-02-25T14:00:00Z
researcher: josh
topic: "Porting master branch examples to v2 islands architecture"
tags: [research, examples, v2, islands, porting]
last_updated: 2026-02-25
last_updated_by: josh
---

# Research: Porting Master Branch Examples to v2 Islands Architecture

## Research Question

What examples exist on the master branch, what features do they demonstrate, and what is needed to create v2 versions using the islands architecture?

## Summary

The master branch has 14 example directories. After deduplication (chat is a library used by cluster; components is a library used by clocks; chart has no Go code), there are **10 distinct examples** showcasing different framework features. The existing v2 branch has only a **counter** example.

Seven examples can be ported directly using features that already exist in v2. Three examples use v1 features that do **not yet exist** in v2: `live-window-keyup` (buttons), `HandleParams`/`live-patch`/`PatchURL` (pagination), and `AllowUploads`/`ConsumeUploads` (uploads).

## Detailed Findings

### Master Branch Examples Inventory

| # | Example | Feature Demonstrated | V1 API Used |
|---|---------|---------------------|-------------|
| 1 | **buttons** | Counter with keyboard shortcuts | `HandleEvent`, `live-click`, `live-window-keyup`, `live-key` |
| 2 | **todo** | Form validation, list management, checkboxes | `HandleEvent`, `live-change`, `live-submit`, `live-debounce`, `live-value-*`, `Params.Checkbox()` |
| 3 | **clock** | Server-pushed timed updates | `HandleSelf`, `s.Self()`, goroutine scheduling |
| 4 | **chat** | Broadcasting, `live-update="append"`, hooks | `HandleEvent`, `HandleSelf`, `s.Broadcast()`, `live-update="append"`, `live-hook`, `live-submit` |
| 5 | **alpine** | Integration with Alpine.js | `HandleEvent`, `live-submit`, `live-change`, `live-click`, `live-value-*` |
| 6 | **error** | Error handling, hooks, `handleEvent` in JS | `HandleEvent`, `ErrorHandler`, `live-hook`, `live-value-*` |
| 7 | **pagination** | URL parameter handling, client/server-side navigation | `HandleParams`, `live-patch`, `s.PatchURL()`, `live-click`, `live-value-*` |
| 8 | **prefill** | Form prefill, validation | `HandleEvent`, `live-change`, `live-submit` |
| 9 | **uploads** | File uploads with validation, progress | `AllowUploads`, `ConsumeUploads`, `live-change`, `live-submit`, `Upload.Progress` |
| 10 | **cluster** | PubSub broadcasting across handler instances | `NewPubSub`, `CloudTransport`, `Broadcast` |
| 11 | **clocks** | Component system (page/component API) | `page.WithComponentMount`, `page.WithComponentRenderer` |
| 12 | **components** | Reusable component library (used by clocks) | `page.Component`, clock widget |
| 13 | **chart** | No Go code — static HTML/JS only | N/A |
| 14 | **buttons** (duplicate listing — same as #1) | — | — |

### Existing V2 Counter Example Pattern

The counter example (`examples/counter/`) establishes the v2 pattern:

**Server-side structure:**
1. Define state struct (`CounterState`)
2. Create island constructor (`NewCounterIsland`) returning `(*live.Island, error)`
3. Use `live.NewIsland("name", live.WithMount(...), live.WithRender(...))`
4. Register event handlers via `island.HandleEvent("name", handler)`
5. Register with global registry: `live.RegisterIsland("counter", NewCounterIsland)`
6. Create engine: `live.NewIslandEngine(ctx, registry, stateStore)`
7. Set up WebSocket/SSE transport endpoints with event loop
8. Serve index page and `live.Javascript{}`

**HTML structure:**
- Index page uses `<live-island type="counter" id="counter-1">` custom elements
- Island template (re-rendered on state change) is simple HTML with `live-click` directives
- Initial content inside `<live-island>` provides server-rendered fallback
- `<script src="/live.js"></script>` loads the client library

**Event loop pattern (per transport):**
```go
for event := range transport.Events() {
    if event.T == "subscribe" && event.Island != "" {
        // Mount island with server-defined props
        engine.MountIsland(sessionID, islandID, islandType, props)
    } else {
        engine.RouteEvent(sessionID, event)
    }
}
```

### V2 Client-Side Capabilities

The TypeScript client supports these event directives:

| Directive | Wired To | Notes |
|-----------|----------|-------|
| `live-click` | `click` | Supported |
| `live-contextmenu` | `contextmenu` | Supported |
| `live-mousedown` | `mousedown` | Supported |
| `live-mouseup` | `mouseup` | Supported |
| `live-focus` | `focus` | Supported |
| `live-blur` | `blur` | Supported |
| `live-keydown` | `keydown` | Supported, with `live-key` filter |
| `live-keyup` | `keyup` | Supported, with `live-key` filter |
| `live-change` | `input` on forms | Serializes full form data |
| `live-submit` | `submit` on forms | Prevents default, serializes form |
| `live-debounce` | Per-element | Milliseconds or `"blur"` |
| `live-value-*` | Event params | Custom data attributes |
| `live-hook` | Hook lifecycle | `mounted`, `updated`, `destroyed`, etc. |

**Not supported in v2 client:**
- `live-window-keyup` / `live-window-keydown` (window-level keyboard events)
- `live-patch` (client-side URL patching)
- File upload progress/validation UI

**Server-side v2 capabilities:**
- `island.HandleEvent(name, handler)` — client events
- `island.HandleSelf(name, handler)` — server-targeted events
- `engine.BroadcastToIslandType(type, event)` — broadcast to all instances of a type
- `engine.BroadcastToIsland(islandID, event)` — broadcast to specific island ID
- `live-update="append|prepend|replace|ignore"` — server-driven patch actions

### Porting Assessment Per Example

#### Can Port Now (v2 features exist)

**1. Todo** — Form validation, list, checkboxes
- Uses: `live-change`, `live-submit`, `live-debounce="blur"`, `live-value-*`
- All directives supported in v2 client
- `Params.String()` and `Params.Checkbox()` — need to verify `Params.Checkbox()` exists in v2
- Port as single island

**2. Clock** — Server-pushed timed updates
- Uses: `HandleSelf`, goroutine with `s.Self()`
- V2 has `island.HandleSelf(name, handler)` for self-events
- Need to verify how to send self-events from within a mount/event handler in v2
- The v2 engine has no direct `Self()` equivalent on instance — events must be routed through the engine
- Port as single island, need mechanism to push self-events

**3. Chat** — Broadcasting with `live-update="append"`
- Uses: `HandleEvent` (send), `HandleSelf` (newmessage), `s.Broadcast()`, `live-update="append"`, `live-hook`, `live-submit`
- V2 has `engine.BroadcastToIslandType()` for broadcasting
- V2 server supports `Append` patch action
- V2 client supports `live-hook` with `mounted` lifecycle
- Port as single island type, multiple instances share broadcasts

**4. Alpine** — Alpine.js integration with autocomplete
- Uses: `live-submit`, `live-change`, `live-click`, `live-value-*`
- All directives supported in v2
- Alpine.js integration is purely client-side, no framework dependency
- Port as single island

**5. Error** — Error handling with hooks
- Uses: `live-click`, `live-value-*`, `live-hook`, `ErrorHandler`
- V2 client supports `live-hook` with `handleEvent` on hook context
- Need to verify v2 error handling mechanism (no `ErrorHandler` field on `Island`)
- Port as single island

**6. Prefill** — Form prefill with validation
- Uses: `live-change`, `live-submit`
- All directives supported in v2
- Port as single island

**7. Cluster** — PubSub broadcasting
- V2 has `BroadcastToIslandType` built into the engine
- Multiple engine instances could share a PubSub layer
- Port as chat island with external PubSub adapter

#### Cannot Port Yet (missing v2 features)

**8. Buttons** — Keyboard shortcuts
- Requires `live-window-keyup` which is NOT in v2 client
- `live-click` and basic counter work fine
- Could port without keyboard shortcuts, or add `live-window-keyup` support to v2 client

**9. Pagination** — URL parameter handling
- Requires `HandleParams` (not in v2 server API)
- Requires `live-patch` (not in v2 client)
- Requires `s.PatchURL()` (not in v2 server API)
- This is a significant feature gap — URL-driven state management

**10. Uploads** — File uploads
- Requires `AllowUploads`, `ConsumeUploads`, `Upload` types (not in v2 server)
- Requires upload progress UI (not in v2 client)
- This is a large feature gap — entire upload subsystem missing from v2

### V2 Server-Side Feature Gaps for Self-Events

The clock and chat examples need a way to send self-events from within handlers. In v1, `s.Self(ctx, event, data)` sends an event to the same socket. In v2:

- `IslandInstance` has `CallSelf(ctx, event, data)` (`instance.go`)
- But handlers receive `(ctx, state, params)` — they don't have access to the instance or engine
- `engine.BroadcastToIsland(islandID, event)` could work but requires knowing the island ID
- The event loop goroutine could schedule self-events if the handler returns metadata

This needs investigation — the clock example requires a recurring self-event pattern.

## Code References

- `examples/counter/main.go` — V2 counter example (277 lines), the pattern for all v2 examples
- `examples/counter/index.html` — V2 HTML template showing `<live-island>` usage
- `examples/counter/counter.html` — V2 island render template
- `island.go` — Island definition with `HandleEvent`, `HandleSelf`
- `instance.go` — `IslandInstance` with `CallSelf`, `CallEvent`
- `engine.go:265-320` — `BroadcastToIslandType`, `BroadcastToIsland`
- `web/src/events.ts` — Client-side event wiring (all supported directives)
- `web/src/hooks.ts` — Hook system with `pushEvent`, `handleEvent`

## Architecture Documentation

### V2 Example Pattern

Every v2 example follows this structure:

```
examples/<name>/
├── main.go          # Island definition + HTTP server setup
├── index.html       # Page template with <live-island> elements
├── <name>.html      # Island render template (re-rendered on state change)
└── README.md        # Optional documentation
```

The main.go file has three sections:
1. **Island definition** — state struct, constructor function, mount/render/event handlers
2. **Registration** — `live.RegisterIsland()`
3. **HTTP server** — engine setup, transport endpoints, page serving

### Boilerplate Reduction Opportunity

The counter example has ~100 lines of WebSocket/SSE transport boilerplate that is identical for every example. The event loop pattern (subscribe → mount, else → route) is the same. This could be extracted into a shared helper, but for examples clarity may be preferred over DRYness.

## Related Research

- `docs/research/2026-01-25-islands-component-architecture.md` — Islands architecture design
- `docs/research/2026-02-22-v2-branch-state-and-testing-gaps.md` — V2 branch state analysis

## Resolved Questions

1. **Params.Checkbox()** — Confirmed: exists in v2 at `params.go:21-31`. The todo example can use it as-is.

2. **Transport boilerplate** — Decision: **keep self-contained**. Each example will have its own full transport setup for clarity. Users see the complete picture.

3. **Self-event mechanism** — Decision: **return self-events from handlers**. Handlers will be able to return scheduled events alongside state. For delayed events (like clock tick), the returned event includes a `Delay` duration. The engine processes immediate events right away and schedules delayed events via `time.AfterFunc`. This keeps handlers pure and testable.

   **Design sketch:**
   ```go
   // New type for handler return values
   type SelfEvent struct {
       Event string
       Data  any
       Delay time.Duration // 0 = immediate
   }

   // Event handler returns state + optional self-events
   // Current: func(ctx, state, params) (any, error)
   // Proposed: func(ctx, state, params) (any, []SelfEvent, error)
   //
   // Or: keep existing signature, add a separate mechanism via context:
   //   live.ScheduleSelf(ctx, "tick", time.Now(), 1*time.Second)
   ```

   The clock example would use this to schedule a tick every second after mount. This requires a framework change to support — it is **not yet implemented**.

4. **Which examples and merging** — Decision: **port all 7, merge where features overlap**.

## Example Porting Plan

### Merged Examples (5 examples from 7 master originals)

| V2 Example | Merges From | Features Demonstrated |
|------------|-------------|----------------------|
| **clock** | clock | Server-pushed timed updates via `HandleSelf`, goroutine scheduling |
| **forms** | todo + prefill | Form validation (`live-change`), submission (`live-submit`), prefill, checkboxes, debounce, `live-value-*`, error display |
| **chat** | chat + cluster | Broadcasting (`BroadcastToIslandType`), `live-update="append"`, hooks (`live-hook`), form submission, clearing input after send |
| **hooks** | error | Error handling, `live-hook` with `handleEvent`/`pushEvent`, server→client event communication, `live-value-*` |
| **alpine** | alpine | Third-party JS framework integration (Alpine.js), autocomplete pattern, `live-submit`, `live-change`, `live-click`, `live-value-*` |

**Rationale for merges:**
- **todo + prefill → forms**: Both demonstrate forms — todo shows list management + validation, prefill shows initial values + validation. Combining into one "forms" example shows the full form story (prefill, validate, submit, list, checkboxes) without redundancy.
- **chat + cluster → chat**: Cluster is just chat with PubSub. In v2, `BroadcastToIslandType` is built-in, so the "cluster" concept is already the default behavior. The chat example can show multiple island instances receiving broadcasts.
- **error → hooks**: The error example is really about hooks (`handleEvent` on the client to receive server errors). Renaming to "hooks" better describes the feature.

### Not Portable Yet (3 examples)

| Example | Missing V2 Feature | Notes |
|---------|-------------------|-------|
| **buttons** | `live-window-keyup` in client | Could add as stretch goal — window-level event listeners |
| **pagination** | `HandleParams`, `live-patch`, `PatchURL` | URL-driven state management not yet in v2 |
| **uploads** | Entire upload subsystem | Large feature gap |

## Decisions

### 1. Self-event Triggering: `SendSelf` + `WithEventDelay`

**Decision: Option A — Mount handler sends initial self-event, `WithEventDelay` handles re-scheduling.**

**`SendSelf` API:**
```go
// SendSelf schedules a self-event to be delivered to the current island instance.
// It uses the context to identify the target session and island.
// The event is delivered asynchronously after the current handler returns.
//
// Arguments:
//   ctx   context.Context  - must contain session/island context (set by engine)
//   event string           - the self-event name (must match a HandleSelf registration)
//   data  any              - arbitrary data passed to the self handler
func SendSelf(ctx context.Context, event string, data any)
```

The engine stores session ID and island ID in the context before calling handlers. `SendSelf` reads these from context and enqueues the event for delivery after the current handler completes.

**`WithEventDelay` API:**
```go
// WithEventDelay configures automatic re-scheduling of a self-event after its
// handler completes. Each time the named self-handler finishes, the engine
// schedules another delivery of the same event after the specified delay.
//
// The delay event carries the return value of the previous handler invocation
// as its data (the current state). To customize the data, the handler can
// use SendSelf to schedule with specific data instead.
//
// Arguments:
//   event string         - the self-event name to re-schedule
//   delay time.Duration  - delay before re-delivery
//
// Returns an IslandOption for use with NewIsland.
func WithEventDelay(event string, delay time.Duration) IslandOption
```

**Complete clock example:**
```go
island, _ := live.NewIsland("clock",
    live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
        // Kick off the first tick immediately after mount
        live.SendSelf(ctx, "tick", time.Now())
        return &ClockState{Time: time.Now()}, nil
    }),
    live.WithRender(renderHandler),
    live.WithEventDelay("tick", 1*time.Second), // re-schedule after each tick handler
)

island.HandleSelf("tick", func(ctx context.Context, state any, data any) (any, error) {
    s := state.(*ClockState)
    s.Time = time.Now()
    return s, nil
})
```

**Flow:**
1. Mount handler calls `SendSelf(ctx, "tick", time.Now())` → engine queues self-event
2. After mount completes, engine delivers the self-event to `HandleSelf("tick", ...)`
3. Handler updates state, returns
4. Engine sees `WithEventDelay("tick", 1s)` → schedules `time.AfterFunc(1s, deliverTick)`
5. After 1 second, engine delivers tick again → goto step 3
6. On unmount, engine cancels any pending `time.AfterFunc` timers

### 2. Error Handling: `WithErrorHandler` + Default

**Decision: Option A with a default error handler.**

```go
// WithErrorHandler configures a custom error handler for the island.
// When an event handler returns an error, this function is called to
// produce an event that is sent to the client.
//
// If not set, the default error handler sends:
//   Event{T: "err", Data: {"err": error.Error()}}
func WithErrorHandler(fn func(ctx context.Context, err error) Event) IslandOption

// Default behavior (when WithErrorHandler is not configured):
// The engine sends an error event to the client automatically.
// Client hooks can handle it via: this.handleEvent("err", (data) => { ... })
```

**Usage in hooks example:**
```go
island, _ := live.NewIsland("hooks-demo",
    live.WithMount(mountHandler),
    live.WithRender(renderHandler),
    // Custom error handler (optional — default sends {"t":"err","d":{"err":"..."}})
    live.WithErrorHandler(func(ctx context.Context, err error) Event {
        data, _ := json.Marshal(map[string]string{"err": err.Error()})
        return Event{T: "err", Data: data}
    }),
)
```

**Default handler** — When `WithErrorHandler` is not set, the engine uses a built-in default that sends `Event{T: "err", Island: instanceID, Data: {"err": error.Error()}}` to the client. This means error events work out of the box without configuration.

## Open Questions

None — all questions resolved. Ready for implementation planning.
