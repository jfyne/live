---
date: 2026-02-22T12:00:00+00:00
researcher: Josh
topic: "Live v2 Branch: Current State, Testing Completeness, and Remaining Work"
tags: [research, v2, testing, gaps, branch-state]
last_updated: 2026-02-22
last_updated_by: Josh
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

## What Needs Doing

### Critical: Client-Server Integration

1. **Connect the real client library to the counter example** - Replace `custom-island.js` with `auto.js`. This requires verifying that the `<live-island>` custom element, ConnectionManager, transport negotiation, and island-scoped patching all work together with the Go server's event routing and patch generation.

2. **Fix the 8 failing negotiator tests** - The `negotiator.spec.ts` failures indicate a bug in the WebSocket-to-SSE fallback path. Since transport negotiation is a core feature, this needs fixing before the client can be considered complete.

3. **Verify patch protocol compatibility** - The Go server sends `Event{T: "patch", Island: "...", Data: [patches]}` where patches use anchor-based targeting. The client's `applyIslandPatches()` expects `IslandPatch{island, patches: Patch[]}`. These wire formats need to be verified as compatible.

### Important: Additional Testing

4. **End-to-end browser test** - Run the counter example with the real client library and verify in a browser that:
   - Islands mount and render initial state from server
   - Click events route to correct island
   - Patches apply correctly within island boundaries
   - Multiple islands operate independently
   - Reconnection works (kill server, restart, verify state)

5. **SSE transport end-to-end** - The SSE transport has server tests and client tests but no end-to-end verification. The counter example serves SSE endpoints but the custom JS doesn't use them.

### Recommended: Examples and Cleanup

6. **More examples** demonstrating:
   - Nested/composed islands (parent renders child `<live-island>` elements)
   - Form handling within islands (text inputs, checkboxes with state preservation)
   - Server-to-client broadcasts (`BroadcastToIslandType`, `BroadcastToIsland`)
   - Hook usage (`live-hook` attribute with lifecycle callbacks)
   - SSE-only island (dashboard/feed pattern)

7. **Remove `_v1_backup/` directory** - v1 code is in git history; no need to keep archived copies on the v2 branch.

8. **Commit uncommitted changes** - The error logging improvements in `engine.go` and `transport_websocket.go` plus the new `context_test.go` should be committed.

### Optional: Coverage Gaps

9. **`transport_endpoints.go` tests** - The HTTP handler factories (`WebSocketHandler`, `SSEHandler`, `UpgradeSSE`) are not directly tested. They're thin wrappers exercised indirectly by integration tests, but direct tests would improve coverage.

10. **`javascript.go` tests** - The JS/sourcemap serving handlers are untested.

## Code References

- `engine.go` - IslandEngine with uncommitted slog.Error improvements
- `transport_websocket.go` - WebSocket transport with uncommitted event drop logging
- `context_test.go` - New untracked test file
- `examples_test.go` - New untracked doc example file
- `examples/counter/custom-island.js` - Hand-written JS bypassing the v2 client library
- `examples/counter/index.html:117` - Loads custom-island.js instead of auto.js
- `web/src/transport/negotiator.spec.ts` - 8 failing tests
- `web/browser/auto.js` - Built v2 client library (19KB minified)
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

## Open Questions

1. **Wire format compatibility**: Does the Go server's patch event format match what the client's `applyIslandPatches()` expects? The server sends `Event{T: "patch", Island: "...", Data: marshaled-patches}` - does the client correctly parse `Data` as `Patch[]`?

2. **Subscribe protocol**: The counter example uses `{t: "subscribe", island: id, d: {type: "counter"}}`. Does the auto.js client send the same subscribe format? The ConnectionManager's registration flow needs to match the server's expectation.

3. **Session ID alignment**: The custom JS doesn't manage session IDs (server generates them per WebSocket). The auto.js client uses cookie-based session IDs. Are these compatible with the server's session management?

4. **Polling transport**: The plan mentions polling as optional. The server has no polling transport implementation. The client has `TransportType.Polling` in the negotiator but no `PollingTransport` class. Is this intentionally deferred?
