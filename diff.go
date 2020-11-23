package live

import (
	"bytes"
	"fmt"
	"log"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

const _debug = false

// PatchAction
type PatchAction uint32

const (
	Noop PatchAction = iota
	Insert
	Replace
)

// Patch a location in the frontend dom.
type Patch struct {
	Path   []int
	Action PatchAction
	HTML   string
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
			Path:   p.Path[3:],
			Action: p.Action,
			HTML:   buf.String(),
		}
	}

	return output, nil
}

// patch describes how to modify a dom.
type patch struct {
	Path   []int
	Action PatchAction
	Node   *html.Node
}

// diffTrees compares two html Nodes and outputs patches.
func diffTrees(current, proposed *html.Node) []patch {
	return compareNodes(current, proposed, []int{0})
}

func compareNodes(oldNode, newNode *html.Node, followedPath []int) []patch {
	debugNodeLog("compareNodes oldNode", oldNode)
	debugNodeLog("compareNodes newNode", newNode)
	patches := []patch{}

	// Same so no patch.
	if oldNode == nil && newNode == nil {
		return patches
	}

	// If oldNode is nothing we need to insert the new node.
	if oldNode == nil {
		return append(patches, generatePatch(newNode, followedPath, Insert))
	}

	// If newNode does not exist, we need to patch a removal.
	if newNode == nil {
		return append(patches, generatePatch(newNode, followedPath, Replace))
	}

	// If nodes at this position are not equal patch a replacement.
	if nodeEqual(oldNode, newNode) == false {
		return append(patches, generatePatch(newNode, followedPath, Replace))
	}

	switch nodeAction(newNode) {
	case toSibling:
		return append(patches, compareNodes(oldNode.NextSibling, newNode.NextSibling, followedPath)...)
	case toChild:
		fallthrough
	case check:
		newChildren := generateNodeList(newNode.FirstChild)
		oldChildren := generateNodeList(oldNode.FirstChild)

		for i := 0; i < len(newChildren) || i < len(oldChildren); i++ {
			// Have to copy the followed path here, otherwise the patches
			// paths get updated.
			nextPath := make([]int, len(followedPath))
			copy(nextPath, followedPath)

			nextPath = append(nextPath, i)
			if i >= len(newChildren) {
				patches = append(patches, compareNodes(oldChildren[i], nil, nextPath)...)
			} else if i >= len(oldChildren) {
				patches = append(patches, compareNodes(nil, newChildren[i], nextPath)...)
			} else {
				patches = append(patches, compareNodes(oldChildren[i], newChildren[i], nextPath)...)
			}
		}
	}

	return patches
}

func nodeAction(node *html.Node) traverseAction {
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

func generatePatch(node *html.Node, followedPath []int, action PatchAction) patch {
	if node == nil {
		return patch{
			Path:   followedPath,
			Action: action,
			Node:   nil,
		}
	}
	debugNodeLog("generatePatch", node)
	switch node.Type {
	case html.TextNode:
		return patch{
			Path:   followedPath[:len(followedPath)-1],
			Action: action,
			Node:   node.Parent,
		}
	default:
		return patch{
			Path:   followedPath,
			Action: action,
			Node:   node,
		}
	}
}

// nodeEqual check if one node is equal to another.
func nodeEqual(oldNode *html.Node, newNode *html.Node) bool {
	// Type check
	if oldNode.Type != newNode.Type {
		return false
	}
	if len(oldNode.Attr) != len(newNode.Attr) {
		return false
	}
	// Deep attr check
	for _, c := range newNode.Attr {
		found := false
		for _, l := range oldNode.Attr {
			if cmp.Equal(c, l) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		return false
	}
	// Data check
	if oldNode.Data != newNode.Data {
		return false
	}
	return true
}

// generateNodeList create a list of sibling nodes.
func generateNodeList(node *html.Node) []*html.Node {
	list := []*html.Node{}
	if node == nil {
		return list
	}

	current := getFirstSibling(node)
	for {
		list = append(list, current)
		if current.NextSibling == nil {
			break
		} else {
			current = current.NextSibling
		}
	}
	return list
}

// getFirstSibling takes a node and finds the "first" node in the sibling
// list.
func getFirstSibling(node *html.Node) *html.Node {
	if node.PrevSibling == nil {
		return node
	}
	return getFirstSibling(node.PrevSibling)
}

func debugNodeLog(msg string, node *html.Node) {
	if !_debug {
		return
	}

	if node == nil {
		log.Println(msg, nil, nil, "")
		return
	}

	var d bytes.Buffer
	html.Render(&d, node)
	log.Println(msg, node.Type, node.Data, d.String())
}
