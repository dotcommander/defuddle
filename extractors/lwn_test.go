package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LWN DOM fixtures.
const lwnArticleHTML = `<html>
<head>
  <meta property="og:description" content="A weekly technical newsletter.">
</head>
<body>
  <div class="PageHeadline"><h1>Kernel development news</h1></div>
  <div class="Byline">Posted January 15, 2024 by corbet</div>
  <div class="ArticleText">
    <main>
      <p>First paragraph of the article.</p>
      <p>Second paragraph with details.</p>
      <hr>
      <br clear="all">
      <details class="CommentBox">
        <summary>
          <div class="CommentPoster"><b>alice</b> <a href="/Articles/987654/">Posted January 16, 2024</a></div>
          <h3 class="CommentTitle">Great article</h3>
        </summary>
        <div class="FormattedComment"><p>Thanks for the write-up!</p></div>
      </details>
    </main>
  </div>
</body>
</html>`

const lwnNoSignalHTML = `<html>
<body><p>Just a plain page.</p></body>
</html>`

const lwnNoCommentsHTML = `<html>
<head>
  <meta property="og:description" content="Short article.">
</head>
<body>
  <div class="PageHeadline"><h1>Brief update</h1></div>
  <div class="Byline">Posted March 3, 2024 by jake</div>
  <div class="ArticleText">
    <main><p>Article body only.</p></main>
  </div>
</body>
</html>`

func parseLWNDoc(t *testing.T, rawHTML string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

func TestLWNExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		html    string
		wantCan bool
	}{
		{"full article page", lwnArticleHTML, true},
		{"missing both signals", lwnNoSignalHTML, false},
		{"no comments page", lwnNoCommentsHTML, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseLWNDoc(t, tc.html)
			ext := NewLWNExtractor(doc, "https://lwn.net/Articles/123456/", nil)
			assert.Equal(t, tc.wantCan, ext.CanExtract())
		})
	}
}

func TestLWNExtractor_Extract_Metadata(t *testing.T) {
	t.Parallel()

	doc := parseLWNDoc(t, lwnArticleHTML)
	ext := NewLWNExtractor(doc, "https://lwn.net/Articles/123456/", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "Kernel development news", result.Variables["title"])
	assert.Equal(t, "corbet", result.Variables["author"])
	assert.Equal(t, "LWN.net", result.Variables["site"])
	assert.Equal(t, "2024-01-15", result.Variables["published"])
	assert.Equal(t, "A weekly technical newsletter.", result.Variables["description"])
}

func TestLWNExtractor_Extract_ArticleContent(t *testing.T) {
	t.Parallel()

	doc := parseLWNDoc(t, lwnArticleHTML)
	ext := NewLWNExtractor(doc, "https://lwn.net/Articles/123456/", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	// Article paragraphs present.
	assert.Contains(t, result.ContentHTML, "First paragraph of the article")
	assert.Contains(t, result.ContentHTML, "Second paragraph with details")

	// Trailing hr/br stripped from article body.
	// Comment boxes stripped from article body.
	assert.NotContains(t, result.ContentHTML, "CommentBox")
}

func TestLWNExtractor_Extract_Comments(t *testing.T) {
	t.Parallel()

	doc := parseLWNDoc(t, lwnArticleHTML)
	ext := NewLWNExtractor(doc, "https://lwn.net/Articles/123456/", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	// Comment content rendered.
	assert.Contains(t, result.ContentHTML, "alice")
	assert.Contains(t, result.ContentHTML, "Thanks for the write-up")
}

func TestLWNExtractor_Extract_NoComments(t *testing.T) {
	t.Parallel()

	doc := parseLWNDoc(t, lwnNoCommentsHTML)
	ext := NewLWNExtractor(doc, "https://lwn.net/Articles/789/", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "Brief update", result.Variables["title"])
	assert.Equal(t, "jake", result.Variables["author"])
	assert.Contains(t, result.ContentHTML, "Article body only")
	// No comments div when there are no comments.
	assert.NotContains(t, result.ContentHTML, `class="comments"`)
}

func TestLWNExtractor_ParseDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"full byline", "Posted January 15, 2024 by corbet", "2024-01-15"},
		{"march date", "Posted March 3, 2024", "2024-03-03"},
		{"december date", "Posted December 31, 2023", "2023-12-31"},
		{"no date", "some text without a date", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseLWNDoc(t, lwnArticleHTML)
			ext := NewLWNExtractor(doc, "", nil)
			assert.Equal(t, tc.want, ext.parseDate(tc.input))
		})
	}
}

func TestLWNExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := parseLWNDoc(t, lwnArticleHTML)
	ext := NewLWNExtractor(doc, "", nil)
	assert.Equal(t, "LWNExtractor", ext.Name())
}
