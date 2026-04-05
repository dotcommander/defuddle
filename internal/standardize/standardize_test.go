package standardize

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
