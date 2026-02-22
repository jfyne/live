# Implementation Plan: Fix broken examples (cluster compile error + chat sender sync)

Fix three categories of bugs in the examples and one related issue in library code:
1. Cluster example fails to compile (`Recieve` typo) and has incorrect log calls
2. Chat example sender doesn't see their own messages due to state revert after broadcast
3. Library `pubsub.go` has the same incorrect log call pattern

## Context

**Research Document**: `docs/research/2026-02-22-113-examples-broken-chat-sync.md`

**Key Files**:
- `examples/cluster/main.go` - Cluster example with typo and log bugs
- `examples/chat/chat.go` - Chat handler with sender sync bug
- `examples/chat/view.html` - Template using `live-update="append"`
- `pubsub.go` - PubSub with `log.Fatal` using format verb
- `engine.go:242-261` - `CallEvent` assigns handler return value then renders

**Architectural Notes**:
- `CallEvent` always calls `sock.Assign(data)` with the handler's return value, then `sock.Render()`. Handlers must return the correct post-event state.
- `Broadcast()` is synchronous by default (calls `self()` inline). With PubSub, `Subscribe()` replaces `BroadcastHandler` with an async publish, so `HandleSelf` runs later in a different goroutine.
- `live-update="append"` makes the diff algorithm produce APPEND patches. A subsequent render with stale state generates a diff that reverts the append.

**Functional Requirements** (EARS notation):
- When a user clicks a button in the cluster example, the example shall compile and run without errors
- When a user sends a chat message, the sender's UI shall display their own message
- When `log.Printf`/`log.Fatalf` is used with format verbs, the error value shall be interpolated into the output string

## Batch Size

| Metric | Count | Rating |
|--------|-------|--------|
| Tasks  | 4     | Small  |
| Files  | 3     | Small  |
| Stages | 1     | Small  |

**Overall: Small**

## Execution Stages

### Stage 1 (all tasks in parallel — no shared files between cluster and chat)

#### Test Creation Phase
No new tests — these are example code fixes and a one-line library log fix. The existing test suite (`go test -race ./...`) validates that library code still works. The examples have no test files.

#### Implementation Phase (parallel)
- T1: Fix cluster example bugs (examples/cluster/main.go)
- T2: Fix chat sender sync + typo (examples/chat/chat.go)
- T3: Fix pubsub log.Fatal format verb (pubsub.go)

## Task List

### Cluster Example Fixes

- [ ] Fix `Recieve` → `Receive` and log calls (`examples/cluster/main.go`) [Stage 1, T1]
  - Files: `examples/cluster/main.go` (modifies)
  - **Line 63**: Change `p.Recieve(t.Topic, t.Msg)` to `p.Receive(t.Topic, t.Msg)`
    - The method is defined as `Receive` at `pubsub.go:63`. Current spelling prevents compilation.
  - **Line 54**: Change `log.Println("receive message failed: %w", err)` to `log.Printf("receive message failed: %v", err)`
    - `Println` doesn't interpret format verbs. Use `%v` (not `%w` — `%w` is only valid in `fmt.Errorf`).
  - **Line 60**: Change `log.Println("malformed message received: %w", err)` to `log.Printf("malformed message received: %v", err)`
    - Same issue as line 54.

### Chat Sender Sync Fix

- [ ] Fix `send` handler to return post-broadcast state (`examples/chat/chat.go`) [Stage 1, T2]
  - Files: `examples/chat/chat.go` (modifies)
  - **Line 83**: Change `return m, nil` to `return NewChatInstance(s), nil`
    - After `s.Broadcast()` returns, `HandleSelf` has already updated `s.Assigns()` for the sender (in the synchronous default case). Returning `NewChatInstance(s)` reads the current state so the engine's subsequent `sock.Assign(data)` and `sock.Render()` don't revert the append patch.
    - In the async/PubSub case (cluster), `HandleSelf` hasn't run yet, so `NewChatInstance(s)` returns the pre-broadcast state — same as `m`. No regression; `HandleSelf` will still produce the correct append patch when it runs later.
  - **Line 81**: Fix typo `"failed braodcasting"` → `"failed broadcasting"`

### Library Log Fix

- [ ] Fix `log.Fatal` format verb in `pubsub.go` [Stage 1, T3]
  - Files: `pubsub.go` (modifies)
  - **Line 36**: Change `log.Fatal("could not listen on pubsub: %w", err)` to `log.Fatalf("could not listen on pubsub: %v", err)`
    - Same pattern as the cluster example: `Fatal` doesn't interpret format verbs. Note: this also violates the project guideline "Never panic in library code" — `log.Fatal` calls `os.Exit(1)`. However, changing it to return an error is out of scope; the fix here is just the format verb.

## Acceptance Criteria

```gherkin
Feature: Example applications work correctly

  Scenario: Cluster example compiles
    Given the cluster example source at examples/cluster/main.go
    When the code is compiled with `go build`
    Then compilation succeeds without errors

  Scenario: Cluster example logs errors with interpolated values
    Given the cluster example's Listen function receives an error
    When the error is logged
    Then the log output contains the error message interpolated into the format string

  Scenario: Chat sender sees their own message
    Given a user is connected to the chat example
    When the user sends a message via the send event
    Then the sender's DOM displays the new message
    And the message is not reverted by a subsequent render

  Scenario: Chat message broadcasts to other users
    Given two users are connected to the chat example
    When user A sends a message
    Then user B's DOM displays the new message via an append patch
```

**Source**: Generated from plan context and research document

## Implementation Notes

- All changes are to example code except the `pubsub.go` log fix.
- The `log` package's `Printf`/`Fatalf` append a newline if the output doesn't end with one, so no trailing `\n` is needed.
- The chat fix is a one-line change that makes the code explicitly correct. In the synchronous default case, `NewChatInstance(s)` returns the same pointer that `m` points to (since `HandleSelf` mutated it in place), so behaviour is identical. The fix matters most for the async/PubSub (cluster) case.
- Run `go build ./examples/...` to verify cluster compilation and `go test -race ./...` to verify no library regressions.

## Refs

- https://github.com/jfyne/live/issues/113
- `docs/research/2026-02-22-113-examples-broken-chat-sync.md`
