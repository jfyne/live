---
date: 2026-02-22T00:00:00+00:00
researcher: josh
topic: "Examples broken (buttons UI, cluster typo) + chat sync issue"
tags: [research, codebase, examples, buttons, cluster, chat, broadcast, render, socket]
last_updated: 2026-02-22
last_updated_by: josh
---

# Research: Examples Broken (issue #113) + Chat Sync Issue

## Research Question

1. **Issue #113** (`jfyne/live/issues/113`): The buttons example UI doesn't update when
   buttons are pressed (requests are logged but the counter value doesn't change), and the
   cluster example fails to compile due to a typo.
2. **Chat sync**: Chat example messages are not being synced properly.

---

## Summary

Three distinct problems are documented here:

1. **Buttons UI not updating** — The root causes exist in the *original* `jfyne/live`
   repository (up to v0.15.5). The `live2` codebase has already landed fixes for two bugs
   that together explain the symptom:
   - `closeSlow` was not initialised in `NewSocket`, so a full message buffer caused a nil
     function panic that silently dropped the WebSocket send goroutine.
   - Concurrent renders had no mutex protection, allowing corrupted render trees and missing
     diffs.

2. **Cluster example compile error** — A method-name typo at
   `examples/cluster/main.go:63` (`p.Recieve` → `p.Receive`) prevents compilation. Two
   `log.Println` calls also use format-verb strings incorrectly.

3. **Chat sender sync** — When a user sends a chat message, the broadcast runs
   synchronously *inside* the `send` event handler. `HandleSelf` assigns new state and
   renders the sender's socket first, then the handler returns the *old* state, which the
   engine immediately re-assigns and re-renders, reverting the sender's DOM to the state
   before the new message was added.

---

## Detailed Findings

### 1. Buttons UI Not Updating

#### How the buttons example works

`examples/buttons/main.go` implements a simple counter:

- **State**: `counter{Value int}` stored in `socket.Assigns()`.
- **Events**: `inc` / `dec` handlers retrieve the current counter, mutate `Value`, and
  return the updated struct.
- **Template** (`examples/buttons/view.html:17`): `{{.Assigns.Value}}` renders the
  current count inside a `<div>`.
- **Attributes**: `live-click="inc"` / `live-click="dec"` on the buttons; keyboard
  shortcuts via `live-window-keyup` + `live-key`.

Expected flow per click:

```
browser click
  → WebSocket event {"t":"inc"}
  → engine.go:557  CallEvent("inc", sock, msg)
  → handler returns counter{Value+1}
  → sock.Assign(counter{Value+1})
  → engine.go:566  sock.Render(ctx)
  → renderSocket → Diff → Send(EventPatch, patches)
  → sock.msgs channel → WebSocket write
  → browser patch.ts applies DOM update
```

#### Bug A — `closeSlow` nil panic (commit `a355c43`)

In `live2` before commit `a355c43`, `NewSocket` did not initialise `closeSlow`:

```go
// before fix (pre-a355c43)
s := &Socket{
    // closeSlow not set → nil
    msgs: make(chan Event, maxMessageBufferSize),  // buffer size 16
}
```

When the message buffer was full (e.g., rapid clicks or a slow consumer),
`socket.go` would execute:

```go
// socket.go:209-210
default:
    go s.closeSlow()   // nil function → runtime panic
```

This silently crashed the goroutine responsible for closing slow connections. In some
configurations this also caused the WebSocket send loop to stop draining `sock.msgs`,
which in turn caused all subsequent renders to block or drop their patch messages.

The fix (now in `socket.go:109`):

```go
closeSlow: func() {},   // no-op until assignWS sets the real closer
```

This bug is **present in the original `jfyne/live` v0.15.5** and all versions up to the
`live2` fix.

#### Bug B — Race condition in RenderSocket (commit `6ee081e`)

Before commit `6ee081e`, `Socket.Render()` had no mutex:

```go
// pre-fix
func (s *Socket) Render(ctx context.Context) error {
    render, err := renderSocket(ctx, s.engine, s)
    ...
    s.updateRender(render)
    return nil
}
```

Two goroutines can call `Render` concurrently:
- The WebSocket event-reader goroutine (`engine.go:532`) after each event.
- `handleEmittedEvent` (`engine.go:189`) on broadcast/self events.

Without a lock, `s.currentRender` could be partially updated while a concurrent diff was
reading it, producing an incorrect (or empty) patch set. The fix adds `renderMu`:

```go
// socket.go:52-61
func (s *Socket) Render(ctx context.Context) error {
    s.renderMu.Lock()
    defer s.renderMu.Unlock()
    render, err := renderSocket(ctx, s.engine, s)
    ...
    s.updateRender(render)
    return nil
}
```

This bug is also **present in `jfyne/live` v0.15.5**.

#### Status in live2

Both fixes are landed on `master` (v0.16.3). The examples `go.mod` uses
`replace github.com/jfyne/live => ../.` so examples in this repo already benefit from
the fixes.

---

### 2. Cluster Example Compile Error

**File**: `examples/cluster/main.go`

#### Typo: `Recieve` → `Receive` (line 63)

```go
// line 63 — does NOT compile
p.Recieve(t.Topic, t.Msg)

// correct spelling (matches pubsub.go:63)
p.Receive(t.Topic, t.Msg)
```

The `PubSub.Receive` method is defined at `pubsub.go:63`.

#### `log.Println` used with format verbs (lines 54, 60)

```go
// lines 54, 60 — format verb %w is not interpreted by Println
log.Println("receive message failed: %w", err)
log.Println("malformed message received: %w", err)

// should use Printf
log.Printf("receive message failed: %w\n", err)
log.Printf("malformed message received: %w\n", err)
```

These are not compile errors, but `%w` is printed literally rather than wrapping the
error.

---

### 3. Chat Sender Sync Issue

#### How broadcast works

When a user submits a chat message, the `send` event handler calls `s.Broadcast`:

```go
// examples/chat/chat.go:80
if err := s.Broadcast(newmessage, data); err != nil { ... }
return m, nil  // ← returns the PRE-broadcast ChatInstance
```

`Broadcast` calls the engine's `BroadcastHandler` **synchronously**:

```go
// engine.go:163-170
func (e *Engine) Broadcast(event string, data any) error {
    ev := Event{T: event, SelfData: data}
    ctx := context.Background()
    e.BroadcastLimiter.Wait(ctx)
    e.BroadcastHandler(ctx, e, ev)   // ← blocks until all sockets are handled
    return nil
}
```

The default handler (`engine.go:139-141`) calls `self(ctx, nil, msg)`, which iterates
**all** connected sockets (including the sender) and for each calls
`handleEmittedEvent`:

```go
// engine.go:189-196
func (e *Engine) handleEmittedEvent(ctx context.Context, s *Socket, msg Event) {
    if err := e.handleSelf(ctx, msg.T, s, msg); err != nil { ... }
    if err := s.Render(ctx); err != nil { ... }      // render with new message
}
```

`handleSelf` invokes `HandleSelf(newmessage, ...)` which assigns the **new** state
(containing only the new message):

```go
// examples/chat/chat.go:87-95
h.HandleSelf(newmessage, func(ctx context.Context, s *live.Socket, data any) (any, error) {
    m := NewChatInstance(s)
    m.Messages = []Message{NewMessage(data)}   // only the new message
    return m, nil
})
```

After `handleEmittedEvent` completes for all sockets (including the sender),
`s.Broadcast()` returns. The `send` handler then returns `m` — the **old**
`ChatInstance` captured before the broadcast.

#### The revert sequence (sender's socket)

```
1. handleSelf  → sock.Assign({new_msg_only})
2.              → sock.Render()  [latestRender updated to show new message]
3. send returns m (old ChatInstance, no new_msg)
4. engine      → sock.Assign(m_old)
5.              → sock.Render()  [diffs against render from step 2]
                                 [generates patch to REVERT to old state]
6. client receives revert patch → sender's DOM loses the new message
```

Other sockets are **not** affected: their state is set by `handleSelf` and there is no
subsequent revert render for them, so they see the new message correctly.

The `live-update="append"` attribute on the `<div class="window">` (chat/view.html:5)
instructs the client to append new DOM nodes. This works for other sockets but the
revert patch in step 5 undoes the append for the sender.

#### Key file references

| File | Lines | Notes |
|------|-------|-------|
| `examples/chat/chat.go` | 69–84 | `send` handler — broadcasts then returns old state |
| `examples/chat/chat.go` | 87–95 | `HandleSelf` for `newmessage` |
| `examples/chat/view.html` | 5 | `live-update="append"` on message container |
| `engine.go` | 163–170 | `Broadcast` — synchronous, blocks until all sockets handled |
| `engine.go` | 139–141 | Default `BroadcastHandler` — broadcasts to nil (all sockets) |
| `engine.go` | 189–196 | `handleEmittedEvent` — HandleSelf + Render per socket |
| `engine.go` | 254–260 | `CallEvent` — assigns handler return value, then Render |
| `engine.go` | 566–568 | Post-event `sock.Render()` call |

---

## Code References

- `examples/buttons/main.go:22-28` — `newCounter` helper reads assigns
- `examples/buttons/view.html:17` — `{{.Assigns.Value}}` template binding
- `examples/cluster/main.go:63` — `p.Recieve` typo (should be `p.Receive`)
- `examples/cluster/main.go:54,60` — `log.Println` with format verbs
- `examples/chat/chat.go:80-83` — Broadcast inside send handler, returns old state
- `socket.go:38` — `renderMu sync.Mutex` (added in fix 6ee081e)
- `socket.go:52-61` — `Render()` with mutex
- `socket.go:109` — `closeSlow: func() {}` initialisation (added in fix a355c43)
- `socket.go:207-211` — `Send()` buffer check and `closeSlow` call
- `render.go:38-48` — Diff and patch dispatch
- `engine.go:163-170` — `Broadcast` method
- `pubsub.go:63` — `PubSub.Receive` (correct spelling)

---

## Architecture Documentation

### Event → Render pipeline

```
WebSocket read (engine.go:532-578, goroutine)
  ↓
json.Unmarshal → Event{T, Data}
  ↓
CallEvent(t, sock, msg)                   engine.go:243-261
  ├─ handler(ctx, sock, params) → data
  └─ sock.Assign(data)
  ↓
sock.Render(ctx)                          engine.go:566, socket.go:52-61
  ├─ renderSocket(ctx, engine, sock)      render.go:21-51
  │   ├─ RenderHandler → HTML
  │   ├─ html.Parse → DOM tree
  │   ├─ shapeTree(render)
  │   └─ Diff(latestRender, render)       diff.go
  │       └─ Send(EventPatch, patches)    socket.go:207
  └─ updateRender(render)
  ↓
sock.msgs channel (buffered 16)
  ↓
Main send loop (engine.go:605-634)
  └─ writeTimeout → WebSocket write
```

### Socket state lifecycle

- `NewSocket` initialises `closeSlow` to a no-op (`socket.go:109`).
- `assignWS` replaces it with the real WebSocket closer (`socket.go:317-320`).
- `Assign` stores data in `socketStateStore` with `infiniteTTL` while connected,
  10s TTL while disconnected (`socket.go:146-153`).

### live-update="append" mechanism

When a DOM node carries `live-update="append"`, the diff algorithm (`diff.go`) marks
generated patches with `PatchAction = Append` (value 2). The client (`web/src/patch.ts`)
appends the rendered HTML as new children rather than replacing the node, preserving the
existing list while adding new items.

---

## External Context

Issue filed: https://github.com/jfyne/live/issues/113

Reporter tested with **Go 1.25** and `go mod tidy` on the original `jfyne/live` repo.
That repo's latest release is v0.15.5 (tagged ~3 years ago). The `live2` repo is the
active development continuation with the same module path (`github.com/jfyne/live`) and
contains all the fixes described above.

---

## Historical Context

- `a355c43` — fix: initialize closeSlow in NewSocket to avoid nil panic
- `6ee081e` — Fix race condition in RenderSocket (#105)
- `2499043` — fix(diff): save and restore live-update modifiers so they don't leak across sub-trees (#108)
- `f351c22` — fix: decouple broadcast iteration from engine socket operations (#110)
- `2399b55` — fix: protect pubsub handlers with mutex (#109)

---

## Open Questions

1. **Should the `send` handler return the state *after* the new message is included?**
   Currently it returns the old `ChatInstance`. If it returned a state that includes the
   new message, the subsequent render would not revert the sender's DOM. However, this
   would then duplicate the message for other sockets (they receive it via HandleSelf AND
   their own state would also have it). A redesign of the send/broadcast pattern may be
   needed.

2. **Does the chat sync issue also affect the cluster example?** The cluster example uses
   the same `chat.NewHandler()`, so the same revert behaviour would apply. Additionally
   the cluster example fails to compile due to the `Recieve` typo, so it has not been
   tested end-to-end.

3. **Is the buttons example broken in the current live2 codebase or only in
   `jfyne/live` v0.15.5?** The two known fixes (`closeSlow` and `renderMu`) are in
   live2's master. If there are further issues they have not yet been identified.
