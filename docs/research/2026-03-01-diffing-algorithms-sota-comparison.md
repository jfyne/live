---
date: 2026-03-01T12:00:00Z
researcher: josh
topic: "DOM Diffing Algorithm Analysis: Current Implementation vs State of the Art"
tags: [research, diffing, dom, rendering, performance, algorithms]
last_updated: 2026-03-01
last_updated_by: josh
---

# Research: DOM Diffing Algorithm Analysis — Current Implementation vs State of the Art

## Research Question

Is the current approach to diffing the rendered output optimal? How does the framework track each element in the DOM? Could this be done differently? What other approaches exist in the wild? Compare the SOTA for diffing with pros and cons of each approach.

## Summary

Live v2 uses a **position-based, server-side tree diff** algorithm. On each render, the server parses the full HTML into `golang.org/x/net/html` node trees, assigns hierarchical anchor attributes based on tree position, compares old vs new trees node-by-node at each position, and sends JSON patches over WebSocket. The client locates target elements via anchor attribute selectors and applies Replace/Append/Prepend operations using native DOM APIs.

This approach is most closely related to **Phoenix LiveView's** server-side model, but with a key architectural difference: LiveView diffs at the template data level (only sending changed dynamic values), while Live v2 diffs at the rendered HTML tree level (full tree comparison on every render). Among client-side approaches, the position-based matching is simpler than morphdom/idiomorph (which use ID-based matching) and lacks the key-based list optimization found in React, Vue, and Snabbdom.

## Detailed Findings

### 1. Live v2's Current Diffing Implementation

#### Server-Side Diff Engine (`diff.go`)

The core algorithm in `diff.go` (~619 lines) operates in four phases:

**Phase 1 — Tree Anchoring** (`diff.go:315-327`):
- Depth-first traversal assigns hierarchical IDs to each relevant node
- Format: `_l_0_1_0_2` (page-level) or `_i_<islandID>_0_1_0_2` (island-scoped)
- Anchors are stored as HTML attribute keys (not values) — e.g. `<div _i_counter-1_0="">`
- The `anchorGenerator` struct (`diff.go:40-47`) tracks position via an `idx []int` slice
- `inc()` increments the last index (next sibling), `level()` appends a new depth level (first child)

**Phase 2 — Node Equality** (`diff.go:552-576`):
- `nodeEqual()` checks: node type, attribute count + deep attribute equality, trimmed text data
- Insignificant whitespace (whitespace-only text nodes) is filtered out by `nodeRelevant()` (`diff.go:540-549`)
- `shapeTree()` (`diff.go:329-353`) removes non-relevant nodes before comparison

**Phase 3 — Recursive Comparison** (`diff.go:377-433`):
- `compareNodes()` walks both trees simultaneously at each child position
- If both nil: no change. If old nil: append new. If new nil: replace with empty (delete). If not equal: replace
- Children are compared index-by-index: `for i := 0; i < max(len(oldChildren), len(newChildren)); i++`
- `live-update` modifier attributes (`diff.go:500-538`) override the default replace action with append/prepend/ignore

**Phase 4 — Patch Generation** (`diff.go:435-464`):
- Each difference produces a `Patch` struct: `{Anchor, Action, HTML, IslandID}`
- Actions: `Noop(0)`, `Replace(1)`, `Append(2)`, `Prepend(3)`
- The entire target node is rendered to an HTML string for the patch — no attribute-only patches exist

#### Render Pipeline (`engine.go:342-373`)

```
Event received → handler updates state → RenderIsland() produces new HTML
→ previousHTML retrieved from instance.lastRenderedHTML → DiffIsland() computes patches
→ patches marshalled to JSON → sent as EventPatch over WebSocket
```

The previous render is stored as `template.HTML` on `IslandInstance.lastRenderedHTML` (`instance.go:38-39`), updated after each render (`instance.go:144-147`).

#### Client-Side Patch Application (`web/src/island.ts:237-293`)

The client applies patches using native DOM APIs:
1. Find target: `this.querySelector(*[${patch.Anchor}])` — scoped to the island element
2. Parse HTML: `html2Node()` uses a `<template>` element for safe parsing
3. Apply action: `target.outerHTML = patch.HTML` (replace), `target.append()`, or `target.prepend()`
4. Post-patch: form state dehydrate/hydrate, event handler re-wiring, lifecycle hooks

No third-party morphing library (morphdom, idiomorph) is used. All DOM operations are native.

#### Element Tracking Strategy

Live v2 uses **pure position-based tracking**. Elements are identified by their structural position in the tree (depth + sibling index), not by any content-based key or user-provided identifier.

**Implications:**
- Inserting an element at the start of a list generates Replace patches for every subsequent sibling
- Reordering elements (e.g. sorting a list) generates Replace patches for all moved positions
- No mechanism exists for the developer to provide keys/IDs to hint at element identity

### 2. The Theoretical Foundation: Tree Edit Distance

The mathematically optimal solution is the **Tree Edit Distance** problem — computing the minimum sequence of insert/delete/replace operations to transform one tree into another.

- **Zhang-Shasha (1989)**: O(n² × m²) dynamic programming
- **RTED (Pawlik & Augsten, 2012)**: O(n³) — the current theoretical best for the general case

For a DOM tree of 1,000 nodes, O(n³) requires ~1 billion comparisons. All production frameworks abandon exact solutions and use heuristics that restrict comparisons to the same tree level, reducing complexity to O(n) or O(n log n) at the cost of occasionally missing the optimal diff.

### 3. SOTA Approach: Phoenix LiveView — Template-Level Diffing

Phoenix LiveView is the closest architectural ancestor to Live. It keeps all state server-side and pushes diffs over WebSocket. However, LiveView's key innovation is **diffing at the template data level, not the DOM level**.

**How it works:**
- HEEx templates are compiled into static parts (literal HTML strings) and dynamic parts (Elixir expressions)
- On initial render, both static and dynamic parts are sent to the browser; static parts are cached client-side
- On subsequent renders, only changed dynamic values are sent as position-indexed payloads: `{"0": "new value", "3": "other value"}`
- The browser reconstructs full HTML from its static cache + new dynamic values, then applies via morphdom/idiomorph

**Diff payload format:**
```json
{"0": "updated text", "c": {"component-1": {"0": "value"}}}
```

**Complexity:** O(d) where d = number of changed dynamic values, not O(n) of all DOM nodes.

| Aspect | Phoenix LiveView | Live v2 |
|--------|-----------------|---------|
| Diff target | Template data structure | Rendered HTML tree |
| Wire payload | Only changed dynamic values | Full node HTML per patch |
| Static content handling | Never re-diffed (compiled out) | Compared every render |
| Client-side patching | morphdom/idiomorph | Custom anchor-based |
| Element tracking | Position indices + `:k` keys | Position-based anchors |

**Pros:** Minimal wire payload; static content has zero runtime cost; compile-time optimization.
**Cons:** Requires compile-time template analysis; tightly coupled to Elixir/Phoenix.

### 4. SOTA Approach: morphdom — In-Place DOM Morphing

morphdom transforms one real DOM tree into another in a single pass without a virtual DOM. Originally created for Marko Widgets, widely used by Phoenix LiveView, htmx, and CableReady.

**Algorithm:**
- Single-pass simultaneous traversal of old and target DOM trees
- **ID-based matching:** nodes with matching `id` attributes are matched across siblings and repositioned rather than destroyed/recreated
- Configurable via `getNodeKey` (defaults to `node.id`)
- `onBeforeElUpdated` callback can short-circuit identical subtrees via `isEqualNode()`

**Complexity:** O(n) single pass; O(1) ID lookup via hash map.

**Pros:** No virtual DOM memory; works directly with server-rendered HTML; preserves form state, focus, scroll position, video playback.
**Cons:** Relies heavily on `id` attributes — without them, falls back to positional matching; does not consider descendant structure for matching.

### 5. SOTA Approach: idiomorph — Structural Morphing via ID-Sets

Created by Carson Gross (htmx author) to solve morphdom's limitation of only matching by a node's own `id`.

**The id-set innovation:**
- Before matching, preprocesses both trees to build an **id-set** for every element: its own `id` plus all descendant `id` values
- Matching rule: two elements match if their id-sets have a non-empty intersection
- A `<div>` without an `id` but containing `<video id="main-video">` correctly matches its counterpart in the new tree

**Users:** Turbo 8 (37signals), htmx, Phoenix LiveView 1.1+.

**Pros:** Better structural matching without requiring IDs everywhere; preserves element state more reliably than morphdom.
**Cons:** ~10% slower than morphdom for large trees (preprocessing cost); additional memory for id-sets.

### 6. SOTA Approach: React Fiber Reconciler — Key-Based Heuristic Diffing

React's reconciler uses two heuristics to reduce O(n³) to O(n):
1. Elements of different types produce entirely different trees (cross-type = tear down + rebuild)
2. Developer `key` props hint which children are stable across renders

**Algorithm:**
- Compare element type first; different type → full subtree replacement
- Same type → update changed attributes, recurse on children
- Lists → use `key` to match children, detect insertions/deletions/moves
- Two-phase: interruptible render phase (builds work-in-progress fiber tree) + synchronous commit phase (flushes to real DOM)

**Complexity:** O(n) heuristic.

**Pros:** Interruptible rendering (concurrent mode); priority scheduling; large ecosystem; predictable reconciliation.
**Cons:** Memory overhead of two fiber trees; O(n) worst case even when few things changed; virtual DOM itself is overhead for small updates; keys must be managed by developers.

### 7. SOTA Approach: Vue 3 — Double-Ended Diff with LIS Optimization

Vue 3 extends Snabbdom's double-ended approach with Longest Increasing Subsequence (LIS) optimization plus compile-time annotations.

**Algorithm stages for keyed lists:**
1. **Preprocessing:** Skip identical head and tail nodes (handles append/prepend/minor edits in O(m))
2. **LIS phase:** For remaining middle nodes — build position map, find LIS, nodes in LIS stay in place, all others move

**Compile-time optimizations:**
- **Patch Flags:** Compiler marks dynamic nodes with bit flags so runtime skips static attribute comparison
- **Static Hoisting:** Static VNodes hoisted to module scope, never re-created or diffed
- **Tree Flattening:** Only dynamic VNodes collected into flat "block" array

**Complexity:** O(n log n) for LIS phase; O(n) for preprocessing.

**Pros:** LIS minimizes actual DOM moves; compile-time flags eliminate redundant comparisons; static hoisting eliminates re-creating unchanged nodes.
**Cons:** More complex algorithm; LIS overhead for small lists; still has virtual DOM overhead.

### 8. SOTA Approach: Svelte 5 / SolidJS — Compile-Time & Fine-Grained Reactivity

These frameworks eliminate runtime diffing entirely.

**Svelte 5:** Compiles templates into imperative DOM update code at build time. Each state variable maps to specific DOM mutations. Runtime cost is O(1) per state change.

**SolidJS:** Components execute once during setup. Signals create direct subscriptions between state and DOM nodes. Updates bypass component re-execution and modify only subscribed DOM nodes. Runtime cost is O(k) where k = DOM nodes subscribed to the changed signal.

**Pros:** Zero virtual DOM overhead; minimal runtime; surgically precise updates.
**Cons:** Require build step; less flexible for highly dynamic structures; cannot defer or schedule work (Svelte); debugging reactive graphs is harder (SolidJS).

### 9. SOTA Approach: Block Virtual DOM (blockdom / million.js)

Rather than element-by-element virtual DOM, a "block" represents a larger chunk with static structure pre-baked and dynamic parts extracted into an **Edit Map**.

**Two phases:**
1. **Static analysis (compile time):** Separate static from dynamic, build Edit Map of `(domPath, stateKey)` pairs
2. **Dirty checking (runtime):** Compare only state values, look up changed keys in Edit Map, apply directly to pre-located DOM nodes

Uses `cloneNode(true)` for initial render (dramatically faster than createElement per element).

**Complexity:** O(d) where d = changed dynamic values.

**Pros:** Near-zero cost for static content; state diff is cheaper than tree diff.
**Cons:** Best suited for templates with lots of static content; block boundaries must be defined at compile time.

### 10. SOTA Approach: Incremental DOM (Google)

Compiles templates into function call sequences that update the DOM in-place. No intermediate tree representation is created.

The real DOM is the only data structure — no shadow copy means update memory is O(delta), not O(tree-size).

**Pros:** Near-zero memory overhead; excellent for tree-shaking.
**Cons:** Not designed for human authorship; limited reordering support; position-based tracking.

## Comparative Analysis

### Comparison Matrix

| Approach | Complexity | Element Tracking | Wire Cost | Memory Overhead | Key Innovation |
|----------|-----------|-----------------|-----------|----------------|----------------|
| **Live v2 (current)** | O(n) | Position-based anchors | Full node HTML per patch | Two full HTML trees server-side | Island-scoped patching |
| Phoenix LiveView | O(d) | Position indices + keys | Only changed dynamic values | Static cache client-side | Template-level diffing |
| morphdom | O(n) | `id` attribute | N/A (client-side) | None (in-place) | Single-pass real DOM morph |
| idiomorph | O(n) | id-sets (descendant IDs) | N/A (client-side) | id-set maps | Structural matching without explicit IDs |
| React Fiber | O(n) | type + `key` prop | N/A (client-side) | Two fiber trees | Interruptible concurrent rendering |
| Vue 3 | O(n log n) | `key` + patch flags | N/A (client-side) | VNode tree | LIS minimizes DOM moves |
| Svelte 5 | O(1)/change | Compile-time binding | N/A (client-side) | None | Compiler eliminates runtime diffing |
| SolidJS | O(k)/signal | Reactive subscriptions | N/A (client-side) | Signal graph | Components run once, signals update DOM |
| Block VDOM | O(d) | Edit Map paths | N/A (client-side) | Block templates | State diff replaces tree diff |
| Incremental DOM | O(n) walk | Position cursor | N/A (client-side) | None | Real DOM as only representation |

Where: n = total nodes, d = changed dynamic values, k = subscribed nodes

### The Role of Keys

Keys are a universal optimization for list diffing. Without keys, positional matching means:
- Adding an element at the front of a list causes O(n) mutations (every position shifts)
- Reordering generates Replace patches for all positions

With keys, frameworks detect that existing elements moved, reducing mutations to O(moves).

Live v2 has no key mechanism. This is the most significant algorithmic gap compared to SOTA.

### Position-Based vs Key-Based vs ID-Based vs Reactive Tracking

| Strategy | Used By | Strengths | Weaknesses |
|----------|---------|-----------|------------|
| Position-based | Live v2, Incremental DOM | Simple, no developer annotation | Reorders = full replacement |
| Key-based | React, Vue, Snabbdom | Handles reorders, developer-controlled | Keys must be managed by developers |
| ID-based | morphdom, idiomorph | Works with server-rendered HTML | Requires `id` attributes (or descendant IDs) |
| Reactive | Svelte, SolidJS | O(1) updates, zero diffing | Requires compiler/signal system |

### Server-Side vs Client-Side Diffing

37signals explored server-side diffing for Turbo and abandoned it in favor of client-side morphing. Their insight: "The client already holds the current page state in memory and on-screen."

Live v2's server-side approach has a distinct advantage for its use case: the server is the single source of truth, and the Go developer never writes JavaScript. The tradeoff is that the server must store previous renders and perform full tree comparisons on every event.

| Aspect | Server-Side (Live v2) | Client-Side (morphdom/idiomorph) | Template-Level (LiveView) |
|--------|----------------------|----------------------------------|--------------------------|
| Where diffing occurs | Server (Go) | Browser (JS) | Server (Elixir) + Browser |
| What is diffed | Full HTML trees | Real DOM vs new HTML | Template data structure |
| State storage | `lastRenderedHTML` per island | Browser DOM is the "old" tree | Static parts cached client-side |
| Wire payload | Patch list with full HTML | Full new HTML string | Only changed dynamic values |
| CPU cost | Server pays diff cost | Client pays diff cost | Minimal on both sides |

## Code References

- `diff.go:25-38` — PatchAction enum (Noop, Replace, Append, Prepend)
- `diff.go:40-88` — anchorGenerator struct and ID generation
- `diff.go:121-139` — Patch struct definition
- `diff.go:185-210` — Diff() entry point (page-level)
- `diff.go:212-267` — DiffIsland() entry point (island-scoped)
- `diff.go:270-284` — anchorIslandTree() island anchor assignment
- `diff.go:315-327` — anchorTree() depth-first anchor assignment
- `diff.go:329-353` — shapeTree() whitespace removal and normalization
- `diff.go:377-433` — compareNodes() core recursive comparison
- `diff.go:435-464` — generatePatch() patch creation
- `diff.go:500-538` — liveUpdateCheck() modifier handling
- `diff.go:540-549` — nodeRelevant() whitespace filtering
- `diff.go:552-576` — nodeEqual() node equality check
- `instance.go:38-39` — lastRenderedHTML storage
- `instance.go:113-150` — Render() method with lastRenderedHTML update
- `engine.go:342-373` — renderAndSendIsland() diff integration point
- `web/src/island.ts:237-293` — Client-side patch application
- `web/src/transport/message.ts` — Patch/PatchAction TypeScript types
- `web/src/events.ts` — Event re-wiring after patches
- `web/src/patch.ts:83-149` — Form state dehydrate/hydrate

## Architecture Documentation

### Current Patterns

- **Island isolation:** Each island is diffed independently with island-scoped anchors (`_i_<id>_...`), ensuring patches never cross island boundaries
- **Full node replacement:** Patches contain the complete rendered HTML of the target node — there are no attribute-only or text-only patch operations
- **Modifier system:** `live-update` attributes (replace/append/prepend/ignore) allow developers to control patch behavior per subtree
- **Form preservation:** Client-side dehydrate/hydrate cycle preserves form input values and focus across patches
- **Event re-wiring:** All event handlers are torn down and re-bound after every patch application

### Design Constraints

- **Go templates:** The framework uses `html/template` which produces flat HTML strings, not structured template ASTs — this makes Phoenix-style template-level diffing impractical without a custom template engine
- **No build step:** The framework is designed to work without a JavaScript build pipeline, ruling out compile-time approaches like Svelte/SolidJS for the client
- **Server-side rendering:** All rendering happens in Go on the server, with the client as a thin patching layer

## External Context

### Key References

- [Phoenix LiveView diff.ex](https://github.com/phoenixframework/phoenix_live_view/blob/main/lib/phoenix_live_view/diff.ex) — Template-level server-side diffing
- [morphdom](https://github.com/patrick-steele-idem/morphdom) — ID-based single-pass DOM morphing
- [idiomorph](https://github.com/bigskysoftware/idiomorph) — Structural matching via descendant id-sets
- [React Reconciliation](https://legacy.reactjs.org/docs/reconciliation.html) — Key-based heuristic diffing
- [Vue 3 LIS Algorithm](https://www.mo4tech.com/parsing-the-dom-vue3-0-diff-core-algorithm-the-longest-increasing-subsequence-punch-brush.html) — LIS-based minimum-move list patching
- [37signals: Exploring server-side diffing in Turbo](https://dev.37signals.com/exploring-server-side-diffing-in-turbo/) — Why they chose client-side morphing over server-side diffing
- [Svelte: Virtual DOM is pure overhead](https://svelte.dev/blog/virtual-dom-is-pure-overhead) — Compile-time elimination of runtime diffing
- [million.js arXiv paper](https://arxiv.org/pdf/2202.08409) — Block virtual DOM academic treatment
- [RTED Paper (Pawlik & Augsten, 2012)](https://vldb.org/pvldb/vol5/p334_mateuszpawlik_vldb2012.pdf) — O(n³) theoretical bound for tree edit distance

### 37signals Experience

37signals prototyped server-side diffing for Turbo (Rails middleware, JSON patch payloads with positional selectors) and ultimately abandoned it in favor of client-side morphing with idiomorph. Their conclusion was that the client already holds current page state, making client-side morphing simpler and more reliable.

## Historical Context (from docs/)

- `docs/knowledge/architecture.md` — Documents the Render/Diff pipeline as a key module: "Parses HTML templates and computes minimal DOM patches"
- `docs/knowledge/project.md` — DOM Diffing listed as a key concept: "Server calculates minimal DOM patches and pushes them to browser for efficient updates"
- `docs/research/2026-01-25-islands-component-architecture.md` — Islands architecture research establishing the island-scoped diffing model

## Related Research

- `docs/research/2026-02-22-v2-branch-state-and-testing-gaps.md` — Testing gaps analysis
- `docs/research/2026-03-01-v2-feature-gaps-and-test-coverage.md` — Feature gap analysis

## Follow-up Research: Phoenix-Style Template Compilation in Go

### Research Question

Can we adopt Phoenix LiveView's template-level diffing approach? Either by "compiling" Go's `html/template` to separate static from dynamic parts, or by building a custom template engine?

### Phoenix LiveView's Rendered Structure (Reference Model)

Phoenix compiles HEEx templates into a `%Phoenix.LiveView.Rendered{}` struct:
```elixir
%Phoenix.LiveView.Rendered{
  static: ["<div><h1>", "</h1></div>"],   # list of static string literals
  dynamic: fn changed -> ["John Doe"] end, # function returning dynamic parts
  fingerprint: 123456789                   # template identity for diffing
}
```

The interleaving rule: `static[0] + dynamic[0] + static[1] + dynamic[1] + static[2]` — static parts always have one more element than dynamic parts. The `dynamic` function receives a `changed` map and returns `nil` for unchanged assigns, avoiding both recomputation and retransmission.

### Go's `text/template/parse` AST — What's Available

Go's `text/template/parse` package exposes a **fully typed AST** with 20+ node types. The `Template.Tree` field is exported (documented as "treat as unexported" but accessible in practice). The `parse.Tree.Root` is a `*ListNode` containing a `Nodes []Node` slice.

The key node types for static/dynamic separation:

| Node Type | Classification | Description |
|-----------|---------------|-------------|
| `*parse.TextNode` | **Static** | Literal text/HTML — `Text []byte` |
| `*parse.ActionNode` | **Dynamic** | `{{.Foo}}`, `{{funcCall .X}}` — contains `Pipe *PipeNode` |
| `*parse.IfNode` | **Dynamic** | `{{if .Cond}}...{{end}}` — contains `BranchNode` with `List`/`ElseList` |
| `*parse.RangeNode` | **Dynamic** | `{{range .Items}}...{{end}}` — contains `BranchNode` |
| `*parse.WithNode` | **Dynamic** | `{{with .X}}...{{end}}` — contains `BranchNode` |
| `*parse.TemplateNode` | **Dynamic** | `{{template "name" .}}` — sub-template invocation |

At the top level of `Root.Nodes`, nodes naturally alternate between `TextNode` (static) and action/control nodes (dynamic) — this mirrors Phoenix's interleaving model exactly.

**Proof of concept — AST walking:**
```go
func extractParts(t *template.Template) (statics []string, dynamicNodes []parse.Node) {
    for _, node := range t.Tree.Root.Nodes {
        switch n := node.(type) {
        case *parse.TextNode:
            statics = append(statics, string(n.Text))
        default:
            dynamicNodes = append(dynamicNodes, n)
        }
    }
    return
}
```

For nested structures (`IfNode.List`, `RangeNode.List`), recursive traversal is needed. The `tylermmorton/tmpl` library provides a `Traverse(node parse.Node, visitors ...Visitor)` depth-first walk utility.

### Template Execution — No Hook Points

Go's template execution engine (`text/template/exec.go`) has **no extension points**:
- The `walk()` function that dispatches on node type is unexported
- The `state.wr` (the `io.Writer`) receives mixed static+dynamic bytes with no way to distinguish them
- `FuncMap` functions receive evaluated values, not AST nodes
- There is no middleware or interceptor mechanism

To separate static from dynamic **during execution**, you must either:
1. Walk the AST **before** execution and build your own execution engine
2. Fork `text/template`'s `exec.go` (as `canopyclimate/golive` does)

### Current Render API — The Integration Point

Live v2's render handler signature:
```go
type IslandRenderHandler func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error)
```

The handler currently returns opaque `io.Reader` (rendered HTML bytes). The framework has no visibility into template structure — it just gets a flat HTML string.

To adopt template-level diffing, the render output would need to change from a flat string to a structured representation separating static and dynamic parts.

### Approach A: Compile `html/template` AST (Deep Dive)

Walk the existing `html/template` AST at parse time to extract static/dynamic separation, then build a custom executor that returns the Phoenix-style `{statics, dynamics}` structure.

#### The Escaping Problem — Solved

The biggest concern was `html/template`'s context-sensitive escaping. Research reveals this is **not a blocker**:

`html/template` modifies the parse tree AST itself during escaping. The escaping is triggered lazily at first `Execute()` time (not at `Parse()` time). It injects escaping function identifiers directly into `ActionNode.Pipe.Cmds`. After escaping:

```
Template: <div>{{.Foo}}</div>
Before escaping: ActionNode{Pipe{Cmds: [Cmd{Args: [FieldNode{.Foo}]}]}}
After escaping:  ActionNode{Pipe{Cmds: [Cmd{Args: [FieldNode{.Foo}]}, Cmd{Args: [IdentifierNode{_html_template_htmlescaper}]}]}}

Template: <a href="{{.URL}}">
After escaping:  ActionNode{Pipe{Cmds: [Cmd{Args: [FieldNode{.URL}]}, Cmd{Args: [IdentifierNode{_html_template_urlfilter}]}, Cmd{Args: [IdentifierNode{_html_template_urlnormalizer}]}, Cmd{Args: [IdentifierNode{_html_template_attrescaper}]}]}}
```

The 17 internal escaping functions (prefixed `_html_template_`) are added to the FuncMap and cover all contexts:

| Context | Escaper(s) Injected |
|---------|-------------------|
| Text content `<div>{{.}}</div>` | `_html_template_htmlescaper` |
| Quoted attribute `class="{{.}}"` | `_html_template_attrescaper` |
| URL attribute `href="{{.}}"` | `_html_template_urlfilter` + `_html_template_urlnormalizer` + `_html_template_attrescaper` |
| JavaScript `<script>var x={{.}}</script>` | `_html_template_jsvalescaper` |
| JS string `onclick='f("{{.}}")'` | `_html_template_jsstrescaper` + `_html_template_attrescaper` |
| CSS value | `_html_template_cssvaluefilter` |
| Unquoted attribute | `_html_template_nospaceescaper` |

**The approach:** Parse with `html/template`, trigger escaping via `t.Execute(io.Discard, zeroValue)`, then walk the now-escaped `t.Tree`. The AST already contains all security escaping information. A custom executor simply needs to evaluate the pipeline commands (including the injected escapers) rather than reimplementing escaping logic.

`mh-cbon/template-compiler` uses this exact approach — it reads the post-escaping AST and generates native Go code from it, achieving 5-30x speedups while maintaining full `html/template` security.

#### Handling `{{range}}` and `{{if}}` — The Nested Rendered Model

Phoenix handles variable-length output through **recursive `Rendered` structs**. Each control flow block produces a nested `Rendered` with its own statics/dynamics. Research into both Phoenix and `go-live-view` reveals the concrete mechanisms:

**`{{range}}` → Comprehension:**
When all loop iterations produce identical static structure (same template), they are bundled into a `Comprehension` — statics sent once, dynamics as a 2D array:

```json
{
  "s": ["<ul>", "</ul>"],
  "0": {
    "s": ["<li>", "</li>"],
    "d": [["Alice"], ["Bob"], ["Charlie"]]
  }
}
```

On re-render (3 items → 5 items), only the diff is sent:
- Phoenix: sends only new indices (items 3,4) + updated `kc` count — supports key-based move tracking
- go-live-view: sends entire new `d` array when count changes (simpler, no move tracking)

**`{{if}}` → Nested Rendered with fingerprint change:**
The if/else branches produce different static structures (different statics list), which changes the fingerprint. When a condition flips from true→false, the fingerprint mismatch triggers a full re-render of that sub-tree:

```
{{if .ShowDetails}} → fingerprint A, statics: ["<div>Details: ", "</div>"]
{{else}}            → fingerprint B, statics: ["<div>Hidden</div>"]
```

**Proposed Go data structures:**

```go
// Rendered represents a compiled template's output with static/dynamic separation.
type Rendered struct {
    Static      []string           // Static HTML fragments (N+1 for N dynamics)
    Dynamic     map[int]any        // Dynamic values: string, *Rendered, or *Comprehension
    Fingerprint uint64             // Hash of Static — changes when template structure changes
}

// Comprehension represents a {{range}} block where all iterations share statics.
type Comprehension struct {
    Static      []string           // Shared static fragments for each iteration
    Dynamics    [][]any            // [iteration][slot] = dynamic value
    Fingerprint uint64             // Hash of shared Static
}
```

#### Concrete Example: Counter Template

Template:
```html
<div>
    <div class="count">{{ .Count }}</div>
    <button live-click="inc">+ Increment</button>
</div>
```

AST after escaping:
```
ListNode{
  TextNode{"<div>\n    <div class=\"count\">"}     // static[0]
  ActionNode{.Count | _html_template_htmlescaper}   // dynamic[0]
  TextNode{"</div>\n    <button live-click=\"inc\">+ Increment</button>\n</div>"} // static[1]
}
```

Initial render wire format:
```json
{"s": ["<div>\n    <div class=\"count\">", "</div>\n    <button live-click=\"inc\">+ Increment</button>\n</div>"], "0": "0", "f": 3847291}
```

After increment event — only the changed dynamic:
```json
{"0": "1"}
```

#### Concrete Example: Chat Template with `{{range}}`

Template:
```html
<div id="messages" live-update="append">
    {{ range .Messages }}
    <div id="{{ .ID }}" class="message"><strong>{{ .User }}:</strong> {{ .Msg }}</div>
    {{ end }}
</div>
```

AST structure (simplified):
```
ListNode{
  TextNode{"<div id=\"messages\" live-update=\"append\">\n    "} // static[0]
  RangeNode{                                                       // dynamic[0] → Comprehension
    Pipe: .Messages
    List: ListNode{
      TextNode{"\n    <div id=\""}       // range static[0]
      ActionNode{.ID | attrescaper}      // range dynamic[0]
      TextNode{"\" class=\"message\"><strong>"} // range static[1]
      ActionNode{.User | htmlescaper}    // range dynamic[1]
      TextNode{":</strong> "}            // range static[2]
      ActionNode{.Msg | htmlescaper}     // range dynamic[2]
      TextNode{"</div>\n    "}           // range static[3]
    }
  }
  TextNode{"\n</div>"}                                            // static[1]
}
```

Initial render (2 messages):
```json
{
  "s": ["<div id=\"messages\" live-update=\"append\">\n    ", "\n</div>"],
  "0": {
    "s": ["\n    <div id=\"", "\" class=\"message\"><strong>", ":</strong> ", "</div>\n    "],
    "d": [
      ["msg-1", "Alice", "Hello"],
      ["msg-2", "Bob", "World"]
    ],
    "f": 9281734
  },
  "f": 3847291
}
```

After new message added (only new item sent):
```json
{
  "0": {
    "d": [
      ["msg-1", "Alice", "Hello"],
      ["msg-2", "Bob", "World"],
      ["msg-3", "Charlie", "Hi there"]
    ]
  }
}
```

With key-based tracking (future optimization), only the new row would be sent.

#### Implementation Steps

1. **Parse & escape:** Use `html/template` to parse and trigger the escape pass
2. **Walk AST:** Recursively walk `Tree.Root.Nodes`, building a `Rendered` tree:
   - `TextNode` → append to current static accumulator
   - `ActionNode` → flush static accumulator, record dynamic slot
   - `RangeNode` → flush static, recurse into `List` to build `Comprehension` template, record as nested dynamic
   - `IfNode` → flush static, recurse into `List`/`ElseList` to build conditional `Rendered` branches
3. **Execute dynamics:** Build a custom executor that evaluates only `ActionNode.Pipe` against current state, using `html/template`'s own escaping functions (they're in the FuncMap)
4. **Diff:** Compare current dynamic values against previous render — only send changed indices
5. **Client reconstruction:** Client caches statics on initial render, applies dynamic diffs to reconstruct HTML, uses morphdom/idiomorph to patch the DOM

#### Implementation Complexity Assessment

| Component | Effort | Notes |
|-----------|--------|-------|
| AST walker (statics/dynamics extraction) | Medium | Recursive tree walk, ~200-300 lines |
| Custom pipeline executor | High | Must handle: field access, method calls, FuncMap calls, variable scoping, pipeline chaining. `text/template/exec.go` is ~900 lines — can be simplified since we only need value evaluation, not full tree walking |
| Comprehension builder ({{range}}) | Medium | Detect shared statics across iterations, build 2D dynamics array |
| Conditional handling ({{if}}/{{with}}) | Medium | Fingerprint per branch, detect branch switches |
| Diff computation | Low | Simple index-by-index string comparison of dynamic values |
| Client-side renderer | Medium | Cache statics, reconstruct HTML from statics+dynamics, apply via morphdom |
| Wire protocol | Low | JSON encoding of `Rendered`/`Comprehension` structs |

**Total estimated effort:** The custom pipeline executor is the critical path. However, `mh-cbon/template-compiler` demonstrates this is achievable — it walks the escaped AST and generates Go code that evaluates each pipeline. The key insight is that after escaping, the pipeline is fully specified (no context-sensitive logic needed at execution time).

**Pros:**
- Developers continue using standard `html/template` syntax — zero learning curve
- Massive reduction in wire payload (only changed dynamic values sent)
- Server-side diff cost drops from O(n) tree comparison to O(d) value comparison
- Static content never stored, compared, or transmitted after initial render
- Compatible with existing `embed.FS` template loading patterns
- Security escaping is handled by `html/template` itself — no reimplementation needed

**Cons:**
- Significant implementation effort (custom executor is ~500-900 lines)
- Reliance on `Template.Tree` API (documented as "treat as unexported" — stable since Go 1.0, used by `html/template` itself, but technically not guaranteed)
- Triggering the escape pass requires a dummy `Execute(io.Discard, zeroValue)` call — the `escape()` method is unexported
- Nested `{{range}}`/`{{if}}` handling adds recursive complexity to both server and client

### Approach B: Build a Custom Template Engine

Create a new template language (or adopt an existing one like `templ`) that compiles to Go code with static/dynamic separation built in.

**Option B1 — Adopt `a-h/templ`:**
`templ` is a popular Go template language that compiles `.templ` files to Go code. Static HTML is written directly as byte literals; dynamic expressions are evaluated at render time. A `templ.Component` renders to `io.Writer`.

The compilation step already separates static from dynamic in the generated Go code. The framework could intercept the generated code or wrap the `Component` interface to capture the static/dynamic split.

**Option B2 — Custom DSL:**
Build a minimal template language specifically designed for LiveView-style rendering, where the compiler outputs `CompiledTemplate{Statics, Dynamics}` directly.

**Option B3 — Runtime Annotation (go-live-view approach):**
Instead of template compilation, developers explicitly mark dynamic parts:
```go
live.Div(
    live.Static("<h1>"),
    live.Dynamic(func() string { return state.Title }),
    live.Static("</h1>"),
)
```
Initial render sends: `{"s": ["<h1>", "</h1>"], "0": "Hello"}`
Updates send: `{"0": "World"}`

This is what `go-live-view/go-live-view` does. It follows the Phoenix wire protocol exactly but requires developers to abandon HTML templates.

**Pros (custom engine):**
- Full control over static/dynamic separation
- Can design for LiveView-style diffing from the start
- No reliance on unexported Go APIs

**Cons (custom engine):**
- Developers must learn a new template syntax (B1, B2) or give up templates entirely (B3)
- Breaks the "just use Go templates" value proposition
- Significant implementation and maintenance burden
- Ecosystem fragmentation (tooling, IDE support, documentation)

### Approach C: Hybrid — `html/template` with Structural Markers

Keep `html/template` but add a post-processing step that marks dynamic regions in the rendered output, allowing the server to diff only those regions.

**How it would work:**
1. Wrap dynamic values in marker comments during template execution: `<!--d:0-->John<!--/d:0-->`
2. On re-render, extract values between markers and compare against previous
3. Send only changed marker indices

This could be achieved with template functions:
```html
<h1>{{ dyn 0 .Name }}</h1>
<p>{{ dyn 1 .Description }}</p>
```

Where `dyn` is a `FuncMap` function that wraps output in markers.

**Pros:** Keeps `html/template`; simpler than full AST compilation; markers survive HTML parsing.
**Cons:** Markers add HTML bloat; requires developer annotation; fragile if markers are stripped.

### Existing Go Projects in This Space

| Project | Approach | Template-Level Diffing |
|---------|----------|----------------------|
| `canopyclimate/golive` | Forked `html/template` with `ExecuteTree()` returning a tree structure | Planned but not implemented |
| `go-live-view/go-live-view` | Runtime API (`dynamic.Text()`, `dynamic.If()`) — no templates | Fully implemented, follows Phoenix wire protocol |
| `mh-cbon/template-compiler` | Compiles `html/template` AST to native Go code with inlined statics | Static/dynamic separated in generated code |
| `a-h/templ` | Custom `.templ` language compiling to Go code | Static/dynamic separated by design |
| `valyala/quicktemplate` | Custom `.qtpl` syntax compiled to Go code | Statics are byte slice literals in generated code |
| `tylermmorton/tmpl` | AST traversal utilities for `html/template` | Analysis only, no execution changes |

### Recommendation Summary

| Approach | Developer Experience | Wire Efficiency | Implementation Effort | Risk |
|----------|---------------------|----------------|----------------------|------|
| A: Compile `html/template` AST | Best (no change) | Excellent | High | Medium (`Tree` API stability) |
| B1: Adopt `templ` | Good (new syntax) | Excellent | Medium | Low |
| B2: Custom DSL | Poor (new syntax) | Excellent | Very High | High |
| B3: Runtime annotation | Poor (no templates) | Excellent | Low | Low |
| C: Structural markers | Good (minor additions) | Good | Low-Medium | Low |
| Current (tree diff) | Best (no change) | Adequate | None | None |

### Client-Side Changes Required

All template-level approaches require matching client-side changes:

1. **Initial render:** Client receives `{statics: [...], dynamics: [...]}` and caches the statics
2. **Updates:** Client receives `{0: "new value", 2: "changed"}` and reconstructs HTML from cached statics + new dynamics
3. **DOM application:** Client can use morphdom/idiomorph to apply the reconstructed HTML, or use targeted DOM updates if dynamic positions map to specific DOM nodes

The current client-side patch system (anchor-based querySelector + outerHTML replacement) would be replaced entirely.

## Open Questions

1. **`html/template` AST stability:** The `Tree` field is documented as "treat as unexported." It has been stable since Go 1.0, is used by `html/template` itself, and `mh-cbon/template-compiler` depends on it in production. The risk is low but non-zero — a Go version could restructure internals. Mitigation: pin Go version in `go.mod` toolchain directive, add CI test that validates AST access.

2. ~~**Nested template handling:**~~ **Resolved.** Phoenix uses recursive `Rendered` structs for `{{if}}`/`{{range}}` and `Comprehension` structs for same-structure loop iterations. `go-live-view` demonstrates a working Go implementation. The model is well-understood and directly applicable.

3. ~~**`html/template` escaping:**~~ **Resolved.** `html/template` modifies the AST during escaping, injecting `_html_template_*` identifier nodes into `ActionNode.Pipe.Cmds`. After triggering escaping via `Execute(io.Discard, zeroValue)`, the AST contains all escaping information. A custom executor evaluates the already-escaped pipeline — no reimplementation of context-sensitive escaping needed.

4. **Change tracking:** Start simple — execute all dynamic slots on every render and compare output strings. This is dramatically cheaper than the current full-tree HTML diff. Fine-grained change tracking (per-field → per-slot mapping) is a future optimization.

5. **Render API migration:** ~~Breaking change concern~~ **Resolved.** This is a complete v2 rewrite — the render API can change freely. `IslandRenderHandler` will return a `*Rendered` struct instead of `io.Reader`.

6. **Key-based list tracking:** Start with the simpler model (full `d` array on count change, like `go-live-view`). Keyed comprehensions with move tracking are a future optimization.
