package live

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

const _debug = false

// LiveRendered is an attribute key that indicates a DOM has been rendered by live.
// The live client JavaScript checks for this attribute to determine if it should
// attempt to connect to the server.
const LiveRendered = "live-rendered"

// liveAnchorPrefix prefixes injected anchors.
const liveAnchorPrefix = "_l"
const islandAnchorPrefix = "_i"
const liveAnchorSep = -1

// PatchAction defines the type of modification a patch will perform on the DOM.
type PatchAction uint32

// Patch actions define how the client should apply DOM updates.
const (
	// Noop indicates no action should be taken for this patch.
	Noop PatchAction = iota
	// Replace indicates the target node should be replaced with new content.
	Replace
	// Append indicates new content should be appended to the target node's children.
	Append
	// Prepend indicates new content should be prepended to the target node's children.
	Prepend
)

// anchorGenerator generates an ID for a node in the tree.
type anchorGenerator struct {
	idx []int
}

func newAnchorGenerator() anchorGenerator {
	return anchorGenerator{idx: []int{}}
}

// islandAnchorGenerator generates island-scoped IDs for nodes in the tree.
type islandAnchorGenerator struct {
	islandID string
	idx      []int
}

func newIslandAnchorGenerator(islandID string) islandAnchorGenerator {
	return islandAnchorGenerator{islandID: islandID, idx: []int{}}
}

// inc increment the current index.
func (n anchorGenerator) inc() anchorGenerator {
	o := make([]int, len(n.idx))
	copy(o, n.idx)
	if len(o) == 0 {
		o = []int{0}
	}
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

// inc increment the current index.
func (n islandAnchorGenerator) inc() islandAnchorGenerator {
	o := make([]int, len(n.idx))
	copy(o, n.idx)
	if len(o) == 0 {
		o = []int{0}
	}
	o[len(o)-1]++
	return islandAnchorGenerator{islandID: n.islandID, idx: o}
}

// level increase the depth.
func (n islandAnchorGenerator) level() islandAnchorGenerator {
	o := make([]int, len(n.idx))
	copy(o, n.idx)
	o = append(o, liveAnchorSep, 0)
	return islandAnchorGenerator{islandID: n.islandID, idx: o}
}

func (n islandAnchorGenerator) String() string {
	out := islandAnchorPrefix + "_" + n.islandID
	for _, i := range n.idx {
		if i == liveAnchorSep {
			out += "_"
		} else {
			out += fmt.Sprintf("%d", i)
		}
	}
	return out
}

// Patch represents a DOM modification to be applied on the client side.
// Each patch targets a specific anchor point in the DOM and contains
// the HTML content and action to perform.
type Patch struct {
	// Anchor is the DOM element identifier where this patch should be applied.
	// Anchors are generated automatically during rendering and use the format
	// "_l" for page-level patches or "_i_<islandID>" for island-scoped patches.
	Anchor string

	// Action specifies how to apply the HTML content (Replace, Append, Prepend, or Noop).
	Action PatchAction

	// HTML is the HTML content to apply at the anchor point.
	HTML string

	// IslandID optionally identifies which island this patch belongs to.
	// This is used for routing patches in multi-island scenarios.
	IslandID string `json:"island_id,omitempty"`
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

// IslandPatch associates a set of patches with a specific island ID.
// This wrapper enables routing patches to the correct island instance
// when multiple islands exist on the same page.
type IslandPatch struct {
	IslandID IslandID `json:"island_id"`
	Patches  []Patch  `json:"patches"`
}

// NewIslandPatch creates an IslandPatch wrapper with the given island ID.
// It automatically sets the IslandID field on each patch for consistency.
func NewIslandPatch(islandID IslandID, patches []Patch) IslandPatch {
	// Set the IslandID on each patch
	for i := range patches {
		patches[i].IslandID = string(islandID)
	}
	return IslandPatch{
		IslandID: islandID,
		Patches:  patches,
	}
}

// Diff compares two HTML node trees and generates a minimal set of patches
// to transform the current tree into the proposed tree.
//
// The function automatically anchors both trees before comparing them,
// ensuring each significant node has a unique identifier for precise targeting.
//
// This is the page-level diff function. For island-scoped diffs, use DiffIsland.
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

// DiffIsland compares two HTML strings within an island scope and returns patches
// with the island ID set. The proposed HTML is parsed and anchored with island-scoped
// anchors before diffing.
func DiffIsland(islandID IslandID, current, proposed string) ([]Patch, error) {
	// Parse current HTML
	currentNode, err := html.Parse(strings.NewReader(current))
	if err != nil {
		return nil, fmt.Errorf("failed to parse current HTML: %w", err)
	}
	shapeTree(currentNode)

	// Parse proposed HTML
	proposedNode, err := html.Parse(strings.NewReader(proposed))
	if err != nil {
		return nil, fmt.Errorf("failed to parse proposed HTML: %w", err)
	}
	shapeTree(proposedNode)

	// Anchor both trees with island-scoped anchors
	idGen := newIslandAnchorGenerator(string(islandID))
	anchorIslandTree(currentNode, idGen)
	anchorIslandTree(proposedNode, idGen)

	// Perform the diff
	patches := diffIslandTrees(currentNode, proposedNode, string(islandID))

	// Convert internal patches to output patches
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
			Anchor:   p.Anchor,
			Action:   p.Action,
			HTML:     buf.String(),
			IslandID: string(islandID),
		}
	}

	return output, nil
}

// anchorIslandTree anchors a tree with island-scoped anchor attributes.
func anchorIslandTree(root *html.Node, id islandAnchorGenerator) {
	// Check siblings first
	if root.NextSibling != nil {
		anchorIslandTree(root.NextSibling, id.inc())
	}
	// Then children
	if root.FirstChild != nil {
		anchorIslandTree(root.FirstChild, id.level())
	}

	// Add anchor if node is relevant and doesn't have one
	if nodeRelevant(root) && !hasAnchor(root) {
		root.Attr = append(root.Attr, html.Attribute{Key: id.String()})
	}
}

// diffIslandTrees compares two html Nodes within an island scope and outputs patches.
func diffIslandTrees(current, proposed *html.Node, islandID string) []patch {
	d := &differ{}
	// Trees are already anchored by caller
	return d.compareNodes(current, proposed, "")
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
		// Check for exact prefix matches: _l or _i followed by _ or end of string
		if strings.HasPrefix(a.Key, liveAnchorPrefix+"_") ||
			strings.HasPrefix(a.Key, islandAnchorPrefix+"_") ||
			a.Key == liveAnchorPrefix ||
			a.Key == islandAnchorPrefix {
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

	// Check for `live-update` modifiers. Save and restore so the modifier
	// doesn't leak into sibling subtrees.
	savedNode, savedModifier := d.updateNode, d.updateModifier
	d.liveUpdateCheck(newNode)

	// If nodes at this position are not equal patch a replacement.
	if !nodeEqual(oldNode, newNode) {
		patches = append(patches, d.generatePatch(newNode, parentAnchor, Replace))
		d.updateNode, d.updateModifier = savedNode, savedModifier
		return patches
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

	d.updateNode, d.updateModifier = savedNode, savedModifier
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
		// Check for both live and island anchors
		if strings.HasPrefix(a.Key, liveAnchorPrefix) || strings.HasPrefix(a.Key, islandAnchorPrefix) {
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
	slog.Debug(msg, "type", node.Type, "data", `s"`+node.Data+`"e`, "render", `s"`+d.String()+`"e`)
}
