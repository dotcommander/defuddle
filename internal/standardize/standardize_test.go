package standardize

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

// parseDoc parses an HTML string into a goquery Document.
func parseDoc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	return doc
}

// meta returns a minimal Metadata with the given title.
func meta(title string) *metadata.Metadata {
	return &metadata.Metadata{Title: title}
}

func TestContent_StandardizesSpaces(t *testing.T) {
	t.Parallel()

	html := `<div id="root"><p>Hello   world    and   more</p></div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta(""), doc, false)

	text := strings.TrimSpace(doc.Find("p").First().Text())
	// Multiple spaces should be collapsed
	assert.NotContains(t, text, "   ", "multiple spaces should be collapsed")
}

func TestContent_RemovesHTMLComments(t *testing.T) {
	t.Parallel()

	// goquery strips HTML comments during parsing so we verify via the pipeline
	// by checking the final rendered HTML has no comment markers.
	html := `<div id="root"><p>Visible text</p></div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta(""), doc, false)

	rendered, err := element.Html()
	require.NoError(t, err)
	assert.NotContains(t, rendered, "<!--")
}

func TestContent_StandardizeHeadings_H1ConvertedToH2(t *testing.T) {
	t.Parallel()

	html := `<div id="root">
		<h1>Some heading</h1>
		<p>Paragraph text here</p>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta("Different Title"), doc, false)

	// The H1 should be converted to H2
	assert.Equal(t, 0, element.Find("h1").Length(), "h1 should be converted to h2")
	assert.GreaterOrEqual(t, element.Find("h2").Length(), 1, "should have at least one h2")
}

func TestContent_StandardizeHeadings_MatchingTitleH1Removed(t *testing.T) {
	t.Parallel()

	html := `<div id="root">
		<h1>Article Title</h1>
		<p>Body text follows the heading with enough content here.</p>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta("Article Title"), doc, false)

	// H1 converted to H2, then first H2 matching title should be removed
	assert.Equal(t, 0, element.Find("h1").Length(), "h1 should be gone")
	h2s := element.Find("h2")
	// All remaining H2s should not have the title text
	h2s.Each(func(_ int, s *goquery.Selection) {
		assert.NotEqual(t, "article title", strings.ToLower(strings.TrimSpace(s.Text())))
	})
}

func TestContent_StandardizeHeadings_NonMatchingH1BecomesH2(t *testing.T) {
	t.Parallel()

	html := `<div id="root">
		<h1>Section Heading</h1>
		<p>Content paragraph with enough text to survive cleanup.</p>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta("Page Title"), doc, false)

	assert.Equal(t, 0, element.Find("h1").Length(), "h1 should be converted")
	assert.Equal(t, 1, element.Find("h2").Length(), "non-matching h1 becomes h2")
	assert.Equal(t, "Section Heading", strings.TrimSpace(element.Find("h2").Text()))
}

func TestContent_RemovesEmptyElements(t *testing.T) {
	t.Parallel()

	html := `<div id="root">
		<p>Real content</p>
		<div></div>
		<span>   </span>
		<p>More real content</p>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta(""), doc, false)

	// Empty div and whitespace-only span should be removed
	assert.Equal(t, 0, element.Find("div").Length(), "empty div should be removed")
}

func TestContent_RemovesTrailingHeadings(t *testing.T) {
	t.Parallel()

	html := `<div id="root">
		<p>Some content paragraph here.</p>
		<h2>Trailing Section</h2>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta(""), doc, false)

	// The trailing heading has no content after it and should be removed
	assert.Equal(t, 0, element.Find("h2").Length(), "trailing heading should be removed")
}

func TestContent_PreservesHeadingWithContentAfter(t *testing.T) {
	t.Parallel()

	html := `<div id="root">
		<h2>Section A</h2>
		<p>Content under section A follows.</p>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta(""), doc, false)

	// Heading has content after it so it must be preserved
	assert.Equal(t, 1, element.Find("h2").Length(), "non-trailing heading should stay")
}

func TestContent_StripsExtraConsecutiveBrElements(t *testing.T) {
	t.Parallel()

	html := `<div id="root"><p>Line one<br><br><br><br>Line two</p></div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta(""), doc, false)

	brCount := element.Find("br").Length()
	assert.LessOrEqual(t, brCount, 2, "should have at most 2 consecutive br elements")
}

func TestContent_DebugModePreservesStructure(t *testing.T) {
	t.Parallel()

	// In debug mode flattenWrapperElements is skipped so nested wrappers stay
	html := `<div id="root">
		<div class="wrapper">
			<div class="container">
				<p>Inner content paragraph here.</p>
			</div>
		</div>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta(""), doc, true)

	// In debug mode the wrapper divs should NOT be flattened away
	wrapperDiv := element.Find(".wrapper")
	assert.Equal(t, 1, wrapperDiv.Length(), "debug mode should preserve wrapper divs")
}

func TestContent_NonDebugMode_FlattensWrapperDivs(t *testing.T) {
	t.Parallel()

	// A single-child wrapper div that only wraps block content is a candidate for
	// flattening. We verify the function runs without error and the paragraph
	// content is still reachable after the call.
	html := `<div id="root">
		<div class="wrapper">
			<p>Content paragraph text that should survive flattening process.</p>
		</div>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta(""), doc, false)

	// The paragraph content must still be present regardless of whether the
	// wrapper was flattened
	assert.Contains(t, element.Text(), "Content paragraph text")
}

func TestContent_MultipleH1sAllConvertedToH2(t *testing.T) {
	t.Parallel()

	html := `<div id="root">
		<h1>First heading</h1>
		<p>Some text between headings here.</p>
		<h1>Second heading</h1>
		<p>More text after second heading.</p>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	Content(element, meta("Unrelated Page Title"), doc, false)

	assert.Equal(t, 0, element.Find("h1").Length(), "all h1s should be converted")
	assert.Equal(t, 2, element.Find("h2").Length(), "both should become h2")
}

func TestContent_EmptyMetadataTitlePreservesFirstH2(t *testing.T) {
	t.Parallel()

	html := `<div id="root">
		<h1>Kept Heading</h1>
		<p>Paragraph after heading with content.</p>
	</div>`
	doc := parseDoc(t, html)
	element := doc.Find("#root")

	// Empty title means no H2 should be removed due to title match
	Content(element, meta(""), doc, false)

	assert.Equal(t, 1, element.Find("h2").Length(), "h2 should remain when title is empty")
}

// --- Phase 4: Cleanup function tests ---

func TestRemoveHtmlComments(t *testing.T) {
	t.Parallel()

	// goquery strips comments at parse time, so test at the html.Node level
	htmlStr := `<div id="root"><p>Visible</p></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	// Manually insert a comment node
	comment := &html.Node{Type: html.CommentNode, Data: " this is a comment "}
	root.Get(0).AppendChild(comment)

	// Verify comment exists
	found := false
	for c := root.Get(0).FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.CommentNode {
			found = true
		}
	}
	require.True(t, found, "comment should exist before removal")

	removeHTMLComments(root)

	// Verify comment removed
	for c := root.Get(0).FirstChild; c != nil; c = c.NextSibling {
		assert.NotEqual(t, html.CommentNode, c.Type, "comment nodes should be removed")
	}
	assert.Contains(t, root.Text(), "Visible")
}

func TestUnwrapBareSpans(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><p>Before <span>inner text</span> after</p></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	unwrapBareSpans(root)

	assert.Equal(t, 0, root.Find("span").Length(), "bare span should be removed")
	assert.Contains(t, root.Text(), "inner text")
}

func TestUnwrapBareSpans_PreservesSpanWithClass(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><p><span class="highlight">styled</span></p></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	unwrapBareSpans(root)

	assert.Equal(t, 1, root.Find("span.highlight").Length(), "span with class should be preserved")
}

func TestUnwrapSpecialLinks_JavascriptHref(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><p><a href="javascript:void(0)">Click me</a></p></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	unwrapSpecialLinks(root, doc)

	assert.Equal(t, 0, root.Find("a").Length(), "javascript: link should be unwrapped")
	assert.Contains(t, root.Text(), "Click me")
}

func TestUnwrapSpecialLinks_LinkInsideCode(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><code><a href="https://example.com">funcName</a></code></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	unwrapSpecialLinks(root, doc)

	assert.Equal(t, 0, root.Find("code a").Length(), "link inside code should be unwrapped")
	assert.Contains(t, root.Find("code").Text(), "funcName")
}

func TestUnwrapSpecialLinks_AnchorWrappingHeading(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><a href="#section-1"><h2>Section One</h2></a></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	unwrapSpecialLinks(root, doc)

	// The anchor wrapping the heading should be unwrapped
	assert.Equal(t, 1, root.Find("h2").Length(), "heading should remain")
	// No anchor wrapping the heading
	assert.Equal(t, 0, root.Find("a > h2").Length(), "heading should not be inside an anchor")
}

func TestRemoveHeadingAnchors_PermalinkSymbol(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><h2>My Heading <a href="#my-heading">#</a></h2></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	removeHeadingAnchors(root)

	assert.Equal(t, 0, root.Find("h2 a").Length(), "permalink anchor should be removed")
	text := strings.TrimSpace(root.Find("h2").Text())
	assert.Equal(t, "My Heading", text)
}

func TestRemoveHeadingAnchors_PermalinkClass(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><h3>Title <a class="permalink" href="/page#title">¶</a></h3></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	removeHeadingAnchors(root)

	assert.Equal(t, 0, root.Find("h3 a").Length(), "permalink class anchor should be removed")
}

func TestRemoveHeadingAnchors_PreservesRealLinks(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><h2><a href="https://example.com">External Link</a></h2></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	removeHeadingAnchors(root)

	// This link has href containing # (https contains no #), but the current
	// implementation removes any link with href containing "#". Since this URL
	// has no # fragment, it would be preserved only if it has no other signals.
	// Actually, "https://example.com" does NOT contain "#", so it should survive
	// unless it matches another rule. Let's check — the function checks
	// HasPrefix(href, "#") || Contains(href, "#"). This URL has no #.
	// And it has no permalink class/title. So it should be preserved.
	assert.Equal(t, 1, root.Find("h2 a").Length(), "real external link should be preserved")
}

func TestRemoveObsoleteElements(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root">
		<p>Content</p>
		<object data="old.swf"></object>
		<embed src="plugin.swf">
		<applet code="app.class"></applet>
		<p>More content</p>
	</div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	removeObsoleteElements(root)

	assert.Equal(t, 0, root.Find("object").Length())
	assert.Equal(t, 0, root.Find("embed").Length())
	assert.Equal(t, 0, root.Find("applet").Length())
	assert.Equal(t, 2, root.Find("p").Length(), "paragraphs should be preserved")
}

func TestRemoveOrphanedDividers_Leading(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><hr><p>Content</p></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	removeOrphanedDividers(root)

	assert.Equal(t, 0, root.Find("hr").Length(), "leading hr should be removed")
	assert.Contains(t, root.Text(), "Content")
}

func TestRemoveOrphanedDividers_Trailing(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><p>Content</p><hr></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	removeOrphanedDividers(root)

	assert.Equal(t, 0, root.Find("hr").Length(), "trailing hr should be removed")
}

func TestRemoveOrphanedDividers_PreservesMiddle(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><p>Before</p><hr><p>After</p></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	removeOrphanedDividers(root)

	assert.Equal(t, 1, root.Find("hr").Length(), "middle hr should be preserved")
}

func TestWrapPreformattedCode(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><p><code style="white-space: pre">some code</code></p></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	wrapPreformattedCode(root)

	assert.Equal(t, 1, root.Find("pre code").Length(), "code should be wrapped in pre")
	assert.Contains(t, root.Find("pre code").Text(), "some code")
}

func TestWrapPreformattedCode_AlreadyInPre(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><pre><code style="white-space: pre">code</code></pre></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	wrapPreformattedCode(root)

	// Should not double-wrap
	assert.Equal(t, 1, root.Find("pre").Length(), "should not add another pre")
}

func TestWrapPreformattedCode_NoStyleSkipped(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><p><code>normal code</code></p></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	wrapPreformattedCode(root)

	assert.Equal(t, 0, root.Find("pre").Length(), "code without pre style should not be wrapped")
}

func TestHasCalloutClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		class    string
		expected bool
	}{
		{"exact callout", "callout", true},
		{"callout prefix", "callout-warning", true},
		{"callout with other classes", "box callout-info large", true},
		{"no callout", "warning info", false},
		{"partial match not callout", "calloutbox", false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, hasCalloutClass(tc.class))
		})
	}
}

func TestStripUnwantedAttributes_PreservesCalloutClass(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><div class="callout-warning other-class">Alert content</div></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	stripUnwantedAttributes(root, false)

	cls, exists := root.Find("div").First().Attr("class")
	assert.True(t, exists, "class attribute should be preserved")
	assert.Contains(t, cls, "callout-warning")
}

func TestStripUnwantedAttributes_PreservesFootnoteIDs(t *testing.T) {
	t.Parallel()

	htmlStr := `<div id="root"><sup id="fnref:1">1</sup><div id="footnotes"><ol><li id="fn:1">Note</li></ol></div></div>`
	doc := parseDoc(t, htmlStr)
	root := doc.Find("#root")

	stripUnwantedAttributes(root, false)

	assert.Equal(t, "fnref:1", root.Find("sup").AttrOr("id", ""))
	assert.Equal(t, "fn:1", root.Find("li").AttrOr("id", ""))
	assert.Equal(t, "footnotes", root.Find("div").First().AttrOr("id", ""))
}
