package live

import (
	"bytes"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

// Patch a location in the frontend dom.
type Patch struct {
	Path []int
	HTML string
}

// Diff compare two node states and return patches.
func Diff(current, proposed *html.Node) ([]Patch, error) {
	patches := diffTrees(current, proposed)
	output := make([]Patch, len(patches))

	for idx, p := range patches {
		var buf bytes.Buffer
		if err := html.Render(&buf, p.Node); err != nil {
			return nil, fmt.Errorf("failed to render patch: %w", err)
		}

		output[idx] = Patch{
			Path: p.Path,
			HTML: buf.String(),
		}
	}

	return output, nil
}

// patch describes how to modify a dom.
type patch struct {
	Path []int
	Node *html.Node
}

// diffTrees compares two html Nodes and outputs patches.
func diffTrees(current, proposed *html.Node) []patch {
	return compareNodes(current, proposed, 0, []int{})
}

func compareNodes(current, proposed *html.Node, currentIndex int, path []int) []patch {
	patches := []patch{}

	// Same so no patch.
	if current == nil && proposed == nil {
		return patches
	}

	// Decide what to do based on node type
	if current.Type == proposed.Type {
		switch proposed.Type {
		case html.ElementNode, html.TextNode:
			break
		case html.DoctypeNode:
			return append(patches, compareNodes(current.NextSibling, proposed.NextSibling, (currentIndex), path)...)
		case html.DocumentNode:
			return append(patches, compareNodes(current.FirstChild, proposed.FirstChild, 0, path)...)
		default:
			return patches
		}
	}

	// If proposed is something, and current is not patch.
	proposedPatch := patch{Path: path, Node: proposed}
	if proposed.Type == html.TextNode {
		proposedPatch.Node = proposed.Parent
	}
	if current == nil && proposed != nil {
		proposedPatch.Path = append(path, currentIndex)
		return append(patches, proposedPatch)
	}

	nextIndex := currentIndex
	if proposed.Type == html.ElementNode {
		nextIndex = currentIndex + 1
	}

	patches = append(patches, compareNodes(current.NextSibling, proposed.NextSibling, nextIndex, path)...)

	proposedPatch.Path = append(path, currentIndex)

	// Quick attr check.
	if len(current.Attr) != len(proposed.Attr) {
		return append(patches, proposedPatch)
	}
	// Deep attr check
	for _, c := range proposed.Attr {
		found := false
		for _, l := range current.Attr {
			if cmp.Equal(c, l) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		return append(patches, proposedPatch)
	}
	// Data check
	if current.Data != proposed.Data {
		return append(patches, proposedPatch)
	}
	// Type check
	if current.Type != proposed.Type {
		return append(patches, proposedPatch)
	}

	return append(patches, compareNodes(current.FirstChild, proposed.FirstChild, 0, append(path, currentIndex))...)
}
