# Implementation Plan: V2 Feature Parity with Master README

Implement all features documented in the master branch README that are missing from V2, plus full test coverage for existing gaps. This covers client-side event directives, server-side navigation/upload APIs, new examples, and test coverage improvements.

## Context

**Research Document**: `docs/research/2026-03-01-v2-feature-gaps-and-test-coverage.md`

**Key Files**:
- `web/src/events.ts` - Client event wiring (EventWiring class, Debouncer, wireStandardEvent/wireKeyEvent patterns)
- `web/src/connection.ts` - ConnectionManager singleton, message routing via routeMessage()
- `web/src/event.ts` - Connection CSS class constants and EventDispatch methods (ClassConnected, ClassDisconnected, ClassError; disconnected(), reconnected(), error() static methods that already manage body classes)
- `web/src/transport/message.ts` - TransportMessage interface, MessageType (already includes Redirect, Params)
- `web/src/forms.ts` - Form utilities with basic XHR file upload
- `island.go` - Island definition, handlers, IslandConfig pattern
- `instance.go` - IslandInstance runtime, CallEvent, CallSelf
- `session.go` - Session event routing, routeToIsland
- `engine.go` - IslandEngine orchestration, RouteEvent, BroadcastToIsland
- `context.go` - Context keys for session, island, engine
- `event.go` - Event types including EventParams, EventRedirect (already defined)
- `transport_endpoints.go` - HTTP handler wrappers for WebSocket/SSE
- `javascript.go` - JS/source map serving handlers

**Architectural Notes**:
- V2 uses island-scoped architecture where each island has isolated state and lifecycle
- EventWiring pattern: query elements within island, attach listeners, push cleanup functions
- Island config pattern: `IslandConfig func(i *Island) error` with `With*()` builders
- Context enrichment: engine adds sessionID, islandID, engine ref, selfEventQueue to context before routing
- Event wire format: `Event{T: "type", Island: "id", Data: json.RawMessage}`
- MessageType constants for Params and Redirect already exist in both Go and TypeScript
- `EventDispatch` in `event.ts` already has `disconnected()`, `reconnected()`, `error()` static methods that manage CSS classes on `document.body` -- ConnectionManager should call these existing methods rather than duplicating logic

**Functional Requirements** (EARS notation):
- When a user presses a key and an element has `live-window-keyup`, the system shall fire a window-level keyboard event routed to the declaring island
- When a user clicks an element with `live-window-focus`, the system shall fire a window-level focus event routed to the declaring island
- When a user clicks an anchor with `live-patch`, the system shall update the browser URL and send a params event to the server
- When an event handler calls `PatchURL(ctx, values)`, the system shall send an EventParams event to the client, which updates the browser URL via history.pushState
- When an event handler calls `Redirect(ctx, url)`, the system shall send an EventRedirect event to the client causing browser navigation
- When an island has a params handler and receives an EventParams event, the system shall route it to the params handler
- When a form with file inputs is submitted, the system shall upload files via multipart POST, validate, stage, and make them available for consumption
- While a `live-throttle` attribute is present, the system shall fire the event immediately then rate-limit at the specified interval
- While the transport is connected, the system shall add `live-connected` class to document.body
- While the transport is disconnected, the system shall add `live-disconnected` class to document.body
- If the server sends an error event, then the system shall add `live-error` class to document.body

## Batch Size

| Metric | Count | Rating |
|--------|-------|--------|
| Tasks | 33 | Large |
| Files | 32 | Large |
| Stages | 6 | Large |

**Overall: Large** (proceeding as single plan per user preference)

## Execution Stages

### Stage 1: Client Event Directives + Test Coverage Foundation

#### Test Creation Phase (parallel)
- T1: Write tests for Throttler, window events, and live-patch (hmm-test-writer)
  - New feature tests (RED): Scenarios 1-6, 16
  - Files: `web/src/events.spec.ts` (modifies)
- T2: Write Go tests for existing coverage gaps (hmm-test-writer)
  - Regression tests: transport endpoints, JS serving, BroadcastToIsland, SetStateTTL, children, counter example
  - Files: `transport_endpoints_test.go` (creates), `javascript_test.go` (creates), `engine_test.go` (modifies), `instance_test.go` (modifies), `examples/counter/main_test.go` (creates)

#### Implementation Phase (parallel, depends on Test Creation Phase)
- T3: Implement Throttler, window events, and live-patch in events.ts (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
  - Files: `web/src/events.ts` (modifies)
- T4: Implement Go test coverage improvements (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
  - Files: `transport_endpoints_test.go`, `javascript_test.go`, `engine_test.go`, `instance_test.go`, `examples/counter/main_test.go`

### Stage 2a: Client Connection Features (depends on Stage 1)

#### Test Creation Phase
- T5: Write tests for connection CSS classes, redirect, and params handling (hmm-test-writer)
  - New feature tests (RED): Scenarios 7-10, 15, 17
  - Files: `web/src/connection.spec.ts` (modifies)

#### Implementation Phase (depends on Test Creation Phase)
- T6: Implement connection CSS, redirect, and params in connection.ts (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
  - Files: `web/src/connection.ts` (modifies)

### Stage 2b: Server HandleParams/PatchURL/Redirect (depends on Stage 1, parallel with Stage 2a)

#### Test Creation Phase
- T7: Write Go tests for HandleParams, PatchURL, Redirect (hmm-test-writer)
  - New feature tests (RED): Scenarios 11-14
  - Files: `island_test.go` (modifies), `engine_test.go` (modifies), `instance_test.go` (modifies)

#### Implementation Phase (depends on Test Creation Phase)
- T8: Implement HandleParams, PatchURL, Redirect in Go (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
  - Files: `island.go` (modifies), `instance.go` (modifies), `session.go` (modifies), `context.go` (modifies)

### Stage 3: Upload System (depends on Stage 2b)

#### Test Creation Phase
- T9: Write Go and client tests for upload system (hmm-test-writer)
  - New feature tests (RED): Scenarios 18-21
  - Files: `upload_test.go` (creates), `web/src/events.spec.ts` (modifies)

#### Implementation Phase (depends on Test Creation Phase)
- T10: Implement upload system in Go and client (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
  - Files: `upload.go` (creates), `island.go` (modifies), `engine.go` (modifies), `transport_endpoints.go` (modifies), `web/src/events.ts` (modifies)

### Stage 4: Buttons + Clocks Examples (depends on Stage 1)

#### Test Creation Phase (parallel)
- T11: Write tests for buttons example (hmm-test-writer)
  - Files: `examples/buttons/main_test.go` (creates)
- T12: Write tests for clocks example (hmm-test-writer)
  - Files: `examples/clocks/main_test.go` (creates)

#### Implementation Phase (parallel, depends on Test Creation Phase)
- T13: Implement buttons example (hmm-implement-worker, TDD mode)
  - Files: `examples/buttons/main.go` (creates), `examples/buttons/buttons.html` (creates), `examples/buttons/index.html` (creates)
- T14: Implement clocks example (hmm-implement-worker, TDD mode)
  - Files: `examples/clocks/main.go` (creates), `examples/clocks/index.html` (creates)

### Stage 5: Pagination + Uploads Examples (depends on Stage 2b and Stage 3 respectively)

#### Test Creation Phase (parallel)
- T15: Write tests for pagination example (hmm-test-writer)
  - Files: `examples/pagination/main_test.go` (creates)
- T16: Write tests for uploads example (hmm-test-writer)
  - Files: `examples/uploads/main_test.go` (creates)

#### Implementation Phase (parallel, depends on Test Creation Phase)
- T17: Implement pagination example (hmm-implement-worker, TDD mode)
  - Files: `examples/pagination/main.go` (creates), `examples/pagination/pagination.html` (creates), `examples/pagination/index.html` (creates)
- T18: Implement uploads example (hmm-implement-worker, TDD mode)
  - Files: `examples/uploads/main.go` (creates), `examples/uploads/uploads.html` (creates), `examples/uploads/index.html` (creates)

### Stage 6: JS Rebuild (depends on all above)

#### Implementation Phase
- T19: Rebuild auto.js with all client-side changes (hmm-implement-worker)
  - Files: `web/browser/auto.js` (modifies), `web/browser/auto.js.map` (modifies)

## Task List

### Client-Side: Event Directives (events.ts)

All events.ts changes are consolidated into single test + implementation tasks to avoid write-write conflicts.

- [ ] T1: Write tests for throttle, window events, and live-patch [Stage 1]
  - Files: `web/src/events.spec.ts` (modifies)
  - **Throttle tests**: immediate first fire, rate-limiting subsequent fires, trailing fire after interval, throttle precedence over debounce, throttle with click/key/change events, cleanup of throttle timers.
  - **Window event tests**: window-focus/blur fires on window events, window-keydown/keyup with key data, live-key filter on window events, cleanup removes window listeners, loading classes, multiple islands receive same window event, events scoped to declaring island.
  - **Live-patch tests**: click prevents default, updates URL via pushState, sends params event with extracted URL params, handles elements without href.
  - Tests are written RED before implementation.

- [ ] T3: Implement Throttler, window events, and live-patch in events.ts [Stage 1, depends: T1]
  - Files: `web/src/events.ts` (modifies)
  - **Throttler class**: Add alongside existing Debouncer. Uses `WeakMap<Element, number>` for per-element `lastFire` timestamps. Methods: `hasThrottle(element)`, `throttle(element, e, fn)`, `cleanup(element)`. Fires immediately on first call, rate-limits subsequent at interval. Stores pending trailing timer. Note: throttle state is per-element-reference via WeakMap, which resets on DOM replacement during patches -- this is acceptable behavior matching debounce.
  - **Throttle integration**: Add `private throttler: Throttler` to EventWiring. Modify `wireStandardEvent`, `wireKeyEvent`, `wireChangeEvents` to check throttle before debounce. Throttle takes precedence.
  - **Window focus/blur**: Add `wireWindowStandardEvent(eventType, attribute)` private method that queries within island but attaches to `window`. Add `wireWindowFocusEvents()` and `wireWindowBlurEvents()`.
  - **Window keydown/keyup**: Add `wireWindowKeyEvent(eventType, attribute)` with `live-key` filter and keyboard metadata. Add `wireWindowKeydownEvents()` and `wireWindowKeyupEvents()`.
  - **Live-patch**: Add `wirePatchEvents()` method for `[live-patch]` anchor elements. On click: prevent default, read `href`, call `history.pushState`, extract URL params, send params event.
  - All new methods called from `wire()`. All push cleanup functions.

### Client-Side: Connection Features (connection.ts)

All connection.ts changes are consolidated into single test + implementation tasks.

- [ ] T5: Write tests for connection CSS, redirect, and params [Stage 2a]
  - Files: `web/src/connection.spec.ts` (modifies)
  - **Connection CSS tests**: live-connected on connect, live-disconnected on disconnect, class swap on reconnect, live-error on error message, error cleared on reconnect.
  - **Redirect tests**: redirect calls window.location.replace, redirect does not call island handlers.
  - **Params tests**: incoming params message updates browser URL via pushState.
  - Tests are written RED before implementation.

- [ ] T6: Implement connection CSS, redirect, and params in connection.ts [Stage 2a, depends: T5]
  - Files: `web/src/connection.ts` (modifies)
  - **Connection CSS**: Call existing `EventDispatch.reconnected()` (adds ClassConnected, removes ClassDisconnected) when state is Connected. Call `EventDispatch.disconnected()` when Closed/Reconnecting. Do NOT duplicate CSS class logic -- `EventDispatch` in `event.ts` already manages these classes; ConnectionManager just needs to call the appropriate static methods at the right lifecycle points.
  - **Error CSS**: In `routeMessage()`, when `message.t === MessageType.Error`, call `EventDispatch.error()` to add ClassError. Still route to island handler if `message.island` set.
  - **Redirect**: In `routeMessage()`, when `message.t === MessageType.Redirect`, call `window.location.replace(message.d)` and return.
  - **Params**: In `routeMessage()`, when `message.t === MessageType.Params`, update browser URL via `history.pushState` using `window.location.pathname + "?" + message.d`.

### Server-Side: HandleParams + PatchURL + Redirect

All island.go, instance.go, session.go, context.go changes consolidated.

- [ ] T7: Write Go tests for HandleParams, PatchURL, Redirect [Stage 2b]
  - Files: `island_test.go` (modifies), `engine_test.go` (modifies), `instance_test.go` (modifies)
  - **HandleParams tests**: handler registration, GetParamsHandler, EventParams routing through engine, state update on params event.
  - **PatchURL tests**: handler calls PatchURL, transport receives EventParams with correct encoded values.
  - **Redirect tests**: handler calls Redirect, transport receives EventRedirect with URL.
  - **Edge case tests**: nil params handler is no-op (no error), PatchURL/Redirect with missing context values.
  - Tests are written RED before implementation.

- [ ] T8: Implement HandleParams, PatchURL, Redirect in Go [Stage 2b, depends: T7]
  - Files: `island.go` (modifies), `instance.go` (modifies), `session.go` (modifies), `context.go` (modifies)
  - **island.go**: Add `paramsHandler IslandEventHandler` field to Island struct. Add `HandleParams(handler IslandEventHandler)` method (mutex-guarded). Add `WithHandleParams(handler IslandEventHandler) IslandConfig`. Add `GetParamsHandler() IslandEventHandler` accessor. Add `PatchURL(ctx context.Context, values url.Values)` function -- extracts session via `sessionFromContext`, creates Event{T: EventParams}, sends. Add `Redirect(ctx context.Context, u *url.URL)` function -- creates Event{T: EventRedirect}, sends.
  - **instance.go**: Add `CallParams(ctx context.Context, params Params) error` method. Gets params handler, calls with state, updates state. Returns nil if no handler.
  - **session.go**: In `routeToIsland()`, add check: if `event.T == EventParams`, call `instance.CallParams()` instead of `instance.CallEvent()`.
  - **context.go**: Add `sessionFromContext(ctx context.Context) (*Session, error)` helper that extracts engine and sessionID from context, looks up session via `engine.GetSession(sessionID)`, returns `(*Session, error)`.

### Server-Side: Upload System

- [ ] T9: Write Go and client tests for upload system [Stage 3]
  - Files: `upload_test.go` (creates), `web/src/events.spec.ts` (modifies)
  - **Go tests**: ValidateUploads with valid/oversized/too-many/wrong-type files, ConsumeUploads handler called per file, UploadConfig on island, Upload.File() returns staged file, upload endpoint stages files correctly.
  - **Client tests**: upload progress event dispatched during XHR upload, file input change triggers validation event.
  - Tests are written RED before implementation.

- [ ] T10: Implement upload system in Go and client [Stage 3, depends: T9]
  - Files: `upload.go` (creates), `island.go` (modifies), `engine.go` (modifies), `transport_endpoints.go` (modifies), `web/src/events.ts` (modifies)
  - **upload.go**: Port and adapt from master: `UploadError` struct, error sentinels (`ErrUploadNotFound`, `ErrUploadTooLarge`, `ErrUploadNotAccepted`, `ErrUploadTooManyFiles`, `ErrUploadMalformed`), `UploadConfig` struct (Name, MaxFiles, MaxSize, Accept), `Upload` struct (Name, Size, Type, LastModified, Errors, Progress, internalLocation), `Upload.File()`, `UploadContext` type `map[string][]*Upload`, `ValidateUploads(params Params, configs []*UploadConfig) (UploadContext, error)`, `ConsumeUploads(uploads UploadContext, name string, handler ConsumeHandler) []error`, `ConsumeHandler func(u *Upload) error`.
  - **island.go**: Add `uploadConfigs []*UploadConfig` field. Add `WithUploadConfig(config *UploadConfig) IslandConfig`. Add `UploadConfigs()` accessor.
  - **engine.go**: Add `MaxUploadSize int64` and `UploadStagingLocation string` fields with config options.
  - **transport_endpoints.go**: Add `UploadHandler(engine *IslandEngine) http.HandlerFunc` for multipart POST. Uses `http.MaxBytesReader` for size enforcement. Validates session ID from form against authenticated transport session (do NOT trust form-supplied session IDs blindly). Stages files, validates against island upload configs.
  - **web/src/events.ts**: Upgrade XHR in `wireSubmitEvents()` to use `request.upload.onprogress` for progress tracking. Dispatch `CustomEvent("live:upload-progress")` on form. Add `onerror` handler. In `wireChangeEvents()`, detect `input[type="file"]` and listen for `"change"` event to serialize file metadata.

### Server-Side: Test Coverage Improvements

- [ ] T2: Write and implement Go coverage tests [Stage 1]
  - Files: `transport_endpoints_test.go` (creates), `javascript_test.go` (creates), `engine_test.go` (modifies), `instance_test.go` (modifies), `examples/counter/main_test.go` (creates)
  - **transport_endpoints_test.go**: WebSocketHandler returns valid handler that upgrades connections. SSEHandler returns valid handler. SSEHandlerWithFactory returns two handlers (SSE + POST). Use httptest.NewServer.
  - **javascript_test.go**: Javascript.ServeHTTP returns Content-Type text/javascript and non-empty body. JavascriptMap.ServeHTTP returns Content-Type application/json and non-empty body.
  - **engine_test.go**: BroadcastToIsland cross-session delivery (2 sessions with same island ID both receive event, session with different ID does not). SetStateTTL configuration applied to subsequent MountIsland calls.
  - **instance_test.go**: NewIslandInstanceWithChildren stores children, makes them available in render context.
  - **examples/counter/main_test.go**: MountWithInitial (props.Int), Increment, Decrement, MountAndRender via engine. Follow clock test pattern.

### Examples

- [ ] T11: Write tests for buttons example [Stage 4]
  - Files: `examples/buttons/main_test.go` (creates)
  - Tests: NewButtonsIsland construction, mount handler, inc/dec event handlers.

- [ ] T12: Write tests for clocks example [Stage 4]
  - Files: `examples/clocks/main_test.go` (creates)
  - Tests: multiple clock islands with different timezones have independent state.

- [ ] T13: Implement buttons example [Stage 4, depends: T11]
  - Files: `examples/buttons/main.go` (creates), `examples/buttons/buttons.html` (creates), `examples/buttons/index.html` (creates)
  - Counter island with `live-click` for inc/dec and `live-window-keyup` with `live-key="ArrowUp"` / `live-key="ArrowDown"`.

- [ ] T14: Implement clocks example [Stage 4, depends: T12]
  - Files: `examples/clocks/main.go` (creates), `examples/clocks/index.html` (creates)
  - Reuse clock island type. Page with multiple `<live-island type="clock">` with different `data-timezone` props.

- [ ] T15: Write tests for pagination example [Stage 5]
  - Files: `examples/pagination/main_test.go` (creates)
  - Tests: mount initializes page 0, HandleParams with page=2 updates items, next-page event handler calls PatchURL.

- [ ] T16: Write tests for uploads example [Stage 5]
  - Files: `examples/uploads/main_test.go` (creates)
  - Tests: mount returns empty uploads, validate with valid/invalid files, save consumes uploads.

- [ ] T17: Implement pagination example [Stage 5, depends: T15, T8]
  - Files: `examples/pagination/main.go` (creates), `examples/pagination/pagination.html` (creates), `examples/pagination/index.html` (creates)
  - ListState with Page/Items. HandleParams reads `page` param. Event handler calls PatchURL. Template uses `live-patch` and `live-click`.

- [ ] T18: Implement uploads example [Stage 5, depends: T16, T10]
  - Files: `examples/uploads/main.go` (creates), `examples/uploads/uploads.html` (creates), `examples/uploads/index.html` (creates)
  - Upload island with WithUploadConfig for "photos" (max 3, 1MB, image/png). Validate event calls ValidateUploads. Save event calls ConsumeUploads.

- [ ] T19: Rebuild auto.js with all client changes [Stage 6]
  - Files: `web/browser/auto.js` (modifies), `web/browser/auto.js.map` (modifies)
  - Run `cd web && npm run build`.

## Acceptance Criteria

~~~gherkin
Feature: Throttle rate-limiting

  Scenario: Throttle fires immediately then rate-limits
    Given an element with live-click="test" and live-throttle="500"
    When the user clicks the element 5 times rapidly
    Then the first click fires immediately
    And no further clicks fire until 500ms have elapsed
    And a trailing fire occurs after the throttle interval

  Scenario: Throttle takes precedence over debounce
    Given an element with live-throttle="500" and live-debounce="200"
    When the user clicks the element
    Then throttle behavior is applied, not debounce

  Scenario: Throttle cleanup on island unmount
    Given an element with live-throttle="500" in an island
    When the island is unmounted
    Then no trailing throttle fires occur

Feature: Window-level events

  Scenario: live-window-keyup fires on window keypress
    Given an element with live-window-keyup="shortcut" inside an island
    When a keyup event fires on the window
    Then the island receives the "shortcut" event with key data

  Scenario: live-key filters window key events
    Given an element with live-window-keyup="up" live-key="ArrowUp" inside an island
    When ArrowUp is pressed on the window
    Then the "up" event fires
    When "a" is pressed on the window
    Then no event fires

  Scenario: Multiple islands receive window events independently
    Given island-A has live-window-keyup="action-a"
    And island-B has live-window-keyup="action-b"
    When a keyup event fires on the window
    Then island-A receives "action-a"
    And island-B receives "action-b"

Feature: Connection CSS classes

  Scenario: Connected class applied on connection
    Given the page has loaded
    When the transport connects successfully
    Then document.body has class "live-connected"
    And document.body does not have class "live-disconnected"

  Scenario: Disconnected class applied on disconnect
    Given the transport is connected
    When the transport disconnects
    Then document.body has class "live-disconnected"
    And document.body does not have class "live-connected"

  Scenario: Error class applied on server error
    Given the transport is connected
    When a server error event is received
    Then document.body has class "live-error"

Feature: Redirect handling

  Scenario: Server redirect navigates browser
    Given an island is connected
    When the server sends an EventRedirect with URL "/success"
    Then window.location.replace is called with "/success"

Feature: HandleParams for islands

  Scenario: Params event routes to params handler
    Given an island with HandleParams registered
    When an EventParams event arrives with page=2
    Then the params handler is called with params containing page=2
    And the island state is updated and re-rendered

  Scenario: No params handler is a no-op
    Given an island without HandleParams
    When an EventParams event arrives
    Then no error occurs and the event is silently ignored

  Scenario: PatchURL sends params to client
    Given an island event handler calls PatchURL with page=3
    When the handler completes
    Then the transport receives an EventParams event with "page=3"

  Scenario: Redirect sends redirect to client
    Given an island event handler calls Redirect with "/done"
    When the handler completes
    Then the transport receives an EventRedirect event with "/done"

  Scenario: Client receives server-originated params and updates URL
    Given the client is connected
    When the server sends an EventParams event with "page=3"
    Then the browser URL is updated to include "?page=3" via history.pushState

Feature: Live-patch client navigation

  Scenario: Clicking live-patch updates URL and sends params
    Given an anchor with live-patch and href="?page=2" inside an island
    When the user clicks the anchor
    Then the browser URL is updated to include page=2
    And a params event is sent to the server with page=2

  Scenario: live-window-focus fires on window focus
    Given an element with live-window-focus="focused" inside an island
    When a focus event fires on the window
    Then the island receives the "focused" event

Feature: File uploads

  Scenario: Valid file upload is accepted
    Given an island with UploadConfig allowing max 1MB PNG files
    When a 500KB PNG file is uploaded
    Then ValidateUploads returns no errors
    And ConsumeUploads provides access to the staged file

  Scenario: Oversized file is rejected
    Given an island with UploadConfig allowing max 1MB files
    When a 2MB file is uploaded
    Then ValidateUploads returns ErrUploadTooLarge

  Scenario: Wrong file type is rejected
    Given an island with UploadConfig allowing only image/png
    When a text/plain file is uploaded
    Then ValidateUploads returns ErrUploadNotAccepted

  Scenario: Too many files are rejected
    Given an island with UploadConfig allowing max 3 files
    When 5 files are uploaded
    Then ValidateUploads returns ErrUploadTooManyFiles

Feature: Test coverage improvements

  Scenario: Transport endpoint handlers serve correctly
    Given a WebSocketHandler is created
    When an HTTP request is sent
    Then the handler attempts a WebSocket upgrade

  Scenario: JavaScript handler serves correct content
    Given a Javascript handler
    When an HTTP GET request is sent
    Then the response has Content-Type text/javascript
    And the response body is non-empty

  Scenario: BroadcastToIsland reaches specific island
    Given two sessions with island "chat-1" and one with island "chat-2"
    When BroadcastToIsland is called for "chat-1"
    Then both sessions with "chat-1" receive the event
    And the session with "chat-2" does not

  Scenario: Counter example constructs and handles events
    Given a counter island mounted with initial=5
    When the "inc" event is handled
    Then the count is 6
~~~

**Source**: Generated from plan context

## Implementation Notes

- **Window listener cleanup is critical**: Unlike element-scoped listeners, window listeners persist globally. Each `wireWindow*` call must push a removal function to `cleanupFunctions` that calls `window.removeEventListener` with the exact same handler reference.
- **Re-wiring after patches**: `wire()` is called after every patch (island.ts). Window event listeners must be cleaned up before re-wiring to avoid duplicates.
- **Throttle state across patches**: The Throttler uses `WeakMap<Element, number>` which resets when DOM elements are replaced during patching. This is consistent with Debouncer behavior and acceptable -- throttle state loss on re-wire is expected.
- **EventDispatch integration**: `EventDispatch` in `event.ts` already has `disconnected()`, `reconnected()`, and `error()` static methods that manage CSS classes on `document.body`. ConnectionManager should call these existing methods rather than duplicating class manipulation logic.
- **PatchURL/Redirect data encoding**: `Event.Data` is `json.RawMessage`. Use `json.Marshal(values.Encode())` for PatchURL and `json.Marshal(u.String())` for Redirect to produce properly quoted JSON strings.
- **Upload state in islands**: Upload metadata (UploadContext) is stored as part of the island state struct, not on a Socket. Handlers manage uploads through normal state mutation.
- **Upload endpoint security**: The upload HTTP endpoint must validate the session ID from form fields against the authenticated session (e.g., by verifying against the `live_session` cookie on the request). Do NOT blindly trust user-supplied session IDs in multipart form fields, as this would allow an attacker to route uploads to another session.
- **Upload size enforcement**: The upload endpoint must use `http.MaxBytesReader` to enforce `MaxUploadSize` at the HTTP layer before processing the multipart form.
- **JSDOM test limitations**: Window-level events, `history.pushState`, and `window.location.replace` need mocking strategies in jest/jsdom environment.
- **JS rebuild required**: After all client-side changes, `web/browser/auto.js` must be rebuilt via `cd web && npm run build`.

## Refs

- `docs/research/2026-03-01-v2-feature-gaps-and-test-coverage.md` - Gap analysis
- `docs/research/2026-02-25-v2-examples-porting.md` - Example porting strategy
- `docs/research/2026-02-22-v2-branch-state-and-testing-gaps.md` - Previous V2 state analysis
