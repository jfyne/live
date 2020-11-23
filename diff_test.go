package live

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

type diffTest struct {
	root     string
	proposed string
	patches  []Patch
}

func TestSingleTextChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     "<div>Hello</div>",
		proposed: "<div>World</div>",
		patches: []Patch{
			{Path: []int{0}, Action: Replace, HTML: "<div>World</div>"},
		},
	}, t)
}

func TestMultipleTextChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div>Hello</div><div>World</div>`,
		proposed: `<div>World</div><div>Hello</div>`,
		patches: []Patch{
			{Path: []int{0}, Action: Replace, HTML: "<div>World</div>"},
			{Path: []int{1}, Action: Replace, HTML: "<div>Hello</div>"},
		},
	}, t)
}

func TestNodeInsertion(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div>World</div>`,
		proposed: `<div>Hello</div><div>World</div>`,
		patches: []Patch{
			{Path: []int{0}, Action: Replace, HTML: "<div>Hello</div>"},
			{Path: []int{1}, Action: Insert, HTML: "<div>World</div>"},
		},
	}, t)
	runDiffTest(diffTest{
		root:     `<div>Hello</div>`,
		proposed: `<div>Hello</div><div>World</div>`,
		patches: []Patch{
			{Path: []int{1}, Action: Insert, HTML: "<div>World</div>"},
		},
	}, t)
}

func TestNodeDeletion(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div>Hello</div><div>World</div>`,
		proposed: `<div>Hello</div>`,
		patches: []Patch{
			{Path: []int{1}, Action: Replace, HTML: ""},
		},
	}, t)
}

func TestAttributeValueChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div place="World">Hello</div>`,
		proposed: `<div place="Change">Hello</div>`,
		patches: []Patch{
			{Path: []int{0}, Action: Replace, HTML: `<div place="Change">Hello</div>`},
		},
	}, t)
}

func TestMultipleAttributeValueChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div place="World">World</div><div place="Hello">Hello</div>`,
		proposed: `<div place="Hello">Hello</div><div place="World">World</div>`,
		patches: []Patch{
			{Path: []int{0}, Action: Replace, HTML: `<div place="Hello">Hello</div>`},
			{Path: []int{1}, Action: Replace, HTML: `<div place="World">World</div>`},
		},
	}, t)
}

func TestNestedInsert(t *testing.T) {
	tests := []diffTest{
		{
			root:     `<form><input type="text"/><input type="submit"/></form>`,
			proposed: `<form><div>Extra</div><input type="text"/><input type="submit"/></form>`,
			patches: []Patch{
				{Path: []int{0, 0}, Action: Replace, HTML: `<div>Extra</div>`},
				{Path: []int{0, 1}, Action: Replace, HTML: `<input type="text"/>`},
				{Path: []int{0, 2}, Action: Insert, HTML: `<input type="submit"/>`},
			},
		},
	}
	for _, d := range tests {
		runDiffTest(d, t)
	}
}

func runDiffTest(tt diffTest, t *testing.T) {
	rootNode, err := html.Parse(strings.NewReader(tt.root))
	if err != nil {
		t.Error(err)
		return
	}
	proposedNode, err := html.Parse(strings.NewReader(tt.proposed))
	if err != nil {
		t.Error(err)
		return
	}
	patches, err := Diff(rootNode, proposedNode)
	if err != nil {
		t.Error(err)
		return
	}

	t.Log("Patches", patches)
	t.Log("Expected", tt.patches)
	for _, expectedPatch := range tt.patches {
		matched := false
		for _, proposedPatch := range patches {
			if expectedPatch.HTML == proposedPatch.HTML {
				if reflect.DeepEqual(expectedPatch.Path, proposedPatch.Path) {
					if expectedPatch.Action == proposedPatch.Action {
						matched = true
					} else {
						t.Error("html match, path matched, but action did not", expectedPatch.Action, proposedPatch.Action)
					}
				} else {
					t.Error("html matched, but path did not", expectedPatch.Path, proposedPatch.Path)
				}
			}
		}
		if !matched {
			t.Error("no match found for expected patch")
			return
		}
	}
}
