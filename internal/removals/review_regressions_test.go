package removals

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestNodePrecedesStrictPreorder(t *testing.T) {
	main, _ := parseMain(t, `<article><div id="left"><span id="nested">first</span></div><p id="right">second</p></article>`)
	root := main.Find("article").Nodes[0]
	left := main.Find("#left").Nodes[0]
	nested := main.Find("#nested").Nodes[0]
	right := main.Find("#right").Nodes[0]
	for _, tt := range []struct {
		name string
		a, b *html.Node
		want bool
	}{
		{"identity", root, root, false},
		{"ancestor", root, nested, true},
		{"descendant", nested, root, false},
		{"siblings", left, right, true},
		{"reverse siblings", right, left, false},
		{"nested before sibling", nested, right, true},
		{"sibling before nested", right, nested, false},
		{"nil first", nil, root, false},
		{"nil second", root, nil, false},
		{"both nil", nil, nil, false},
		{"disconnected", root, &html.Node{Type: html.ElementNode, Data: "p"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.want, nodePrecedes(tt.a, tt.b)) })
	}
}

func TestRemovalEmptySelections(t *testing.T) {
	for _, main := range []*goquery.Selection{nil, {}} {
		assert.NotPanics(t, func() { RemoveByContentPattern(main, nil, false, "https://example.com/") })
		assert.NotPanics(t, func() { removeTrailingThinSections(main, false) })
	}
}

func TestStandaloneTimePreservesSelectedRoot(t *testing.T) {
	for _, tag := range []string{"p", "span", "strong"} {
		t.Run(tag, func(t *testing.T) {
			_, doc := parseMain(t, `<`+tag+` id="root"><em><time>January 1 2024</time></em></`+tag+`><p id="article">Useful article output survives.</p>`)
			root := doc.Find("#root")
			parent := root.Nodes[0].Parent
			removeStandaloneTimeElements(root, false)
			assert.Same(t, parent, root.Nodes[0].Parent)
			assert.Equal(t, 1, doc.Find("#root").Length())
			assert.Zero(t, root.Find("time").Length())
			assert.Contains(t, doc.Text(), "Useful article output survives.")
			outer, err := goquery.OuterHtml(root)
			require.NoError(t, err)
			assert.Contains(t, outer, `id="root"`)
		})
	}
	_, doc := parseMain(t, `<p id="root">Published <time>January 1 2024</time> during the project.</p>`)
	root := doc.Find("#root")
	removeStandaloneTimeElements(root, false)
	assert.Equal(t, "Published  during the project.", root.Text())
	assert.Equal(t, 1, doc.Find("#root").Length())
}

func TestPromotionalAndBreadcrumbHeadingWrappersSurvive(t *testing.T) {
	main, _ := parseMain(t, `<article><a id="banner" href="/news"><div>New announcement</div></a><a id="title" href="/blog"><div><h1>Article heading</h1></div></a><p>Useful article prose remains in the output.</p></article>`)
	removePromotionalBanners(main, false)
	assert.Zero(t, main.Find("#banner").Length())
	assert.Equal(t, 1, main.Find("#title h1").Length())
	assert.Contains(t, main.Text(), "Useful article prose")

	main, _ = parseMain(t, `<article><a id="title" href="/blog"><h1>Article heading</h1></a><p>Useful article prose.</p></article>`)
	removeSectionBreadcrumbs(main, "https://example.com/blog/post", false)
	assert.Equal(t, 1, main.Find("#title h1").Length())
	assert.Contains(t, main.Text(), "Useful article prose")

	main, _ = parseMain(t, `<article><div id="crumb"><ul><li><a href="/">Home</a></li><li>Current</li></ul></div><h1>Retained heading</h1><p>Retained article prose.</p></article>`)
	root := main.Find("article")
	removeBreadcrumbList(root, root.Nodes[0], false)
	assert.Zero(t, root.Find("#crumb").Length())
	assert.Contains(t, root.Text(), "Retained heading")
	assert.Contains(t, root.Text(), "Retained article prose.")
}

func TestTrailingLinkListsRequireProvenExternalWebLinks(t *testing.T) {
	for _, tt := range []struct {
		name, href, page string
		remove           bool
	}{
		{"relative path", "/internal", "https://www.example.com/post", false},
		{"fragment", "#footnote", "https://www.example.com/post", false},
		{"empty", "", "https://www.example.com/post", false},
		{"same domain", "https://example.com/internal", "https://www.example.com/post", false},
		{"same subdomain", "https://blog.example.com/internal", "https://www.example.com/post", false},
		{"suffix comparison", "https://news.example.co.uk/post", "https://www.example.co.uk/post", false},
		{"non web", "mailto:person@elsewhere.org", "https://www.example.com/post", false},
		{"javascript", "javascript:alert(1)", "https://www.example.com/post", false},
		{"malformed", "https://%zz", "https://www.example.com/post", false},
		{"hostless", "http:elsewhere.org", "https://www.example.com/post", false},
		{"invalid base", "https://elsewhere.org/post", "mailto:author@example.com", false},
		{"absolute external", "https://elsewhere.org/post", "https://www.example.com/post", true},
		{"protocol relative", "//elsewhere.org/post", "https://www.example.com/post", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			main, _ := parseMain(t, `<article><p>Article body preserved.</p><h2 id="heading">Resources</h2><ul id="links"><li><a href="`+tt.href+`">First resource</a></li><li><a href="https://other.org/post">Second resource</a></li></ul></article>`)
			removeTrailingExternalLinkLists(main.Find("article"), tt.page, false)
			assert.Equal(t, !tt.remove, main.Find("#links").Length() == 1)
			assert.Equal(t, !tt.remove, main.Find("#heading").Length() == 1)
			assert.Contains(t, main.Text(), "Article body preserved.")
		})
	}
}

func TestMetadataMissingTextAfterEarlierRemovalIsPreserved(t *testing.T) {
	// Overlapping selections visit the inner time before its enclosing byline.
	// Removing that time produces a byline absent from the original text snapshot.
	_, doc := parseMain(t, `<article><div id="byline">By <span id="inner"><time>Jan 1 | 3 min read</time></span>Jane Smith</div><p>This article has substantial prose that must survive all metadata pruning decisions.</p></article>`)
	root := doc.Find("article")
	main := doc.Find("#inner").AddSelection(root)
	removeSinglePassMetadata(main, root.Nodes[0], false)
	assert.Zero(t, root.Find("time").Length())
	assert.Equal(t, 1, root.Find("#byline").Length())
	assert.Equal(t, "By Jane Smith", root.Find("#byline").Text())
}

func TestThinPrecedingSectionUsesSharedCJKWordCount(t *testing.T) {
	for _, tt := range []struct {
		name, content string
		preserve      bool
	}{
		{"short English", "short preamble", false},
		{"long English", strings.Repeat("word ", 50), true},
		{"long CJK", strings.Repeat("文", 50), true},
		{"mixed scripts", strings.Repeat("文", 48) + " English prose", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			main, _ := parseMain(t, `<div id="before">`+tt.content+`</div><h2 id="target">Related posts</h2>`)
			removeThinPrecedingSection(main.Find("#target").Nodes[0])
			assert.Equal(t, tt.preserve, main.Find("#before").Length() == 1)
		})
	}
}
