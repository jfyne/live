# Implementation Plan: Live v2 Islands-Only Architecture

A complete rewrite of the Live framework from full-page live views to a pure islands architecture, where islands are the only abstraction, fully isolated with their own state, and share a single transport connection.

## Context

**Research Document**: `docs/research/2026-01-25-islands-component-architecture.md`

**Key Files (v1 - being replaced)**:
- `handler.go` - Full-page Handler struct (removed in v2)
- `engine.go` - Engine with 1:1 Handler coupling (replaced by IslandEngine)
- `socket.go` - WebSocket-coupled Socket (replaced by Session)
- `page/component.go` - Component abstraction (removed - islands replace this)
- `render.go` - Full-page RenderSocket (replaced by RenderIsland)
- `diff.go` - Position-based anchors (replaced by island-scoped anchors)
- `web/src/socket.ts` - Single WebSocket per page (replaced by transport abstraction)
- `web/src/events.ts` - Document-wide event wiring (replaced by island-scoped)

**Architectural Notes**:
- v2 is a **breaking change** with no backward compatibility
- Islands are the only abstraction (no full-page views)
- Transport abstraction: WebSocket (primary), SSE, polling fallback
- Single transport connection multiplexes all islands
- Each island has isolated state and lifecycle
- Custom element client API: `<live-island type="counter" id="counter-1">`
- **No v2/ directories**: Replace old code directly (we're on v2 branch)
- **No deprecations**: Remove v1 Handler/Engine/Component code as we build replacements

## Execution Strategy

Each deliverable below represents a complete, tested feature that should be implemented and committed as a single unit of work. Tests are written alongside implementation.

### Deliverable 1: Island Core Types ✓ Commit Point
**Files**: `island.go`, `island_test.go`, `errors.go`

Implementation:
- Define `Props` type with helper methods (`String()`, `Int()`, `Bool()`)
- Define handler function types (`IslandMountHandler`, `IslandRenderHandler`, etc.)
- Define `Island` struct with event/self handler maps
- Implement `NewIsland()` constructor with functional options
- Implement `HandleEvent()`, `HandleSelf()` methods
- Add island-specific errors to `errors.go`

Tests (`island_test.go`):
- Test `Props` helper methods with various input types
- Test `NewIsland()` with and without config options
- Test `HandleEvent()` registration and retrieval
- Test `HandleSelf()` registration and retrieval
- Test error cases (duplicate events, missing handlers)

**Commit**: `feat(v2): add Island core types with event registration`

---

### Deliverable 2: Island Registry ✓ Commit Point
**Files**: `registry.go`, `registry_test.go`

Implementation:
- Define `IslandConstructor` type
- Implement `IslandRegistry` with thread-safe map
- Implement `RegisterIsland()`, `GetIsland()`, `ListIslands()`
- Global singleton registry instance

Tests (`registry_test.go`):
- Test `RegisterIsland()` success case
- Test duplicate registration error
- Test `GetIsland()` found and not found
- Test `ListIslands()` returns all registered islands
- Test concurrent registration safety (goroutine race test)

**Commit**: `feat(v2): add IslandRegistry with thread-safe island registration`

---

### Deliverable 3: Island Instance ✓ Commit Point
**Files**: `instance.go`, `instance_test.go`

Implementation:
- Define `IslandInstance` struct (ID, Type, island ref, state, props, children)
- Implement `NewIslandInstance()` constructor
- Implement lifecycle: `Mount()`, `Render()`, `Unmount()`
- Implement event handling: `CallEvent()`, `CallSelf()`
- Implement state access: `State()`, `SetState()`

Tests (`instance_test.go`):
- Test `NewIslandInstance()` with props
- Test `Mount()` calls island's mount handler and stores state
- Test `Render()` calls island's render handler with current state
- Test `CallEvent()` updates state and calls handler
- Test `CallSelf()` handles self-targeted events
- Test `Unmount()` cleanup
- Test state isolation between instances

**Commit**: `feat(v2): add IslandInstance with lifecycle management`

---

### Deliverable 4: Types and Protocol Changes ✓ Commit Point
**Files**: `types.go`, `event.go`, `patch.go`

Implementation:
- Create `IslandID` and `SessionID` type aliases
- Add `Island string` field to `Event` struct
- Create `IslandPatch` wrapper type with island ID
- Add `IslandID` field to `Patch` struct

Tests (modify existing `event_test.go`, `diff_test.go`):
- Test Event JSON serialization with island field
- Test IslandPatch wrapping and unwrapping
- Test Patch with island ID field

**Commit**: `feat(v2): add island-scoped event and patch protocol`

---

### Deliverable 5: Transport Interface (Server) ✓ Commit Point
**Files**: `transport.go`, `transport_test.go`

Implementation:
- Define `Transport` interface (`Send()`, `Events()`, `Close()`)
- Define `TransportFactory` interface for HTTP upgrades
- Define transport configuration types

Tests (`transport_test.go`):
- Create mock transport implementing interface
- Test Send/Receive contract
- Test Close cleanup
- Test context cancellation

**Commit**: `feat(v2): add Transport interface abstraction`

---

### Deliverable 6: WebSocket Transport (Server) ✓ Commit Point
**Files**: `transport_websocket.go`, `transport_websocket_test.go`, `transport_endpoints.go`

Implementation:
- Extract WebSocket logic from v1 `engine.go:491-652`
- Implement `WebSocketTransport` struct
- Implement `Send()`, `Events()`, `Close()`
- Handle Safari compression workaround
- Create `WebSocketHandler()` endpoint

Tests (`transport_websocket_test.go`):
- Test WebSocket upgrade handshake
- Test bidirectional message flow
- Test reconnection behavior
- Test Safari compression handling
- Integration test with real WebSocket connection

**Commit**: `feat(v2): add WebSocket transport implementation`

---

### Deliverable 7: Island State Store ✓ Commit Point
**Files**: `statestore.go`, `statestore_test.go`

Implementation:
- Define `IslandStateStore` interface with composite keys
- Implement `MemoryIslandStateStore` with janitor
- Implement `Get(SessionID, IslandID)`, `Set()`, `Delete()`
- TTL-based cleanup goroutine

Tests (`statestore_test.go`):
- Test Get/Set/Delete with composite keys
- Test TTL expiration via janitor
- Test concurrent access safety
- Test cleanup on session end

**Commit**: `feat(v2): add IslandStateStore with composite key storage`

---

### Deliverable 8: Session ✓ Commit Point
**Files**: `session.go`, `session_test.go`

Implementation:
- Define `Session` struct (ID, transport, islands map, channels)
- Implement session lifecycle methods
- Implement island instance tracking per session
- Message routing to islands within session

Tests (`session_test.go`):
- Test session creation with transport
- Test adding/removing islands from session
- Test message routing to correct island
- Test session cleanup

**Commit**: `feat(v2): add Session for transport-agnostic connection management`

---

### Deliverable 9: Island-Scoped Anchoring ✓ Commit Point
**Files**: `diff.go`, `diff_test.go`

Implementation:
- Add `islandAnchorPrefix = "_i"` constant
- Create `islandAnchorGenerator` struct
- Implement island-scoped anchor format: `_i_{islandID}_{path}`
- Update `hasAnchor()` to check both `_l` and `_i` prefixes

Tests (modify `diff_test.go`):
- Test `islandAnchorGenerator.String()` format
- Test anchor generation for nested elements
- Test anchor uniqueness across different islands
- Test backward compatibility with `_l` prefix

**Commit**: `feat(v2): add island-scoped anchor generation`

---

### Deliverable 10: Island Rendering and Diffing ✓ Commit Point
**Files**: `render.go`, `diff.go`, `render_test.go`

Implementation:
- Create `IslandRenderContext` struct
- Implement `RenderIsland(ctx, island, state)` function
- Implement `DiffIsland(islandID, current, proposed)` function
- Update `Patch` generation to include island ID

Tests (`render_test.go`, extend `diff_test.go`):
- Test `RenderIsland()` with template
- Test `DiffIsland()` generates island-scoped patches
- Test patch island ID is set correctly
- Test multiple islands diff independently

**Commit**: `feat(v2): add island-scoped rendering and diffing`

---

### Deliverable 11: Island Engine ✓ Commit Point
**Files**: `engine.go` (create new v2 version), `engine_test.go`

Implementation:
- Create `IslandEngine` struct (registry, sessions, stateStore)
- Implement channel-based session management
- Implement `AddSession()`, `GetSession()`, `DeleteSession()`
- Implement `MountIsland()`, `UnmountIsland()`
- Implement `RouteEvent()` message routing
- Implement `BroadcastToIslandType()`, `BroadcastToIsland()`

Tests (`engine_test.go`):
- Test session lifecycle (add/get/delete)
- Test island mounting with props
- Test event routing to correct island
- Test state updates after events
- Test unmounting cleanup
- Test broadcast targeting
- Test concurrent operations safety

**Commit**: `feat(v2): add IslandEngine with session and island lifecycle management`

---

### Deliverable 12: Client Transport Layer ✓ Commit Point
**Files**: `web/src/transport/transport.ts`, `web/src/transport/message.ts`, `web/src/transport/websocket.ts`, `web/src/transport/websocket.spec.ts`

**Delete**: `web/src/socket.ts` (replaced by transport abstraction)

Implementation:
- Define TypeScript `Transport` interface
- Define `TransportMessage` and `IslandPatch` types
- Implement `WebSocketTransport` class (extract logic from old `socket.ts`)
- Auto-reconnection with backoff
- Session ID management (cookie-based)
- Island subscription routing

Tests (`websocket.spec.ts`):
- Test connection establishment
- Test message send/receive with island field
- Test island subscription
- Test reconnection with backoff
- Test session ID persistence

**Commit**: `feat(v2): replace socket.ts with transport abstraction and WebSocket transport`

---

### Deliverable 13: SSE and Transport Negotiation ✓ Commit Point
**Files**: `web/src/transport/sse.ts`, `web/src/transport/sse.spec.ts`, `web/src/transport/negotiator.ts`, `web/src/transport/negotiator.spec.ts`

Implementation:
- Implement `SSETransport` class (EventSource + fetch POST)
- Implement `TransportNegotiator` with fallback chain
- Auto-select transport: WebSocket → SSE → Polling

Tests:
- `sse.spec.ts`: Test SSE stream, POST events, reconnection
- `negotiator.spec.ts`: Test fallback chain, timeout handling

**Commit**: `feat(v2): add SSE transport and auto-negotiation`

---

### Deliverable 14: Connection Manager ✓ Commit Point
**Files**: `web/src/connection.ts`, `web/src/connection.spec.ts`

Implementation:
- Implement `ConnectionManager` singleton
- Manage shared transport connection across all islands
- Island registration/deregistration
- Message routing to island subscribers
- Reconnection with island re-subscription

Tests (`connection.spec.ts`):
- Test singleton pattern
- Test island registration
- Test message routing to correct island
- Test reconnection re-subscribes islands

**Commit**: `feat(v2): add ConnectionManager for shared island connections`

---

### Deliverable 15: LiveIsland Custom Element ✓ Commit Point
**Files**: `web/src/island.ts`, `web/src/island.spec.ts`

**Delete**: `web/src/live.ts` (old single-page initialization)

Implementation:
- Define `IslandProps`, `IslandConfig` types
- Implement `LiveIsland` custom element class
- `connectedCallback()`: extract props, register with ConnectionManager
- `disconnectedCallback()`: unsubscribe, cleanup
- `attributeChangedCallback()`: handle prop updates
- Props extraction from `type`, `id`, `data-*` attributes

Tests (`island.spec.ts`):
- Test custom element registration
- Test `connectedCallback()` props extraction
- Test connection registration on connect
- Test disconnection cleanup
- Test attribute changes trigger updates

**Commit**: `feat(v2): replace Live class with LiveIsland custom element`

---

### Deliverable 16: Island-Scoped Event Wiring ✓ Commit Point
**Files**: `web/src/events.ts` (replace contents), `web/src/events.spec.ts`

Implementation:
- Replace document-wide event handlers with island-scoped
- `querySelectorAll` within island element, not document
- Support `live-*` attributes within island boundary
- Debounce and limiter support preserved
- Loading state management per island

Tests (`events.spec.ts`):
- Replace existing tests with island-scoped versions
- Test click, submit, keydown handlers within island
- Test debounce functionality
- Test loading class application
- Test events don't leak between islands

**Commit**: `feat(v2): replace document-wide events with island-scoped wiring`

---

### Deliverable 17: Island-Scoped Patch Handling ✓ Commit Point
**Files**: `web/src/patch.ts` (replace contents), `web/src/patch.spec.ts`

Implementation:
- Replace document-wide patching with island-scoped
- Query anchors within island element only
- Form state preservation within island (keep `forms.ts` logic)
- Lifecycle events (beforeUpdate, updated, destroyed)
- Re-wire events after patching

Tests (`patch.spec.ts`):
- Replace existing tests with island-scoped versions
- Test patch application within island
- Test replace, append, prepend actions
- Test form state preservation
- Test lifecycle event dispatch
- Test event re-wiring after patch

**Commit**: `feat(v2): replace document-wide patching with island-scoped patches`

---

### Deliverable 18: Island Hooks and Entry Point ✓ Commit Point
**Files**: `web/src/hooks.ts`, `web/src/auto.ts` (replace contents), `web/src/index.ts` (replace contents)

**Keep**: `web/src/interop.ts` (Hook interface still valid), `web/src/forms.ts` (form utils still needed)
**Delete**: Old files no longer needed

Implementation:
- Update hooks to be island-scoped
- `pushEvent` bound to island transport/ID
- Replace `auto.ts` initialization with custom element registration
- Update `index.ts` to export v2 API

Tests (`hooks.spec.ts`):
- Test hook registration per island
- Test `pushEvent` sends to correct island
- Test lifecycle hooks called at right times

**Commit**: `feat(v2): update hooks and entry point for islands architecture`

---

### Deliverable 19: Client Build Configuration ✓ Commit Point
**Files**: `web/package.json`

Implementation:
- Update build script to bundle from new entry point
- Ensure all new transport/island files are included
- Update test patterns for new file structure

**Commit**: `chore(v2): update client build for islands architecture`

---

### Deliverable 22: SSE Transport (Server) ✓ Commit Point
**Files**: `transport_sse.go`, `transport_sse_test.go`

Implementation:
- Implement `SSETransport` struct
- Server-Sent Events streaming
- HTTP POST endpoint for client events
- Last-Event-ID support for reconnection

Tests (`transport_sse_test.go`):
- Test SSE stream setup
- Test event delivery to client
- Test POST event reception
- Test Last-Event-ID reconnection

**Commit**: `feat(v2): add SSE transport for server-to-client streaming`

---

### Deliverable 23: Remove v1 Code ✓ Commit Point
**Files Deleted**:
- `page/component.go` - v1 component abstraction (replaced by Island)
- `page/configuration.go` - v1 component configs
- `page/render.go` - v1 component rendering
- `handler.go` - v1 full-page Handler (replaced by Island)
- Old `engine.go` - v1 Engine (replaced by IslandEngine)
- `socket.go` - v1 Socket (replaced by Session)
- All v1 examples in `examples/` (replaced by v2 examples)

Implementation:
- Remove all v1 Handler/Engine/Component code
- Remove v1 full-page examples
- Clean up any v1-specific imports

**Commit**: `chore(v2): remove v1 Handler, Engine, Component, and Socket code`

---

### Deliverable 24: Counter Example ✓ Commit Point
**Files**: `examples/counter/main.go`, `examples/counter/index.html`, `examples/counter/counter.html`

Implementation:
- Create counter island with inc/dec events
- Register island with `live.RegisterIsland()`
- HTML with `<live-island type="counter" id="counter-1">`
- Demonstrate props passing (`initial-value="5"`)
- Serve all transports (WebSocket, SSE)

Manual Testing:
- Load example in browser
- Test increment/decrement
- Test WebSocket transport
- Test SSE fallback
- Test reconnection
- Test multiple counter instances on same page

**Commit**: `feat(v2): add counter example demonstrating islands API`

---

### Deliverable 25: Integration Tests ✓ Commit Point
**Files**: `integration_test.go`

Implementation:
- End-to-end test with real transport
- Test island mount → event → render → patch flow
- Test multiple islands on same session
- Test transport reconnection with state preservation

Tests (`integration_test.go`):
- Full lifecycle integration test
- Multi-island coordination test
- Reconnection state preservation test

**Commit**: `test(v2): add end-to-end integration tests`

---

### Optional Deliverable: Polling Transport ✓ Commit Point
**Files**: `transport_polling.go`, `transport_polling_test.go`, `web/src/transport/polling.ts`, `web/src/transport/polling.spec.ts`

Implementation:
- Server: Long-polling implementation
- Client: Fetch-based polling
- Event ID tracking for continuity
- Integrate into negotiator fallback chain

Tests: Server and client polling tests

**Commit**: `feat(v2): add polling transport fallback`

## Implementation Order Summary

The 25 deliverables above should be implemented in order, as each builds on previous work. Key milestones:

**Deliverables 1-3**: Island foundation (types, registry, instances)
**Deliverables 4-8**: Protocol changes and server infrastructure
**Deliverables 9-11**: Rendering and engine implementation
**Deliverables 12-19**: Client rewrite (transport, custom element, events, patches)
**Deliverable 20-22**: SSE transport and final client integration
**Deliverable 23**: **Remove all v1 code** (Handler, Engine, Component, Socket)
**Deliverables 24-25**: Examples and integration tests

Each deliverable is a complete, tested feature with a commit point.

## Implementation Notes

### Cross-cutting Concerns

1. **State Isolation**: Each island instance has completely isolated state. Cross-island communication only via explicit events.

2. **Transport Abstraction**: All server code uses `Transport` interface, never WebSocket directly. Client negotiates best transport.

3. **Session vs Socket**: v2 `Session` is transport-agnostic. It manages multiple `IslandInstance` objects, each with their own lifecycle.

4. **Anchor Format**: v2 anchors use `_i_{islandID}_{path}` format. Client patches only within `<live-island>` container.

5. **Protocol Changes**: Events include `island` field. Patches wrapped with island ID for routing.

### Edge Cases

1. **Nested Islands**: Parent island render may contain child `<live-island>` elements. Treat as opaque boundaries - don't diff into child content.

2. **Island Reconnection**: If transport reconnects, islands re-subscribe. State restored from IslandStateStore.

3. **Props Changes**: If HTML attributes change, `attributeChangedCallback` fires. Consider whether to re-mount or update.

4. **Multiple Same-Type Islands**: `<live-island type="counter" id="a">` and `<live-island type="counter" id="b">` are fully independent instances.

### Testing Requirements

- All Go tests pass with race detection (`go test -race ./...`)
- All TypeScript/Jest tests pass
- Counter example works end-to-end with WebSocket
- Counter example works with SSE fallback

## Code Removal Strategy

### v1 Code to Remove (Deliverable 23)

**Server-side (Go)**:
- `handler.go` - Full-page Handler struct → Replaced by `island.go`
- `engine.go` - v1 Engine with Handler coupling → Replaced by new IslandEngine
- `socket.go` - WebSocket-coupled Socket → Replaced by `session.go`
- `page/component.go` - v1 component abstraction → Replaced by Island/IslandInstance
- `page/configuration.go` - Component configs → Integrated into Island
- `page/render.go` - Component rendering helpers → Integrated into Island

**Client-side (TypeScript)**:
- `web/src/socket.ts` - Single WebSocket → Replaced by `web/src/transport/websocket.ts`
- `web/src/live.ts` - Old initialization → Replaced by `web/src/island.ts` custom element
- `web/src/events.ts` - Document-wide → Replaced with island-scoped version
- `web/src/patch.ts` - Position-based → Replaced with island-scoped version

**Examples**:
- All `examples/` subdirectories → Replaced by new v2 examples

### Confirmation: No v1 Component Code Remains

After Deliverable 23, the following v1 patterns are **completely removed**:
- ❌ `Handler` with page-level MountHandler/RenderHandler
- ❌ `Engine` with 1:1 Handler coupling
- ❌ `Socket` with WebSocket dependency
- ❌ `page.Component` abstraction
- ❌ Full-page render cycle
- ❌ Position-based anchors (`_l_0_1_0`)
- ❌ Document-wide event wiring
- ❌ Single-page WebSocket connection

Replaced by v2 patterns:
- ✅ `Island` and `IslandInstance` for component logic
- ✅ `IslandEngine` with session management
- ✅ `Session` transport-agnostic connections
- ✅ `Transport` interface with WebSocket/SSE implementations
- ✅ Island-scoped anchors (`_i_islandID_0_1`)
- ✅ Island-scoped event wiring
- ✅ `<live-island>` custom element
- ✅ Shared connection with island routing

## Refs

- Research document: `docs/research/2026-01-25-islands-component-architecture.md`
- Phoenix LiveView components pattern
- Astro Islands architecture
- Web Components / Custom Elements spec
