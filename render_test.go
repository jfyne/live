package live

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestRenderIsland(t *testing.T) {
	t.Run("simple template", func(t *testing.T) {
		island, _ := NewIsland("test",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]string{"value": "test"}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>Hello World</div>"), nil
			}),
		)

		instance := &IslandInstance{
			ID:       "test-1",
			Type:     "test",
			island:   island,
			props:    Props{},
			children: "",
			mounted:  true,
			state:    map[string]string{"value": "test"},
		}

		html, err := RenderIsland(context.Background(), instance)
		if err != nil {
			t.Fatalf("RenderIsland() error = %v", err)
		}

		if !strings.Contains(html, "<div>") {
			t.Error("rendered HTML should contain <div>")
		}
		if !strings.Contains(html, "Hello World") {
			t.Error("rendered HTML should contain content")
		}
	})

	t.Run("returns raw template output without anchors", func(t *testing.T) {
		island, _ := NewIsland("test",
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div><span>Content</span></div>"), nil
			}),
		)

		instance := &IslandInstance{
			ID:       "test-1",
			Type:     "test",
			island:   island,
			props:    Props{},
			children: "",
			mounted:  true,
			state:    nil,
		}

		html, err := RenderIsland(context.Background(), instance)
		if err != nil {
			t.Fatalf("RenderIsland() error = %v", err)
		}

		// RenderIsland should NOT add anchors - that is DiffIsland's job
		if strings.Contains(html, "_i_") {
			t.Errorf("RenderIsland should not add island anchors, got: %s", html)
		}
		if strings.Contains(html, "_l") {
			t.Errorf("RenderIsland should not add page-level anchors, got: %s", html)
		}
	})

	t.Run("with state interpolation", func(t *testing.T) {
		type CounterState struct {
			Count int
		}

		island, _ := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return &CounterState{Count: props.Int("initial")}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(*CounterState)
				html := "<div class=\"counter\">" + strings.Repeat("X", state.Count) + "</div>"
				return strings.NewReader(html), nil
			}),
		)

		instance := &IslandInstance{
			ID:       "counter-42",
			Type:     "counter",
			island:   island,
			props:    Props{"initial": 5},
			children: "",
			mounted:  true,
			state:    &CounterState{Count: 5},
		}

		html, err := RenderIsland(context.Background(), instance)
		if err != nil {
			t.Fatalf("RenderIsland() error = %v", err)
		}

		if !strings.Contains(html, "XXXXX") {
			t.Errorf("rendered HTML should contain 5 X's, got: %s", html)
		}
	})

	t.Run("nil instance", func(t *testing.T) {
		_, err := RenderIsland(context.Background(), nil)
		if err == nil {
			t.Error("RenderIsland(nil) should return error")
		}
		if !strings.Contains(err.Error(), "nil") {
			t.Errorf("error should mention nil, got: %v", err)
		}
	})

	t.Run("render error propagates", func(t *testing.T) {
		expectedErr := "render failed"
		island, _ := NewIsland("failing",
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return nil, fmt.Errorf("%s", expectedErr)
			}),
		)

		island.Render = func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
			return nil, fmt.Errorf("%s", expectedErr)
		}

		instance := &IslandInstance{
			ID:       "fail-1",
			Type:     "failing",
			island:   island,
			props:    Props{},
			children: "",
			mounted:  true,
			state:    nil,
		}

		_, err := RenderIsland(context.Background(), instance)
		if err == nil {
			t.Error("RenderIsland should propagate render error")
		}
	})

	t.Run("stores lastRenderedHTML for subsequent diff", func(t *testing.T) {
		island, _ := NewIsland("test",
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>Content</div>"), nil
			}),
		)

		instance := &IslandInstance{
			ID:       "test-1",
			Type:     "test",
			island:   island,
			props:    Props{},
			children: "",
			mounted:  true,
			state:    nil,
		}

		html, err := RenderIsland(context.Background(), instance)
		if err != nil {
			t.Fatalf("RenderIsland() error = %v", err)
		}

		// lastRenderedHTML should match the returned value
		if string(instance.lastRenderedHTML) != html {
			t.Errorf("lastRenderedHTML = %q, want %q", instance.lastRenderedHTML, html)
		}
	})
}

func TestDiffIsland(t *testing.T) {
	t.Run("generates island-scoped patches", func(t *testing.T) {
		current := "<div>Old</div>"
		proposed := "<div>New</div>"

		patches, err := DiffIsland("island-1", current, proposed)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}

		if len(patches) == 0 {
			t.Fatal("DiffIsland() should generate patches")
		}

		for _, patch := range patches {
			if patch.IslandID != "island-1" {
				t.Errorf("patch.IslandID = %q, want %q", patch.IslandID, "island-1")
			}
		}

		for _, patch := range patches {
			if !strings.HasPrefix(patch.Anchor, "_i_island-1") {
				t.Errorf("patch.Anchor = %q, should start with _i_island-1", patch.Anchor)
			}
		}
	})

	t.Run("patch island ID is set correctly", func(t *testing.T) {
		current := "<div>Test</div>"
		proposed := "<div>Changed</div>"

		patches, err := DiffIsland("my-counter-42", current, proposed)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}

		for i, patch := range patches {
			if patch.IslandID != "my-counter-42" {
				t.Errorf("patches[%d].IslandID = %q, want %q", i, patch.IslandID, "my-counter-42")
			}
		}
	})

	t.Run("multiple islands diff independently", func(t *testing.T) {
		current := "<div>Content</div>"
		proposed := "<div>Updated</div>"

		patches1, err := DiffIsland("island-1", current, proposed)
		if err != nil {
			t.Fatalf("DiffIsland(island-1) error = %v", err)
		}

		patches2, err := DiffIsland("island-2", current, proposed)
		if err != nil {
			t.Fatalf("DiffIsland(island-2) error = %v", err)
		}

		if len(patches1) == 0 || len(patches2) == 0 {
			t.Fatal("both islands should generate patches")
		}

		if patches1[0].IslandID == patches2[0].IslandID {
			t.Error("patches from different islands should have different IslandIDs")
		}

		if patches1[0].Anchor == patches2[0].Anchor {
			t.Error("patches from different islands should have different anchors")
		}

		for _, patch := range patches1 {
			if !strings.Contains(patch.Anchor, "island-1") {
				t.Errorf("island-1 patch anchor should contain 'island-1', got: %s", patch.Anchor)
			}
		}

		for _, patch := range patches2 {
			if !strings.Contains(patch.Anchor, "island-2") {
				t.Errorf("island-2 patch anchor should contain 'island-2', got: %s", patch.Anchor)
			}
		}
	})

	t.Run("no cross-contamination between islands", func(t *testing.T) {
		html := "<div><span>Test</span></div>"

		patches1, _ := DiffIsland("alpha", html, html)
		patches2, _ := DiffIsland("beta", html, html)

		if len(patches1) != 0 {
			t.Errorf("identical HTML should generate 0 patches for island alpha, got %d", len(patches1))
		}
		if len(patches2) != 0 {
			t.Errorf("identical HTML should generate 0 patches for island beta, got %d", len(patches2))
		}

		changed := "<div><span>Changed</span></div>"
		patchesA, _ := DiffIsland("alpha", html, changed)
		patchesB, _ := DiffIsland("beta", html, changed)

		for _, p := range patchesA {
			if p.IslandID != "alpha" {
				t.Error("alpha patches should have IslandID='alpha'")
			}
		}
		for _, p := range patchesB {
			if p.IslandID != "beta" {
				t.Error("beta patches should have IslandID='beta'")
			}
		}
	})

	t.Run("island anchors in generated HTML", func(t *testing.T) {
		current := "<div>Test</div>"
		proposed := "<div>Updated</div>"

		patches, err := DiffIsland("test-island", current, proposed)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}

		for _, patch := range patches {
			if patch.HTML != "" && !strings.Contains(patch.HTML, "_i_test-island") {
				t.Errorf("patch HTML should contain island anchor, got: %s", patch.HTML)
			}
		}
	})

	t.Run("diff with nested elements in island", func(t *testing.T) {
		current := `
			<div class="container">
				<h1>Title</h1>
				<p>Old paragraph</p>
			</div>
		`
		proposed := `
			<div class="container">
				<h1>Title</h1>
				<p>New paragraph</p>
			</div>
		`

		patches, err := DiffIsland("nested-island", current, proposed)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}

		if len(patches) == 0 {
			t.Fatal("should generate patches for nested element changes")
		}

		for _, patch := range patches {
			if patch.IslandID != "nested-island" {
				t.Errorf("patch.IslandID = %q, want %q", patch.IslandID, "nested-island")
			}
		}

		hasNestedAnchor := false
		for _, patch := range patches {
			if strings.Contains(patch.Anchor, "_i_nested-island_") && strings.Count(patch.Anchor, "_") > 3 {
				hasNestedAnchor = true
				break
			}
		}
		if !hasNestedAnchor {
			t.Error("should have anchors referencing nested structure")
		}
	})

	t.Run("append operation", func(t *testing.T) {
		current := "<div>Item 1</div>"
		proposed := "<div>Item 1</div><div>Item 2</div>"

		patches, err := DiffIsland("list-island", current, proposed)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}

		hasAppend := false
		for _, patch := range patches {
			if patch.Action == Append {
				hasAppend = true
				if patch.IslandID != "list-island" {
					t.Errorf("append patch IslandID = %q, want %q", patch.IslandID, "list-island")
				}
			}
		}
		if !hasAppend {
			t.Error("should generate append patch")
		}
	})

	t.Run("replace operation", func(t *testing.T) {
		current := "<div>Old</div>"
		proposed := "<span>New</span>"

		patches, err := DiffIsland("replace-island", current, proposed)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}

		hasReplace := false
		for _, patch := range patches {
			if patch.Action == Replace {
				hasReplace = true
				if patch.IslandID != "replace-island" {
					t.Errorf("replace patch IslandID = %q, want %q", patch.IslandID, "replace-island")
				}
			}
		}
		if !hasReplace {
			t.Error("should generate replace patch")
		}
	})

	t.Run("delete operation", func(t *testing.T) {
		current := "<div>Item 1</div><div>Item 2</div>"
		proposed := "<div>Item 1</div>"

		patches, err := DiffIsland("delete-island", current, proposed)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}

		hasDelete := false
		for _, patch := range patches {
			if patch.HTML == "" {
				hasDelete = true
				if patch.IslandID != "delete-island" {
					t.Errorf("delete patch IslandID = %q, want %q", patch.IslandID, "delete-island")
				}
			}
		}
		if !hasDelete {
			t.Error("should generate delete patch (empty HTML)")
		}
	})
}

func TestIslandAnchorUniquenessInRender(t *testing.T) {
	t.Run("same HTML different islands have unique anchors", func(t *testing.T) {
		html := "<div><span>Content</span></div>"

		patches1, _ := DiffIsland("island-a", html, "<div><span>Updated</span></div>")
		patches2, _ := DiffIsland("island-b", html, "<div><span>Updated</span></div>")

		if len(patches1) == 0 || len(patches2) == 0 {
			t.Fatal("both should generate patches")
		}

		anchor1 := patches1[0].Anchor
		anchor2 := patches2[0].Anchor

		if anchor1 == anchor2 {
			t.Errorf("anchors should be unique, both are: %s", anchor1)
		}

		if !strings.Contains(anchor1, "island-a") {
			t.Errorf("anchor1 should contain 'island-a', got: %s", anchor1)
		}
		if !strings.Contains(anchor2, "island-b") {
			t.Errorf("anchor2 should contain 'island-b', got: %s", anchor2)
		}
	})
}

// TestRenderIslandIntegration tests the full render→diff→render→diff cycle
// that the engine performs. This is the critical path: RenderIsland returns
// raw HTML, which is stored as previousHTML. On the next render, DiffIsland
// receives the raw previous HTML and the raw new HTML, anchors both
// identically, and produces minimal granular patches.
func TestRenderIslandIntegration(t *testing.T) {
	t.Run("render and diff workflow produces granular patches", func(t *testing.T) {
		type CounterState struct {
			Count int
		}

		island, _ := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return &CounterState{Count: 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(*CounterState)
				html := fmt.Sprintf(`<div><div class="count">%d</div><div><button live-click="inc">+</button></div></div>`, state.Count)
				return strings.NewReader(html), nil
			}),
		)

		instance := &IslandInstance{
			ID:       "counter-1",
			Type:     "counter",
			island:   island,
			props:    Props{},
			children: "",
			mounted:  true,
			state:    &CounterState{Count: 0},
		}

		// Step 1: Initial render (simulates mount, previousHTML is empty)
		html1, err := RenderIsland(context.Background(), instance)
		if err != nil {
			t.Fatalf("RenderIsland() error = %v", err)
		}

		// Initial diff: empty → first render
		initialPatches, err := DiffIsland("counter-1", "", html1)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}
		if len(initialPatches) == 0 {
			t.Fatal("initial mount should produce patches")
		}

		// The initial patch should be an Append at the body-level anchor
		if initialPatches[0].Action != Append {
			t.Errorf("initial patch action = %d, want Append (%d)", initialPatches[0].Action, Append)
		}

		// Initial patch HTML should NOT contain <body> wrapper
		if strings.Contains(initialPatches[0].HTML, "<body") {
			t.Errorf("initial patch HTML should not contain <body> wrapper, got: %s", initialPatches[0].HTML)
		}

		// Initial patch HTML should contain anchored content
		if !strings.Contains(initialPatches[0].HTML, "_i_counter-1_0") {
			t.Errorf("initial patch HTML should contain content-level anchor _i_counter-1_0, got: %s", initialPatches[0].HTML)
		}

		// Step 2: Update state and re-render (simulates event handling)
		instance.state = &CounterState{Count: 1}
		previousHTML := html1 // lastRenderedHTML from previous render

		html2, err := RenderIsland(context.Background(), instance)
		if err != nil {
			t.Fatalf("RenderIsland() error = %v", err)
		}

		// Diff the two renders
		updatePatches, err := DiffIsland("counter-1", previousHTML, html2)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}
		if len(updatePatches) == 0 {
			t.Fatal("state change should produce patches")
		}

		// The update patch should be GRANULAR - targeting just the count div,
		// not the entire island content
		for _, patch := range updatePatches {
			if patch.IslandID != "counter-1" {
				t.Errorf("patch.IslandID = %q, want counter-1", patch.IslandID)
			}

			// Patch HTML should NOT contain <body> wrapper
			if strings.Contains(patch.HTML, "<body") {
				t.Errorf("update patch HTML should not contain <body>, got: %s", patch.HTML)
			}

			// Patch should contain the new count value
			if patch.HTML != "" && !strings.Contains(patch.HTML, "1") {
				t.Errorf("update patch should contain new count value '1', got: %s", patch.HTML)
			}
		}

		// The update patch should NOT replace the entire content - it should
		// target a specific element (the count div), not the body/root
		if updatePatches[0].Anchor == "_i_counter-1" {
			t.Error("update patch targets root anchor _i_counter-1 (entire content) - should target a deeper element for granular update")
		}

		// Verify the update anchor exists in the initial patch HTML
		// (the client DOM was set from the initial patch)
		updateAnchor := updatePatches[0].Anchor
		if !strings.Contains(initialPatches[0].HTML, updateAnchor) {
			t.Errorf("update anchor %q not found in initial patch HTML - client DOM won't have it\ninitial HTML: %s",
				updateAnchor, initialPatches[0].HTML)
		}
	})

	t.Run("counter template exact scenario", func(t *testing.T) {
		// This test uses the exact counter template HTML to verify
		// the engine produces granular patches matching the counter example.
		count0 := `<div>
    <div class="count">0</div>
    <div style="text-align: center;">
        <button live-click="dec">- Decrement</button>
        <button live-click="inc">+ Increment</button>
    </div>
</div>`

		count1 := `<div>
    <div class="count">1</div>
    <div style="text-align: center;">
        <button live-click="dec">- Decrement</button>
        <button live-click="inc">+ Increment</button>
    </div>
</div>`

		// Initial mount: empty → count=0
		initialPatches, err := DiffIsland("counter-1", "", count0)
		if err != nil {
			t.Fatalf("initial DiffIsland error: %v", err)
		}
		if len(initialPatches) == 0 {
			t.Fatal("initial mount should produce patches")
		}

		// Initial patch should NOT contain <body>
		if strings.Contains(initialPatches[0].HTML, "<body") {
			t.Errorf("initial patch should not have body wrapper, got: %s", initialPatches[0].HTML)
		}

		// Update: count=0 → count=1
		updatePatches, err := DiffIsland("counter-1", count0, count1)
		if err != nil {
			t.Fatalf("update DiffIsland error: %v", err)
		}
		if len(updatePatches) == 0 {
			t.Fatal("update should produce patches")
		}

		// Update should be granular: only the count div changes
		patch := updatePatches[0]

		if patch.Action != Replace {
			t.Errorf("update action = %d, want Replace (%d)", patch.Action, Replace)
		}

		// The patch should target the count div, not the root or body
		if patch.Anchor == "_i_counter-1" {
			t.Error("patch targets root (_i_counter-1) instead of the count div - not granular")
		}

		// The patch HTML should be just the count div, not the entire island
		if strings.Contains(patch.HTML, "live-click") {
			t.Errorf("granular patch should NOT contain the buttons, got: %s", patch.HTML)
		}
		if !strings.Contains(patch.HTML, "1") {
			t.Errorf("patch should contain new count '1', got: %s", patch.HTML)
		}
		if !strings.Contains(patch.HTML, `class="count"`) {
			t.Errorf("patch should target the count div, got: %s", patch.HTML)
		}

		// Verify the update anchor exists in the initial HTML
		if !strings.Contains(initialPatches[0].HTML, patch.Anchor) {
			t.Errorf("update anchor %q not found in initial HTML %q",
				patch.Anchor, initialPatches[0].HTML)
		}
	})

	t.Run("multiple sequential updates produce granular patches", func(t *testing.T) {
		makeHTML := func(count int) string {
			return fmt.Sprintf(`<div><span class="count">%d</span></div>`, count)
		}

		prev := ""
		for i := 0; i <= 3; i++ {
			current := makeHTML(i)
			patches, err := DiffIsland("counter-1", prev, current)
			if err != nil {
				t.Fatalf("DiffIsland step %d error: %v", i, err)
			}
			if len(patches) == 0 {
				t.Fatalf("step %d should produce patches", i)
			}

			// After the initial mount (i=0), all updates should be granular
			if i > 0 {
				if patches[0].Anchor == "_i_counter-1" {
					t.Errorf("step %d: patch targets root, expected granular update", i)
				}
				// Should NOT contain the entire island content
				if strings.Contains(patches[0].HTML, "<body") {
					t.Errorf("step %d: patch contains body wrapper", i)
				}
			}

			prev = current
		}
	})
}
