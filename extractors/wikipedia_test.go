package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wikipedia DOM fixtures.
const wikipediaArticleHTML = `<html>
<head>
  <meta property="og:title" content="Go (programming language) - Wikipedia">
  <meta property="og:description" content="Go is a statically typed language.">
</head>
<body>
  <div id="mw-content-text">
    <div class="mw-parser-output">
      <p>Go is a statically typed, compiled programming language.</p>
      <h2>History <span class="mw-editsection">[<a href="/w/index.php?action=edit">edit</a>]</span></h2>
      <p>Go was designed at Google.</p>
      <div class="navbox">Navigation box content</div>
      <sup class="reference">[1]</sup>
    </div>
  </div>
</body>
</html>`

const wikipediaNoContentTextHTML = `<html>
<head><title>Generic page</title></head>
<body><p>No MediaWiki structure here.</p></body>
</html>`

const wikipediaNoSuffixTitleHTML = `<html>
<head>
  <meta property="og:title" content="Go (programming language)">
</head>
<body>
  <div id="mw-content-text">
    <div class="mw-parser-output"><p>Content here.</p></div>
  </div>
</body>
</html>`

const wikipediaEmptySuffixAfterStripHTML = `<html>
<head>
  <meta property="og:title" content="Wikipedia">
</head>
<body>
  <div id="mw-content-text">
    <div class="mw-parser-output"><p>Content here.</p></div>
  </div>
</body>
</html>`

func parseWikipediaDoc(t *testing.T, rawHTML string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

func TestWikipediaExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		html    string
		wantCan bool
	}{
		{"has #mw-content-text", wikipediaArticleHTML, true},
		{"no MediaWiki structure", wikipediaNoContentTextHTML, false},
		{"no suffix title still extracts", wikipediaNoSuffixTitleHTML, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseWikipediaDoc(t, tc.html)
			ext := NewWikipediaExtractor(doc, "https://en.wikipedia.org/wiki/Go_(programming_language)", nil)
			assert.Equal(t, tc.wantCan, ext.CanExtract())
		})
	}
}

func TestWikipediaExtractor_Extract_TitleStripping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		html      string
		wantTitle string
	}{
		{
			name:      "strips dash-Wikipedia suffix",
			html:      wikipediaArticleHTML,
			wantTitle: "Go (programming language)",
		},
		{
			name:      "title without Wikipedia suffix unchanged",
			html:      wikipediaNoSuffixTitleHTML,
			wantTitle: "Go (programming language)",
		},
		{
			name:      "bare 'Wikipedia' og:title falls back to raw",
			html:      wikipediaEmptySuffixAfterStripHTML,
			wantTitle: "Wikipedia",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseWikipediaDoc(t, tc.html)
			ext := NewWikipediaExtractor(doc, "https://en.wikipedia.org/wiki/Go", nil)
			require.True(t, ext.CanExtract())

			result := ext.Extract()
			require.NotNil(t, result)
			assert.Equal(t, tc.wantTitle, result.Variables["title"])
		})
	}
}

func TestWikipediaExtractor_Extract_Metadata(t *testing.T) {
	t.Parallel()

	doc := parseWikipediaDoc(t, wikipediaArticleHTML)
	ext := NewWikipediaExtractor(doc, "https://en.wikipedia.org/wiki/Go_(programming_language)", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "Wikipedia", result.Variables["author"])
	assert.Equal(t, "Wikipedia", result.Variables["site"])
}

func TestWikipediaExtractor_Extract_ContentStripping(t *testing.T) {
	t.Parallel()

	doc := parseWikipediaDoc(t, wikipediaArticleHTML)
	ext := NewWikipediaExtractor(doc, "https://en.wikipedia.org/wiki/Go_(programming_language)", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	// Article content present.
	assert.Contains(t, result.ContentHTML, "Go is a statically typed")

	// Edit links stripped.
	assert.NotContains(t, result.ContentHTML, "mw-editsection")
	assert.NotContains(t, result.ContentHTML, "[edit]")

	// Navboxes stripped.
	assert.NotContains(t, result.ContentHTML, "navbox")
	assert.NotContains(t, result.ContentHTML, "Navigation box content")

	// Reference superscripts stripped.
	assert.NotContains(t, result.ContentHTML, `class="reference"`)
}

func TestWikipediaExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := parseWikipediaDoc(t, wikipediaArticleHTML)
	ext := NewWikipediaExtractor(doc, "", nil)
	assert.Equal(t, "WikipediaExtractor", ext.Name())
}

func TestWikipediaExtractor_HostRegex(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://en.wikipedia.org/wiki/Go",
		"https://de.wikipedia.org/wiki/Go",
		"https://zh.wikipedia.org/wiki/Go",
		"https://simple.wikipedia.org/wiki/Go",
	}
	for _, u := range urls {
		t.Run(u, func(t *testing.T) {
			t.Parallel()
			assert.True(t, wikipediaHostRe.MatchString(u), "expected host regex to match %s", u)
		})
	}
}
