package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mastodonStatusHTML is a minimal Mastodon status page with detailed-status DOM.
const mastodonStatusHTML = `<html>
<head>
  <meta name="application-name" content="Mastodon">
  <meta property="og:title" content="Alice on Mastodon">
  <meta property="og:description" content="Hello from the fediverse!">
  <meta property="og:site_name" content="mastodon.social">
  <meta name="author" content="Alice">
  <meta property="article:published_time" content="2024-06-01T10:00:00Z">
  <title>Alice: Hello from the fediverse! - mastodon.social</title>
</head>
<body>
  <div class="detailed-status">
    <a class="detailed-status__display-name">
      <strong>Alice</strong>
    </a>
    <div class="status__content">
      <p>Hello from the fediverse!</p>
    </div>
    <a class="detailed-status__datetime" datetime="2024-06-01T10:00:00Z">Jun 1, 2024</a>
  </div>
</body>
</html>`

// mastodonWithRepliesHTML includes thread replies in the activity stream.
const mastodonWithRepliesHTML = `<html>
<head>
  <meta name="application-name" content="Mastodon">
  <meta property="og:title" content="Thread status">
  <meta name="author" content="Bob">
</head>
<body>
  <div class="detailed-status">
    <div class="status__content"><p>Original toot.</p></div>
  </div>
  <div class="activity-stream">
    <div class="entry">
      <div class="detailed-status">
        <div class="status__content"><p>Original toot.</p></div>
      </div>
    </div>
    <div class="entry">
      <span class="account__display-name"><strong>Carol</strong></span>
      <div class="status__content"><p>A reply to the toot.</p></div>
      <a class="status__relative-time" datetime="2024-06-01T10:05:00Z" href="https://mastodon.social/@carol/123">5m</a>
    </div>
    <div class="entry">
      <span class="account__display-name"><strong>Dave</strong></span>
      <div class="status__content"><p>Another reply.</p></div>
    </div>
  </div>
</body>
</html>`

// mastodonOgURLHTML has no app-name meta but carries a Mastodon-shaped og:url.
const mastodonOgURLHTML = `<html>
<head>
  <meta property="og:url" content="https://fosstodon.org/@user/109876543210">
  <meta property="og:title" content="Status from fosstodon">
  <meta name="author" content="User">
</head>
<body>
  <div class="detailed-status">
    <div class="status__content"><p>Fosstodon post.</p></div>
  </div>
</body>
</html>`

// mastodonNoSignalHTML has no Mastodon signals — CanExtract must return false.
const mastodonNoSignalHTML = `<html>
<head><title>Generic page</title></head>
<body><p>Nothing Mastodon here.</p></body>
</html>`

func parseMastodonDoc(t *testing.T, rawHTML string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

func TestMastodonExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		html    string
		wantCan bool
	}{
		{"app-name meta", mastodonStatusHTML, true},
		{"og:url status path", mastodonOgURLHTML, true},
		{"with replies", mastodonWithRepliesHTML, true},
		{"no mastodon signals", mastodonNoSignalHTML, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseMastodonDoc(t, tc.html)
			ext := NewMastodonExtractor(doc, "https://mastodon.social/@alice/1", nil)
			assert.Equal(t, tc.wantCan, ext.CanExtract())
		})
	}
}

func TestMastodonExtractor_Extract_BasicStatus(t *testing.T) {
	t.Parallel()

	doc := parseMastodonDoc(t, mastodonStatusHTML)
	ext := NewMastodonExtractor(doc, "https://mastodon.social/@alice/1", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "Alice on Mastodon", result.Variables["title"])
	assert.Equal(t, "Alice", result.Variables["author"])
	assert.Equal(t, "2024-06-01T10:00:00Z", result.Variables["published"])
	assert.Equal(t, "Mastodon", result.Variables["site"])
	assert.Contains(t, result.ContentHTML, "Hello from the fediverse!")
	assert.Contains(t, result.ContentHTML, `class="extractor-content extractor-mastodon"`)
}

func TestMastodonExtractor_Extract_WithReplies(t *testing.T) {
	t.Parallel()

	doc := parseMastodonDoc(t, mastodonWithRepliesHTML)
	ext := NewMastodonExtractor(doc, "https://mastodon.social/@bob/2", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.ContentHTML, "Original toot.")
	// Replies should appear in the comments section
	assert.Contains(t, result.ContentHTML, "Carol")
	assert.Contains(t, result.ContentHTML, "A reply to the toot.")
	assert.Contains(t, result.ContentHTML, "Dave")
	// Primary detailed-status should not be duplicated in the reply thread
	replySection := strings.SplitN(result.ContentHTML, `class="comments"`, 2)
	if len(replySection) == 2 {
		assert.NotContains(t, replySection[1], "Original toot.")
	}
}

func TestMastodonExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := parseMastodonDoc(t, mastodonStatusHTML)
	ext := NewMastodonExtractor(doc, "", nil)
	assert.Equal(t, "MastodonExtractor", ext.Name())
}

func TestMastodonExtractor_RegistryEntry(t *testing.T) {
	t.Parallel()

	doc := parseMastodonDoc(t, mastodonStatusHTML)
	// The catch-all regexp.MustCompile(".") matches any URL, but CanExtract
	// gates on DOM signals — so this should resolve to MastodonExtractor.
	registry := NewRegistry()
	registry.initializeBuiltins()
	ext := registry.FindExtractor(doc, "https://mastodon.social/@alice/1", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "MastodonExtractor", ext.Name())
}

func TestMastodonExtractor_RegistryEntry_NonMastodonPageReturnsNil(t *testing.T) {
	t.Parallel()

	doc := parseMastodonDoc(t, mastodonNoSignalHTML)
	registry := NewRegistry()
	registry.initializeBuiltins()
	// Generic page with no known domain patterns and no Mastodon DOM — nil expected.
	ext := registry.FindExtractor(doc, "https://example.com/some-page", nil)
	assert.Nil(t, ext)
}
