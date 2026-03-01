---
date: 2026-03-01T12:00:00Z
researcher: josh
topic: "V2 Feature Gaps vs Master README and Test Coverage Analysis"
tags: [research, v2, feature-gaps, test-coverage, master-comparison]
last_updated: 2026-03-01
last_updated_by: josh
---

# Research: V2 Feature Gaps vs Master README and Test Coverage

## Research Question

What features documented in the master branch README are missing from V2? What is the current test coverage state? What needs implementing to achieve feature parity and full test coverage?

## Summary

The V2 branch has a structurally complete islands architecture with **381 Go tests** (82.3% statement coverage in the main package) and **205 TypeScript tests** (all passing). Six examples exist (counter, clock, chat, forms, alpine, hooks).

Comparing against the master README, features fall into three categories:

1. **Intentionally removed** (architectural change to islands): `page/` component package, `NewHandler()`/`NewHttpHandler()`, `WithTemplateRenderer`, full-page `HandleParams`/`PatchURL`
2. **Not yet ported** (could be added to V2): `live-window-*` events, `live-throttle`, `live-patch` navigation, file uploads, redirect handling, page-level connection CSS classes
3. **Missing examples**: buttons, pagination, uploads, clocks/components, cluster — 5 of these require unimplemented V2 features

Test coverage gaps exist primarily in: transport HTTP endpoint handlers, `BroadcastToIsland()`, `SetStateTTL()`, JavaScript serving, and the counter example (no test file).

## Detailed Findings

### V2 Server-Side API: Complete for Islands Architecture

The V2 server exposes 100+ public APIs across 19 Go source files:

| Area | Key APIs | Status |
|------|----------|--------|
| Island lifecycle | `NewIsland`, `WithMount`, `WithRender`, `WithUnmount`, `HandleEvent`, `HandleSelf` | Complete |
| Self-events | `SendSelf`, `WithEventDelay` | Complete |
| Engine | `NewIslandEngine`, `MountIsland`, `UnmountIsland`, `RouteEvent` | Complete |
| Broadcasting | `BroadcastToIslandType`, `BroadcastSelfToIslandType`, `BroadcastToIsland` | Complete |
| Sessions | `NewSession`, `AddIsland`, `GetIsland`, `Send` | Complete |
| State persistence | `IslandStateStore` interface, `MemoryIslandStateStore` with TTL | Complete |
| Transports | `Transport` interface, WebSocket, SSE implementations | Complete |
| Error handling | `WithErrorHandler`, `EventError` type | Complete |
| Params | `Params` with `String()`, `Int()`, `Float32()`, `Checkbox()` | Complete |
| Props | `Props` with `String()`, `Int()`, `Bool()`, `Float32()` | Complete |
| Registry | `RegisterIsland`, `GetIsland`, `ListIslands` | Complete |
| Context | `Request(ctx)`, `Writer(ctx)` | Complete |
| Broadcasting infra | `Broadcast`, `BroadcastTransport` interface, `LocalTransport` | Complete |

### V2 Client Library: Complete with Gaps

The TypeScript client in `web/src/` implements island-scoped architecture with 205 passing tests:

**Implemented event directives:**
- `live-click`, `live-contextmenu`, `live-mousedown`, `live-mouseup`
- `live-focus`, `live-blur`
- `live-keydown`, `live-keyup` (with `live-key` filter)
- `live-change`, `live-submit`
- `live-debounce` (milliseconds and "blur" mode)
- `live-value-*` (custom data attributes)
- `live-hook` (full lifecycle: mounted, beforeUpdate, updated, beforeDestroy, destroyed, disconnected, reconnected)

**Implemented features:**
- `<live-island>` custom element with props extraction from `data-*` attributes
- `ConnectionManager` singleton with island registration
- Transport negotiation (WebSocket -> SSE fallback)
- Island-scoped DOM patching (Replace, Append, Prepend actions)
- Form state preservation across patches (dehydrate/hydrate)
- Loading CSS classes (`live-click-loading`, `live-submit-loading`, etc.)
- Hook system with `pushEvent` and `handleEvent` context methods
- File upload detection in forms (XHR-based, basic)

**Not implemented in V2 client:**
- `live-window-keyup` / `live-window-keydown` / `live-window-focus` / `live-window-blur`
- `live-throttle`
- `live-patch` (client-side URL navigation)
- `live-capture-click` (never implemented in master either)
- Page-level connection CSS classes (`live-connected`, `live-disconnected`, `live-error`)
- Redirect event handling (`EventRedirect` constant exists but no handler)
- Upload progress tracking, validation UI, drag-and-drop

### Feature Gap Analysis: Master README vs V2

| Feature | README Status | V2 Status | Assessment |
|---------|--------------|-----------|------------|
| `live-click` | Implemented | Implemented | Parity |
| `live-value-*` | Implemented | Implemented | Parity |
| `live-capture-click` | Not implemented | Not implemented | No regression |
| `live-window-focus` | Implemented | **Missing** | Gap — needs client work |
| `live-window-blur` | Implemented | **Missing** | Gap — needs client work |
| `live-focus` | Implemented | Implemented | Parity |
| `live-blur` | Implemented | Implemented | Parity |
| `live-window-keyup` | Implemented | **Missing** | Gap — needs client work |
| `live-window-keydown` | Implemented | **Missing** | Gap — needs client work |
| `live-keyup` | Implemented | Implemented | Parity |
| `live-keydown` | Implemented | Implemented | Parity |
| `live-key` | Implemented | Implemented | Parity |
| `live-submit` | Implemented | Implemented | Parity |
| `live-change` | Implemented | Implemented | Parity |
| `live-debounce` | Implemented | Implemented | Parity |
| `live-throttle` | Not implemented | Not implemented | No regression |
| `live-update` | Implemented | Implemented (server-side patch actions) | Parity |
| `live-hook` | Implemented | Implemented | Parity |
| `live-patch` | Implemented | **Missing** | Gap — needs client + server |
| HandleParams | Implemented | **Missing** | Intentional removal (islands) |
| PatchURL | Implemented | **Missing** | Intentional removal (islands) |
| Redirect | Implemented | **Partial** (constant only) | Gap — needs handler |
| Uploads | Implemented | **Missing** | Large gap |
| page/ Components | Implemented | **Removed** | Intentional (replaced by islands) |
| WithTemplateRenderer | Implemented | **Removed** | Intentional (use io.Reader) |
| Connection CSS classes | Implemented | **Missing** | Gap — needs client work |
| Loading CSS classes | Implemented | Implemented | Parity |
| PubSub/Broadcasting | Implemented | Implemented (enhanced) | Parity+ |

### Examples: Ported vs Missing

**Ported to V2 (6 examples):**

| V2 Example | Master Origin | Features |
|------------|--------------|----------|
| counter | New | Basic state mutation, inc/dec |
| clock | clock | `SendSelf`, `WithEventDelay`, HandleSelf |
| chat | chat + cluster | Broadcasting, `live-update="append"`, hooks, multi-server |
| forms | todo + prefill | `live-change`, `live-submit`, validation, checkboxes, prefill |
| alpine | alpine | Alpine.js integration, autocomplete |
| hooks | error | Error handling, `WithErrorHandler`, hooks |

**Not ported (require missing V2 features):**

| Master Example | Blocking Feature | Notes |
|----------------|-----------------|-------|
| buttons | `live-window-keyup` | Counter works but keyboard shortcuts need window events |
| pagination | `HandleParams`, `live-patch`, `PatchURL` | URL-driven state — architectural gap |
| uploads | Upload subsystem | Entire pipeline missing |
| clocks | `page/` component system | Could port as multiple islands on one page |
| components | `page/` component library | Reusable widget — could be a shared island constructor |

### Test Coverage State

**Go tests: 381 tests, 82.3% main package coverage**

| Test File | Tests | Coverage Area |
|-----------|-------|---------------|
| `broadcast_test.go` | 8 | Pub/sub, multi-engine scenarios |
| `context_test.go` | 10 | Context utilities |
| `diff_test.go` | 26 | DOM diffing algorithm |
| `engine_test.go` | 15 | Engine lifecycle, sessions |
| `event_test.go` | 2 | Event handling |
| `http_test.go` | 1 | HTTP utilities |
| `instance_test.go` | 14 | Island instance lifecycle |
| `integration_test.go` | 4 | End-to-end scenarios |
| `island_test.go` | 16 | Island definition |
| `params_test.go` | 5 | Parameter parsing |
| `registry_test.go` | 8 | Island registry |
| `render_test.go` | 4 | Rendering system |
| `session_test.go` | 12 | Session management |
| `statestore_test.go` | 16 | State persistence |
| `transport_test.go` | 11 | Transport config |
| `transport_websocket_test.go` | 8 | WebSocket transport |
| `transport_sse_test.go` | 9 | SSE transport |
| `transport_endpoints_test.go` | exists | Endpoint handlers |

**Example tests: 5 of 6 examples covered**
- alpine, chat, clock, forms, hooks — all have `main_test.go`
- counter — **NO test file**

**TypeScript tests: 205 tests, 8 suites, all passing**

| Test Suite | Tests | Coverage |
|------------|-------|----------|
| `events.spec.ts` | 96 | Event wiring, loading classes, debounce |
| `hooks.spec.ts` | 47 | Hook lifecycle, pushEvent/handleEvent |
| `island.spec.ts` | 48 | LiveIsland element, patching |
| `patch.spec.ts` | 24 | DOM patch application |
| `connection.spec.ts` | varies | ConnectionManager |
| `transport/websocket.spec.ts` | varies | WebSocket transport |
| `transport/sse.spec.ts` | varies | SSE transport |
| `transport/negotiator.spec.ts` | varies | Transport fallback |

**Functions with 0% coverage (key gaps):**
- `BroadcastToIsland()` — direct island broadcast untested
- `SetStateTTL()` — state TTL configuration untested
- `WebSocketHandler()`, `SSEHandler()` — HTTP endpoint wrappers untested
- `Javascript.ServeHTTP()` — JS serving untested
- `NewIslandInstanceWithChildren()` — children-based construction untested
- Counter example `NewCounterIsland()` — no test file

### Actionable Implementation Gaps

**To achieve feature parity with master README:**

1. **`live-window-*` events** — Add window-level event listeners to V2 client (`events.ts`). Server needs no changes. Enables buttons example.

2. **Redirect handling** — Implement `EventRedirect` handler in client (`connection.ts` or `island.ts`). Uses `window.location.replace()`. Server already sends the event type.

3. **Connection CSS classes** — Add `live-connected`/`live-disconnected`/`live-error` class management to `connection.ts` or island element. Apply to `document.body` or each `<live-island>`.

4. **`live-throttle`** — Add throttle support alongside existing debounce in `events.ts`. Server needs no changes.

5. **Buttons example** — Requires #1 above. Port counter with keyboard shortcuts.

6. **Clocks example** — Can port NOW as multiple `<live-island type="clock">` elements on one page. No new features needed.

7. **Counter test file** — Write `examples/counter/main_test.go`.

**Larger features (deferred):**

8. **Upload subsystem** — Full pipeline needed: server-side `AllowUploads`/`ConsumeUploads`/`ValidateUploads`, client-side progress tracking, staging area. Large effort.

9. **`live-patch` + `HandleParams`** — URL-driven state for islands. Requires design decision: per-island URL params vs page-level.

10. **Pagination example** — Requires #9 above.

**Test coverage improvements:**

11. Transport endpoint handler tests (`WebSocketHandler`, `SSEHandler`)
12. `BroadcastToIsland()` direct broadcast tests
13. `SetStateTTL()` state expiration tests
14. JavaScript serving endpoint tests
15. `NewIslandInstanceWithChildren()` tests
16. SSE heartbeat edge case coverage improvement

## Code References

- `island.go` — Island definition with all lifecycle handlers and `SendSelf`
- `engine.go` — IslandEngine orchestration, broadcasting, event routing
- `instance.go` — Runtime island instances with `CallEvent`, `CallSelf`
- `session.go` — Transport-agnostic session management
- `transport_websocket.go` — WebSocket transport implementation
- `transport_sse.go` — SSE transport implementation
- `transport_endpoints.go` — HTTP handler factories (untested)
- `broadcast.go` — Pub/sub broadcasting infrastructure
- `statestore.go` — State persistence with TTL
- `diff.go` — DOM diffing with island anchors
- `web/src/events.ts` — Client event wiring (all directives)
- `web/src/island.ts` — `<live-island>` custom element
- `web/src/connection.ts` — ConnectionManager singleton
- `web/src/hooks.ts` — Hook registry and lifecycle
- `web/src/forms.ts` — Form serialization and state preservation
- `web/src/transport/negotiator.ts` — Transport fallback chain
- `examples/counter/main.go` — Basic counter (no test file)

## Architecture Documentation

### V2 Island Architecture (Current)

Every V2 example follows this structure:
```
examples/<name>/
├── main.go          # Island definition + HTTP server
├── index.html       # Page with <live-island> elements
├── <name>.html      # Island render template
└── main_test.go     # Tests
```

Event flow: User interaction → `live-*` attribute → EventWiring captures → ConnectionManager.sendEvent() → Transport sends to server → Engine.RouteEvent() → Island handler → state mutation → RenderIsland() → DiffIsland() → patches sent via session → client applies patches within island DOM → forms rehydrated → hooks executed.

### Intentional V2 Design Departures from V1

1. **Islands replace Handlers**: V1's `NewHandler()` with `MountHandler`/`RenderHandler`/`HandleEvent` becomes V2's `NewIsland()` with `WithMount`/`WithRender`/`HandleEvent` — similar API, different scope (page vs island).

2. **No page-level Socket**: V1's `Socket` with `Assigns()`, `Self()`, `Broadcast()`, `PatchURL()`, `Redirect()` is replaced by context-based `SendSelf()` and engine-level `BroadcastToIslandType()`.

3. **Transport abstraction**: V1 was WebSocket-only. V2 supports WebSocket + SSE with negotiation.

4. **State persistence**: V1 stored state in Socket memory. V2 uses `IslandStateStore` interface with TTL-based cleanup, enabling reconnection state recovery.

## Historical Context (from docs/)

- `docs/research/2026-01-25-islands-component-architecture.md` — Original V2 architecture design
- `docs/research/2026-02-22-v2-branch-state-and-testing-gaps.md` — Previous V2 state analysis (at 283 tests, now 381)
- `docs/research/2026-02-25-v2-examples-porting.md` — Examples porting strategy and decisions

## Related Research

- `docs/research/2026-02-22-v2-branch-state-and-testing-gaps.md` — Wire format analysis, session ID issues
- `docs/research/2026-02-25-v2-examples-porting.md` — Self-event design (`SendSelf`, `WithEventDelay`)

## Resolved Questions

1. **`live-window-*` event scoping: Global listeners, routed to declaring island.** Window events (keyup, keydown, focus, blur) attach to the `window` object globally, but the event is routed to whichever island contains the element with the `live-window-*` attribute. Multiple islands can each declare `live-window-keyup` — all receive the event. This matches V1 behavior (global) while preserving V2's island event routing.

2. **Clocks example: Reuse existing clock island type.** Multiple `<live-island type="clock" id="clock-london" data-timezone="Europe/London">` elements on one page. Each gets its own instance with different props. No new code needed beyond the page template.

3. **Upload support: Yes, implement for V2.** Full upload subsystem including server-side `AllowUploads`/`ConsumeUploads`/`ValidateUploads` and client-side progress tracking.

4. **`live-patch`/`HandleParams`: Yes, implement for V2.** URL-driven state management adapted for islands. Needs design for per-island vs page-level URL params.

5. **Feature scope: Implement everything.** All small gaps (window events, redirect, connection CSS, throttle, clocks example, counter tests, test coverage) plus uploads AND live-patch/HandleParams.

## Open Questions

None — all questions resolved. Ready for implementation planning.
