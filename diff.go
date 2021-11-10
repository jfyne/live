package live

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

const _debug = false

// LiveRendered an attribute key to show that a DOM has been rendered by live.
const LiveRendered = "live-rendered"

// liveAnchorPrefix prefixes injected anchors.
const liveAnchorPrefix = "_l"
const liveAnchorSep = -1

// PatchAction available actions to take by a patch.
type PatchAction uint32

// Actions available.
const (
	Noop PatchAction = iota
	Replace
	Append
	Prepend
)

// anchorGenerator generates an ID for a node in the tree.
type anchorGenerator struct {
	idx []int
}

func newAnchorGenerator() anchorGenerator {
	return anchorGenerator{idx: []int{}}
}

// inc increment the current index.
func (n anchorGenerator) inc() anchorGenerator {
	o := make([]int, len(n.idx))
	copy(o, n.idx)
	o[len(o)-1]++
	return anchorGenerator{idx: o}
}

// level increase the depth.
func (n anchorGenerator) level() anchorGenerator {
	o := make([]int, len(n.idx))
	copy(o, n.idx)
	o = append(o, liveAnchorSep, 0)
	return anchorGenerator{idx: o}
}

func (n anchorGenerator) String() string {
	out := liveAnchorPrefix
	for _, i := range n.idx {
		if i == liveAnchorSep {
			out += "_"
		} else {
			out += fmt.Sprintf("%d", i)
		}
	}
	return out
}

// Patch a location in the frontend dom.
type Patch struct {
	Anchor string
	Action PatchAction
	HTML   string
}

func (p Patch) String() string {
	action := ""
	switch p.Action {
	case Noop:
		action = "NO"
	case Replace:
		action = "RE"
	case Append:
		action = "AP"
	case Prepend:
		action = "PR"
	}

	return fmt.Sprintf("%s %s %s", p.Anchor, action, p.HTML)
}

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
			Anchor: p.Anchor,
			//Path:   p.Path[2:],
			Action: p.Action,
			HTML:   buf.String(),
		}
	}

	return output, nil
}

// patch describes how to modify a dom.
type patch struct {
	Anchor string
	Action PatchAction
	Node   *html.Node
}

// differ handles state for recursive diffing.
type differ struct {
	// `live-update` handler.
	updateNode     *html.Node
	updateModifier PatchAction
}

// diffTrees compares two html Nodes and outputs patches.
func diffTrees(current, proposed *html.Node) []patch {
	d := &differ{}
	anchorTree(current, newAnchorGenerator())
	anchorTree(proposed, newAnchorGenerator())
	return d.compareNodes(current, proposed, "")
}

func anchorTree(root *html.Node, id anchorGenerator) {
	// Check this node.
	if root.NextSibling != nil {
		anchorTree(root.NextSibling, id.inc())
	}
	if root.FirstChild != nil {
		anchorTree(root.FirstChild, id.level())
	}

	if nodeRelevant(root) && !hasAnchor(root) {
		root.Attr = append(root.Attr, html.Attribute{Key: id.String()})
	}
}

func shapeTree(root *html.Node) {
	// Check this node.
	if root.NextSibling != nil {
		shapeTree(root.NextSibling)
	}
	if root.FirstChild != nil {
		shapeTree(root.FirstChild)
	}

	// Live is rendering this DOM tree so indicate that it has done so
	// so that the client side knows to attempt to connect.
	if root.Type == html.ElementNode && root.Data == "body" {
		if !hasAttr(root, LiveRendered) {
			root.Attr = append(root.Attr, html.Attribute{Key: LiveRendered})
		}
	}

	debugNodeLog("checking", root)
	if !nodeRelevant(root) {
		if root.Parent != nil {
			debugNodeLog("removingNode", root)
			root.Parent.RemoveChild(root)
		}
	}
}

func hasAnchor(node *html.Node) bool {
	for _, a := range node.Attr {
		if strings.HasPrefix(a.Key, liveAnchorPrefix) {
			return true
		}
	}
	return false
}

func hasAttr(node *html.Node, key string) bool {
	for _, a := range node.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func (d *differ) compareNodes(oldNode, newNode *html.Node, parentAnchor string) []patch {
	debugNodeLog("compareNodes oldNode", oldNode)
	debugNodeLog("compareNodes newNode", newNode)
	patches := []patch{}

	// Same so no patch.
	if oldNode == nil && newNode == nil {
		return patches
	}

	// If oldNode is nothing we need to append the new node.
	if oldNode == nil {
		if !nodeRelevant(newNode) {
			return []patch{}
		}
		return append(
			patches,
			d.generatePatch(newNode, parentAnchor, Append),
		)
	}

	// If newNode does not exist, we need to patch a removal.
	if newNode == nil {
		if !nodeRelevant(oldNode) {
			return []patch{}
		}
		return append(patches, d.generatePatch(newNode, findAnchor(oldNode), Replace))
	}

	// Check for `live-update` modifiers.
	d.liveUpdateCheck(newNode)

	// If nodes at this position are not equal patch a replacement.
	if !nodeEqual(oldNode, newNode) {
		return append(patches, d.generatePatch(newNode, parentAnchor, Replace))
	}

	newChildren := generateNodeList(newNode.FirstChild)
	oldChildren := generateNodeList(oldNode.FirstChild)

	for i := 0; i < len(newChildren) || i < len(oldChildren); i++ {
		if i >= len(newChildren) {
			patches = append(patches, d.compareNodes(oldChildren[i], nil, findAnchor(oldNode))...)
		} else if i >= len(oldChildren) {
			patches = append(patches, d.compareNodes(nil, newChildren[i], findAnchor(oldNode))...)
		} else {
			patches = append(patches, d.compareNodes(oldChildren[i], newChildren[i], findAnchor(oldNode))...)
		}
	}

	return patches
}

func (d *differ) generatePatch(node *html.Node, target string, action PatchAction) patch {
	if node == nil {
		return patch{
			Anchor: d.patchAnchor(target),
			Action: d.patchAction(action),
			Node:   nil,
		}
	}
	debugNodeLog("generatePatch", node)
	switch {
	case node.Type == html.TextNode:
		return patch{
			Anchor: d.patchAnchor(target),
			Action: d.patchAction(action),
			Node:   node.Parent,
		}
	case action == Append:
		return patch{
			Anchor: d.patchAnchor(target),
			Action: d.patchAction(action),
			Node:   node,
		}
	default:
		return patch{
			Anchor: d.patchAnchor(findAnchor(node)),
			Action: d.patchAction(action),
			Node:   node,
		}
	}
}

func findAnchor(node *html.Node) string {
	for _, a := range node.Attr {
		if strings.HasPrefix(a.Key, liveAnchorPrefix) {
			return a.Key
		}
	}
	return ""
}

// liveUpdateCheck check for an update modifier for this node.
func (d *differ) liveUpdateCheck(node *html.Node) {
	for _, attr := range node.Attr {
		if attr.Key != "live-update" {
			continue
		}
		d.updateNode = node

		switch attr.Val {
		case "replace":
			d.updateModifier = Replace
		case "ignore":
			d.updateModifier = Noop
		case "append":
			d.updateModifier = Append
		case "prepend":
			d.updateModifier = Prepend
		}
		break
	}
}

// patchAction in the current state of the differ get the patch
// action.
func (d *differ) patchAction(action PatchAction) PatchAction {
	if d.updateNode != nil {
		return d.updateModifier
	}
	return action
}

// patchAnchor in the current state of the differ get the patch
// anchor.
func (d *differ) patchAnchor(path string) string {
	if d.updateNode != nil {
		return findAnchor(d.updateNode)
	}
	return path
}

// nodeRelevant check if this node is relevant.
func nodeRelevant(node *html.Node) bool {
	if node.Type == html.TextNode {
		debugNodeLog("textNode", node)
	}
	if node.Type == html.TextNode && len(strings.TrimSpace(node.Data)) == 0 {
		return false
	}
	return true
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
	return strings.TrimSpace(oldNode.Data) == strings.TrimSpace(newNode.Data)
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
		return
	}

	var d bytes.Buffer
	html.Render(&d, node)
	log.Println(msg, node.Type, `s"`+node.Data+`"e`, `s"`+d.String()+`"e`)
}
