package scoring

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseHTML(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	return doc
}

func TestScoreElement_ContentRichElement(t *testing.T) {
	t.Parallel()
	html := `<div class="article-content">
		<p>This is a paragraph with some meaningful content about the topic.</p>
		<p>Another paragraph with additional details and information.</p>
		<p>A third paragraph to add more text density to this element.</p>
	</div>`
	doc := parseHTML(t, html)
	el := doc.Find(".article-content")

	score := ScoreElement(el)
	assert.Greater(t, score, 0.0, "content-rich element should have positive score")
}

func TestScoreElement_NavigationElement(t *testing.T) {
	t.Parallel()
	html := `<nav>
		<a href="/home">Home</a>
		<a href="/about">About</a>
		<a href="/contact">Contact</a>
	</nav>`
	doc := parseHTML(t, html)
	el := doc.Find("nav")

	score := ScoreElement(el)

	// Navigation has very few words relative to links — high link density penalty
	contentHTML := `<article class="content">
		<p>This is a long paragraph with lots of meaningful content that should score highly because it has many words and few links.</p>
		<p>Another paragraph with even more content to increase the word count significantly above what a nav element would have.</p>
	</article>`
	contentDoc := parseHTML(t, contentHTML)
	contentScore := ScoreElement(contentDoc.Find("article"))

	assert.Greater(t, contentScore, score, "content element should score higher than navigation")
}

func TestScoreElement_MoreTextScoresHigher(t *testing.T) {
	t.Parallel()
	shortHTML := `<div><p>Short text.</p></div>`
	longHTML := `<div>
		<p>This is a much longer piece of text that contains many more words and should therefore receive a higher score from the scoring algorithm.</p>
		<p>Adding another paragraph increases the word count even further.</p>
	</div>`

	shortDoc := parseHTML(t, shortHTML)
	longDoc := parseHTML(t, longHTML)

	shortScore := ScoreElement(shortDoc.Find("div"))
	longScore := ScoreElement(longDoc.Find("div"))

	assert.Greater(t, longScore, shortScore, "more text should score higher")
}

func TestScoreElement_LinkDensityPenalty(t *testing.T) {
	t.Parallel()
	// Many links relative to text
	linksHTML := `<div>
		<a href="/1">Link</a> <a href="/2">Link</a> <a href="/3">Link</a>
		<a href="/4">Link</a> <a href="/5">Link</a> <a href="/6">Link</a>
	</div>`
	// Same amount of text, no links
	textHTML := `<div><p>Link Link Link Link Link Link</p></div>`

	linksDoc := parseHTML(t, linksHTML)
	textDoc := parseHTML(t, textHTML)

	linksScore := ScoreElement(linksDoc.Find("div"))
	textScore := ScoreElement(textDoc.Find("div"))

	assert.Greater(t, textScore, linksScore, "high link density should penalize score")
}

func TestScoreElement_ContentClassBoost(t *testing.T) {
	t.Parallel()
	withClass := `<div class="article-content"><p>Some text here for scoring.</p></div>`
	withoutClass := `<div class="sidebar"><p>Some text here for scoring.</p></div>`

	withDoc := parseHTML(t, withClass)
	withoutDoc := parseHTML(t, withoutClass)

	withScore := ScoreElement(withDoc.Find("div"))
	withoutScore := ScoreElement(withoutDoc.Find("div"))

	assert.Greater(t, withScore, withoutScore, "content/article class should boost score")
}

func TestScoreElement_ParagraphBoost(t *testing.T) {
	t.Parallel()
	withP := `<div><p>Paragraph one.</p><p>Paragraph two.</p><p>Paragraph three.</p></div>`
	withoutP := `<div>Paragraph one. Paragraph two. Paragraph three.</div>`

	withDoc := parseHTML(t, withP)
	withoutDoc := parseHTML(t, withoutP)

	withScore := ScoreElement(withDoc.Find("div"))
	withoutScore := ScoreElement(withoutDoc.Find("div"))

	assert.Greater(t, withScore, withoutScore, "paragraphs should boost score")
}

func TestFindBestElement_AboveMinScore(t *testing.T) {
	t.Parallel()
	html := `<div>
		<section class="content"><p>Rich content with many words to ensure a high score from the algorithm.</p><p>More content here.</p></section>
		<aside><a href="/">nav</a></aside>
	</div>`
	doc := parseHTML(t, html)

	elements := []*goquery.Selection{
		doc.Find("section"),
		doc.Find("aside"),
	}

	best := FindBestElement(elements, 0)
	require.NotNil(t, best)
	assert.Equal(t, "section", goquery.NodeName(best))
}

func TestFindBestElement_NoneAboveMinScore(t *testing.T) {
	t.Parallel()
	html := `<div><span>tiny</span></div>`
	doc := parseHTML(t, html)

	elements := []*goquery.Selection{doc.Find("span")}

	best := FindBestElement(elements, 9999)
	assert.Nil(t, best)
}

func TestFindBestElement_EmptySlice(t *testing.T) {
	t.Parallel()
	best := FindBestElement(nil, 0)
	assert.Nil(t, best)
}
