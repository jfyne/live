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
		// Create a test island that renders simple HTML
		island, _ := NewIsland("test",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]string{"value": "test"}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div>Hello World</div>"), nil
			}),
		)

		// Create instance
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

		// Should contain the div with island-scoped anchor
		if !strings.Contains(html, "<div") {
			t.Error("rendered HTML should contain <div>")
		}
		if !strings.Contains(html, "_i_test-1") {
			t.Error("rendered HTML should contain island-scoped anchor _i_test-1")
		}
		if !strings.Contains(html, "Hello World") {
			t.Error("rendered HTML should contain content")
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
		if !strings.Contains(html, "_i_counter-42") {
			t.Error("rendered HTML should contain island-scoped anchor _i_counter-42")
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

		// Force render to fail by returning error from Render
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

		// Note: This test expects the instance.Render to propagate errors
		// We'll check that RenderIsland handles render errors
		_, err := RenderIsland(context.Background(), instance)
		if err == nil {
			t.Error("RenderIsland should propagate render error")
		}
	})

	t.Run("nested elements", func(t *testing.T) {
		island, _ := NewIsland("nested",
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				html := `
					<div class="outer">
						<div class="inner">
							<span>Nested content</span>
						</div>
					</div>
				`
				return strings.NewReader(html), nil
			}),
		)

		instance := &IslandInstance{
			ID:       "nested-1",
			Type:     "nested",
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

		// Check that island anchors are present for multiple elements
		if !strings.Contains(html, "_i_nested-1") {
			t.Error("rendered HTML should contain island-scoped anchors")
		}
		if !strings.Contains(html, "Nested content") {
			t.Error("rendered HTML should preserve nested content")
		}
	})

	t.Run("island anchors have correct format", func(t *testing.T) {
		island, _ := NewIsland("format-test",
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				return strings.NewReader("<div><span>Test</span></div>"), nil
			}),
		)

		instance := &IslandInstance{
			ID:       "test-123",
			Type:     "format-test",
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

		// Island anchors should have format _i_<island-id>_<path>
		// For example: _i_test-123_0, _i_test-123_0_0, etc.
		if !strings.Contains(html, "_i_test-123_") {
			t.Errorf("rendered HTML should contain island anchor with ID, got: %s", html)
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

		// All patches should have island ID set
		for _, patch := range patches {
			if patch.IslandID != "island-1" {
				t.Errorf("patch.IslandID = %q, want %q", patch.IslandID, "island-1")
			}
		}

		// Patches should have island-scoped anchors
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

		// Both should generate patches
		if len(patches1) == 0 || len(patches2) == 0 {
			t.Fatal("both islands should generate patches")
		}

		// Patches should have different island IDs
		if patches1[0].IslandID == patches2[0].IslandID {
			t.Error("patches from different islands should have different IslandIDs")
		}

		// Anchors should be different (include island ID)
		if patches1[0].Anchor == patches2[0].Anchor {
			t.Error("patches from different islands should have different anchors")
		}

		// Island 1 patches should have island-1 in anchor
		for _, patch := range patches1 {
			if !strings.Contains(patch.Anchor, "island-1") {
				t.Errorf("island-1 patch anchor should contain 'island-1', got: %s", patch.Anchor)
			}
		}

		// Island 2 patches should have island-2 in anchor
		for _, patch := range patches2 {
			if !strings.Contains(patch.Anchor, "island-2") {
				t.Errorf("island-2 patch anchor should contain 'island-2', got: %s", patch.Anchor)
			}
		}
	})

	t.Run("no cross-contamination between islands", func(t *testing.T) {
		// Same HTML, different islands
		html := "<div><span>Test</span></div>"

		patches1, _ := DiffIsland("alpha", html, html)
		patches2, _ := DiffIsland("beta", html, html)

		// No patches should be generated for identical HTML
		if len(patches1) != 0 {
			t.Errorf("identical HTML should generate 0 patches for island alpha, got %d", len(patches1))
		}
		if len(patches2) != 0 {
			t.Errorf("identical HTML should generate 0 patches for island beta, got %d", len(patches2))
		}

		// Now test with changes
		changed := "<div><span>Changed</span></div>"
		patchesA, _ := DiffIsland("alpha", html, changed)
		patchesB, _ := DiffIsland("beta", html, changed)

		// Each should have its own island ID
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

		// Check that generated HTML contains island anchors
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

		// All patches should have correct island ID
		for _, patch := range patches {
			if patch.IslandID != "nested-island" {
				t.Errorf("patch.IslandID = %q, want %q", patch.IslandID, "nested-island")
			}
		}

		// Patches should reference nested structure in anchors
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

		// Should generate append patch
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

		// Should generate replace patch
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

		// Should generate patch with empty HTML (deletion)
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

		// Anchors should be unique across islands
		anchor1 := patches1[0].Anchor
		anchor2 := patches2[0].Anchor

		if anchor1 == anchor2 {
			t.Errorf("anchors should be unique, both are: %s", anchor1)
		}

		// Both should contain their respective island IDs
		if !strings.Contains(anchor1, "island-a") {
			t.Errorf("anchor1 should contain 'island-a', got: %s", anchor1)
		}
		if !strings.Contains(anchor2, "island-b") {
			t.Errorf("anchor2 should contain 'island-b', got: %s", anchor2)
		}
	})
}

func TestRenderIslandIntegration(t *testing.T) {
	t.Run("render and diff workflow", func(t *testing.T) {
		// Create a counter island
		island, _ := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				return map[string]int{"count": 0}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(map[string]int)
				html := "<div class=\"counter\"><span>" + strings.Repeat("X", state["count"]) + "</span></div>"
				return strings.NewReader(html), nil
			}),
		)

		instance := &IslandInstance{
			ID:       "counter-test",
			Type:     "counter",
			island:   island,
			props:    Props{},
			children: "",
			mounted:  true,
			state:    map[string]int{"count": 3},
		}

		// Initial render
		html1, err := RenderIsland(context.Background(), instance)
		if err != nil {
			t.Fatalf("RenderIsland() error = %v", err)
		}

		// Update state
		instance.state = map[string]int{"count": 5}

		// Render again
		html2, err := RenderIsland(context.Background(), instance)
		if err != nil {
			t.Fatalf("RenderIsland() error = %v", err)
		}

		// Diff the two renders
		patches, err := DiffIsland("counter-test", html1, html2)
		if err != nil {
			t.Fatalf("DiffIsland() error = %v", err)
		}

		if len(patches) == 0 {
			t.Fatal("should generate patches for state change")
		}

		// Verify patches have island ID
		for _, patch := range patches {
			if patch.IslandID != "counter-test" {
				t.Errorf("patch.IslandID = %q, want %q", patch.IslandID, "counter-test")
			}
		}
	})
}
