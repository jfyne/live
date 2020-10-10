package live

import (
	"log"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

// Patch describes how to modify a dom.
type Patch struct {
	Path []int
	Node *html.Node
}

// DiffTrees compares two html Nodes and outputs patches.
func DiffTrees(current, proposed *html.Node) []Patch {
	return compareNodes(current, proposed, []int{0})
}

func nextSiblingPath(path []int) []int {
	path[len(path)-1] = path[len(path)-1] + 1
	return path
}

func compareNodes(current, proposed *html.Node, path []int) []Patch {
	patches := []Patch{}

	// Same so no patch.
	if current == nil && proposed == nil {
		return patches
	}
	// If proposed is something, and current is not patch.
	proposedPatch := Patch{Path: path, Node: proposed}
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
