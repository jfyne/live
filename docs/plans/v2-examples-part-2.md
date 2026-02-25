# Implementation Plan: V2 Examples — Clock + Hooks

Create two v2 examples that exercise the new framework APIs from Part 1: the clock example (SendSelf + WithEventDelay for server-pushed timed updates) and the hooks example (WithErrorHandler + client error routing for server-to-client hook communication).

## Context

**Research Document**: `docs/research/2026-02-25-v2-examples-porting.md`

**Key Files**:
- `examples/counter/main.go` - Canonical v2 example pattern (island constructor, registration, engine, WS/SSE transports)
- `examples/counter/index.html` - Page template with `<live-island>` elements and inline CSS
- `examples/counter/counter.html` - Island render template (simple HTML fragment)
- `island.go` - NewIsland, WithMount, WithRender, WithEventDelay, WithErrorHandler, SendSelf (from Part 1)
- `engine.go` - MountIsland, RouteEvent with self-event/error handling (from Part 1)
- `web/src/hooks.ts` - Client hook system with handleEvent callback

**Architectural Notes**:
- Every v2 example follows the same structure: `main.go` with embed.FS, island constructor, RegisterIsland, full HTTP server with WS + SSE, serve index page and `live.Javascript{}`
- The event loop pattern: `for event := range transport.Events()` → subscribe events mount islands, other events route through engine
- Island templates are simple HTML fragments (no `<html>` wrapper, no `{{ define }}` blocks)
- All examples are self-contained with their own transport boilerplate for clarity

**Functional Requirements** (EARS notation):
- The clock island shall display the current server time, updating every second without user interaction
- When the clock island mounts, it shall trigger an initial "tick" self-event via SendSelf
- While the clock island is mounted, the engine shall re-deliver "tick" every second via WithEventDelay
- When the "Make a problem" button is clicked in the hooks example, the server shall return an error
- When a handler error occurs, the engine shall send an error event to the client
- When the client hook receives an error event, it shall display the error message via alert

**Branch**: `v2`
**Stack**: 2 of 3 (base: Part 1 branch)
**Stack Plans**:
- 1: `docs/plans/v2-examples-part-1.md`
- 2: `docs/plans/v2-examples-part-2.md` (this plan)
- 3: `docs/plans/v2-examples-part-3.md`

## Batch Size

| Metric | Count | Rating |
|--------|-------|--------|
| Tasks | 8 | Medium |
| Files | 8 | Medium |
| Stages | 2 | Small |

**Overall: Medium**

## Execution Stages

### Stage 1

#### Test Creation Phase (parallel)
- T-test-clock: Write integration test verifying clock island mounts and receives tick events (`examples/clock/main_test.go`) (hmm-test-writer)
  - New feature tests (RED): island mounts with time, self-event triggers, re-render occurs
- T-test-hooks: Write integration test verifying hooks island sends error events (`examples/hooks/main_test.go`) (hmm-test-writer)
  - New feature tests (RED): "problem" event returns error, error event sent to transport

#### Implementation Phase (parallel, depends on Test Creation Phase)
- T-impl-clock: Create clock example (main.go, index.html, clock.html) (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
- T-impl-hooks: Create hooks example (main.go, index.html, hooks.html) (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)

### Stage 2 (depends on Stage 1)

#### Implementation Phase
- T-verify: Verify all examples compile and tests pass (hmm-implement-worker)
  - Run `go build ./examples/clock/...` and `go build ./examples/hooks/...`
  - Run `go test ./examples/clock/...` and `go test ./examples/hooks/...`

## Task List

### Clock Example

- [x] Write clock integration tests (`examples/clock/main_test.go`) [Stage 1, Test Creation Phase]
  - Files: `examples/clock/main_test.go` (creates)
  - **TestClockIsland_MountWithTimezone**: Mount with `Props{"timezone": "America/New_York", "label": "NYC"}`, verify state.Location is New York, state.Label is "NYC"
  - **TestClockIsland_MountUTC**: Mount with `Props{"timezone": "UTC", "label": "UTC"}`, verify state.Location is UTC
  - **TestClockIsland_SendSelfOnMount**: Mount the island via engine with a mock transport, verify that a self-event is queued (check transport receives a patch after mount + tick)
  - **TestClockIsland_TickUpdatesTime**: Call the "tick" self handler directly, verify state.Time is updated
  - **TestClockIsland_FormattedTimeUsesLocation**: Set location to Tokyo, verify FormattedTime returns time in JST
  - Test strategy: use `50ms` WithEventDelay in tests, assert state changes within 200ms timeout
  - Helper: create registry, engine, mock transport, session — same pattern as `engine_test.go`

- [x] Create clock island definition and HTTP server (`examples/clock/main.go`) [Stage 1]
  - Files: `examples/clock/main.go` (creates)
  - Follow `examples/counter/main.go` pattern exactly
  - State struct:
    ```go
    type ClockState struct {
        Time     time.Time
        Location *time.Location
        Label    string
    }
    func (s *ClockState) FormattedTime() string {
        return s.Time.In(s.Location).Format("15:04:05")
    }
    ```
  - Server-side config — multiple clocks with different timezones:
    ```go
    type clockConfig struct {
        ID       string
        Label    string
        Timezone string // e.g., "America/New_York", "Europe/London", "Asia/Tokyo"
    }
    var clocks = []clockConfig{
        {ID: "clock-utc", Label: "UTC", Timezone: "UTC"},
        {ID: "clock-nyc", Label: "New York", Timezone: "America/New_York"},
        {ID: "clock-london", Label: "London", Timezone: "Europe/London"},
        {ID: "clock-tokyo", Label: "Tokyo", Timezone: "Asia/Tokyo"},
    }
    ```
  - Island constructor `NewClockIsland() (*live.Island, error)`:
    - `live.WithMount`: reads `props.String("timezone")`, loads `time.LoadLocation`, calls `live.SendSelf(ctx, "tick", nil)`, returns `&ClockState{Time: time.Now(), Location: loc, Label: props.String("label")}`
    - `live.WithRender`: parses `clock.html` via `template.ParseFS(content, "clock.html")`, executes with state
    - `live.WithEventDelay("tick", 1*time.Second)`
  - Self handler: `island.HandleSelf("tick", ...)` — sets `s.Time = time.Now()`
  - Registration: `live.RegisterIsland("clock", NewClockIsland)`
  - HTTP server: engine setup, WS at `/ws`, SSE at `/live/sse` and `/live/post`, page at `/`, `live.Javascript{}` at `/live.js`
  - Subscribe handler: lookup clock config by island ID, pass `Props{"timezone": config.Timezone, "label": config.Label}`
  - Multiple instances of same island type, each with different timezone props (like counter with different initial values)

- [x] Create clock page template (`examples/clock/index.html`) [Stage 1]
  - Files: `examples/clock/index.html` (creates)
  - Follow `examples/counter/index.html` CSS/layout pattern
  - Title: "Live v2 Clock Example"
  - Render a grid of clock islands using server-side `clocks` config:
    ```html
    {{ range . }}
    <div class="clock-card">
        <h2>{{ .Label }}</h2>
        <live-island type="clock" id="{{ .ID }}">
            <div class="clock-display">Loading...</div>
        </live-island>
    </div>
    {{ end }}
    ```
  - CSS grid layout: `display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 20px;`
  - Each card styled with centered time, timezone label, card shadow
  - Feature list: HandleSelf, server push, WithEventDelay, SendSelf, multiple timezones, state isolation
  - `<script src="/live.js"></script>`

- [x] Create clock island template (`examples/clock/clock.html`) [Stage 1]
  - Files: `examples/clock/clock.html` (creates)
  - Simple HTML fragment showing formatted time for the island's timezone:
    ```html
    <div>
        <div class="clock-display">{{ .FormattedTime }}</div>
    </div>
    ```

### Hooks Example

- [x] Write hooks integration tests (`examples/hooks/main_test.go`) [Stage 1, Test Creation Phase]
  - Files: `examples/hooks/main_test.go` (creates)
  - **TestHooksIsland_ProblemReturnsError**: Create island, call "problem" event handler, verify it returns an error
  - **TestHooksIsland_ErrorEventSentToTransport**: Mount island via engine with mock transport, route "problem" event, verify mock transport receives an error event with `T: "err"` and data containing `{"err": "something went wrong"}`
  - Helper: create registry, engine, mock transport, session — same pattern as `engine_test.go`

- [x] Create hooks island definition and HTTP server (`examples/hooks/main.go`) [Stage 1]
  - Files: `examples/hooks/main.go` (creates)
  - Follow `examples/counter/main.go` pattern exactly
  - Minimal state: `type HooksState struct{}` (or empty interface)
  - Island constructor `NewHooksIsland() (*live.Island, error)`:
    - `live.WithMount`: returns `&HooksState{}`
    - `live.WithRender`: parses `hooks.html`, executes with state
    - Optionally `live.WithErrorHandler(customHandler)` to demonstrate customization
  - Event handler: `island.HandleEvent("problem", ...)` — returns `nil, fmt.Errorf("something went wrong")`
  - Registration: `live.RegisterIsland("hooks", NewHooksIsland)`
  - HTTP server: same pattern as clock
  - Error handling: Part 1 implements engine-level error handling via `WithErrorHandler`. When `RouteEvent` encounters an error from a handler, the engine automatically calls the island's error handler and sends the error event to the client. The event loop only needs to log the error:
    ```go
    if err := engine.RouteEvent(sessionID, event); err != nil {
        log.Printf("Event error: %v", err)
    }
    ```
  - The default error handler (from Part 1) sends `Event{T: "err", Data: {"err": "..."}}` to the client automatically

- [x] Create hooks page template (`examples/hooks/index.html`) [Stage 1]
  - Files: `examples/hooks/index.html` (creates)
  - Follow counter index.html pattern
  - Title: "Live v2 Hooks Example"
  - Single `<live-island type="hooks" id="hooks-demo">` with fallback button
  - JavaScript hook definition:
    ```javascript
    window.Hooks = {
        "err": {
            mounted: function() {
                this.handleEvent("err", function(data) {
                    console.error("Server error:", data);
                    window.alert("Error from server: " + data.err);
                });
            }
        }
    };
    ```
  - `<script src="/live.js"></script>` (after hooks definition)
  - Description explaining error handling and hook communication

- [x] Create hooks island template (`examples/hooks/hooks.html`) [Stage 1]
  - Files: `examples/hooks/hooks.html` (creates)
  - Simple HTML fragment:
    ```html
    <div live-hook="err">
        <p>Click the button below to trigger a server error. The error will be caught and sent to this hook.</p>
        <button live-click="problem">Make a problem</button>
    </div>
    ```

## Acceptance Criteria

~~~gherkin
Feature: Clock example

  Scenario: Multiple clocks display different timezones
    Given the clock example server is running
    When a browser connects to the page
    Then four clock islands render (UTC, New York, London, Tokyo)
    And each displays the current time in its respective timezone

  Scenario: Clock updates every second
    Given a browser is connected to the clock example
    When 2 seconds elapse
    Then the displayed time has changed at least once

  Scenario: Clock stops on disconnect
    Given a browser is connected to the clock example
    When the WebSocket connection closes
    Then the tick timer is cancelled and no further renders occur

Feature: Hooks example

  Scenario: Error event received by client hook
    Given the hooks example server is running
    And a browser is connected with the "err" hook mounted
    When the user clicks "Make a problem"
    Then the server returns an error
    And the client hook receives the error event
    And an alert displays "Error from server: something went wrong"

  Scenario: Error does not crash the server
    Given the hooks example server is running
    When the user clicks "Make a problem" multiple times
    Then each click results in an error event sent to the client
    And the server continues to handle events normally
~~~

**Source**: Generated from plan context

## Implementation Notes

- **Multiple clock instances**: Like counter, clock uses multiple instances of the same island type with different props. Each clock gets a timezone and label via props. The subscribe handler looks up the clock config by island ID to determine props. The grid layout uses CSS Grid for responsive display.
- **Hooks error handling**: Part 1 implements engine-level error handling. The event loop just logs errors; the engine sends error events automatically via the island's error handler (default or custom).
- **Testing clock timing**: Use short `WithEventDelay` (50ms) in tests and assert state changes within 200ms. Do not use 1-second delays in tests.
- **Hooks window.Hooks**: The v2 client auto-discovers hooks from `window.Hooks` via `autoRegisterHooks()` in `hooks.ts`. IMPORTANT: The inline `<script>` block defining `window.Hooks` must appear BEFORE the `<script src="/live.js">` tag.
- **SendSelf from mount triggers initial tick**: The clock's mount handler calls `SendSelf(ctx, "tick", nil)`. The engine drains the self-event queue after mount completes, delivering the first tick. `WithEventDelay("tick", 1s)` then re-schedules after each tick handler. This chain (mount → SendSelf → HandleSelf → WithEventDelay → HandleSelf → ...) is the core mechanism.

## Refs

- `docs/research/2026-02-25-v2-examples-porting.md`
- `docs/plans/v2-examples-part-1.md` — Framework API changes (prerequisite)
