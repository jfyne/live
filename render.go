package live

import (
	"context"
	"fmt"
)

// RenderIsland renders an island instance to its raw HTML string.
//
// The rendering process calls the island's render handler to generate HTML
// and returns the raw output. Tree shaping, anchoring, and diffing are
// handled by DiffIsland, which ensures consistent anchor assignment
// starting from the content root (body) rather than the document wrapper.
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

	return string(htmlContent), nil
}
