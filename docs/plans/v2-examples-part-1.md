# Implementation Plan: V2 Framework APIs (SendSelf, WithEventDelay, WithErrorHandler, Broadcast)

Add framework APIs to support the v2 examples: `SendSelf` for triggering self-events from handlers, `WithEventDelay` for recurring self-events, `WithErrorHandler` for custom error handling, `BroadcastTransport` for cross-server broadcasting, and fix client-side error event routing.

## Context

**Research Document**: `docs/research/2026-02-25-v2-examples-porting.md`

**Key Files**:
- `context.go` - Context key pattern (contextKey type, unexported setters, exported getters)
- `island.go` - Island struct, IslandConfig options (WithMount, WithRender, WithUnmount), handler types
- `instance.go` - IslandInstance lifecycle (CallSelf, CallEvent, Mount, Unmount)
- `engine.go` - IslandEngine orchestration (MountIsland, RouteEvent, BroadcastToIslandType)
- `session.go` - Session event routing (handleEvent, routeToIsland)
- `event.go` - Event struct with T, Island, Data, SelfData fields
- `web/src/connection.ts` - Client ConnectionManager.routeMessage (currently only routes "patch")
- `web/src/hooks.ts` - HookRegistry with handleServerEvent
- `git show master:pubsub.go` - V1 broadcast pattern: `PubSubTransport` interface, `PubSub` struct with topic-based routing, `LocalTransport` (to be adapted as `BroadcastTransport`/`Broadcast`)

**Architectural Notes**:
- IslandConfig is `func(i *Island) error` — all With* options follow this pattern
- Context keys use unexported `contextKey string` type with unexported setter / exported getter
- Event routing: client events go through `session.handleEvent` → `instance.CallEvent`; self events use `SelfData != nil` → `instance.CallSelf`
- Engine is the only layer that knows sessionID + islandID + state store — it must enrich handler contexts

**Functional Requirements** (EARS notation):
- When a handler calls `SendSelf(ctx, event, data)`, the engine shall deliver that event to the same island instance after the current handler returns
- When a self-event handler completes for an event configured with `WithEventDelay`, the engine shall schedule re-delivery after the specified delay
- When an event handler returns an error, the engine shall call the island's error handler and send the resulting event to the client
- When no custom error handler is configured, the engine shall use a default that sends `{t: "err", d: {err: "message"}}`
- When an island is unmounted, the engine shall cancel all pending event delay timers
- When the client receives a non-patch event with an island field, the connection shall route it to the island's hook system
- When `Broadcast.Subscribe(topic, islandType, engine)` is called, the engine shall be registered to receive messages on that topic for islands of that type
- When `Broadcast.Publish(ctx, topic, event)` is called, the transport shall deliver the event to all subscribed engines
- When the Broadcast receives a message on a topic, it shall route the event as a self-event to all islands of the subscribed type in each registered engine via `BroadcastSelfToIslandType`

**Branch**: `v2`
**Stack**: 1 of 3 (base: `v2`)
**Stack Plans**:
- 1: `docs/plans/v2-examples-part-1.md` (this plan)
- 2: `docs/plans/v2-examples-part-2.md`
- 3: `docs/plans/v2-examples-part-3.md`

## Batch Size

| Metric | Count | Rating |
|--------|-------|--------|
| Tasks | 11 | Large |
| Files | 14 | Large |
| Stages | 2 | Small |

**Overall: Large** (acceptable — Broadcast tasks are self-contained and parallel with existing Stage 2 work)

## Execution Stages

### Stage 1

#### Test Creation Phase (parallel)
- T-test-context: Write tests for new context helpers (`context_test.go`) (hmm-test-writer)
  - Regression tests: existing `TestContextWithRequest`, `TestContextWithWriter` must still pass
  - New feature tests (RED): round-trip set/get for sessionID, islandID, engine, selfEventQueue; nil/missing returns
- T-test-island-opts: Write tests for WithErrorHandler, WithEventDelay, SendSelf (`island_test.go`) (hmm-test-writer)
  - Regression tests: existing island tests must pass
  - New feature tests (RED): WithErrorHandler custom/default; WithEventDelay stores delay; GetEventDelay hit/miss; SendSelf enqueues/no-panic-without-queue
- T-test-instance-timers: Write tests for timer cancellation on unmount (`instance_test.go`) (hmm-test-writer)
  - Regression tests: existing instance tests must pass
  - New feature tests (RED): pendingTimers cancelled on Unmount; CancelTimers clears map

#### Implementation Phase (parallel, depends on Test Creation Phase)
- T-impl-context: Add context keys and helpers for sessionID, islandID, engine, selfEventQueue (`context.go`) (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
  - Files: `context.go` (modifies)
- T-impl-island-opts: Add errorHandler/eventDelays fields to Island, implement WithErrorHandler, WithEventDelay, SendSelf, defaultErrorHandler (`island.go`) (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
  - Files: `island.go` (modifies)
- T-impl-instance-timers: Add pendingTimers tracking to IslandInstance, CancelTimers(), integrate with Unmount (`instance.go`) (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
  - Files: `instance.go` (modifies)

### Stage 2 (depends on Stage 1)

#### Test Creation Phase (parallel)
- T-test-engine-integration: Write integration tests for SendSelf, WithErrorHandler, WithEventDelay through engine (`engine_test.go`) (hmm-test-writer)
  - Regression tests: existing engine tests must pass with updated session.handleEvent signature
  - New feature tests (RED): SendSelf from handler triggers self-handler; error handler sends error event; event delay re-schedules; timer cancellation on unmount; SendSelf from mount handler

#### Implementation Phase (sequential — session first, then engine in parallel with client)
- T-impl-session-ctx: Update session.handleEvent and routeToIsland to accept context parameter, fix all existing test call sites (`session.go`, `session_test.go`, `engine_test.go`) (hmm-implement-worker, TDD mode)
  - Files: `session.go` (modifies), `session_test.go` (modifies), `engine_test.go` (modifies)
  - Update `handleEvent(event Event)` → `handleEvent(ctx context.Context, event Event)` and `routeToIsland(event Event)` → `routeToIsland(ctx context.Context, event Event)`
  - Update ALL existing test call sites that use the old signature (use `context.Background()` or session context)
  - Must complete before T-impl-engine-integration starts (engine calls `session.handleEvent(ctx, event)`)
- T-impl-engine-integration: Enrich handler context in RouteEvent/MountIsland, process self-event queue, error handling, event delay scheduling (`engine.go`) (hmm-implement-worker, TDD mode) [depends: T-impl-session-ctx]
  - Make RED tests pass (GREEN)
  - Files: `engine.go` (modifies)
  - Add max recursion depth guard (10 levels) when processing self-event queue to prevent infinite recursion
- T-impl-client-routing: Fix client routeMessage to route non-patch events to hooks (`web/src/connection.ts`) (hmm-implement-worker, TDD mode)
  - Manually verify: no existing client-side test infrastructure; verify by running the hooks example in Part 2
  - Files: `web/src/connection.ts` (modifies)

#### Broadcast Phase (parallel with client routing, depends on engine integration)
- T-test-broadcast: Write tests for BroadcastTransport, Broadcast, LocalTransport, and BroadcastSelfToIslandType (`broadcast_test.go`) (hmm-test-writer)
  - New feature tests (RED): Subscribe registers engine, Publish delivers through transport, Receive routes self-events to matching islands, LocalTransport round-trip, BroadcastSelfToIslandType routes to all matching sessions
- T-impl-broadcast: Implement BroadcastTransport interface, Broadcast struct, LocalTransport, BroadcastSelfToIslandType (`broadcast.go`, `engine.go`) (hmm-implement-worker, TDD mode) [depends: T-test-broadcast, T-impl-engine-integration]
  - Make RED tests pass (GREEN)
  - Files: `broadcast.go` (creates), `engine.go` (modifies)

## Task List

### Context Helpers

- [ ] Add context keys and helpers for SendSelf support (`context.go`) [Stage 1]
  - Files: `context.go` (modifies)
  - Add four new context keys: `sessionIDCtxKey`, `islandIDCtxKey`, `engineCtxKey`, `selfEventQueueCtxKey`
  - Add unexported setters: `contextWithSessionID(ctx, SessionID)`, `contextWithIslandID(ctx, IslandID)`, `contextWithEngine(ctx, *IslandEngine)`, `contextWithSelfEventQueue(ctx, *[]Event)`
  - Add getters: unexported `sessionIDFromContext`, `islandIDFromContext`, `engineFromContext`, `selfEventQueueFromContext`
  - Follow exact pattern of existing `requestKey`/`contextWithRequest`/`Request`

- [ ] Write context helper tests (`context_test.go`) [Stage 1]
  - Files: `context_test.go` (modifies)
  - Test round-trip set/get for each new key
  - Test nil/missing returns empty values
  - Test type safety

### Island API Changes

- [ ] Add `errorHandler` and `eventDelays` fields to Island struct (`island.go`) [Stage 1]
  - Files: `island.go` (modifies)
  - Add `errorHandler func(ctx context.Context, err error) Event` field
  - Add `eventDelays map[string]time.Duration` field
  - Initialize `eventDelays` to `make(map[string]time.Duration)` in `NewIsland()`
  - Set `errorHandler` to `defaultErrorHandler` in `NewIsland()`
  - Add import for `encoding/json` and `time`

- [ ] Implement `defaultErrorHandler` (`island.go`) [Stage 1]
  - Files: `island.go` (modifies)
  - `func defaultErrorHandler(ctx context.Context, err error) Event` — marshals `{"err": err.Error()}` into `Event{T: EventError, Data: jsonBytes}`

- [ ] Implement `WithErrorHandler` IslandConfig (`island.go`) [Stage 1]
  - Files: `island.go` (modifies)
  - `func WithErrorHandler(fn func(ctx context.Context, err error) Event) IslandConfig` — sets `i.errorHandler = fn`
  - Follow WithMount/WithRender pattern

- [ ] Implement `WithEventDelay` IslandConfig (`island.go`) [Stage 1]
  - Files: `island.go` (modifies)
  - `func WithEventDelay(event string, delay time.Duration) IslandConfig` — sets `i.eventDelays[event] = delay`
  - Add `func (i *Island) GetEventDelay(event string) (time.Duration, bool)` getter with RLock

- [ ] Implement `SendSelf` function (`island.go`) [Stage 1, depends: context helpers]
  - Files: `island.go` (modifies)
  - `func SendSelf(ctx context.Context, event string, data any)` — reads selfEventQueue from context, appends `Event{T: event, Island: islandID, SelfData: data}`
  - Silent no-op if queue not in context (called outside handler)

- [ ] Write island API tests (`island_test.go`) [Stage 1]
  - Files: `island_test.go` (modifies)
  - TestWithErrorHandler_Custom, TestWithErrorHandler_Default
  - TestWithEventDelay_ConfiguresDelay, TestGetEventDelay_Unknown
  - TestSendSelf_Enqueues, TestSendSelf_NoPanic

### Instance Timer Tracking

- [ ] Add pending timer tracking to IslandInstance (`instance.go`) [Stage 1]
  - Files: `instance.go` (modifies)
  - Add `pendingTimers map[string]*time.Timer` field
  - Initialize in `NewIslandInstanceFromRegistry` to `make(map[string]*time.Timer)`
  - Add `CancelTimers()` method: iterates and Stop()s all timers, clears map; protect with `i.mu` mutex
  - Call `CancelTimers()` at start of `Unmount()`
  - Thread safety: `pendingTimers` is accessed by `time.AfterFunc` goroutines (engine adds timers) and by `Unmount` → `CancelTimers()`. Both must hold `i.mu`

- [ ] Write instance timer tests (`instance_test.go`) [Stage 1]
  - Files: `instance_test.go` (modifies)
  - TestIslandInstance_CancelTimers: add timers, call Unmount, verify stopped

### Session Context Parameter

- [ ] Update handleEvent and routeToIsland to accept context, fix all test call sites (`session.go`, `session_test.go`, `engine_test.go`) [Stage 2, depends: Stage 1, must complete before engine integration]
  - Files: `session.go` (modifies), `session_test.go` (modifies), `engine_test.go` (modifies)
  - Change `handleEvent(event Event) error` → `handleEvent(ctx context.Context, event Event) error`
  - Change `routeToIsland(event Event) error` → `routeToIsland(ctx context.Context, event Event) error`
  - Pass `ctx` through to `instance.CallSelf(ctx, ...)` and `instance.CallEvent(ctx, ...)`
  - Update ALL existing test call sites in `session_test.go` and `engine_test.go` to pass `context.Background()` as first arg
  - This ensures the codebase compiles after the signature change, before engine integration begins

### Engine Integration

- [ ] Enrich handler context and process self-events in RouteEvent (`engine.go`) [Stage 2, depends: Session context task must complete first]
  - Files: `engine.go` (modifies)
  - In `RouteEvent`: create enriched context with sessionID, islandID, engine, selfEventQueue
  - Pass enriched context to `session.handleEvent(ctx, event)`
  - After handler returns: if error → call island's errorHandler, send error event, return
  - After successful handling + render: drain self-event queue, process each via RouteEvent with max recursion depth of 10
  - After self-event handler: check `island.GetEventDelay(event.T)`, schedule `time.AfterFunc`, store timer on `instance.pendingTimers` (hold `instance.mu` when writing)
  - In `MountIsland`: enrich context before calling `instance.Mount(ctx)`, drain self-event queue after mount

- [ ] Write engine integration tests (`engine_test.go`) [Stage 2]
  - Files: `engine_test.go` (modifies)
  - TestEngineSendSelfFromHandler: handler calls SendSelf, verify self-handler executes
  - TestEngineErrorHandler: handler returns error, verify error event sent to transport
  - TestEngineEventDelay: verify self-event re-scheduled after delay, cancelled on unmount

### Client Error Event Routing

- [ ] Fix client routeMessage to dispatch non-patch events to hooks (`web/src/connection.ts`) [Stage 2]
  - Files: `web/src/connection.ts` (modifies)
  - In `routeMessage`, after patch handling, route messages with `message.island` and `message.t !== "patch"` to `HookRegistry.handleServerEvent(message.island, message.t, message.d)`
  - Note: `message.d` is already parsed JSON (the transport deserializes it), so the client hook receives a JS object
  - No client-side test infrastructure exists; verify manually via the hooks example (Part 2)
  - Must rebuild the client bundle (`web/`) and regenerate embedded JS after this change

### Broadcast Transport (cross-server broadcasting)

- [ ] Add `BroadcastSelfToIslandType` method to IslandEngine (`engine.go`) [Stage 2, depends: engine integration]
  - Files: `engine.go` (modifies)
  - `func (e *IslandEngine) BroadcastSelfToIslandType(islandType string, event Event)` — iterates all sessions, finds islands of the given type, and calls `e.RouteEvent(sessionID, eventCopy)` for each with `SelfData` set
  - Unlike `BroadcastToIslandType` (which sends directly to client transport), this routes through `HandleSelf` handlers so server-side state is updated before re-rendering
  - Thread safety: takes RLock on sessions map, releases before calling RouteEvent

- [ ] Implement `BroadcastTransport` interface, `Broadcast` struct, and `LocalTransport` (`broadcast.go`) [Stage 2, depends: BroadcastSelfToIslandType]
  - Files: `broadcast.go` (creates)
  - **`BroadcastTransport` interface**:
    ```go
    type BroadcastTransport interface {
        Publish(ctx context.Context, topic string, msg Event) error
        Listen(ctx context.Context, b *Broadcast) error
    }
    ```
  - **`Broadcast` struct**:
    ```go
    type subscription struct {
        islandType string
        engine     *IslandEngine
    }
    type Broadcast struct {
        transport  BroadcastTransport
        mu         sync.RWMutex
        handlers   map[string][]subscription
    }
    ```
  - **`NewBroadcast(ctx, transport)`** — creates Broadcast, starts `transport.Listen` in goroutine
  - **`Subscribe(topic, islandType, engine)`** — registers engine+islandType for the topic
  - **`Publish(ctx, topic, event)`** — delegates to `transport.Publish`
  - **`Receive(topic, event)`** — iterates subscriptions for topic, calls `sub.engine.BroadcastSelfToIslandType(sub.islandType, event)` for each
  - **`LocalTransport`** — in-memory channel-based transport:
    ```go
    type LocalTransport struct { queue chan BroadcastMessage }
    func NewLocalTransport() *LocalTransport
    func (l *LocalTransport) Publish(ctx, topic, msg) error  // sends to channel
    func (l *LocalTransport) Listen(ctx, b) error            // receives from channel, calls b.Receive
    ```

- [ ] Write Broadcast tests (`broadcast_test.go`) [Stage 2]
  - Files: `broadcast_test.go` (creates)
  - **TestBroadcast_SubscribeAndReceive**: Subscribe engine to topic, call Receive, verify BroadcastSelfToIslandType routes self-event to matching islands
  - **TestBroadcast_PublishThroughLocalTransport**: Create Broadcast with LocalTransport, publish message, verify it arrives at subscribed engine
  - **TestBroadcast_MultipleEngines**: Subscribe two engines to same topic, verify both receive messages
  - **TestLocalTransport_RoundTrip**: Publish and Listen, verify message delivered
  - **TestBroadcastSelfToIslandType**: Mount islands in engine, call BroadcastSelfToIslandType, verify self-handlers are invoked and state is updated

## Acceptance Criteria

~~~gherkin
Feature: SendSelf API

  Scenario: Handler sends self-event during execution
    Given an island with a "process" event handler that calls SendSelf(ctx, "notify", data)
    And a "notify" self handler registered on the island
    When the engine routes a "process" event
    Then the "notify" self handler is called after "process" completes
    And the island re-renders after both handlers

  Scenario: SendSelf called outside handler context
    Given a context without session/island/engine values
    When SendSelf is called with that context
    Then no panic occurs and the call is a no-op

  Scenario: Multiple self-events from one handler
    Given a handler that calls SendSelf twice with different events
    When the engine routes the triggering event
    Then both self-event handlers execute in order

  Scenario: SendSelf from mount handler
    Given an island whose mount handler calls SendSelf(ctx, "init", data)
    And an "init" self handler registered
    When the island is mounted via MountIsland
    Then the "init" self handler is called after mount completes

Feature: WithEventDelay API

  Scenario: Self-event re-scheduled after handler completes
    Given an island with WithEventDelay("tick", 100ms)
    And a "tick" self handler registered
    When a "tick" self-event is delivered
    Then the handler executes
    And a new "tick" delivery is scheduled 100ms later
    And the handler executes again after the delay

  Scenario: Timer cancelled on island unmount
    Given an island with WithEventDelay("tick", 100ms) and a pending timer
    When the island is unmounted
    Then the pending timer is cancelled
    And no further "tick" deliveries occur

Feature: WithErrorHandler API

  Scenario: Custom error handler receives handler errors
    Given an island with a custom WithErrorHandler
    And a "fail" event handler that returns an error
    When the engine routes a "fail" event
    Then the custom error handler is called with the error
    And the resulting event is sent to the client transport

  Scenario: Default error handler sends error event
    Given an island without a custom error handler
    And a "fail" event handler that returns an error
    When the engine routes a "fail" event
    Then an event with type "err" and data {"err": "error message"} is sent to the client

Feature: Client error event routing

  Scenario: Error event dispatched to hook
    Given a client with a hook registered for "err" events on an island
    When the server sends an "err" event targeting that island
    Then the hook's handleEvent callback receives the error data

Feature: Broadcast Transport cross-server broadcasting

  Scenario: Message delivered to subscribed engine via LocalTransport
    Given a Broadcast with LocalTransport
    And engine A subscribed to topic "chat" for island type "chat"
    And engine A has a session with a "chat" island mounted
    When a message is published to topic "chat"
    Then the "chat" island's self-handler receives the message
    And the island state is updated and re-rendered

  Scenario: Multiple engines receive broadcast
    Given a Broadcast with LocalTransport
    And engine A and engine B are both subscribed to topic "chat" for island type "chat"
    When a message is published to topic "chat"
    Then both engines receive the message
    And all matching islands in both engines have their self-handlers invoked

  Scenario: BroadcastSelfToIslandType routes through handlers
    Given an engine with two sessions, each containing a "chat" island
    When BroadcastSelfToIslandType("chat", event) is called
    Then the self-handler on both island instances is invoked
    And both islands are re-rendered with updated state
~~~

**Source**: Generated from plan context

## Implementation Notes

- **Self-event queue vs goroutine**: Use a queue-in-context approach (`*[]Event` pointer) rather than spawning goroutines from SendSelf. This is deterministic and avoids races between handler completion and self-event delivery. The queue is only accessed within a single handler execution flow (not across goroutines), so no synchronization is needed for the queue itself.
- **Recursive RouteEvent depth**: Self-event processing calls RouteEvent recursively. Add a max recursion depth guard of 10 levels to prevent infinite recursion from handlers that always call SendSelf.
- **Timer goroutine lifecycle**: `time.AfterFunc` callbacks run in separate goroutines. When they fire after session/island removal, `RouteEvent` will return "session/island not found" — this is safe but should not log at ERROR level.
- **pendingTimers thread safety**: The `pendingTimers` map on `IslandInstance` is accessed by `time.AfterFunc` goroutines (engine stores timers) and by `Unmount` → `CancelTimers()`. Both code paths must hold `instance.mu`.
- **`json` import**: `defaultErrorHandler` needs `encoding/json` in `island.go`.
- **handleEvent signature change**: This is an unexported method, but test code calls it directly. All existing test call sites are updated as part of the session context task in Stage 2, ensuring the codebase compiles before engine integration begins.
- **Client connection.ts change**: Must be rebuilt (`npm run build` or equivalent in `web/`) to update the compiled JS. The `live.Javascript{}` handler serves the embedded compiled output. No client-side test infrastructure exists; verify manually.
- **Broadcast v1→v2 adaptation**: V1's `PubSub.Subscribe(topic, engine)` assumed one handler per engine. V2's `Broadcast.Subscribe(topic, islandType, engine)` takes an additional `islandType` parameter because `IslandEngine` manages multiple island types. `Receive` calls `engine.BroadcastSelfToIslandType(islandType, event)` instead of v1's `engine.self(ctx, nil, msg)`.
- **BroadcastSelfToIslandType vs BroadcastToIslandType**: `BroadcastToIslandType` sends events directly to the client transport (for push notifications, patches). `BroadcastSelfToIslandType` routes through `HandleSelf` handlers via `RouteEvent` with `SelfData` set, so server-side state is updated before re-rendering. The Broadcast system uses the latter because chat messages need to update state.
- **LocalTransport channel**: Uses an unbuffered channel like v1. `Publish` blocks until `Listen` receives. For production use, users plug in Redis/NATS `BroadcastTransport` implementations that handle async delivery.

## Refs

- `docs/research/2026-02-25-v2-examples-porting.md` — Research document with API design decisions
