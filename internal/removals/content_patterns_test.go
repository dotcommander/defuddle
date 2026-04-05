package removals

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseMain parses html, wraps the body in a goquery.Selection, and
// returns (mainContent, document) ready for RemoveByContentPattern.
func parseMain(t *testing.T, html string) (*goquery.Selection, *goquery.Document) {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	body := doc.Find("body")
	require.Greater(t, body.Length(), 0)
	return body, doc
}

// text returns the trimmed text of a selection, collapsing whitespace.
func text(sel *goquery.Selection) string {
	return strings.Join(strings.Fields(sel.Text()), " ")
}

// ---- hero header removal ----

func TestRemoveHeroHeader(t *testing.T) {
	t.Parallel()

	html := `<body>
<div id="header">
  <h1>My Article Title</h1>
  <time datetime="2024-01-01">January 1, 2024</time>
</div>
<p>This is the real article body with enough words to matter for scoring purposes.</p>
<p>Second paragraph with more content about the topic at hand.</p>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/blog/post")

	// hero header div should be gone
	assert.Equal(t, 0, main.Find("#header").Length(), "hero header div should be removed")
	// article body should be preserved
	assert.Contains(t, text(main), "real article body", "article body must survive")
}

// ---- breadcrumb list removal ----

func TestRemoveBreadcrumbList(t *testing.T) {
	t.Parallel()

	html := `<body>
<ul>
  <li><a href="/">Home</a></li>
  <li><a href="/blog/">Blog</a></li>
  <li>Current Post</li>
</ul>
<h1>Article Title</h1>
<p>Article content paragraph with substantial text that should be preserved.</p>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/blog/current-post")

	assert.Equal(t, 0, main.Find("ul").Length(), "breadcrumb ul should be removed")
	assert.Contains(t, text(main), "Article Title", "heading must survive")
	assert.Contains(t, text(main), "Article content", "paragraph must survive")
}

func TestBreadcrumbListPreservesContentList(t *testing.T) {
	t.Parallel()

	// A real content list — more than 8 items, or external links, should NOT be removed.
	html := `<body>
<h1>Article</h1>
<ul>
  <li>Feature one</li>
  <li>Feature two</li>
  <li>Feature three</li>
  <li>Feature four</li>
  <li>Feature five</li>
  <li>Feature six</li>
  <li>Feature seven</li>
  <li>Feature eight</li>
  <li>Feature nine</li>
</ul>
<p>Paragraph content that matters here.</p>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/")

	assert.Equal(t, 1, main.Find("ul").Length(), "content ul must NOT be removed")
}

// ---- byline removal ----

func TestRemoveByline(t *testing.T) {
	t.Parallel()

	html := `<body>
<h1>Article Title</h1>
<p>By Jane Smith</p>
<p>This is the first real paragraph of the article with enough words to qualify as prose content.</p>
<p>Second paragraph with additional substantive content about the subject.</p>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/article")

	assert.NotContains(t, text(main), "By Jane Smith", "byline must be removed")
	assert.Contains(t, text(main), "first real paragraph", "body must survive")
}

// ---- boilerplate removal ----

func TestRemoveBoilerplate(t *testing.T) {
	t.Parallel()

	html := `<body>
<h1>Article Title</h1>
<p>This is real content paragraph one with many interesting words about the topic.</p>
<p>This is real content paragraph two that continues the discussion further.</p>
<p>This is real content paragraph three with even more depth and analysis here.</p>
<p>This article appeared in The Daily Gazette.</p>
<p>Subscribe to our newsletter for more updates about this topic.</p>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/article")

	assert.NotContains(t, text(main), "This article appeared in", "boilerplate must be removed")
	assert.NotContains(t, text(main), "Subscribe to our newsletter", "post-boilerplate trailing content must also be gone")
	assert.Contains(t, text(main), "real content paragraph one", "real content must survive")
}

func TestRemoveBoilerplateCopyright(t *testing.T) {
	t.Parallel()

	html := `<body>
<h1>Title</h1>
<p>Long enough article body paragraph with real meaningful content that matters here.</p>
<p>Second paragraph with more substantive text about the important topic being covered.</p>
<p>Third paragraph that adds even more depth to the discussion being presented here.</p>
<p>© 2024 Publisher Name. All rights reserved.</p>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/article")

	assert.NotContains(t, text(main), "© 2024", "copyright boilerplate must be removed")
	assert.Contains(t, text(main), "Long enough article body", "real content must survive")
}

// ---- newsletter removal ----

func TestRemoveNewsletterSignup(t *testing.T) {
	t.Parallel()

	html := `<body>
<h1>Article</h1>
<p>Real content paragraph one with interesting information about the subject matter here.</p>
<p>Real content paragraph two continuing the discussion with additional insights provided.</p>
<div class="newsletter-box">
  Subscribe to our newsletter for weekly updates
</div>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/article")

	assert.Equal(t, 0, main.Find(".newsletter-box").Length(), "newsletter div must be removed")
	assert.Contains(t, text(main), "Real content paragraph one", "article body must survive")
}

// ---- related heading removal ----

func TestRemoveRelatedPostsHeading(t *testing.T) {
	t.Parallel()

	// Build a doc where "Related Posts" appears after substantial content
	// (>500 chars before its position) and is wrapped inside a div so
	// walkUpIsolated can walk up from the heading.
	html := `<body>
<h1>Main Article</h1>
<p>First paragraph with enough words that it contributes to the 500-char threshold check for the related posts heading removal logic that requires substantial preceding content.</p>
<p>Second paragraph adds more content to the body making the total character count exceed the five hundred character minimum required for the related posts removal heuristic to activate.</p>
<p>Third paragraph continues the story with additional details and information for readers ensuring that the position of the related posts heading is well past the threshold marker.</p>
<div class="related-section">
  <h3>Related Posts</h3>
  <ul><li><a href="/other">Other Post</a></li></ul>
</div>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/article")

	assert.Equal(t, 0, main.Find(".related-section").Length(), "related-section div must be removed")
	assert.Contains(t, text(main), "First paragraph", "article body must survive")
}

// ---- content preservation ----

func TestPreservesRealContent(t *testing.T) {
	t.Parallel()

	html := `<body>
<h1>Technical Deep Dive</h1>
<p>In this article we explore the nuances of distributed systems with careful analysis.</p>
<p>The CAP theorem states that a distributed data store cannot simultaneously provide
more than two of the following guarantees: consistency, availability, and partition tolerance.</p>
<pre><code>func main() {
    fmt.Println("hello, world")
}</code></pre>
<table>
  <tr><th>Column A</th><th>Column B</th></tr>
  <tr><td>Value 1</td><td>Value 2</td></tr>
</table>
<blockquote>This is a quote from an important source that must be preserved.</blockquote>
<p>Conclusion paragraph wrapping up the key points of this thorough technical article.</p>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/article")

	assert.Contains(t, text(main), "CAP theorem", "prose must survive")
	assert.Equal(t, 1, main.Find("pre").Length(), "code block must survive")
	assert.Equal(t, 1, main.Find("table").Length(), "table must survive")
	assert.Equal(t, 1, main.Find("blockquote").Length(), "blockquote must survive")
}

// ---- standalone time element removal ----

func TestRemoveStandaloneTimeElement(t *testing.T) {
	t.Parallel()

	html := `<body>
<p><time datetime="2024-03-01">March 1, 2024</time></p>
<h1>Article Title</h1>
<p>This is a proper content paragraph with enough words to be considered real article prose.</p>
<p>Another content paragraph that adds substance and meaning to the overall article text.</p>
</body>`

	main, doc := parseMain(t, html)
	RemoveByContentPattern(main, doc, false, "https://example.com/article")

	assert.Equal(t, 0, main.Find("time").Length(), "standalone time element must be removed")
	assert.Contains(t, text(main), "proper content paragraph", "article body must survive")
}

// ---- isBreadcrumbList unit tests ----

func TestIsBreadcrumbList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		html string
		want bool
	}{
		{
			name: "valid breadcrumb",
			html: `<ul><li><a href="/">Home</a></li><li><a href="/blog/">Blog</a></li><li>Post</li></ul>`,
			want: true,
		},
		{
			name: "too many items",
			html: `<ul>` + strings.Repeat(`<li><a href="/x/">X</a></li>`, 9) + `</ul>`,
			want: false,
		},
		{
			name: "external link disqualifies",
			html: `<ul><li><a href="https://other.com/">Home</a></li><li>Post</li></ul>`,
			want: false,
		},
		{
			name: "no root or shallow link",
			html: `<ul><li><a href="/very/deep/path/">X</a></li><li>Post</li></ul>`,
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc, err := goquery.NewDocumentFromReader(strings.NewReader("<body>" + tc.html + "</body>"))
			require.NoError(t, err)
			listNode := doc.Find("ul").Nodes[0]
			assert.Equal(t, tc.want, isBreadcrumbList(listNode))
		})
	}
}
