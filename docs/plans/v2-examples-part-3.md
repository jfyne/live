# Implementation Plan: V2 Examples — Forms + Chat + Alpine

Create three v2 examples: forms (merged todo + prefill), chat (multi-server broadcasting via framework PubSub with live-update="append"), and alpine (Alpine.js integration with autocomplete).

## Context

**Research Document**: `docs/research/2026-02-25-v2-examples-porting.md`

**Key Files**:
- `examples/counter/main.go` - Canonical v2 example pattern
- `examples/counter/index.html` - Page template pattern with `<live-island>` and CSS
- `examples/counter/counter.html` - Island render template pattern
- `island.go` - NewIsland, WithMount, WithRender, HandleEvent, HandleSelf
- `engine.go` - MountIsland, RouteEvent, BroadcastToIslandType, BroadcastSelfToIslandType
- `broadcast.go` - BroadcastTransport interface, Broadcast struct, LocalTransport (from Part 1)
- `params.go` - Params.String(), Params.Int(), Params.Checkbox()
- `web/src/events.ts` - Client directives: live-change, live-submit, live-click, live-debounce, live-value-*
- `web/src/hooks.ts` - Client hooks with mounted/updated/destroyed lifecycle

**Architectural Notes**:
- All examples follow the counter example structure exactly
- Forms uses live-change, live-submit, live-debounce="blur", live-value-*, Params.Checkbox()
- Chat uses the framework `Broadcast` with `LocalTransport` for cross-server message delivery via `BroadcastSelfToIslandType`, and live-update="append" for incremental DOM
- Alpine uses Alpine.js v3 via CDN alongside live.js — no build step needed
- Each example is self-contained with full transport boilerplate

**Functional Requirements** (EARS notation):
- When a form input changes with live-change, the system shall send a validate event to the server
- When a form is submitted with live-submit, the system shall send a save event with serialized form data
- When a chat message is sent, the system shall broadcast it to all connected clients via the framework `Broadcast` and `BroadcastSelfToIslandType`
- When a broadcast event arrives, the system shall append the new message to the DOM without replacing existing messages
- When the user types in the Alpine autocomplete input, the system shall filter suggestions on the server and update the dropdown
- When a suggestion is selected, the system shall add it to the selected list without duplicates

**Branch**: `v2`
**Stack**: 3 of 3 (base: Part 2 branch)
**Stack Plans**:
- 1: `docs/plans/v2-examples-part-1.md`
- 2: `docs/plans/v2-examples-part-2.md`
- 3: `docs/plans/v2-examples-part-3.md` (this plan)

## Batch Size

| Metric | Count | Rating |
|--------|-------|--------|
| Tasks | 8 | Medium |
| Files | 13 | Medium |
| Stages | 2 | Small |

**Overall: Medium**

## Execution Stages

### Stage 1

#### Test Creation Phase (parallel)
- T-test-forms: Write test verifying forms island handles validate/save/done events (`examples/forms/main_test.go`) (hmm-test-writer)
  - New feature tests (RED): validate returns errors, save appends task, done toggles completion, prefill mounts with initial values
- T-test-chat: Write test verifying chat island handles send/newmessage events (`examples/chat/main_test.go`) (hmm-test-writer)
  - New feature tests (RED): send appends message, newmessage from broadcast updates state
- T-test-alpine: Write test verifying alpine island handles suggest/selected events (`examples/alpine/main_test.go`) (hmm-test-writer)
  - New feature tests (RED): suggest filters items, selected adds to list, no duplicates

#### Implementation Phase (parallel, depends on Test Creation Phase)
- T-impl-forms: Create forms example (main.go, index.html, todo.html, prefill.html) (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
- T-impl-chat: Create chat example (main.go, index.html, chat.html) (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)
- T-impl-alpine: Create alpine example (main.go, index.html, alpine.html) (hmm-implement-worker, TDD mode)
  - Make RED tests pass (GREEN)

### Stage 2 (depends on Stage 1)

#### Implementation Phase
- T-verify: Verify all examples compile and tests pass (hmm-implement-worker)
  - Run `go build ./examples/forms/...`, `go build ./examples/chat/...`, `go build ./examples/alpine/...`
  - Run `go test ./examples/forms/...`, `go test ./examples/chat/...`, `go test ./examples/alpine/...`

## Task List

### Forms Example (merged todo + prefill)

- [ ] Write forms integration tests (`examples/forms/main_test.go`) [Stage 1, Test Creation Phase]
  - Files: `examples/forms/main_test.go` (creates)
  - **TestTodoIsland_Validate**: Create todo island, call "validate" with empty task name, verify `Errors["message"]` is set
  - **TestTodoIsland_Save**: Call "save" with valid task name, verify task appended, NextID incremented
  - **TestTodoIsland_Done**: Save a task, call "done" with its ID, verify Complete toggled
  - **TestPrefillIsland_Mount**: Mount prefill island with `Props{"name": "Test User", "age": 35}`, verify state has those values
  - **TestPrefillIsland_Validate**: Call "validate" with short name, verify Validation message set
  - Helper: create registry, engine, mock transport, session — same pattern as `engine_test.go`

- [ ] Create forms island definitions and HTTP server (`examples/forms/main.go`) [Stage 1]
  - Files: `examples/forms/main.go` (creates)
  - Two island types registered: "todo" and "prefill"
  - **TodoState**:
    ```go
    type Task struct {
        ID       string
        Name     string
        Complete bool
    }
    type TodoState struct {
        Tasks  []Task
        Errors map[string]string
        NextID int
    }
    ```
  - **NewTodoIsland()**: mount returns empty TodoState (NextID: 1, Errors: empty map). Three event handlers:
    - `"validate"`: extract `params.String("task")`, validate length (must be > 0 and < 100 chars), set/clear `Errors["message"]`
    - `"save"`: validate, if valid append `Task{ID: fmt.Sprintf("task-%d", s.NextID), Name: name}`, increment NextID, clear errors
    - `"done"`: extract `params.String("id")`, toggle `Complete` on matching task
  - **PrefillState**:
    ```go
    type PrefillState struct {
        Name       string
        Age        int
        Validation string
    }
    ```
  - **NewPrefillIsland()**: mount returns `PrefillState{Name: props.String("name"), Age: props.Int("age")}`. Two event handlers:
    - `"validate"`: extract `params.String("name")`, validate: empty → `"Name is required"`, len > 200 → `"Name must be under 200 characters"`, valid → clear Validation
    - `"save"`: extract name and age, update state
  - HTTP server: register both islands, engine setup, WS/SSE transport
  - Subscribe handler: for "prefill" type, pass `Props{"name": "Test User", "age": 35}`; for "todo", pass empty Props
  - Page handler: render index.html with static data (island IDs/types)

- [ ] Create forms page template (`examples/forms/index.html`) [Stage 1]
  - Files: `examples/forms/index.html` (creates)
  - Two sections: Todo List and Prefill Form
  - `<live-island type="todo" id="todo-list">` with fallback form
  - `<live-island type="prefill" id="prefill-form">` with fallback form
  - CSS for .error (red), .task (flex row), .done (strikethrough), form layout
  - Feature list: live-change, live-submit, live-debounce, checkboxes, validation, prefill
  - `<script src="/live.js"></script>`

- [ ] Create todo island template (`examples/forms/todo.html`) [Stage 1]
  - Files: `examples/forms/todo.html` (creates)
  - Form with `id="todo-form" live-change="validate" live-submit="save"`
  - Error display: `{{ if index .Errors "message" }}<div class="error">{{ index .Errors "message" }}</div>{{ end }}`
  - Text input: `<input type="text" name="task" live-debounce="blur" placeholder="Enter a task...">`
  - Submit button: `<button type="submit">Add Task</button>`
  - Task list: `{{ range .Tasks }}` with checkbox `<input type="checkbox" live-click="done" live-value-id="{{ .ID }}" {{ if .Complete }}checked{{ end }}>` and name with conditional strikethrough

- [ ] Create prefill island template (`examples/forms/prefill.html`) [Stage 1]
  - Files: `examples/forms/prefill.html` (creates)
  - Form with `id="prefill-form" live-change="validate" live-submit="save"`
  - Validation: `{{ if .Validation }}<div class="error">{{ .Validation }}</div>{{ end }}`
  - Name input: `<input type="text" name="name" value="{{ .Name }}">`
  - Age input: `<input type="number" name="age" value="{{ .Age }}">`
  - Submit button: `<button type="submit">Save</button>`
  - Current values display: `<div>Current: {{ .Name }}, age {{ .Age }}</div>`

### Chat Example (merged chat + cluster)

- [ ] Write chat integration tests (`examples/chat/main_test.go`) [Stage 1, Test Creation Phase]
  - Files: `examples/chat/main_test.go` (creates)
  - **TestChatIsland_NewMessage**: Create chat island, call "newmessage" self handler with Message data, verify state.Messages contains the message
  - **TestChatIsland_NewMessageAppend**: Call "newmessage" twice, verify state only contains the latest message (single-message for append mode)
  - **TestChatIsland_Mount**: Mount chat island with Props{"user": "session-1"}, verify initial state has welcome message
  - Helper: create registry, engine, mock transport, session — same pattern as `engine_test.go`

- [ ] Create chat island definition and HTTP server (`examples/chat/main.go`) [Stage 1]
  - Files: `examples/chat/main.go` (creates)
  - **State**:
    ```go
    type Message struct {
        ID   string
        User string
        Msg  string
    }
    type ChatState struct {
        Messages []Message
    }
    ```
  - **NewChatIsland()**: mount returns `ChatState{Messages: []Message{welcomeMsg}}` where welcomeMsg uses `props.String("user")` as user. Event handlers:
    - `"send"`: validates message, returns state unchanged (broadcast handles display for all clients)
    - `"newmessage"`: receives Message via `data`, sets `state.Messages = []Message{msg}` (single message for append mode), returns state
  - Registration: `live.RegisterIsland("chat", NewChatIsland)`
  - **Broadcast pattern — framework `Broadcast` with `LocalTransport`**:
    - Uses the framework-level `live.Broadcast` (from Part 1) instead of example-local abstractions
    - `BroadcastToIslandType` sends events directly to the client transport, NOT through `HandleSelf`. The `Broadcast` uses `BroadcastSelfToIslandType` instead, which routes through handlers so state is updated.
    - **Multi-server demo**: The example runs multiple HTTP servers on different ports (`:8080` and `:8081`) in the same process, each with their own engine, sharing one `Broadcast`:
      ```go
      func main() {
          ctx := context.Background()
          broadcast := live.NewBroadcast(ctx, live.NewLocalTransport())
          // Start server 1 on :8080
          go startServer(":8080", broadcast)
          // Start server 2 on :8081
          startServer(":8081", broadcast)
      }
      ```
    - Each `startServer` creates its own engine and subscribes to the broadcast:
      ```go
      func startServer(addr string, broadcast *live.Broadcast) {
          engine := live.NewIslandEngine(ctx, registry, store)
          broadcast.Subscribe("chat-room", "chat", engine)
          // ... transport setup ...
      }
      ```
    - The "send" event handler validates the message and returns state unchanged. The event loop publishes to the broadcast:
      ```go
      // In the event loop, after RouteEvent:
      if event.T == "send" {
          broadcast.Publish(ctx, "chat-room", live.Event{
              T:        "newmessage",
              SelfData: Message{ID: id, User: user, Msg: msg},
          })
      }
      ```
    - The `Broadcast` delivers the event via `BroadcastSelfToIslandType("chat", event)` to all subscribed engines, routing through each island's "newmessage" `HandleSelf` handler
  - HTTP server: `startServer(addr string, broadcast *live.Broadcast)` function with engine setup, WS/SSE transport
  - Subscribe handler: `engine.MountIsland(sessionID, islandID, "chat", Props{"user": string(sessionID)})`

- [ ] Create chat page template (`examples/chat/index.html`) [Stage 1]
  - Files: `examples/chat/index.html` (creates)
  - Single `<live-island type="chat" id="chat-room">`
  - Description: explains cluster demo — open two browser tabs on different ports (8080 and 8081) to see cross-server messaging
  - JavaScript hook for "chat" (form clearing on send):
    ```javascript
    window.Hooks = {
        "chat": {
            mounted: function() {
                this.el.addEventListener("submit", function() {
                    setTimeout(function() {
                        document.querySelector("[name='message']").value = "";
                    }, 100);
                });
            }
        }
    };
    ```
  - CSS for .messages (scrollable), .message (flex row), .user (bold), form layout
  - `<script src="/live.js"></script>`

- [ ] Create chat island template (`examples/chat/chat.html`) [Stage 1]
  - Files: `examples/chat/chat.html` (creates)
  - Messages container: `<div id="messages" class="messages" live-update="append">{{ range .Messages }}<div id="{{ .ID }}" class="message"><strong>{{ .User }}:</strong> {{ .Msg }}</div>{{ end }}</div>`
  - Form: `<form id="chat-form" live-submit="send" live-hook="chat"><input type="text" name="message" placeholder="Type a message..." autocomplete="off"><button type="submit">Send</button></form>`

### Alpine Example

- [ ] Write alpine integration tests (`examples/alpine/main_test.go`) [Stage 1, Test Creation Phase]
  - Files: `examples/alpine/main_test.go` (creates)
  - **TestAlpineIsland_Suggest**: Create island, call "suggest" with search "go", verify Suggestions contains matching items
  - **TestAlpineIsland_SuggestNoMatch**: Call "suggest" with search "xyz", verify Suggestions is empty
  - **TestAlpineIsland_Selected**: Call "selected" with valid item ID, verify item added to Selected
  - **TestAlpineIsland_SelectedNoDuplicate**: Select same item twice, verify Selected has it only once
  - Helper: create registry, engine, mock transport, session

- [ ] Create alpine island definition and HTTP server (`examples/alpine/main.go`) [Stage 1]
  - Files: `examples/alpine/main.go` (creates)
  - **State**:
    ```go
    type Item struct {
        ID   string
        Name string
    }
    func (i Item) Match(search string) bool {
        return strings.Contains(strings.ToLower(i.Name), strings.ToLower(search))
    }
    type AlpineState struct {
        Items       []Item
        Suggestions []Item
        Selected    []Item
    }
    ```
  - **NewAlpineIsland()**: mount returns state with predefined items list (e.g., programming languages: Go, JavaScript, Python, Rust, TypeScript). Event handlers:
    - `"suggest"`: extract `params.String("search")`, filter items by match, set `state.Suggestions`
    - `"selected"`: extract `params.String("id")`, find item, add to `state.Selected` if not already present (dedup by ID)
    - `"submit"`: return state unchanged
  - Registration: `live.RegisterIsland("alpine", NewAlpineIsland)`
  - HTTP server: same pattern as counter
  - Subscribe handler: mount with empty props

- [ ] Create alpine page template (`examples/alpine/index.html`) [Stage 1]
  - Files: `examples/alpine/index.html` (creates)
  - Alpine.js v3 via CDN: `<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3/dist/cdn.min.js"></script>`
  - Single `<live-island type="alpine" id="autocomplete-1">`
  - Alpine `autocomplete()` function inline:
    ```javascript
    function autocomplete() {
        return {
            isOpen: false,
            open() { this.isOpen = true; },
            close() { this.isOpen = false; }
        };
    }
    ```
  - CSS for autocomplete dropdown, suggestions list, selected items
  - `<script src="/live.js"></script>` (before Alpine to ensure live.js loads first)
  - Note: Use Alpine.js v3 `@click.outside` instead of v2 `@click.away`

- [ ] Create alpine island template (`examples/alpine/alpine.html`) [Stage 1]
  - Files: `examples/alpine/alpine.html` (creates)
  - Wrapper: `<div class="autocomplete" x-data="autocomplete()" @click.outside="close()">`
  - Form: `<form id="autocomplete-form" live-submit="submit" live-change="suggest">`
  - Input: `<input type="text" name="search" x-on:focus="open" placeholder="Search..." autocomplete="off">`
  - Suggestions dropdown: `<ul class="suggestions" x-show="isOpen">{{ range .Suggestions }}<li live-click="selected" live-value-id="{{ .ID }}">{{ .Name }}</li>{{ end }}</ul>`
  - Selected items: `{{ if .Selected }}<div class="selected"><h3>Selected:</h3><ul>{{ range .Selected }}<li>{{ .Name }}</li>{{ end }}</ul></div>{{ end }}`

## Acceptance Criteria

~~~gherkin
Feature: Forms example — Todo list

  Scenario: Add a task via form submission
    Given the forms example is running and the todo island is mounted
    When the user types "Buy groceries" in the task input
    And submits the form
    Then "Buy groceries" appears in the task list

  Scenario: Validate task input on change
    Given the todo island is mounted
    When the user blurs the task input while it is empty
    Then a validation error message is displayed

  Scenario: Toggle task completion
    Given a task "Buy groceries" exists in the list
    When the user clicks the checkbox next to it
    Then the task is marked as complete with strikethrough styling

Feature: Forms example — Prefill form

  Scenario: Form loads with initial values
    Given the prefill island is mounted with props name="Test User" age=35
    Then the name input shows "Test User"
    And the age input shows "35"

  Scenario: Update prefill values
    Given the prefill form is displayed
    When the user changes name to "New Name" and submits
    Then the displayed values update to "New Name"

Feature: Chat example

  Scenario: Send a message within same server
    Given two browser tabs are connected to the chat example on port 8080
    When Tab 1 sends "Hello everyone"
    Then both tabs display the message "Hello everyone"

  Scenario: Cross-server message sync
    Given one browser is connected to the chat on port 8080
    And another browser is connected to the chat on port 8081
    When the first browser sends "Hello from 8080"
    Then the second browser also displays "Hello from 8080"

  Scenario: Messages append without replacing
    Given a chat with existing messages
    When a new message arrives via broadcast
    Then the new message is appended to the existing messages
    And previous messages remain visible

Feature: Alpine example — Autocomplete

  Scenario: Suggestions appear on typing
    Given the alpine example is running
    When the user types "go" in the search input
    Then a dropdown shows matching suggestions (e.g., "Go")

  Scenario: Select a suggestion
    Given suggestions are showing
    When the user clicks "Go"
    Then "Go" appears in the selected items list
    And the suggestions dropdown closes

  Scenario: No duplicate selections
    Given "Go" is already in the selected list
    When the user searches and clicks "Go" again
    Then the selected list still shows "Go" only once
~~~

**Source**: Generated from plan context

## Implementation Notes

- **Forms ID generation**: No `live.NewID()` in v2. Use `fmt.Sprintf("task-%d", state.NextID)` with incrementing counter.
- **Forms error map access**: Use `{{ index .Errors "message" }}` in Go templates for map access.
- **Chat Broadcast pattern**: Uses the framework-level `live.Broadcast` with `live.LocalTransport` (from Part 1). `BroadcastToIslandType` sends directly to client transport — but `Broadcast` uses `BroadcastSelfToIslandType` which routes through `HandleSelf` handlers, so server-side state is updated. No example-local broadcast abstraction needed.
- **Chat multi-server demo**: The example runs two HTTP servers on different ports (`:8080`, `:8081`) sharing a single `live.Broadcast`. This demonstrates cross-server message sync. In production, users implement the `BroadcastTransport` interface for Redis, NATS, or similar.
- **Chat live-update="append"**: The diff engine produces Append patches when this attribute is present. The "newmessage" handler sets `state.Messages = []Message{newMsg}` (single message), and the re-rendered HTML contains only that message, which gets appended to the existing DOM.
- **Chat reconnect caveat**: On reconnect, the state store only has the last single message, not full history. The mount handler provides a welcome message. Full message history would require external storage (out of scope for this example).
- **Alpine.js v3 vs v2**: Use `@click.outside` instead of `@click.away`. Alpine v3 is loaded via CDN with `defer`.
- **Alpine DOM reconciliation**: When v2 patches replace elements with `x-data`, Alpine.js v3 re-initializes from the attribute. Client-side state (like `isOpen`) resets on re-render, which is acceptable for the autocomplete pattern.
- **Form state preservation**: The v2 client's `Forms.dehydrate()`/`Forms.hydrate()` preserves input focus and values across re-renders. Forms must have unique `id` attributes.

## Refs

- `docs/research/2026-02-25-v2-examples-porting.md`
- `docs/plans/v2-examples-part-1.md` — Framework API changes
- `docs/plans/v2-examples-part-2.md` — Clock + Hooks examples
