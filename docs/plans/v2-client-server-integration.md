# Implementation Plan: v2 Client-Server Integration

Fix the critical gaps between the Go server and TypeScript client library, then connect the real v2 client library (`auto.js`) to the counter example, replacing the hand-written `custom-island.js`.

## Context

**Research Document**: `docs/research/2026-02-22-v2-branch-state-and-testing-gaps.md`

**Key Files**:
- `diff.go:124-139` - Patch struct with no json tags (fragile wire format)
- `http.go` - HTTP helpers (needs shared session ID extraction)
- `transport_sse.go:391-405` - Private `getSessionIDFromRequest()` (needs exporting)
- `web/src/transport/negotiator.ts:142-148` - Negotiator fallback bug
- `web/src/transport/websocket.ts:127` - Unhandled reconnection promise
- `web/src/transport/sse.ts:136` - Same unhandled reconnection promise
- `web/src/island.ts` - LiveIsland custom element (missing event wiring)
- `web/src/events.ts` - EventWiring (exists, tested, never called by LiveIsland)
- `examples/counter/main.go` - Counter example server
- `examples/counter/index.html` - Counter HTML template
- `examples/counter/custom-island.js` - Hand-written client (to be removed)
- `javascript.go` - `live.Javascript{}` handler for serving auto.js

**Architectural Notes**:
- Wire format is JSON. Patch struct fields serialize as capitalized (`Anchor`, `Action`, `HTML`) due to missing json tags - accidental compatibility with TypeScript client.
- Session IDs are incompatible: client generates UUIDs in `live_session` cookie, server ignores cookie and generates timestamp-based IDs. Reconnection cannot restore state.
- 8 negotiator tests fail due to unhandled promise rejections from WebSocket reconnection attempts during transport fallback.
- LiveIsland custom element registers with ConnectionManager and applies patches, but never calls `wireIslandEvents()` to attach `live-click`/`live-submit` handlers. Buttons won't work.
- Counter example SSE endpoints (`/sse`, `/sse/post`) don't match client defaults (`/live/sse`, `/live/post`).

**Functional Requirements** (EARS notation):
- When a `Patch` struct is JSON-serialized, the system shall produce field names `Anchor`, `Action`, `HTML`, and `island_id` matching the TypeScript client expectations.
- When a WebSocket or SSE connection is established, the system shall read the session ID from the client's `live_session` cookie, falling back to generating a new ID if no cookie exists.
- When a transport reconnection attempt fails after explicit `close()`, the system shall suppress the rejection to prevent unhandled promise errors.
- When a `<live-island>` element connects, the system shall wire `live-click` and other event handlers within the island's DOM boundary.
- When a patch is applied to a `<live-island>`, the system shall re-wire event handlers on the new DOM content.

## Batch Size

| Metric | Count | Rating |
|--------|-------|--------|
| Tasks | 11 | Large |
| Files | 14 | Large |
| Stages | 3 | Medium |

**Overall: Large**

## Execution Stages

### Stage 1: Fix Foundational Issues

All tasks in this stage are independent and can run in parallel.

#### Test Creation Phase (parallel)

- T-test-1a: Write JSON serialization contract test for Patch struct (diff_test.go)
- T-test-1b: Write GetSessionIDFromRequest tests (http_test.go)

#### Implementation Phase (parallel, depends on Test Creation Phase)

- T-impl-1a: Add explicit JSON tags to Patch struct (diff.go)
- T-impl-1b: Export GetSessionIDFromRequest to http.go (http.go, transport_sse.go)
- T-impl-1c: Fix unhandled reconnection promises in transports (websocket.ts, sse.ts)
- T-impl-1d: Fix negotiator test mocks (negotiator.spec.ts)
- T-impl-1e: Wire events in LiveIsland custom element (island.ts)

### Stage 2: Build Client Bundle (depends on Stage 1)

#### Implementation Phase

- T-impl-2a: Rebuild auto.js bundle (web/browser/auto.js)

### Stage 3: Integrate Counter Example (depends on Stage 2)

#### Implementation Phase

- T-impl-3a: Update counter example to use auto.js, cookie sessions, and correct endpoints (main.go, index.html)
- T-impl-3b: Delete custom-island.js

## Task List

### Wire Format Contract

- [ ] Add explicit JSON tags to Patch struct (`diff.go`) [Stage 1]
  - Files: `diff.go` (modifies)
  - Change `Anchor string` to `Anchor string \`json:"Anchor"\``
  - Change `Action PatchAction` to `Action PatchAction \`json:"Action"\``
  - Change `HTML string` to `HTML string \`json:"HTML"\``
  - Keep existing `IslandID string \`json:"island_id,omitempty"\`` unchanged
  - Tags MUST use capitalized names to match TypeScript client (`web/src/transport/message.ts:27-29`)

- [ ] Add JSON serialization contract test (`diff_test.go`) [Stage 1]
  - Files: `diff_test.go` (modifies)
  - Marshal a `Patch` to JSON, unmarshal into `map[string]interface{}`
  - Assert exact key names: `"Anchor"`, `"Action"`, `"HTML"`, `"island_id"`
  - Assert `"island_id"` is omitted when `IslandID` is empty
  - Follow pattern of existing `TestIslandPatchJSON` test

### Session ID Handling

- [ ] Export `GetSessionIDFromRequest` to `http.go` (`http.go`, `transport_sse.go`) [Stage 1]
  - Files: `http.go` (modifies), `transport_sse.go` (modifies)
  - Move `getSessionIDFromRequest` from `transport_sse.go:391-405` to `http.go`
  - Rename to `GetSessionIDFromRequest` (exported)
  - Update call sites in `transport_sse.go` (lines 311, 343)
  - Remove old private function from `transport_sse.go`

- [ ] Add tests for `GetSessionIDFromRequest` (`http_test.go`) [Stage 1]
  - Files: `http_test.go` (creates)
  - Test cookie extraction (`live_session` cookie)
  - Test header extraction (`X-Live-Session`)
  - Test cookie takes priority over header
  - Test empty return when neither exists

### Transport Negotiation Fix

- [ ] Add `.catch()` to reconnection `connect()` in WebSocketTransport (`web/src/transport/websocket.ts`) [Stage 1]
  - Files: `web/src/transport/websocket.ts` (modifies)
  - At line 127, change `this.connect();` to `this.connect().catch(() => {});`
  - Prevents unhandled promise rejections when `close()` is called during pending reconnection

- [ ] Add `.catch()` to reconnection `connect()` in SSETransport (`web/src/transport/sse.ts`) [Stage 1]
  - Files: `web/src/transport/sse.ts` (modifies)
  - At line 136, change `this.connect();` to `this.connect().catch(() => {});`
  - Same pattern as WebSocket fix

- [ ] Fix negotiator test mocks for timeout scenarios (`web/src/transport/negotiator.spec.ts`) [Stage 1]
  - Files: `web/src/transport/negotiator.spec.ts` (modifies)
  - "should timeout and fallback" test (lines 248-274): Replace broken `MockWebSocket.prototype.constructor` override with a full global `WebSocket` replacement class that never fires `open`
  - "should use custom timeout value" test (lines 276-308): Same fix for both WebSocket and EventSource mocks
  - `prototype.constructor` override does NOT change `new` behavior in JavaScript - must replace the global constructor entirely

### LiveIsland Event Wiring

- [ ] Wire `live-click` and other events in LiveIsland (`web/src/island.ts`) [Stage 1]
  - Files: `web/src/island.ts` (modifies)
  - Import `wireIslandEvents` from `./events`
  - Add `private eventCleanup: (() => void) | null = null;`
  - In `connectedCallback()`, after registration: `this.eventCleanup = wireIslandEvents(this, this.islandId);`
  - In `handlePatch()`, after applying patches: clean up old wiring, re-wire new DOM
  - In `disconnectedCallback()`: call cleanup function
  - Without this, `live-click` buttons in the counter example will render but do nothing

### Client Bundle

- [ ] Rebuild auto.js bundle [Stage 2] *(build step, not code edit)*
  - Files: `web/browser/auto.js` (modifies), `web/browser/auto.js.map` (modifies)
  - Shell command: `cd web && npm run build`
  - Must happen AFTER island.ts changes (Stage 1) but BEFORE Go compilation (Stage 3)
  - `javascript.go` embeds `web/browser/auto.js` at compile time

### Counter Example Integration

- [ ] Update counter example to use auto.js and cookie-based sessions (`examples/counter/main.go`, `examples/counter/index.html`) [Stage 3]
  - Files: `examples/counter/main.go` (modifies), `examples/counter/index.html` (modifies)
  - **main.go changes:**
    - Replace `/custom-island.js` handler with `http.Handle("/live.js", live.Javascript{})`
    - Change SSE endpoint from `/sse` to `/live/sse` (matches client default)
    - Change SSE POST from `/sse/post` to `/live/post` (matches client default)
    - Note: `sseFactory.HandlePost` is path-agnostic (it processes the request body, not the URL path), so changing endpoints is safe
    - Replace session ID generation in both WS and SSE handlers with:
      ```go
      sessionID := live.SessionID(live.GetSessionIDFromRequest(r))
      if sessionID == "" {
          sessionID = live.SessionID(fmt.Sprintf("session-%d", time.Now().UnixNano()))
      }
      ```
  - **index.html changes:**
    - Change `<script src="/custom-island.js"></script>` to `<script src="/live.js"></script>`

- [ ] Delete custom-island.js (`examples/counter/custom-island.js`) [Stage 3]
  - Files: `examples/counter/custom-island.js` (deletes)
  - Replaced by auto.js (the v2 client library)

## Acceptance Criteria

```gherkin
Feature: v2 Client-Server Integration

  Scenario: Patch struct JSON serialization matches wire format contract
    Given a Patch struct with Anchor "_i_test_0", Action Replace, HTML "<span>5</span>"
    When the struct is JSON-marshaled
    Then the JSON contains keys "Anchor", "Action", "HTML" with correct values
    And "island_id" is omitted when IslandID is empty

  Scenario: Server reads session ID from client cookie on WebSocket upgrade
    Given a client with "live_session" cookie set to "abc-123"
    When the client opens a WebSocket connection
    Then the server uses "abc-123" as the session ID

  Scenario: Server generates session ID when no cookie exists
    Given a client with no "live_session" cookie
    When the client opens a WebSocket connection
    Then the server generates a new session ID

  Scenario: Transport negotiator falls back to SSE when WebSocket fails
    Given WebSocket connections are blocked
    When the client negotiates a transport
    Then the negotiator falls back to SSE transport
    And no unhandled promise rejections occur

  Scenario: Transport negotiator handles connection timeout
    Given WebSocket connection hangs without responding
    When the negotiator timeout expires
    Then the negotiator falls back to the next transport
    And the hanging transport is closed

  Scenario: LiveIsland wires click events after mount
    Given a <live-island> with a <button live-click="inc"> inside
    When the island connects and receives initial HTML
    Then clicking the button sends an "inc" event to the server

  Scenario: LiveIsland re-wires events after patch
    Given a mounted <live-island> with click handlers wired
    When a patch replaces the island's DOM content
    Then click handlers are re-wired on the new DOM elements

  Scenario: Counter example works end-to-end with auto.js
    Given the counter example server is running
    When a browser loads the page
    Then three independent counter islands render with initial values
    And clicking increment/decrement updates the correct counter
    And other counters are unaffected

  Scenario: Counter example serves auto.js instead of custom-island.js
    Given the counter example server is running
    When the browser requests /live.js
    Then the server returns the v2 auto.js client library
```

**Source**: Generated from plan context and research document findings.

## Implementation Notes

### Cross-cutting Concerns

1. **Build ordering**: TypeScript changes (Stage 1) must be built into auto.js (Stage 2) before Go compilation (Stage 3), because `javascript.go` embeds `web/browser/auto.js` via `//go:embed`.

2. **Wire format backward compatibility**: The JSON tag values MUST be capitalized (`"Anchor"`, `"Action"`, `"HTML"`) to match the TypeScript client. Using lowercase would silently break all clients.

3. **Cookie timing on first visit**: On the very first page load, no `live_session` cookie exists yet (client JS hasn't run). The WebSocket/SSE connection happens after JS loads, so the cookie is set by the transport constructor before the connection upgrade request. The server fallback to generate a new ID handles the edge case.

4. **Cookie TTL is 60 seconds**: The client sets `Max-Age=60` on the `live_session` cookie. If a tab is idle for >60s, the cookie expires and reconnection gets a new session. This is a pre-existing design choice, not addressed in this plan.

5. **`wireIslandEvents` cleanup**: The cleanup function from previous wiring MUST be called before re-wiring after a patch, to avoid duplicate event listeners on elements that survived the patch.

### Pre-work (can be done independently before or after this plan)

These are git operations, not implementation tasks:

- **Commit existing Go changes**: `engine.go` (slog.Error improvements) and `transport_websocket.go` (event drop logging) have uncommitted changes that should be committed: `fix(v2): add error logging for broadcast failures and event drops`
- **Commit new test files**: `context_test.go` and `examples_test.go` are untracked: `test(v2): add context utility tests and broadcast doc examples`
- **Remove `_v1_backup/` directory**: Dead code on v2 branch: `chore(v2): remove v1 backup directory`

### Deferred (not in this plan)

- **Tighten TypeScript `TransportMessage` types**: Replace `d?: any` with discriminated union per message type. Low risk to defer since the wire format is compatible. Would add type safety for future development.
- **Polling transport**: Intentionally deferred per original plan. WebSocket + SSE covers virtually all environments.
- **Additional examples**: Nested islands, forms, broadcasts, hooks. Can be added after this plan validates the integration works.

## Refs

- Research document: `docs/research/2026-02-22-v2-branch-state-and-testing-gaps.md`
- Original architecture research: `docs/research/2026-01-25-islands-component-architecture.md`
- Implementation plan: `docs/plans/v2-islands-architecture.md`
