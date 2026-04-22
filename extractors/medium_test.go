package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Medium DOM fixtures.
const mediumArticleHTML = `<html>
<head>
  <meta property="og:site_name" content="Medium">
  <meta property="al:android:app_name" content="Medium">
  <meta property="og:title" content="How Go Changed My Life">
</head>
<body>
  <article>
    <h1 data-testid="storyTitle">How Go Changed My Life</h1>
    <p class="pw-subtitle-paragraph">A personal journey into systems programming.</p>
    <span data-testid="authorName">Jane Doe</span>
    <p>Go is a wonderful language with clean concurrency primitives.</p>
    <p>Member-only story</p>
    <p>5 min read</p>
    <div class="navbox">Navigation</div>
  </article>
</body>
</html>`

const mediumMeteredHTML = `<html>
<head>
  <meta property="og:site_name" content="Towards Data Science">
</head>
<body>
  <article class="meteredContent">
    <h1>Understanding Transformers</h1>
    <p>Transformers revolutionized NLP research.</p>
  </article>
</body>
</html>`

const mediumPublicationHTML = `<html>
<head>
  <meta property="og:site_name" content="Better Programming">
</head>
<body>
  <article>
    <h1 data-testid="storyTitle">Writing Better Go</h1>
    <p>Clean code matters.</p>
  </article>
</body>
</html>`

const mediumNoArticleHTML = `<html>
<head><meta property="og:site_name" content="Medium"></head>
<body><div>No article element here.</div></body>
</html>`

const mediumNonMediumHTML = `<html>
<head><meta property="og:site_name" content="Example Blog"></head>
<body>
  <article><p>This is not Medium.</p></article>
</body>
</html>`

func parseMediumDoc(t *testing.T, rawHTML string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

func TestMediumExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		html    string
		wantCan bool
	}{
		{"og:site_name Medium", mediumArticleHTML, true},
		{"metered class", mediumMeteredHTML, true},
		{"named publication", mediumPublicationHTML, false},
		{"no article element", mediumNoArticleHTML, false},
		{"non-Medium site", mediumNonMediumHTML, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseMediumDoc(t, tc.html)
			ext := NewMediumExtractor(doc, "https://medium.com/p/abc123", nil)
			assert.Equal(t, tc.wantCan, ext.CanExtract())
		})
	}
}

func TestMediumExtractor_CanExtract_MeteredAlwaysTrue(t *testing.T) {
	t.Parallel()
	doc := parseMediumDoc(t, mediumMeteredHTML)
	ext := NewMediumExtractor(doc, "https://towardsdatascience.com/p/abc", nil)
	// meteredContent class gates on article class alone — true even without og:site_name=Medium.
	assert.True(t, ext.CanExtract())
}

func TestMediumExtractor_Extract_Metadata(t *testing.T) {
	t.Parallel()

	doc := parseMediumDoc(t, mediumArticleHTML)
	ext := NewMediumExtractor(doc, "https://medium.com/p/abc123", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "How Go Changed My Life", result.Variables["title"])
	assert.Equal(t, "Jane Doe", result.Variables["author"])
	assert.Equal(t, "Medium", result.Variables["site"])
}

func TestMediumExtractor_Extract_SubtitleAsDescription(t *testing.T) {
	t.Parallel()

	doc := parseMediumDoc(t, mediumArticleHTML)
	ext := NewMediumExtractor(doc, "https://medium.com/p/abc123", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "A personal journey into systems programming.", result.Variables["description"])
}

func TestMediumExtractor_Extract_Publication(t *testing.T) {
	t.Parallel()

	// Publication name comes from og:site_name when it's not "Medium".
	const pubHTML = `<html>
<head>
  <meta property="og:site_name" content="Towards Data Science">
  <meta property="al:android:app_name" content="Medium">
</head>
<body>
  <article class="meteredContent">
    <h1 data-testid="storyTitle">My Article</h1>
    <p>Some content here for testing purposes.</p>
  </article>
</body>
</html>`

	doc := parseMediumDoc(t, pubHTML)
	ext := NewMediumExtractor(doc, "https://towardsdatascience.com/p/abc", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "Towards Data Science", result.Variables["site"])
	assert.Equal(t, "Towards Data Science", result.ExtractedContent["publication"])
}

func TestMediumExtractor_Extract_UINoiseStripped(t *testing.T) {
	t.Parallel()

	doc := parseMediumDoc(t, mediumArticleHTML)
	ext := NewMediumExtractor(doc, "https://medium.com/p/abc123", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	// UI noise should be absent.
	assert.NotContains(t, result.ContentHTML, "Member-only story")
	assert.NotContains(t, result.ContentHTML, "min read")

	// Real content preserved.
	assert.Contains(t, result.ContentHTML, "Go is a wonderful language")
}

func TestMediumExtractor_Extract_ContentPresent(t *testing.T) {
	t.Parallel()

	doc := parseMediumDoc(t, mediumMeteredHTML)
	ext := NewMediumExtractor(doc, "https://towardsdatascience.com/p/abc", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "Transformers revolutionized")
	assert.NotEmpty(t, result.Content)
}

func TestMediumExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := parseMediumDoc(t, mediumArticleHTML)
	ext := NewMediumExtractor(doc, "", nil)
	assert.Equal(t, "MediumExtractor", ext.Name())
}
