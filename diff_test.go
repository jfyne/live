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
			{Path: []int{1, 0}, Action: Replace, HTML: "<div>World</div>"},
		},
	}, t)
}

func TestMultipleTextChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div>Hello</div><div>World</div>`,
		proposed: `<div>World</div><div>Hello</div>`,
		patches: []Patch{
			{Path: []int{1, 0}, Action: Replace, HTML: "<div>World</div>"},
			{Path: []int{1, 1}, Action: Replace, HTML: "<div>Hello</div>"},
		},
	}, t)
}

func TestNodeAppend(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div>World</div>`,
		proposed: `<div>Hello</div><div>World</div>`,
		patches: []Patch{
			{Path: []int{1, 0}, Action: Replace, HTML: "<div>Hello</div>"},
			{Path: []int{1}, Action: Append, HTML: "<div>World</div>"},
			{Path: []int{1}, Action: Append, HTML: ""},
		},
	}, t)
	runDiffTest(diffTest{
		root:     `<div>Hello</div>`,
		proposed: `<div>Hello</div><div>World</div>`,
		patches: []Patch{
			{Path: []int{1}, Action: Append, HTML: "<div>World</div>"},
			{Path: []int{1}, Action: Append, HTML: ""},
		},
	}, t)
}

func TestNodeDeletion(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div>Hello</div><div>World</div>`,
		proposed: `<div>World</div>`,
		patches: []Patch{
			{Path: []int{1, 0}, Action: Replace, HTML: "<div>World</div>"},
			{Path: []int{1, 1}, Action: Replace, HTML: ""},
		},
	}, t)
	runDiffTest(diffTest{
		root:     `<div>Hello</div><div>World</div>`,
		proposed: `<div>Hello</div>`,
		patches: []Patch{
			{Path: []int{1, 1}, Action: Replace, HTML: ""},
		},
	}, t)
}

func TestAttributeValueChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div place="World">Hello</div>`,
		proposed: `<div place="Change">Hello</div>`,
		patches: []Patch{
			{Path: []int{1, 0}, Action: Replace, HTML: `<div place="Change">Hello</div>`},
		},
	}, t)
}

func TestMultipleAttributeValueChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div place="World">World</div><div place="Hello">Hello</div>`,
		proposed: `<div place="Hello">Hello</div><div place="World">World</div>`,
		patches: []Patch{
			{Path: []int{1, 0}, Action: Replace, HTML: `<div place="Hello">Hello</div>`},
			{Path: []int{1, 1}, Action: Replace, HTML: `<div place="World">World</div>`},
		},
	}, t)
}

func TestNestedAppend(t *testing.T) {
	tests := []diffTest{
		{
			root:     `<form><input type="text"/><input type="submit"/></form>`,
			proposed: `<form><div>Extra</div><input type="text"/><input type="submit"/></form>`,
			patches: []Patch{
				{Path: []int{1, 0, 0}, Action: Replace, HTML: `<div>Extra</div>`},
				{Path: []int{1, 0, 1}, Action: Replace, HTML: `<input type="text"/>`},
				{Path: []int{1, 0}, Action: Append, HTML: `<input type="submit"/>`},
				{Path: []int{1, 0}, Action: Append, HTML: ``},
			},
		},
	}
	for _, d := range tests {
		runDiffTest(d, t)
	}
}

func TestDoc(t *testing.T) {
	runDiffTest(diffTest{
		root:     "<!doctype><html><html><head><title>1</title></head><body><div>1</div></body></html>",
		proposed: "<!doctype><html><html><head><title>2</title></head><body><div>2</div></body></html>",
		patches: []Patch{
			{Path: []int{0, 0}, Action: Replace, HTML: "<title>2</title>"},
			{Path: []int{1, 0}, Action: Replace, HTML: "<div>2</div>"},
		},
	}, t)
}

func TestEarlyChildDeletion(t *testing.T) {
	tests := []diffTest{
		{
			root: `
		    <form>
		        <div>1</div>
		        <div>2</div>
		        <div>3</div>
		        <input type="text"/>
		        <input type="submit"/>
		    </form>`,
			proposed: `
		    <form>
		        <input type="text"/>
		        <input type="submit"/>
		    </form>`,
			patches: []Patch{
				{Path: []int{1, 0, 1}, Action: Replace, HTML: `<input type="text"/>`},
				{Path: []int{1, 0, 3}, Action: Replace, HTML: `<input type="submit"/>`},
				{Path: []int{1, 0, 5}, Action: Replace, HTML: ``},
				{Path: []int{1, 0, 7}, Action: Replace, HTML: ``},
				{Path: []int{1, 0, 9}, Action: Replace, HTML: ``},
			},
		},
	}
	for _, d := range tests {
		runDiffTest(d, t)
	}
}

func TestInsignificantWhitespace(t *testing.T) {
	tests := []diffTest{
		{
			root: `
		    <form>
		        <input type="text"/>
		        <input type="submit"/>
		    </form>`,
			proposed: `
		    <form>
		    <div>Extra</div>
		    <input type="text"/>
		    <input type="submit"/>
		    </form>`,
			patches: []Patch{
				{Path: []int{1, 0, 1}, Action: Replace, HTML: `<div>Extra</div>`},
				{Path: []int{1, 0, 3}, Action: Replace, HTML: `<input type="text"/>`},
				{Path: []int{1, 0}, Action: Append, HTML: `<input type="submit"/>`},
				{Path: []int{1, 0}, Action: Append, HTML: ``},
			},
		},
	}
	for _, d := range tests {
		runDiffTest(d, t)
	}
}

func TestLiveUpdate(t *testing.T) {
	tests := []diffTest{
		{
			root:     `<div live-update="append"><div>Hello</div></div>`,
			proposed: `<div live-update="append"><div>World</div></div>`,
			patches: []Patch{
				{Path: []int{1, 0}, Action: Append, HTML: `<div>World</div>`},
			},
		},
		{
			root: `
            <div live-update="append">
                <div>Hello</div>
            </div>`,
			proposed: `
            <div live-update="append">
                <div>World</div>
            </div>`,
			patches: []Patch{
				{Path: []int{1, 0}, Action: Append, HTML: `<div>World</div>`},
			},
		},
		{
			root:     `<div live-update="prepend"><div>Hello</div></div>`,
			proposed: `<div live-update="prepend"><div>World</div></div>`,
			patches: []Patch{
				{Path: []int{1, 0}, Action: Prepend, HTML: `<div>World</div>`},
			},
		},
		{
			root:     `<div live-update="replace"><div>Hello</div></div>`,
			proposed: `<div live-update="replace"><div>World</div></div>`,
			patches: []Patch{
				{Path: []int{1, 0, 0}, Action: Replace, HTML: `<div>World</div>`},
			},
		},
		{
			root:     `<div live-update="ignore"><div>Hello</div></div>`,
			proposed: `<div live-update="ignore"><div>World</div></div>`,
			patches: []Patch{
				{Path: []int{1, 0}, Action: Noop, HTML: `<div>World</div>`},
			},
		},
	}
	for _, d := range tests {
		runDiffTest(d, t)
	}
}

func TestIssue6(t *testing.T) {
	tests := []diffTest{
		{
			root: `
		    <form>
		        <input type="text"/>
		        <input type="submit"/>
		    </form>

            <script src="./live.js"></script>
            `,
			proposed: `
		    <form>
                <input type="text"/>
                <input type="submit"/>
		    </form>

            <pre>1</pre>

            <script src="./live.js"></script>
            `,
			patches: []Patch{
				{Path: []int{1, 2}, Action: Replace, HTML: `<pre>1</pre>`},
				{Path: []int{1}, Action: Append, HTML: `<script src="./live.js"></script>`},
				{Path: []int{1}, Action: Append, HTML: ``},
			},
		},
		{
			root: `
		    <form>
                <input type="text"/>
                <input type="submit"/>
		    </form>

            <pre>1</pre>

            <script src="./live.js"></script>
            `,
			proposed: `
		    <form>
                <input type="text"/>
                <input type="submit"/>
		    </form>

            <pre>1</pre>
            <pre>2</pre>

            <script src="./live.js"></script>
            `,
			patches: []Patch{
				{Path: []int{1, 4}, Action: Replace, HTML: `<pre>2</pre>`},
				{Path: []int{1}, Action: Append, HTML: `<script src="./live.js"></script>`},
				{Path: []int{1}, Action: Append, HTML: ``},
			},
		},
		{
			root: `
		    <form>
                <input type="text"/>
                <input type="submit"/>
		    </form>

            <pre>1</pre>
            <pre>2</pre>

            <script src="./live.js"></script>
            `,
			proposed: `
		    <form>
                <input type="text"/>
                <input type="submit"/>
		    </form>

            <pre>1</pre>
            <pre>2</pre>
            <pre>3</pre>

            <script src="./live.js"></script>
            `,
			patches: []Patch{
				{Path: []int{1, 6}, Action: Replace, HTML: `<pre>3</pre>`},
				{Path: []int{1}, Action: Append, HTML: `<script src="./live.js"></script>`},
				{Path: []int{1}, Action: Append, HTML: ``},
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

	t.Log("Patches ", patches)
	t.Log("Expected", tt.patches)

	if len(patches) != len(tt.patches) {
		t.Error("different amount of patches", "expected", len(tt.patches), "got", len(patches))
		return
	}

	for pidx, expectedPatch := range tt.patches {
		if expectedPatch.HTML != patches[pidx].HTML {
			t.Error("patch html does not match", "expected", `"`+expectedPatch.HTML+`"`, "got", `"`+patches[pidx].HTML+`"`)
			return
		}
		if !reflect.DeepEqual(expectedPatch.Path, patches[pidx].Path) {
			t.Error("patch patch does not match", "expected", expectedPatch.Path, "got", patches[pidx].Path)
			return
		}
		if expectedPatch.Action != patches[pidx].Action {
			t.Error("patch action does not match", "expected", expectedPatch.Action, "got", patches[pidx].Action)
			return
		}
	}
}
