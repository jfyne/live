package live

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

type diffTest struct {
	root    string
	propsed string
	patches []Patch
}

func TestSingleTextChange(t *testing.T) {
	runDiffTest(diffTest{
		root:    "<div>Hello</div>",
		propsed: "<div>World</div>",
		patches: []Patch{
			// The {1, 0} here indicate the selection of the <body>
			// as it gets automatically injected by html.Parse.
			{Path: []int{1, 0}, HTML: "<div>World</div>"},
		},
	}, t)
}

func TestMultipleTextChange(t *testing.T) {
	runDiffTest(diffTest{
		root:    `<div>Hello</div><div>World</div>`,
		propsed: `<div>World</div><div>Hello</div>`,
		patches: []Patch{
			{Path: []int{1, 0}, HTML: "<div>World</div>"},
			{Path: []int{1, 1}, HTML: "<div>Hello</div>"},
		},
	}, t)
}

func TestNodeAddition(t *testing.T) {
	runDiffTest(diffTest{
		root:    `<div>Hello</div>`,
		propsed: `<div>Hello</div><div>World</div>`,
		patches: []Patch{
			{Path: []int{1, 1}, HTML: "<div>World</div>"},
		},
	}, t)
}

func TestNodeDeletion(t *testing.T) {
	runDiffTest(diffTest{
		root:    `<div>Hello</div><div>World</div>`,
		propsed: `<div>Hello</div>`,
		patches: []Patch{
			{Path: []int{1, 1}, HTML: ""},
		},
	}, t)
}

func TestAttributeValueChange(t *testing.T) {
	runDiffTest(diffTest{
		root:    `<div place="World">Hello</div>`,
		propsed: `<div place="Change">Hello</div>`,
		patches: []Patch{
			{Path: []int{1, 0}, HTML: `<div place="Change">Hello</div>`},
		},
	}, t)
}

func TestMultipleAttributeValueChange(t *testing.T) {
	runDiffTest(diffTest{
		root:    `<div place="World">World</div><div place="Hello">Hello</div>`,
		propsed: `<div place="Hello">Hello</div><div place="World">World</div>`,
		patches: []Patch{
			{Path: []int{1, 0}, HTML: `<div place="Hello">Hello</div>`},
			{Path: []int{1, 1}, HTML: `<div place="World">World</div>`},
		},
	}, t)
}

func runDiffTest(tt diffTest, t *testing.T) {
	rootNode, err := html.Parse(strings.NewReader(tt.root))
	if err != nil {
		t.Error(err)
		return
	}
	proposedNode, err := html.Parse(strings.NewReader(tt.propsed))
	if err != nil {
		t.Error(err)
		return
	}
	patches, err := Diff(rootNode, proposedNode)
	if err != nil {
		t.Error(err)
		return
	}

	for _, expectedPatch := range tt.patches {
		matched := false
		t.Log("Expected patch", expectedPatch)
		for _, proposedPatch := range patches {
			t.Log("Proposed patch", proposedPatch)
			if expectedPatch.HTML == proposedPatch.HTML {
				if reflect.DeepEqual(expectedPatch.Path, proposedPatch.Path) {
					matched = true
				} else {
					t.Error("html matched, but path did not", expectedPatch.Path, proposedPatch.Path)
					return
				}
			}
		}
		if !matched {
			t.Error("no match found for expected patch")
			return
		}
	}
}
