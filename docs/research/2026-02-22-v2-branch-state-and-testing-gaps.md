---
date: 2026-02-22T12:00:00+00:00
researcher: Josh
topic: "Live v2 Branch: Current State, Testing Completeness, and Remaining Work"
tags: [research, v2, testing, gaps, branch-state, wire-format, session-ids, protobuf]
last_updated: 2026-02-22
last_updated_by: Josh
last_updated_note: "Resolved all 4 open questions: wire format, subscribe protocol, session IDs, polling transport. Added wire format contract analysis."
---

# Research: Live v2 Branch State and Remaining Work

## Research Question

What is the current state of the v2 branch? Is testing complete? What else needs doing?

## Summary

The v2 islands-only architecture is **structurally complete** on the server side. All 25 planned deliverables from the implementation plan are marked done. The Go server compiles cleanly, passes 283 tests with race detection at 82.2% coverage, and has no vet issues.

However, there are **significant gaps between the server and client integration**:

1. The counter example uses a hand-written `custom-island.js` instead of the built v2 client library (`auto.js`)
2. The v2 client library (transport negotiation, island-scoped patching, hooks) has **never been verified working with the Go server**
3. 8 client tests fail in `negotiator.spec.ts` (transport fallback scenarios)
4. There are uncommitted changes (error logging improvements) and untracked test files
5. Only 1 example exists (counter) - no nested islands, forms, broadcasts, or hooks demos
6. **Session ID handling is incompatible between server and client for WebSocket** (see Resolved Questions)

## Detailed Findings

### Go Server: Complete and Tested

All 19 Go source files implement the v2 architecture:

| File | Purpose | Tests |
|------|---------|-------|
| `island.go` | Island definition, handlers, Props | `island_test.go` (10 tests) |
| `instance.go` | Runtime island instances | `instance_test.go` (11 tests) |
| `registry.go` | Thread-safe island registry | `registry_test.go` (8 tests) |
| `engine.go` | IslandEngine orchestrator | `engine_test.go` (7 tests) |
| `session.go` | Transport-agnostic sessions | `session_test.go` (11 tests) |
| `transport.go` | Transport interface | `transport_test.go` (11 tests) |
| `transport_websocket.go` | WebSocket transport | `transport_websocket_test.go` (8 tests) |
| `transport_sse.go` | SSE transport | `transport_sse_test.go` (9 tests) |
| `statestore.go` | Island state persistence | `statestore_test.go` (12 tests) |
| `diff.go` | HTML diffing with island anchors | `diff_test.go` (17 tests) |
| `render.go` | Island rendering pipeline | `render_test.go` (4 tests) |
| `event.go` | Event protocol | `event_test.go` (2 tests) |
| `params.go` | Parameter parsing | `params_test.go` (5 tests) |
| `context.go` | HTTP context utilities | `context_test.go` (6 tests, untracked) |
| `types.go` | IslandID, SessionID aliases | N/A (type aliases) |
| `errors.go` | Error definitions | N/A (constants) |
| `http.go` | HTTP helpers | Tested indirectly |
| `transport_endpoints.go` | HTTP endpoint factories | Tested indirectly |
| `javascript.go` | JS serving handlers | Not tested |

When `go test -race ./...` runs, all 283 tests pass with 82.2% statement coverage.

### Client Library: Complete but Not Integration-Tested

The TypeScript client in `web/src/` is a full v2 rewrite:

| File | Purpose | Tests |
|------|---------|-------|
| `island.ts` | `<live-island>` custom element | `island.spec.ts` |
| `connection.ts` | ConnectionManager singleton | `connection.spec.ts` |
| `events.ts` | Island-scoped event wiring | `events.spec.ts` |
| `patch.ts` | Island-scoped patch application | `patch.spec.ts` |
| `hooks.ts` | Island-aware hook registry | `hooks.spec.ts` |
| `transport/websocket.ts` | WebSocket transport | `websocket.spec.ts` |
| `transport/sse.ts` | SSE transport | `sse.spec.ts` |
| `transport/negotiator.ts` | Auto transport selection | `negotiator.spec.ts` (FAILING) |
| `transport/message.ts` | Wire message types | N/A |
| `transport/transport.ts` | Transport interface | N/A |
| `auto.ts` | Auto-init entry point | N/A |
| `event.ts` | Event dispatch, lifecycle | N/A |
| `forms.ts` | Form state preservation | N/A |
| `element.ts` | Element helpers | N/A |
| `interop.ts` | Backward compat interfaces | N/A |

**Client test results**: 197 pass, 8 fail. All 8 failures are in `negotiator.spec.ts`:
- `should fallback to SSE when WebSocket fails`
- `should track all failed transport types`
- `should timeout and fallback if WebSocket takes too long`
- `should reject when all transports fail`
- `should include failed transport types in error`
- `should pass custom SSE endpoint to transport`
- `should use default endpoints when not specified`
- `should close failed transports during negotiation`

The failures are all related to the WebSocket-to-SSE fallback path, where `"WebSocket connection failed"` is thrown during negotiation cleanup. This suggests a timing/cleanup issue in the negotiator's error handling path.

### Counter Example: Uses Custom JS, Not the Library

The counter example (`examples/counter/`) works but has a critical architectural gap:

- `index.html:117` loads `/custom-island.js` (hand-written, 157 lines)
- It does NOT load `auto.js` (the built v2 client library)
- The custom JS implements a simplified island model:
  - Simple `innerHTML` replacement (`custom-island.js:130`) instead of anchor-based patching
  - No transport negotiation (WebSocket only)
  - No hook support
  - No form state preservation
  - No debounce or loading states
  - No lifecycle events

This means the entire v2 client library - transport negotiation, island-scoped patching, hooks, ConnectionManager, EventWiring - has **never been tested against the actual Go server**.

### Uncommitted Changes

**Modified files** (unstaged):
- `engine.go`: 3 changes upgrading silent error suppression to `slog.Error` logging in `renderAndSendIsland`, `BroadcastToIslandType`, `BroadcastToIsland`
- `transport_websocket.go`: 1 change adding `slog.Error` logging when events channel is full and an event is dropped

**Untracked files**:
- `context_test.go`: 6 tests for HTTP context utilities (170 lines)
- `examples_test.go`: Example documentation for broadcast operations (14 lines, no actual test functions)

### v1 Backup Directory

The `_v1_backup/` directory contains archived v1 files:
- `handler.go.v1`, `socket.go.v1`, `pubsub.go.v1`, `upload.go.v1`, `socketstate.go.v1`
- `page.v1/component.go`, `page.v1/configuration.go`, `page.v1/render.go`
- Test files: `handler_test.go.v1`, `example_test.go.v1`, `socketstate_test.go.v1`, `page.v1/example_test.go`

This directory serves no functional purpose on the v2 branch. The v1 code is preserved in git history and on the master branch.

## Resolved Questions

### 1. Wire Format Compatibility: COMPATIBLE (but fragile)

The Go server and TypeScript client wire formats match:

| Field | Go JSON Output | TypeScript Expects | Match |
|-------|---------------|-------------------|-------|
| `Event.T` | `"t"` (json tag) | `t` | Yes |
| `Event.Island` | `"island"` (json tag) | `island?` | Yes |
| `Event.Data` | `"d"` (json tag) | `d?` | Yes |
| `Patch.Anchor` | `"Anchor"` (no json tag) | `Anchor` | Yes |
| `Patch.Action` | `"Action"` (no json tag, number) | `Action` (PatchAction enum) | Yes |
| `Patch.HTML` | `"HTML"` (no json tag) | `HTML` | Yes |
| `Patch.IslandID` | `"island_id"` (json tag) | `island_id?` | Yes |

**Example server output:**
```json
{
  "t": "patch",
  "island": "counter-1",
  "d": [
    {"Anchor": "_i_counter-1_0", "Action": 1, "HTML": "<span>5</span>", "island_id": "counter-1"}
  ]
}
```

**Fragility risk:** The `Patch` struct's `Anchor`, `Action`, and `HTML` fields have **no json struct tags** (`diff.go:124-139`). Go defaults to the capitalized field name, which happens to match TypeScript. But adding a `json:"anchor"` tag would silently break the client. This should be made intentional by adding explicit json tags that match the current behavior.

### 2. Subscribe Protocol: COMPATIBLE

Both custom-island.js and the v2 client library send identical subscribe messages:

```json
{"t": "subscribe", "island": "counter-1", "d": {"type": "counter"}}
```

**Client library flow** (`connection.ts:203-218`):
1. `LiveIsland.connectedCallback()` calls `connectionManager.registerIsland(id, type, handler)`
2. ConnectionManager calls `subscribeIsland(islandId, islandType)`
3. Sends `{t: "subscribe", island: islandId, d: {type: islandType}}`

**One difference:** The v2 client also sends `{t: "unsubscribe", island: islandId}` on disconnect (`connection.ts:224-237`). The server has no handler for this - events are silently ignored. Island state persists until TTL expiration. This is safe but could be improved.

**Server connect event:** The WebSocket transport sends `{t: "connect"}` after upgrade (`transport_websocket.go:299-303`). The v2 client receives this via its message routing but doesn't depend on it for subscription - subscription is triggered by island registration, not connect events.

### 3. Session ID Alignment: INCOMPATIBLE for WebSocket

This is a real bug that prevents reconnection from restoring state.

**Server behavior** (`examples/counter/main.go:132`):
```go
sessionID := live.SessionID(fmt.Sprintf("session-%d", time.Now().UnixNano()))
```
Server generates a new timestamp-based session ID per WebSocket connection. The client's cookie is ignored.

**Client behavior** (`web/src/transport/websocket.ts:33-49`):
- Reads or generates a UUID v4 session ID
- Stores in `live_session` cookie (60-second TTL)
- Expects the same session to persist across reconnections

**Result:** Every WebSocket connection gets a new server-side session ID. On reconnect, the client sends the same cookie, but the server creates a fresh `session-{timestamp}` and mounts new island instances. State is lost.

**SSE is partially better:** The server has `getSessionIDFromRequest()` (`transport_sse.go:391-405`) that reads the `live_session` cookie or `X-Live-Session` header. However, the counter example doesn't use it - it also generates a timestamp-based ID (`main.go:196`).

**Fix needed:** The server's connection handlers should read the session ID from the client's `live_session` cookie (available on the HTTP upgrade request), falling back to generating one only if no cookie exists. This applies to both WebSocket and SSE handlers.

### 4. Polling Transport: Intentionally Deferred

The server has no polling transport implementation. The client has `TransportType.Polling` in the negotiator enum but no `PollingTransport` class. The implementation plan lists it as "Optional Deliverable." WebSocket + SSE covers virtually all environments, so this is a reasonable deferral.

## Wire Format Contract Analysis

### Current State: JSON with No Formal Schema

The wire protocol uses 6 system message types plus user-defined event names:

| Message Type | Direction | `d` Payload |
|-------------|-----------|-------------|
| `connect` | Server -> Client | None |
| `ack` | Server -> Client | Optional ID |
| `err` | Server -> Client | `{source: Event, err: string}` |
| `patch` | Server -> Client | `[]Patch` (Anchor, Action, HTML, island_id) |
| `params` | Bidirectional | URL params map |
| `redirect` | Server -> Client | URL string |
| User events (`inc`, `dec`, etc.) | Client -> Server | Arbitrary params |

The Go side uses `json.RawMessage` for `Event.Data` and the TypeScript side uses `d?: any`. Neither enforces type safety on the payload.

### Protobuf Assessment: Not Recommended

**Why protobuf is a poor fit for this project:**

1. **String-heavy payloads negate size advantage.** Patches carry raw HTML. For string data, protobuf is only ~84% of gzipped JSON (vs 16% for numeric data). WebSocket per-frame compression handles both equally.

2. **SSE is text-only.** Binary protobuf over SSE requires base64 encoding, negating size savings and adding complexity.

3. **User-defined event names can't be captured in a `.proto` enum.** The `T` field is open-ended by design. Protobuf would only type-check the 6 system message types, not the most important direction (client-to-server user events).

4. **Dependency bloat.** Would add `buf`/`protoc` binary + `@bufbuild/protobuf` npm runtime dep to a project that currently has 3 Go deps and minimal npm deps.

5. **Ecosystem precedent.** Phoenix LiveView shipped JSON after years of development (considered BERT binary, never shipped it). Hotwire/Turbo sends raw HTML over WebSocket. HTMX uses raw HTML fragments. None use binary wire formats.

### Recommended Approach: Explicit JSON Tags + Typed TypeScript

**Tier 1 (do now):** Make the contract intentional without new tooling.

On the Go side, add explicit json tags to the `Patch` struct so the field names are a deliberate choice:
```go
type Patch struct {
    Anchor   string      `json:"Anchor"`
    Action   PatchAction `json:"Action"`
    HTML     string      `json:"HTML"`
    IslandID string      `json:"island_id,omitempty"`
}
```

On the TypeScript side, replace `d?: any` in `TransportMessage` with a discriminated union:
```typescript
interface PatchMessage { t: "patch"; island: string; d: Patch[]; }
interface ErrorMessage { t: "err"; d: { source: TransportMessage; err: string }; }
interface ConnectMessage { t: "connect"; }
// etc.
type ServerMessage = PatchMessage | ErrorMessage | ConnectMessage | ...;
```

**Tier 2 (consider later):** If more message types are added, use **JSON Typedef** (`jtd-codegen`) to generate both Go structs and TypeScript interfaces from a single `.jtd.json` schema file. No wire format change, no runtime deps, single binary tool. Gives compile-time guarantee that both sides match.

## What Needs Doing

### Critical: Client-Server Integration

1. **Fix session ID handling** - Server WebSocket/SSE handlers must read session ID from the client's `live_session` cookie on the HTTP upgrade request, falling back to generating one only if none exists. Without this, reconnection cannot restore state.

2. **Connect the real client library to the counter example** - Replace `custom-island.js` with `auto.js`. This requires verifying that the `<live-island>` custom element, ConnectionManager, transport negotiation, and island-scoped patching all work together with the Go server's event routing and patch generation.

3. **Fix the 8 failing negotiator tests** - The `negotiator.spec.ts` failures indicate a bug in the WebSocket-to-SSE fallback path. Since transport negotiation is a core feature, this needs fixing before the client can be considered complete.

4. **Add explicit json tags to Patch struct** - The current compatibility is accidental (Go defaults to capitalized field names). Adding `json:"Anchor"` etc. makes the contract intentional and prevents future breakage.

### Important: Additional Testing

5. **End-to-end browser test** - Run the counter example with the real client library and verify in a browser that:
   - Islands mount and render initial state from server
   - Click events route to correct island
   - Patches apply correctly within island boundaries
   - Multiple islands operate independently
   - Reconnection works (kill server, restart, verify state)

6. **SSE transport end-to-end** - The SSE transport has server tests and client tests but no end-to-end verification. The counter example serves SSE endpoints but the custom JS doesn't use them.

### Recommended: Examples and Cleanup

7. **More examples** demonstrating:
   - Nested/composed islands (parent renders child `<live-island>` elements)
   - Form handling within islands (text inputs, checkboxes with state preservation)
   - Server-to-client broadcasts (`BroadcastToIslandType`, `BroadcastToIsland`)
   - Hook usage (`live-hook` attribute with lifecycle callbacks)
   - SSE-only island (dashboard/feed pattern)

8. **Remove `_v1_backup/` directory** - v1 code is in git history; no need to keep archived copies on the v2 branch.

9. **Commit uncommitted changes** - The error logging improvements in `engine.go` and `transport_websocket.go` plus the new `context_test.go` should be committed.

10. **Tighten TypeScript types** - Replace `d?: any` in `TransportMessage` with discriminated union types per message type.

### Optional: Coverage Gaps

11. **`transport_endpoints.go` tests** - The HTTP handler factories (`WebSocketHandler`, `SSEHandler`, `UpgradeSSE`) are not directly tested. They're thin wrappers exercised indirectly by integration tests, but direct tests would improve coverage.

12. **`javascript.go` tests** - The JS/sourcemap serving handlers are untested.

## Code References

- `engine.go` - IslandEngine with uncommitted slog.Error improvements
- `transport_websocket.go` - WebSocket transport with uncommitted event drop logging
- `context_test.go` - New untracked test file
- `examples_test.go` - New untracked doc example file
- `examples/counter/custom-island.js` - Hand-written JS bypassing the v2 client library
- `examples/counter/index.html:117` - Loads custom-island.js instead of auto.js
- `examples/counter/main.go:132` - Server generates timestamp-based session IDs (incompatible with client)
- `web/src/transport/negotiator.spec.ts` - 8 failing tests
- `web/src/transport/websocket.ts:33-49` - Client generates UUID session IDs in cookie
- `web/src/transport/message.ts` - Wire message types with `d?: any` (untyped)
- `web/browser/auto.js` - Built v2 client library (19KB minified)
- `diff.go:124-139` - Patch struct with no json tags (fragile)
- `transport_sse.go:391-405` - `getSessionIDFromRequest()` reads cookie/header
- `event.go:45-65` - Event struct with json tags
- `_v1_backup/` - Archived v1 code (can be removed)

## Architecture Documentation

### Current v2 Data Flow (Server)

When a client event arrives:
1. Transport receives JSON message with `island` field
2. `engine.RouteEvent()` looks up session, finds island instance
3. Island's `CallEvent()` updates state
4. `renderAndSendIsland()` calls `RenderIsland()` with new state
5. `DiffIsland()` compares against `lastRenderedHTML`
6. Patches sent via `session.Send()` through transport

### Current v2 Data Flow (Counter Example Client - custom-island.js)

1. Custom element `connectedCallback` sends `subscribe` message
2. Server mounts island, renders initial HTML, sends patch
3. Client receives patch, does `island.innerHTML = message.d.html` (full replacement)
4. Click events send `{t: eventType, island: id, d: {}}` via WebSocket
5. Server routes event, updates state, sends new patch

### Expected v2 Data Flow (Library Client - auto.js)

1. `<live-island>` custom element `connectedCallback` registers with ConnectionManager
2. ConnectionManager negotiates transport (WebSocket -> SSE fallback)
3. Sends subscribe via transport
4. Server mounts island, renders, sends patches
5. `applyIslandPatches()` applies anchor-based patches within island DOM
6. `wireIslandEvents()` wires `live-click`/`live-submit`/etc. within island
7. Events sent via ConnectionManager through negotiated transport
8. Hooks executed on mount/update/destroy lifecycle

### Session ID Flow (Current - Broken)

```
Client                          Server
  |                               |
  |-- WebSocket Upgrade --------->|
  |   (Cookie: live_session=UUID) |
  |                               |-- Ignores cookie
  |                               |-- Generates session-{timestamp}
  |<-- {t: "connect"} -----------|
  |                               |
  |-- {t: "subscribe"} --------->|-- MountIsland(session-{timestamp}, ...)
  |                               |
  |   ... connection drops ...    |
  |                               |-- DeleteSession(session-{timestamp})
  |                               |
  |-- WebSocket Upgrade --------->|
  |   (Cookie: live_session=UUID) |  (same UUID!)
  |                               |-- Ignores cookie AGAIN
  |                               |-- Generates session-{NEW timestamp}
  |                               |-- State from old session is LOST
```

### Session ID Flow (Fixed)

```
Client                          Server
  |                               |
  |-- WebSocket Upgrade --------->|
  |   (Cookie: live_session=UUID) |
  |                               |-- Reads cookie: UUID
  |                               |-- Uses UUID as SessionID
  |<-- {t: "connect"} -----------|
  |                               |
  |-- {t: "subscribe"} --------->|-- MountIsland(UUID, ...)
  |                               |
  |   ... connection drops ...    |
  |                               |-- Keeps session in store (TTL)
  |                               |
  |-- WebSocket Upgrade --------->|
  |   (Cookie: live_session=UUID) |  (same UUID!)
  |                               |-- Reads cookie: UUID
  |                               |-- Finds existing session
  |                               |-- Restores island state from store
```

## External Context

### Wire Format Precedent in LiveView-Style Frameworks

- **Phoenix LiveView**: JSON arrays over WebSocket. Custom diff protocol. Considered BERT binary encoding (community project showed 2-10x faster encoding) but never shipped it in mainline. Chose JSON pragmatism.
- **Hotwire/Turbo**: Raw HTML strings wrapped in `<turbo-stream>` XML tags over WebSocket/SSE. Zero schema contract.
- **HTMX**: Raw HTML fragments. No wire format contract at all.
- **Conclusion**: None of the major server-rendered real-time frameworks use binary wire formats. JSON (or raw HTML) is the standard.

### Schema Contract Options (Evaluated)

| Approach | Effort | Value for Live v2 | New Dependencies |
|----------|--------|-------------------|------------------|
| Explicit json tags + TS union types | Low | High - prevents accidental breakage | None |
| JSON Typedef (`jtd-codegen`) | Moderate | Good - compile-time guarantee | `jtd-codegen` binary (build only) |
| Protobuf (`buf` + `protobuf-es`) | High | Modest - string-heavy payloads negate size advantage | `buf` binary, `@bufbuild/protobuf` npm |
| Flatbuffers | Very high | Minimal - complex API, larger wire format | Multiple |

## Historical Context (from docs/)

- `docs/research/2026-01-25-islands-component-architecture.md` - Original v2 architecture research
- `docs/plans/v2-islands-architecture.md` - Implementation plan with 25 deliverables (all marked complete)
