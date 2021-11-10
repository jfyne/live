package live

import (
	"bytes"
	"fmt"
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
			{Anchor: "_l_0_1_0", Action: Replace, HTML: `<div _l_0_1_0="">World</div>`},
		},
	}, t)
}

func TestMultipleTextChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div>Hello</div><div>World</div>`,
		proposed: `<div>World</div><div>Hello</div>`,
		patches: []Patch{
			{Anchor: "_l_0_1_0", Action: Replace, HTML: `<div _l_0_1_0="">World</div>`},
			{Anchor: "_l_0_1_1", Action: Replace, HTML: `<div _l_0_1_1="">Hello</div>`},
		},
	}, t)
}

func TestNodeAppend(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div>World</div>`,
		proposed: `<div>Hello</div><div>World</div>`,
		patches: []Patch{
			{Anchor: "_l_0_1_0", Action: Replace, HTML: `<div _l_0_1_0="">Hello</div>`},
			{Anchor: "_l_0_1", Action: Append, HTML: `<div _l_0_1_1="">World</div>`},
		},
	}, t)
	runDiffTest(diffTest{
		root:     `<div>Hello</div>`,
		proposed: `<div>Hello</div><div>World</div>`,
		patches: []Patch{
			{Anchor: "_l_0_1", Action: Append, HTML: `<div _l_0_1_1="">World</div>`},
		},
	}, t)
}

func TestNodeDeletion(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div>Hello</div><div>World</div>`,
		proposed: `<div>World</div>`,
		patches: []Patch{
			{Anchor: "_l_0_1_0", Action: Replace, HTML: `<div _l_0_1_0="">World</div>`},
			{Anchor: "_l_0_1_1", Action: Replace, HTML: ""},
		},
	}, t)
	runDiffTest(diffTest{
		root:     `<div>Hello</div><div>World</div>`,
		proposed: `<div>Hello</div>`,
		patches: []Patch{
			{Anchor: "_l_0_1_1", Action: Replace, HTML: ""},
		},
	}, t)
}

func TestAttributeValueChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div place="World">Hello</div>`,
		proposed: `<div place="Change">Hello</div>`,
		patches: []Patch{
			{Anchor: "_l_0_1_0", Action: Replace, HTML: `<div place="Change" _l_0_1_0="">Hello</div>`},
		},
	}, t)
}

func TestMultipleAttributeValueChange(t *testing.T) {
	runDiffTest(diffTest{
		root:     `<div place="World">World</div><div place="Hello">Hello</div>`,
		proposed: `<div place="Hello">Hello</div><div place="World">World</div>`,
		patches: []Patch{
			{Anchor: "_l_0_1_0", Action: Replace, HTML: `<div place="Hello" _l_0_1_0="">Hello</div>`},
			{Anchor: "_l_0_1_1", Action: Replace, HTML: `<div place="World" _l_0_1_1="">World</div>`},
		},
	}, t)
}

func TestNestedAppend(t *testing.T) {
	tests := []diffTest{
		{
			root:     `<form><input type="text"/><input type="submit"/></form>`,
			proposed: `<form><div>Extra</div><input type="text"/><input type="submit"/></form>`,
			patches: []Patch{
				{Anchor: "_l_0_1_0_0", Action: Replace, HTML: `<div _l_0_1_0_0="">Extra</div>`},
				{Anchor: "_l_0_1_0_1", Action: Replace, HTML: `<input type="text" _l_0_1_0_1=""/>`},
				{Anchor: "_l_0_1_0", Action: Append, HTML: `<input type="submit" _l_0_1_0_2=""/>`},
			},
		},
	}
	for _, d := range tests {
		runDiffTest(d, t)
	}
}

func TestDoc(t *testing.T) {
	runDiffTest(diffTest{
		root:     "<!doctype><html><head><title>1</title></head><body><div>1</div></body></html>",
		proposed: "<!doctype><html><head><title>2</title></head><body><div>2</div></body></html>",
		patches: []Patch{
			{Anchor: "_l_1_0_0", Action: Replace, HTML: `<title _l_1_0_0="">2</title>`},
			{Anchor: "_l_1_1_0", Action: Replace, HTML: `<div _l_1_1_0="">2</div>`},
		},
	}, t)
}

func TestTreeShape(t *testing.T) {
	h := `<html>
            <head></head>
            <body>
                <form>
                    <div>1</div>
                    <div>2</div>
                    <div>3</div>
                    <input type="text"/>
                    <input type="submit"/>
                </form>
            </body>
        </html>
    `
	e := `<html><head></head><body live-rendered=""><form><div>1</div><div>2</div><div>3</div><input type="text"/><input type="submit"/></form></body></html>`
	tree, err := html.Parse(strings.NewReader(h))
	if err != nil {
		t.Fatal(err)
	}
	shapeTree(tree)

	var d bytes.Buffer
	html.Render(&d, tree)
	if e != d.String() {
		t.Fatal(fmt.Printf("prune failed\nexpected\n'%s'\ngot\n'%s'\n", e, d.String()))
	}
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
				{Anchor: "_l_0_1_0_0", Action: Replace, HTML: `<input type="text" _l_0_1_0_0=""/>`},
				{Anchor: "_l_0_1_0_1", Action: Replace, HTML: `<input type="submit" _l_0_1_0_1=""/>`},
				{Anchor: "_l_0_1_0_2", Action: Replace, HTML: ``},
				{Anchor: "_l_0_1_0_3", Action: Replace, HTML: ``},
				{Anchor: "_l_0_1_0_4", Action: Replace, HTML: ``},
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
				{Anchor: "_l_0_1_0_0", Action: Replace, HTML: `<div _l_0_1_0_0="">Extra</div>`},
				{Anchor: "_l_0_1_0_1", Action: Replace, HTML: `<input type="text" _l_0_1_0_1=""/>`},
				{Anchor: "_l_0_1_0", Action: Append, HTML: `<input type="submit" _l_0_1_0_2=""/>`},
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
				{Anchor: "_l_0_1_0", Action: Append, HTML: `<div _l_0_1_0_0="">World</div>`},
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
				{Anchor: "_l_0_1_0", Action: Append, HTML: `<div _l_0_1_0_0="">World</div>`},
			},
		},
		{
			root:     `<div live-update="prepend"><div>Hello</div></div>`,
			proposed: `<div live-update="prepend"><div>World</div></div>`,
			patches: []Patch{
				{Anchor: "_l_0_1_0", Action: Prepend, HTML: `<div _l_0_1_0_0="">World</div>`},
			},
		},
		{
			root:     `<div live-update="replace"><div>Hello</div></div>`,
			proposed: `<div live-update="replace"><div>World</div></div>`,
			patches: []Patch{
				{Anchor: "_l_0_1_0", Action: Replace, HTML: `<div _l_0_1_0_0="">World</div>`},
			},
		},
		{
			root:     `<div live-update="ignore"><div>Hello</div></div>`,
			proposed: `<div live-update="ignore"><div>World</div></div>`,
			patches: []Patch{
				{Anchor: "_l_0_1_0", Action: Noop, HTML: `<div _l_0_1_0_0="">World</div>`},
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
				{Anchor: "_l_0_1_1", Action: Replace, HTML: `<pre _l_0_1_1="">1</pre>`},
				{Anchor: "_l_0_1", Action: Append, HTML: `<script src="./live.js" _l_0_1_2=""></script>`},
			},
		},
		{
			root:     `<form><input type="text"/><input type="submit"/></form><script src="./live.js"></script>`,
			proposed: `<form><input type="text"/><input type="submit"/></form><pre>1</pre><script src="./live.js"></script>`,
			patches: []Patch{
				{Anchor: "_l_0_1_1", Action: Replace, HTML: `<pre _l_0_1_1="">1</pre>`},
				{Anchor: "_l_0_1", Action: Append, HTML: `<script src="./live.js" _l_0_1_2=""></script>`},
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
				{Anchor: "_l_0_1_2", Action: Replace, HTML: `<pre _l_0_1_2="">2</pre>`},
				{Anchor: "_l_0_1", Action: Append, HTML: `<script src="./live.js" _l_0_1_3=""></script>`},
			},
		},
		{
			root:     `<form><input type="text"/><input type="submit"/></form><pre>1</pre><script src="./live.js"></script>`,
			proposed: `<form><input type="text"/><input type="submit"/></form><pre>1</pre><pre>2</pre><script src="./live.js"></script>`,
			patches: []Patch{
				{Anchor: "_l_0_1_2", Action: Replace, HTML: `<pre _l_0_1_2="">2</pre>`},
				{Anchor: "_l_0_1", Action: Append, HTML: `<script src="./live.js" _l_0_1_3=""></script>`},
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
				{Anchor: "_l_0_1_3", Action: Replace, HTML: `<pre _l_0_1_3="">3</pre>`},
				{Anchor: "_l_0_1", Action: Append, HTML: `<script src="./live.js" _l_0_1_4=""></script>`},
			},
		},
		{
			root:     `<form><input type="text"/><input type="submit"/></form><pre>1</pre><pre>2</pre><script src="./live.js"></script>`,
			proposed: `<form><input type="text"/><input type="submit"/></form><pre>1</pre><pre>2</pre><pre>3</pre><script src="./live.js"></script>`,
			patches: []Patch{
				{Anchor: "_l_0_1_3", Action: Replace, HTML: `<pre _l_0_1_3="">3</pre>`},
				{Anchor: "_l_0_1", Action: Append, HTML: `<script src="./live.js" _l_0_1_4=""></script>`},
			},
		},
	}
	for _, d := range tests {
		runDiffTest(d, t)
	}
}

func TestListReplace(t *testing.T) {
	runDiffTest(diffTest{
		root: `
        <table>
            <tbody>
                <tr><td>1</td><td>Thinger 1</td></tr>
                <tr><td>2</td><td>Thinger 2</td></tr>
                <tr><td>3</td><td>Thinger 3</td></tr>
            </tbody>
        </table>
        `,
		proposed: `
        <table>
            <tbody>
                <tr><td colspan="2">No thingers</td></tr>
            </tbody>
        </table>
        `,
		patches: []Patch{
			{Anchor: "_l_0_1_0_0_0_0", Action: Replace, HTML: `<td colspan="2" _l_0_1_0_0_0_0="">No thingers</td>`},
			{Anchor: "_l_0_1_0_0_0_1", Action: Replace, HTML: ``},
			{Anchor: "_l_0_1_0_0_1", Action: Replace, HTML: ``},
			{Anchor: "_l_0_1_0_0_2", Action: Replace, HTML: ``},
		},
	}, t)
}

func BenchmarkDiff(b *testing.B) {
	root, err := html.Parse(strings.NewReader(testPage))
	if err != nil {
		b.Fatal(err)
	}

	for n := 0; n < b.N; n++ {
		diffTrees(root, root)
	}
}

func runDiffTest(tt diffTest, t *testing.T) {
	rootNode, err := html.Parse(strings.NewReader(tt.root))
	if err != nil {
		t.Error(err)
		return
	}
	shapeTree(rootNode)
	proposedNode, err := html.Parse(strings.NewReader(tt.proposed))
	if err != nil {
		t.Error(err)
		return
	}
	shapeTree(proposedNode)
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
		if expectedPatch.Anchor != patches[pidx].Anchor {
			t.Error("patch anchor does not match", "expected", expectedPatch.Anchor, "got", patches[pidx].Anchor)
			return
		}
		if expectedPatch.Action != patches[pidx].Action {
			t.Error("patch action does not match", "expected", expectedPatch.Action, "got", patches[pidx].Action)
			return
		}
	}
}

var testPage string = `
<html lang="en" op="news"><head><meta name="referrer" content="origin"><meta name="viewport" content="width=device-width, initial-scale=1.0"><link rel="stylesheet" type="text/css" href="news.css?qNWwHkd4E8FELceGdmI5">
        <link rel="shortcut icon" href="favicon.ico">
          <link rel="alternate" type="application/rss+xml" title="RSS" href="rss">
        <title>Hacker News</title></head><body><center><table id="hnmain" border="0" cellpadding="0" cellspacing="0" width="85%" bgcolor="#f6f6ef">
        <tr><td bgcolor="#ff6600"><table border="0" cellpadding="0" cellspacing="0" width="100%" style="padding:2px"><tr><td style="width:18px;padding-right:4px"><a href="https://news.ycombinator.com"><img src="y18.gif" width="18" height="18" style="border:1px white solid;"></a></td>
                  <td style="line-height:12pt; height:10px;"><span class="pagetop"><b class="hnname"><a href="news">Hacker News</a></b>
              <a href="newest">new</a> | <a href="threads?id=jfyne">threads</a> | <a href="front">past</a> | <a href="newcomments">comments</a> | <a href="ask">ask</a> | <a href="show">show</a> | <a href="jobs">jobs</a> | <a href="submit">submit</a>            </span></td><td style="text-align:right;padding-right:4px;"><span class="pagetop">
                              <a id='me' href="user?id=jfyne">jfyne</a>                (44) |
                <a id='logout' href="logout?auth=c8de92c70e0e0256439c8ceb13e0c2366b629ac3&amp;goto=news">logout</a>                          </span></td>
              </tr></table></td></tr>
<tr id="pagespace" title="" style="height:10px"></tr><tr><td><table border="0" cellpadding="0" cellspacing="0" class="itemlist">
              <tr class='athing' id='26262465'>
      <td align="right" valign="top" class="title"><span class="rank">1.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26262465' onclick='return vote(event, this, "up")' href='vote?id=26262465&amp;how=up&amp;auth=91cddb98b3cb6307fe38dfebc18839a43c805276&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://www.sec.gov/Archives/edgar/data/1582961/000119312521055798/d898181ds1.htm" class="storylink">DigitalOcean S-1</a><span class="sitebit comhead"> (<a href="from?site=sec.gov"><span class="sitestr">sec.gov</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26262465">143 points</span> by <a href="user?id=marc__1" class="hnuser">marc__1</a> <span class="age"><a href="item?id=26262465">1 hour ago</a></span> <span id="unv_26262465"></span> | <a href="flag?id=26262465&amp;auth=91cddb98b3cb6307fe38dfebc18839a43c805276&amp;goto=news">flag</a> | <a href="hide?id=26262465&amp;auth=91cddb98b3cb6307fe38dfebc18839a43c805276&amp;goto=news" onclick="return hidestory(event, this, 26262465)">hide</a> | <a href="item?id=26262465">71&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26262170'>
      <td align="right" valign="top" class="title"><span class="rank">2.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26262170' onclick='return vote(event, this, "up")' href='vote?id=26262170&amp;how=up&amp;auth=f15a0fe58dafd28688c8d1ba6b1072c9933576af&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://www.sec.gov/Archives/edgar/data/1679788/000162828021003168/coinbaseglobalincs-1.htm" class="storylink">Coinbase S-1</a><span class="sitebit comhead"> (<a href="from?site=sec.gov"><span class="sitestr">sec.gov</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26262170">194 points</span> by <a href="user?id=kacy" class="hnuser">kacy</a> <span class="age"><a href="item?id=26262170">2 hours ago</a></span> <span id="unv_26262170"></span> | <a href="flag?id=26262170&amp;auth=f15a0fe58dafd28688c8d1ba6b1072c9933576af&amp;goto=news">flag</a> | <a href="hide?id=26262170&amp;auth=f15a0fe58dafd28688c8d1ba6b1072c9933576af&amp;goto=news" onclick="return hidestory(event, this, 26262170)">hide</a> | <a href="item?id=26262170">222&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26262653'>
      <td align="right" valign="top" class="title"><span class="rank">3.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26262653' onclick='return vote(event, this, "up")' href='vote?id=26262653&amp;how=up&amp;auth=cf74e3e1c7026b596c8b03ab1ecb6db88269ad84&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://galois.com/blog/2020/12/proofs-should-repair-themselves/" class="storylink">Proofs Should Repair Themselves</a><span class="sitebit comhead"> (<a href="from?site=galois.com"><span class="sitestr">galois.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26262653">22 points</span> by <a href="user?id=harperlee" class="hnuser">harperlee</a> <span class="age"><a href="item?id=26262653">1 hour ago</a></span> <span id="unv_26262653"></span> | <a href="flag?id=26262653&amp;auth=cf74e3e1c7026b596c8b03ab1ecb6db88269ad84&amp;goto=news">flag</a> | <a href="hide?id=26262653&amp;auth=cf74e3e1c7026b596c8b03ab1ecb6db88269ad84&amp;goto=news" onclick="return hidestory(event, this, 26262653)">hide</a> | <a href="item?id=26262653">1&nbsp;comment</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26263097'>
      <td align="right" valign="top" class="title"><span class="rank">4.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26263097' onclick='return vote(event, this, "up")' href='vote?id=26263097&amp;how=up&amp;auth=54a6c798b3a1becfbca6e3f6f557031d0bbfb232&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://docs.amplify.aws/start/q/integration/flutter" class="storylink">Getting Started with AWS Amplify Flutter</a><span class="sitebit comhead"> (<a href="from?site=amplify.aws"><span class="sitestr">amplify.aws</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26263097">10 points</span> by <a href="user?id=voidmain0001" class="hnuser">voidmain0001</a> <span class="age"><a href="item?id=26263097">17 minutes ago</a></span> <span id="unv_26263097"></span> | <a href="flag?id=26263097&amp;auth=54a6c798b3a1becfbca6e3f6f557031d0bbfb232&amp;goto=news">flag</a> | <a href="hide?id=26263097&amp;auth=54a6c798b3a1becfbca6e3f6f557031d0bbfb232&amp;goto=news" onclick="return hidestory(event, this, 26263097)">hide</a> | <a href="item?id=26263097">4&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26258773'>
      <td align="right" valign="top" class="title"><span class="rank">5.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26258773' onclick='return vote(event, this, "up")' href='vote?id=26258773&amp;how=up&amp;auth=17dba9e14f69f0019f11478a2fcf87cd46d11e1f&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://github.com/DidierRLopes/GamestonkTerminal" class="storylink">Show HN: Can’t afford Bloomberg Terminal? No prob, I built the next best thing</a><span class="sitebit comhead"> (<a href="from?site=github.com/didierrlopes"><span class="sitestr">github.com/didierrlopes</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26258773">1199 points</span> by <a href="user?id=sexy_year" class="hnuser"><font color="#3c963c">sexy_year</font></a> <span class="age"><a href="item?id=26258773">13 hours ago</a></span> <span id="unv_26258773"></span> | <a href="flag?id=26258773&amp;auth=17dba9e14f69f0019f11478a2fcf87cd46d11e1f&amp;goto=news">flag</a> | <a href="hide?id=26258773&amp;auth=17dba9e14f69f0019f11478a2fcf87cd46d11e1f&amp;goto=news" onclick="return hidestory(event, this, 26258773)">hide</a> | <a href="item?id=26258773">222&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26262658'>
      <td align="right" valign="top" class="title"><span class="rank">6.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26262658' onclick='return vote(event, this, "up")' href='vote?id=26262658&amp;how=up&amp;auth=da3b1a43de89cc2ec5554b47c57ffc329bdeea35&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="http://m.nautil.us/issue/97/wonder/if-aliens-exist-heres-how-well-find-them" class="storylink">If Aliens Exist, Here’s How We’ll Find Them</a><span class="sitebit comhead"> (<a href="from?site=nautil.us"><span class="sitestr">nautil.us</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26262658">14 points</span> by <a href="user?id=CapitalistCartr" class="hnuser">CapitalistCartr</a> <span class="age"><a href="item?id=26262658">1 hour ago</a></span> <span id="unv_26262658"></span> | <a href="flag?id=26262658&amp;auth=da3b1a43de89cc2ec5554b47c57ffc329bdeea35&amp;goto=news">flag</a> | <a href="hide?id=26262658&amp;auth=da3b1a43de89cc2ec5554b47c57ffc329bdeea35&amp;goto=news" onclick="return hidestory(event, this, 26262658)">hide</a> | <a href="item?id=26262658">15&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26261314'>
      <td align="right" valign="top" class="title"><span class="rank">7.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26261314' onclick='return vote(event, this, "up")' href='vote?id=26261314&amp;how=up&amp;auth=d8d087b48530ef1e95e9e273daf751556d800c60&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://taler.net/en/" class="storylink">GNU Taler – Payment system for privacy-friendly, fast, easy online transactions</a><span class="sitebit comhead"> (<a href="from?site=taler.net"><span class="sitestr">taler.net</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26261314">112 points</span> by <a href="user?id=2pEXgD0fZ5cF" class="hnuser">2pEXgD0fZ5cF</a> <span class="age"><a href="item?id=26261314">4 hours ago</a></span> <span id="unv_26261314"></span> | <a href="flag?id=26261314&amp;auth=d8d087b48530ef1e95e9e273daf751556d800c60&amp;goto=news">flag</a> | <a href="hide?id=26261314&amp;auth=d8d087b48530ef1e95e9e273daf751556d800c60&amp;goto=news" onclick="return hidestory(event, this, 26261314)">hide</a> | <a href="item?id=26261314">29&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26260390'>
      <td align="right" valign="top" class="title"><span class="rank">8.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26260390' onclick='return vote(event, this, "up")' href='vote?id=26260390&amp;how=up&amp;auth=d925038cad991c9038b2a98cd26cd13a24ebd8a8&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://mac.getutm.app/" class="storylink">Show HN: QEMU front end for M1 and Intel Macs</a><span class="sitebit comhead"> (<a href="from?site=getutm.app"><span class="sitestr">getutm.app</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26260390">278 points</span> by <a href="user?id=osy" class="hnuser">osy</a> <span class="age"><a href="item?id=26260390">7 hours ago</a></span> <span id="unv_26260390"></span> | <a href="flag?id=26260390&amp;auth=d925038cad991c9038b2a98cd26cd13a24ebd8a8&amp;goto=news">flag</a> | <a href="hide?id=26260390&amp;auth=d925038cad991c9038b2a98cd26cd13a24ebd8a8&amp;goto=news" onclick="return hidestory(event, this, 26260390)">hide</a> | <a href="item?id=26260390">65&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26262667'>
      <td align="right" valign="top" class="title"><span class="rank">9.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26262667' onclick='return vote(event, this, "up")' href='vote?id=26262667&amp;how=up&amp;auth=97186ac3b6a74caa6db48cd8956c6a8cec2d150b&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://www.media.mit.edu/events/will-technology-save-us-from-climate-change/" class="storylink">Will technology save us from climate change?</a><span class="sitebit comhead"> (<a href="from?site=media.mit.edu"><span class="sitestr">media.mit.edu</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26262667">18 points</span> by <a href="user?id=HourglassFR" class="hnuser">HourglassFR</a> <span class="age"><a href="item?id=26262667">1 hour ago</a></span> <span id="unv_26262667"></span> | <a href="flag?id=26262667&amp;auth=97186ac3b6a74caa6db48cd8956c6a8cec2d150b&amp;goto=news">flag</a> | <a href="hide?id=26262667&amp;auth=97186ac3b6a74caa6db48cd8956c6a8cec2d150b&amp;goto=news" onclick="return hidestory(event, this, 26262667)">hide</a> | <a href="item?id=26262667">7&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26238376'>
      <td align="right" valign="top" class="title"><span class="rank">10.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26238376' onclick='return vote(event, this, "up")' href='vote?id=26238376&amp;how=up&amp;auth=1337e1eaac1f0ef0a54a554f63fb6c40c375f8b2&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://cacm.acm.org/magazines/2021/3/250710-the-decline-of-computers-as-a-general-purpose-technology/fulltext" class="storylink">The decline of computers as a general-purpose technology</a><span class="sitebit comhead"> (<a href="from?site=acm.org"><span class="sitestr">acm.org</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26238376">210 points</span> by <a href="user?id=matt_d" class="hnuser">matt_d</a> <span class="age"><a href="item?id=26238376">9 hours ago</a></span> <span id="unv_26238376"></span> | <a href="flag?id=26238376&amp;auth=1337e1eaac1f0ef0a54a554f63fb6c40c375f8b2&amp;goto=news">flag</a> | <a href="hide?id=26238376&amp;auth=1337e1eaac1f0ef0a54a554f63fb6c40c375f8b2&amp;goto=news" onclick="return hidestory(event, this, 26238376)">hide</a> | <a href="item?id=26238376">111&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26259955'>
      <td align="right" valign="top" class="title"><span class="rank">11.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26259955' onclick='return vote(event, this, "up")' href='vote?id=26259955&amp;how=up&amp;auth=03951144ab54b35e486b22c5869e93413e31d608&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://blog.detectify.com/2020/11/10/common-nginx-misconfigurations/" class="storylink">Common Nginx misconfigurations that leave your web server open to attack</a><span class="sitebit comhead"> (<a href="from?site=detectify.com"><span class="sitestr">detectify.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26259955">248 points</span> by <a href="user?id=sshroot" class="hnuser">sshroot</a> <span class="age"><a href="item?id=26259955">9 hours ago</a></span> <span id="unv_26259955"></span> | <a href="flag?id=26259955&amp;auth=03951144ab54b35e486b22c5869e93413e31d608&amp;goto=news">flag</a> | <a href="hide?id=26259955&amp;auth=03951144ab54b35e486b22c5869e93413e31d608&amp;goto=news" onclick="return hidestory(event, this, 26259955)">hide</a> | <a href="item?id=26259955">33&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26261976'>
      <td align="right" valign="top" class="title"><span class="rank">12.</span></td>      <td></td><td class="title"><a href="https://boards.greenhouse.io/pachyderm/jobs/4281816003" class="storylink" rel="nofollow">Pachyderm is hiring a Sr Python Dev to own our Jupyter Integration</a><span class="sitebit comhead"> (<a href="from?site=greenhouse.io"><span class="sitestr">greenhouse.io</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="age"><a href="item?id=26261976">2 hours ago</a></span> | <a href="hide?id=26261976&amp;auth=6577cabf2b88f158cfb419a1492468831d1c4ebd&amp;goto=news" onclick="return hidestory(event, this, 26261976)">hide</a>      </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26261310'>
      <td align="right" valign="top" class="title"><span class="rank">13.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26261310' onclick='return vote(event, this, "up")' href='vote?id=26261310&amp;how=up&amp;auth=41ed619b754451d7c16dd647a071424d4650c8a5&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://k3a.me/linux-capabilities-in-a-nutshell/" class="storylink">Linux Capabilities in a nutshell (2019)</a><span class="sitebit comhead"> (<a href="from?site=k3a.me"><span class="sitestr">k3a.me</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26261310">69 points</span> by <a href="user?id=todsacerdoti" class="hnuser">todsacerdoti</a> <span class="age"><a href="item?id=26261310">4 hours ago</a></span> <span id="unv_26261310"></span> | <a href="flag?id=26261310&amp;auth=41ed619b754451d7c16dd647a071424d4650c8a5&amp;goto=news">flag</a> | <a href="hide?id=26261310&amp;auth=41ed619b754451d7c16dd647a071424d4650c8a5&amp;goto=news" onclick="return hidestory(event, this, 26261310)">hide</a> | <a href="item?id=26261310">9&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26261098'>
      <td align="right" valign="top" class="title"><span class="rank">14.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26261098' onclick='return vote(event, this, "up")' href='vote?id=26261098&amp;how=up&amp;auth=a02e5bed25f2644736288a0f339a2997e88f71eb&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://www.timmons.dev/posts/static-executables-with-sbcl-v2.html" class="storylink">Static Executables with SBCL v2</a><span class="sitebit comhead"> (<a href="from?site=timmons.dev"><span class="sitestr">timmons.dev</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26261098">69 points</span> by <a href="user?id=todsacerdoti" class="hnuser">todsacerdoti</a> <span class="age"><a href="item?id=26261098">5 hours ago</a></span> <span id="unv_26261098"></span> | <a href="flag?id=26261098&amp;auth=a02e5bed25f2644736288a0f339a2997e88f71eb&amp;goto=news">flag</a> | <a href="hide?id=26261098&amp;auth=a02e5bed25f2644736288a0f339a2997e88f71eb&amp;goto=news" onclick="return hidestory(event, this, 26261098)">hide</a> | <a href="item?id=26261098">11&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26262192'>
      <td align="right" valign="top" class="title"><span class="rank">15.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26262192' onclick='return vote(event, this, "up")' href='vote?id=26262192&amp;how=up&amp;auth=564c81ce01264b80bfc4aa0c638adeef4d669680&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://github.com/JakePartusch/serverlessui" class="storylink">Show HN: A command-line utility for deploying serverless applications to AWS</a><span class="sitebit comhead"> (<a href="from?site=github.com/jakepartusch"><span class="sitestr">github.com/jakepartusch</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26262192">17 points</span> by <a href="user?id=jpartusch" class="hnuser"><font color="#3c963c">jpartusch</font></a> <span class="age"><a href="item?id=26262192">2 hours ago</a></span> <span id="unv_26262192"></span> | <a href="flag?id=26262192&amp;auth=564c81ce01264b80bfc4aa0c638adeef4d669680&amp;goto=news">flag</a> | <a href="hide?id=26262192&amp;auth=564c81ce01264b80bfc4aa0c638adeef4d669680&amp;goto=news" onclick="return hidestory(event, this, 26262192)">hide</a> | <a href="item?id=26262192">2&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26263085'>
      <td align="right" valign="top" class="title"><span class="rank">16.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26263085' onclick='return vote(event, this, "up")' href='vote?id=26263085&amp;how=up&amp;auth=9249153b2222067a9dab522b39502a86d9258f03&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://blainsmith.com/articles/plain-text-protocols/" class="storylink" rel="nofollow">Plain Text Protocols</a><span class="sitebit comhead"> (<a href="from?site=blainsmith.com"><span class="sitestr">blainsmith.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26263085">4 points</span> by <a href="user?id=tate" class="hnuser">tate</a> <span class="age"><a href="item?id=26263085">19 minutes ago</a></span> <span id="unv_26263085"></span> | <a href="flag?id=26263085&amp;auth=9249153b2222067a9dab522b39502a86d9258f03&amp;goto=news">flag</a> | <a href="hide?id=26263085&amp;auth=9249153b2222067a9dab522b39502a86d9258f03&amp;goto=news" onclick="return hidestory(event, this, 26263085)">hide</a> | <a href="item?id=26263085">1&nbsp;comment</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26262989'>
      <td align="right" valign="top" class="title"><span class="rank">17.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26262989' onclick='return vote(event, this, "up")' href='vote?id=26262989&amp;how=up&amp;auth=7f884c25d9cac6469c17b1f7dfe6496adbe7865b&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://draculatheme.com/pro/journey" class="storylink" rel="nofollow">Show HN: How I made $101,578.04 selling colors online</a><span class="sitebit comhead"> (<a href="from?site=draculatheme.com"><span class="sitestr">draculatheme.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26262989">5 points</span> by <a href="user?id=zenorocha" class="hnuser">zenorocha</a> <span class="age"><a href="item?id=26262989">27 minutes ago</a></span> <span id="unv_26262989"></span> | <a href="flag?id=26262989&amp;auth=7f884c25d9cac6469c17b1f7dfe6496adbe7865b&amp;goto=news">flag</a> | <a href="hide?id=26262989&amp;auth=7f884c25d9cac6469c17b1f7dfe6496adbe7865b&amp;goto=news" onclick="return hidestory(event, this, 26262989)">hide</a> | <a href="item?id=26262989">1&nbsp;comment</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26261327'>
      <td align="right" valign="top" class="title"><span class="rank">18.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26261327' onclick='return vote(event, this, "up")' href='vote?id=26261327&amp;how=up&amp;auth=936311cabed7e59a020185c339cc966e97469489&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://www.anandtech.com/show/16072/sipearl-lets-rhea-design-leak-72x-zeus-cores-4x-hbm2e-46-ddr5" class="storylink">SiPearl Lets Rhea Design Leak: 72x Zeus Cores, 4x HBM2E, 4-6 DDR5</a><span class="sitebit comhead"> (<a href="from?site=anandtech.com"><span class="sitestr">anandtech.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26261327">39 points</span> by <a href="user?id=rbanffy" class="hnuser">rbanffy</a> <span class="age"><a href="item?id=26261327">4 hours ago</a></span> <span id="unv_26261327"></span> | <a href="flag?id=26261327&amp;auth=936311cabed7e59a020185c339cc966e97469489&amp;goto=news">flag</a> | <a href="hide?id=26261327&amp;auth=936311cabed7e59a020185c339cc966e97469489&amp;goto=news" onclick="return hidestory(event, this, 26261327)">hide</a> | <a href="item?id=26261327">7&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26244829'>
      <td align="right" valign="top" class="title"><span class="rank">19.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26244829' onclick='return vote(event, this, "up")' href='vote?id=26244829&amp;how=up&amp;auth=f9fc669a0d3297d1fc0852703c238c7452d0d360&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://arstechnica.com/science/2021/02/scientists-create-new-class-of-turing-patterns-in-colonies-of-e-coli/" class="storylink">Scientists create new class of “Turing patterns” in colonies of E. coli</a><span class="sitebit comhead"> (<a href="from?site=arstechnica.com"><span class="sitestr">arstechnica.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26244829">19 points</span> by <a href="user?id=vo2maxer" class="hnuser">vo2maxer</a> <span class="age"><a href="item?id=26244829">4 hours ago</a></span> <span id="unv_26244829"></span> | <a href="flag?id=26244829&amp;auth=f9fc669a0d3297d1fc0852703c238c7452d0d360&amp;goto=news">flag</a> | <a href="hide?id=26244829&amp;auth=f9fc669a0d3297d1fc0852703c238c7452d0d360&amp;goto=news" onclick="return hidestory(event, this, 26244829)">hide</a> | <a href="item?id=26244829">discuss</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26260174'>
      <td align="right" valign="top" class="title"><span class="rank">20.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26260174' onclick='return vote(event, this, "up")' href='vote?id=26260174&amp;how=up&amp;auth=77da0cea59756ee1e10c78d94b55eedc979af331&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="item?id=26260174" class="storylink">Shopify says remove Stripe billing or get booted from their app store</a></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26260174">475 points</span> by <a href="user?id=ponny" class="hnuser">ponny</a> <span class="age"><a href="item?id=26260174">8 hours ago</a></span> <span id="unv_26260174"></span> | <a href="flag?id=26260174&amp;auth=77da0cea59756ee1e10c78d94b55eedc979af331&amp;goto=news">flag</a> | <a href="hide?id=26260174&amp;auth=77da0cea59756ee1e10c78d94b55eedc979af331&amp;goto=news" onclick="return hidestory(event, this, 26260174)">hide</a> | <a href="item?id=26260174">136&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26262687'>
      <td align="right" valign="top" class="title"><span class="rank">21.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26262687' onclick='return vote(event, this, "up")' href='vote?id=26262687&amp;how=up&amp;auth=090ed3aaae634cd45a0a92d403e8d6d5023f9930&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://journals.sagepub.com/doi/abs/10.1177/0956797617752640" class="storylink" rel="nofollow">Hypothetical Judgment versus Real-Life Behavior in Trolley-Style Moral Dilemmas</a><span class="sitebit comhead"> (<a href="from?site=sagepub.com"><span class="sitestr">sagepub.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26262687">4 points</span> by <a href="user?id=nabla9" class="hnuser">nabla9</a> <span class="age"><a href="item?id=26262687">57 minutes ago</a></span> <span id="unv_26262687"></span> | <a href="flag?id=26262687&amp;auth=090ed3aaae634cd45a0a92d403e8d6d5023f9930&amp;goto=news">flag</a> | <a href="hide?id=26262687&amp;auth=090ed3aaae634cd45a0a92d403e8d6d5023f9930&amp;goto=news" onclick="return hidestory(event, this, 26262687)">hide</a> | <a href="item?id=26262687">discuss</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26248603'>
      <td align="right" valign="top" class="title"><span class="rank">22.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26248603' onclick='return vote(event, this, "up")' href='vote?id=26248603&amp;how=up&amp;auth=65472097d1e22bf346c75be7a6733bfb03b72e38&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://www.fpgatutorial.com/" class="storylink">FPGA Developer Tutorials</a><span class="sitebit comhead"> (<a href="from?site=fpgatutorial.com"><span class="sitestr">fpgatutorial.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26248603">105 points</span> by <a href="user?id=Alekhine" class="hnuser">Alekhine</a> <span class="age"><a href="item?id=26248603">9 hours ago</a></span> <span id="unv_26248603"></span> | <a href="flag?id=26248603&amp;auth=65472097d1e22bf346c75be7a6733bfb03b72e38&amp;goto=news">flag</a> | <a href="hide?id=26248603&amp;auth=65472097d1e22bf346c75be7a6733bfb03b72e38&amp;goto=news" onclick="return hidestory(event, this, 26248603)">hide</a> | <a href="item?id=26248603">16&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26247813'>
      <td align="right" valign="top" class="title"><span class="rank">23.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26247813' onclick='return vote(event, this, "up")' href='vote?id=26247813&amp;how=up&amp;auth=37be6e4be6ade919cdfbfffc5789fc3e7e568afc&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://github.com/teal-language/tl" class="storylink">The compiler for Teal, a typed dialect of Lua</a><span class="sitebit comhead"> (<a href="from?site=github.com/teal-language"><span class="sitestr">github.com/teal-language</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26247813">28 points</span> by <a href="user?id=harporoeder" class="hnuser">harporoeder</a> <span class="age"><a href="item?id=26247813">5 hours ago</a></span> <span id="unv_26247813"></span> | <a href="flag?id=26247813&amp;auth=37be6e4be6ade919cdfbfffc5789fc3e7e568afc&amp;goto=news">flag</a> | <a href="hide?id=26247813&amp;auth=37be6e4be6ade919cdfbfffc5789fc3e7e568afc&amp;goto=news" onclick="return hidestory(event, this, 26247813)">hide</a> | <a href="item?id=26247813">5&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26247260'>
      <td align="right" valign="top" class="title"><span class="rank">24.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26247260' onclick='return vote(event, this, "up")' href='vote?id=26247260&amp;how=up&amp;auth=a7b1819b9b78fb8b05fbc4b2866f079389c20f98&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://johnresig.com/blog/programming-book-profits/" class="storylink">Programming Book Profits (2008)</a><span class="sitebit comhead"> (<a href="from?site=johnresig.com"><span class="sitestr">johnresig.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26247260">14 points</span> by <a href="user?id=max_" class="hnuser">max_</a> <span class="age"><a href="item?id=26247260">2 hours ago</a></span> <span id="unv_26247260"></span> | <a href="flag?id=26247260&amp;auth=a7b1819b9b78fb8b05fbc4b2866f079389c20f98&amp;goto=news">flag</a> | <a href="hide?id=26247260&amp;auth=a7b1819b9b78fb8b05fbc4b2866f079389c20f98&amp;goto=news" onclick="return hidestory(event, this, 26247260)">hide</a> | <a href="item?id=26247260">5&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26247954'>
      <td align="right" valign="top" class="title"><span class="rank">25.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26247954' onclick='return vote(event, this, "up")' href='vote?id=26247954&amp;how=up&amp;auth=fa89829bda96a04f645b1b9255090311af127db4&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://c3.handmade.network/blogs/p/7641-implementing_defer" class="storylink">Implementing "defer"</a><span class="sitebit comhead"> (<a href="from?site=handmade.network"><span class="sitestr">handmade.network</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26247954">27 points</span> by <a href="user?id=lerno" class="hnuser">lerno</a> <span class="age"><a href="item?id=26247954">4 hours ago</a></span> <span id="unv_26247954"></span> | <a href="flag?id=26247954&amp;auth=fa89829bda96a04f645b1b9255090311af127db4&amp;goto=news">flag</a> | <a href="hide?id=26247954&amp;auth=fa89829bda96a04f645b1b9255090311af127db4&amp;goto=news" onclick="return hidestory(event, this, 26247954)">hide</a> | <a href="item?id=26247954">25&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26249143'>
      <td align="right" valign="top" class="title"><span class="rank">26.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26249143' onclick='return vote(event, this, "up")' href='vote?id=26249143&amp;how=up&amp;auth=cf3f1abd63eb07faa6ca92cfa5ec483da629d5be&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="http://www.romhacking.net/start/" class="storylink" rel="nofollow">So, you want to be a ROMhacker? (2006)</a><span class="sitebit comhead"> (<a href="from?site=romhacking.net"><span class="sitestr">romhacking.net</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26249143">5 points</span> by <a href="user?id=mkl95" class="hnuser">mkl95</a> <span class="age"><a href="item?id=26249143">1 hour ago</a></span> <span id="unv_26249143"></span> | <a href="flag?id=26249143&amp;auth=cf3f1abd63eb07faa6ca92cfa5ec483da629d5be&amp;goto=news">flag</a> | <a href="hide?id=26249143&amp;auth=cf3f1abd63eb07faa6ca92cfa5ec483da629d5be&amp;goto=news" onclick="return hidestory(event, this, 26249143)">hide</a> | <a href="item?id=26249143">discuss</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26260710'>
      <td align="right" valign="top" class="title"><span class="rank">27.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26260710' onclick='return vote(event, this, "up")' href='vote?id=26260710&amp;how=up&amp;auth=8145bd78dbc82db152ffed65e064f68382f441cc&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://codesubmit.io/blog/the-evolution-of-developer-salaries/#tracing-developer-salaries-in-america-from-2001-to-2019" class="storylink">The Evolution of Developer Salaries: Looking Back 20 Years</a><span class="sitebit comhead"> (<a href="from?site=codesubmit.io"><span class="sitestr">codesubmit.io</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26260710">47 points</span> by <a href="user?id=fagnerbrack" class="hnuser">fagnerbrack</a> <span class="age"><a href="item?id=26260710">6 hours ago</a></span> <span id="unv_26260710"></span> | <a href="flag?id=26260710&amp;auth=8145bd78dbc82db152ffed65e064f68382f441cc&amp;goto=news">flag</a> | <a href="hide?id=26260710&amp;auth=8145bd78dbc82db152ffed65e064f68382f441cc&amp;goto=news" onclick="return hidestory(event, this, 26260710)">hide</a> | <a href="item?id=26260710">63&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26260165'>
      <td align="right" valign="top" class="title"><span class="rank">28.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26260165' onclick='return vote(event, this, "up")' href='vote?id=26260165&amp;how=up&amp;auth=838e05f5dea913283e5f78a4e19db47c8c69d31b&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://www.collaborativefund.com/blog/speculation/" class="storylink">When Everyone’s a Genius: A Few Thoughts on Speculation</a><span class="sitebit comhead"> (<a href="from?site=collaborativefund.com"><span class="sitestr">collaborativefund.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26260165">49 points</span> by <a href="user?id=luord" class="hnuser">luord</a> <span class="age"><a href="item?id=26260165">8 hours ago</a></span> <span id="unv_26260165"></span> | <a href="flag?id=26260165&amp;auth=838e05f5dea913283e5f78a4e19db47c8c69d31b&amp;goto=news">flag</a> | <a href="hide?id=26260165&amp;auth=838e05f5dea913283e5f78a4e19db47c8c69d31b&amp;goto=news" onclick="return hidestory(event, this, 26260165)">hide</a> | <a href="item?id=26260165">45&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26251143'>
      <td align="right" valign="top" class="title"><span class="rank">29.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26251143' onclick='return vote(event, this, "up")' href='vote?id=26251143&amp;how=up&amp;auth=93c54c18cd026d785f933af96fa494be9ff3320b&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://daliaawad28.medium.com/my-experience-as-a-gazan-girl-getting-into-silicon-valley-companies-488062d769a1" class="storylink">My experience as a Gazan girl getting into Silicon Valley companies</a><span class="sitebit comhead"> (<a href="from?site=daliaawad28.medium.com"><span class="sitestr">daliaawad28.medium.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26251143">1601 points</span> by <a href="user?id=daliaawad" class="hnuser"><font color="#3c963c">daliaawad</font></a> <span class="age"><a href="item?id=26251143">22 hours ago</a></span> <span id="unv_26251143"></span> | <a href="flag?id=26251143&amp;auth=93c54c18cd026d785f933af96fa494be9ff3320b&amp;goto=news">flag</a> | <a href="hide?id=26251143&amp;auth=93c54c18cd026d785f933af96fa494be9ff3320b&amp;goto=news" onclick="return hidestory(event, this, 26251143)">hide</a> | <a href="item?id=26251143">413&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
                <tr class='athing' id='26249254'>
      <td align="right" valign="top" class="title"><span class="rank">30.</span></td>      <td valign="top" class="votelinks"><center><a id='up_26249254' onclick='return vote(event, this, "up")' href='vote?id=26249254&amp;how=up&amp;auth=5b21ca45d6457cbc9b1393e59dd53a474d501ce4&amp;goto=news'><div class='votearrow' title='upvote'></div></a></center></td><td class="title"><a href="https://www.bbc.com/future/article/20210223-the-battery-invented-120-years-too-soon" class="storylink">A battery invented 120 years before its time</a><span class="sitebit comhead"> (<a href="from?site=bbc.com"><span class="sitestr">bbc.com</span></a>)</span></td></tr><tr><td colspan="2"></td><td class="subtext">
        <span class="score" id="score_26249254">73 points</span> by <a href="user?id=fpoling" class="hnuser">fpoling</a> <span class="age"><a href="item?id=26249254">11 hours ago</a></span> <span id="unv_26249254"></span> | <a href="flag?id=26249254&amp;auth=5b21ca45d6457cbc9b1393e59dd53a474d501ce4&amp;goto=news">flag</a> | <a href="hide?id=26249254&amp;auth=5b21ca45d6457cbc9b1393e59dd53a474d501ce4&amp;goto=news" onclick="return hidestory(event, this, 26249254)">hide</a> | <a href="item?id=26249254">37&nbsp;comments</a>              </td></tr>
      <tr class="spacer" style="height:5px"></tr>
            <tr class="morespace" style="height:10px"></tr><tr><td colspan="2"></td><td class="title"><a href="news?p=2" class="morelink" rel="next">More</a></td></tr>
  </table>
</td></tr>
<tr><td><img src="s.gif" height="10" width="0"><table width="100%" cellspacing="0" cellpadding="1"><tr><td bgcolor="#ff6600"></td></tr></table><br><center><span class="yclinks"><a href="newsguidelines.html">Guidelines</a>
        | <a href="newsfaq.html">FAQ</a>
        | <a href="lists">Lists</a>
        | <a href="https://github.com/HackerNews/API">API</a>
        | <a href="security.html">Security</a>
        | <a href="http://www.ycombinator.com/legal/">Legal</a>
        | <a href="http://www.ycombinator.com/apply/">Apply to YC</a>
        | <a href="mailto:hn@ycombinator.com">Contact</a></span><br><br><form method="get" action="//hn.algolia.com/">Search:
          <input type="text" name="q" value="" size="17" autocorrect="off" spellcheck="false" autocapitalize="off" autocomplete="false"></form>
            </center></td></tr>
      </table></center></body><script type='text/javascript' src='hn.js?qNWwHkd4E8FELceGdmI5'></script></html>
`
