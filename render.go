package live

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// RenderIsland renders a single island instance and returns the HTML with
// island-scoped anchors. The island ID is extracted from the instance ID.
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
