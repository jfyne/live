package live

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// RenderIsland renders an island instance to HTML with island-scoped anchors.
//
// The rendering process:
// 1. Calls the island's render handler to generate HTML
// 2. Parses the HTML into a node tree
// 3. Shapes the tree (removes insignificant whitespace)
// 4. Anchors all significant nodes with island-scoped identifiers
// 5. Renders the tree back to HTML string
//
// The anchored HTML enables precise diffing and patching on subsequent renders.
// Island-scoped anchors use the format "_i_<islandID>_<path>" to ensure
// uniqueness across multiple island instances on the same page.
//
// Returns the rendered HTML string or an error if rendering fails.
func RenderIsland(ctx context.Context, instance *IslandInstance) (string, error) {
	if instance == nil {
		return "", fmt.Errorf("instance is nil")
	}

	// Render the island using its render handler
	htmlContent, err := instance.Render(ctx)
	if err != nil {
		return "", fmt.Errorf("render error: %w", err)
	}

	// Parse the HTML
	render, err := html.Parse(strings.NewReader(string(htmlContent)))
	if err != nil {
		return "", fmt.Errorf("html parse error: %w", err)
	}

	// Shape the tree (remove insignificant whitespace, etc.)
	shapeTree(render)

	// Anchor the tree with island-scoped anchors
	anchorIslandTree(render, newIslandAnchorGenerator(instance.ID))

	// Render back to HTML
	var buf bytes.Buffer
	if err := html.Render(&buf, render); err != nil {
		return "", fmt.Errorf("html render error: %w", err)
	}

	return buf.String(), nil
}
