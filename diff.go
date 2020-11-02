package live

import (
	"bytes"
	"fmt"
	"log"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

const _debug = false

// Patch a location in the frontend dom.
type Patch struct {
	Path []int
	HTML string
}

type traverseAction uint32

const (
	toSibling traverseAction = iota
	toChild
	check
)

// Diff compare two node states and return patches.
func Diff(current, proposed *html.Node) ([]Patch, error) {
	patches := diffTrees(current, proposed)
	output := make([]Patch, len(patches))

	for idx, p := range patches {
		var buf bytes.Buffer
		if p.Node != nil {
			if err := html.Render(&buf, p.Node); err != nil {
				return nil, fmt.Errorf("failed to render patch: %w", err)
			}
		} else {
			if _, err := buf.WriteString(""); err != nil {
				return nil, fmt.Errorf("failed to render blank patch: %w", err)
			}
		}

		output[idx] = Patch{
			Path: p.Path[1:],
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
	return compareNodes(current, proposed, 0, []int{0})
}

func compareNodes(current, proposed *html.Node, currentBranch int, followedPath []int) []patch {
	patches := []patch{}

	// Same so no patch.
	if current == nil && proposed == nil {
		return patches
	}

	// If current exists and proposed does not, we need to patch a removal.
	if current != nil && proposed == nil {
		return append(patches, generatePatch(proposed, currentBranch, followedPath))
	}

	// Decide if we need to skip over where we are, doctype <html> tag etc.
	switch nodesToSkip(proposed) {
	case toChild:
		return append(patches, compareNodes(current.FirstChild, proposed.FirstChild, 0, followedPath)...)
	case toSibling:
		return append(patches, compareNodes(current.NextSibling, proposed.NextSibling, currentBranch+1, followedPath)...)
	case check:
		break
	default:
		break
	}

	// If proposed is something, and current is not patch.
	if current == nil && proposed != nil {
		patch := generatePatch(proposed, currentBranch, followedPath)
		//log.Println("selectedPatch", patch.Path, patch.Node.Data)
		return append(patches, patch)
	}

	// Go traverse sibling nodes.
	switch proposed.Type {
	case html.ElementNode:
		// If we are an alement node we want to record the fact that the next sibling is actually at the next index.
		patches = append(patches, nextDestination(current.NextSibling, proposed.NextSibling, currentBranch+1, followedPath)...)
	case html.TextNode:
		// Text nodes don't count as another branch in the tree.
		patches = append(patches, nextDestination(current.NextSibling, proposed.NextSibling, currentBranch, followedPath)...)
	}

	proposedPatch := generatePatch(proposed, currentBranch, followedPath)
	// Quick attr check.
	if len(current.Attr) != len(proposed.Attr) {
		//log.Println("selectedPatch", proposedPatch.Path, proposedPatch.Node.Data)
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
		//log.Println("selectedPatch", proposedPatch.Path, proposedPatch.Node.Data)
		return append(patches, proposedPatch)
	}
	// Data check
	if current.Data != proposed.Data {
		//log.Println("selectedPatch", proposedPatch.Path, proposedPatch.Node.Data)
		return append(patches, proposedPatch)
	}
	// Type check
	if current.Type != proposed.Type {
		//log.Println("selectedPatch", proposedPatch.Path, proposedPatch.Node.Data)
		return append(patches, proposedPatch)
	}

	// Add to path as we step down a level in the tree.
	return append(patches, nextDestination(current.FirstChild, proposed.FirstChild, 0, append(followedPath, 0))...)
}

func nodesToSkip(node *html.Node) traverseAction {
	switch {
	case node.Type == html.DocumentNode:
		return toChild
	case node.Type == html.DoctypeNode:
		return toSibling
	case node.Type == html.ElementNode && node.Data == "html":
		return toChild
	}
	return check
}

func nextDestination(current, proposed *html.Node, currentBranch int, followedPath []int) []patch {
	branchPath := append(followedPath[:0:0], followedPath...)
	checkNode := proposed
	if proposed == nil {
		checkNode = current
	}
	if checkNode == nil {
		return compareNodes(current, proposed, currentBranch, branchPath)
	}
	switch checkNode.Type {
	case html.TextNode:
		return compareNodes(current, proposed, currentBranch, branchPath)
	case html.ElementNode:
		//log.Println("modifypath", checkNode.Data, branchPath, currentBranch)
		branchPath[len(branchPath)-1] = currentBranch
		return compareNodes(current, proposed, currentBranch, branchPath)
	default:
		debugNodeLog("unhandled", proposed)
		panic("Should not be here")
	}
}

func generatePatch(node *html.Node, currentBranch int, followedPath []int) patch {
	if node == nil {
		return patch{
			Path: followedPath,
			Node: nil,
		}
	}
	debugNodeLog("generatePatch", node)
	switch node.Type {
	case html.TextNode:
		return patch{
			Path: followedPath[:len(followedPath)-1],
			Node: node.Parent,
		}
	default:
		return patch{
			Path: followedPath,
			Node: node,
		}
	}
}

func debugNodeLog(msg string, node *html.Node) {
	if !_debug {
		return
	}

	var d bytes.Buffer
	html.Render(&d, node)
	log.Println(msg, node.Type, node.Data, d.String())
}
