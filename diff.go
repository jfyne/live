package live

import (
	"bytes"
	"fmt"
	"log"

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
	return compareNodes(current, proposed, []int{0})
}

func nextSiblingPath(path []int) []int {
	path[len(path)-1] = path[len(path)-1] + 1
	return path
}

func compareNodes(current, proposed *html.Node, path []int) []patch {
	patches := []patch{}

	// Same so no patch.
	if current == nil && proposed == nil {
		return patches
	}
	// If proposed is something, and current is not patch.
	proposedPatch := patch{Path: path, Node: proposed}
	if current == nil && proposed != nil {
		log.Println("patch applied", "complete")
		return append(patches, proposedPatch)
	}

	// Check siblings.
	patches = append(patches, compareNodes(current.NextSibling, proposed.NextSibling, nextSiblingPath(path))...)

	// Quick attr check.
	if len(current.Attr) != len(proposed.Attr) {
		log.Println("patch applied", "len attr")
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
		log.Println("patch applied", "attr")
		return append(patches, proposedPatch)
	}
	// Data check
	if current.Data != proposed.Data {
		log.Println("patch applied", "data")
		return append(patches, proposedPatch)
	}
	// Type check
	if current.Type != proposed.Type {
		log.Println("patch applied", "type")
		return append(patches, proposedPatch)
	}

	return append(patches, compareNodes(current.FirstChild, proposed.FirstChild, append(path, 0))...)
}
