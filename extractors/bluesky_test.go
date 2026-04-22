package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blueskyThreadHTML is a minimal Bluesky thread page: two posts by user1 (thread)
// and one reply by user2.
const blueskyThreadHTML = `<html>
<head>
  <meta name="twitter:value1" content="2024-03-15T10:00:00Z">
</head>
<body>
  <div data-testid="postThreadScreen">
    <div data-testid="postThreadItem-by-user1.bsky.social">
      <div><!-- no connector: top of thread --></div>
      <a aria-label="Alice's avatar" href="/profile/user1.bsky.social"></a>
      <div data-word-wrap="1">Hello from Bluesky!</div>
      <a href="/profile/user1.bsky.social/post/abc123" aria-label="March 15, 2024 at 10:00 AM">March 15</a>
    </div>
    <div data-testid="postThreadItem-by-user1.bsky.social">
      <div><!-- no connector --></div>
      <a aria-label="Alice's avatar" href="/profile/user1.bsky.social"></a>
      <div data-word-wrap="1">Second post in thread.</div>
    </div>
    <div data-testid="postThreadItem-by-user2.bsky.social">
      <div><!-- no connector --></div>
      <a aria-label="Bob's avatar" href="/profile/user2.bsky.social"></a>
      <div data-word-wrap="1">A reply from Bob.</div>
      <a href="/profile/user2.bsky.social/post/def456" aria-label="March 15, 2024 at 10:05 AM">March 15</a>
    </div>
  </div>
</body>
</html>`

// blueskyConnectorHTML has a reply with a thread connector div (width: 2px + background-color).
const blueskyConnectorHTML = `<html><body>
  <div data-testid="postThreadScreen">
    <div data-testid="postThreadItem-by-user1.bsky.social">
      <div></div>
      <div data-word-wrap="1">Root post.</div>
    </div>
    <div data-testid="postThreadItem-by-user2.bsky.social">
      <div>
        <div style="width: 2px; background-color: rgb(100,100,100);"></div>
      </div>
      <div data-word-wrap="1">Connected reply.</div>
    </div>
    <div data-testid="postThreadItem-by-user3.bsky.social">
      <div></div>
      <div data-word-wrap="1">Disconnected reply.</div>
    </div>
  </div>
</body></html>`

// blueskyImageHTML has a feed_thumbnail image that should be upgraded to feed_fullsize.
const blueskyImageHTML = `<html><body>
  <div data-testid="postThreadScreen">
    <div data-testid="postThreadItem-by-user1.bsky.social">
      <div></div>
      <div data-word-wrap="1">Post with image.</div>
      <img src="https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:abc/img@jpeg" alt="photo" />
    </div>
  </div>
</body></html>`

// blueskyBidiHTML has bidi markers in the post text that should be stripped.
const blueskyBidiHTML = "<html><body>\n" +
	`  <div data-testid="postThreadScreen">` + "\n" +
	`    <div data-testid="postThreadItem-by-user1.bsky.social">` + "\n" +
	"      <div></div>\n" +
	"      <div data-word-wrap=\"1\">Hello ‎@friend‏ world.</div>\n" +
	"    </div>\n" +
	"  </div>\n" +
	"</body></html>"

// blueskyNoThreadHTML has no postThreadScreen — CanExtract must return false.
const blueskyNoThreadHTML = `<html><body><p>Nothing here.</p></body></html>`

func parseBlueskyDoc(t *testing.T, rawHTML string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

func TestBlueskyExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		html    string
		wantCan bool
	}{
		{"thread page", blueskyThreadHTML, true},
		{"no thread screen", blueskyNoThreadHTML, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseBlueskyDoc(t, tc.html)
			ext := NewBlueskyExtractor(doc, "https://bsky.app/profile/user1.bsky.social/post/abc", nil)
			assert.Equal(t, tc.wantCan, ext.CanExtract())
		})
	}
}

func TestBlueskyExtractor_Extract_ThreadAndReply(t *testing.T) {
	t.Parallel()

	doc := parseBlueskyDoc(t, blueskyThreadHTML)
	ext := NewBlueskyExtractor(doc, "https://bsky.app/profile/user1.bsky.social/post/abc", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "Bluesky", result.Variables["site"])
	assert.Contains(t, result.Variables["author"], "Alice")
	assert.Contains(t, result.Variables["title"], "Bluesky")
	assert.Equal(t, "2024-03-15", result.Variables["published"])
	assert.Contains(t, result.ContentHTML, "Hello from Bluesky!")
	assert.Contains(t, result.ContentHTML, "Second post in thread.")
	// Reply should appear in the comments section.
	assert.Contains(t, result.ContentHTML, "A reply from Bob.")
	assert.Contains(t, result.ContentHTML, `class="extractor-content extractor-bluesky"`)
	// Thread posts are joined with <hr>.
	assert.Contains(t, result.ContentHTML, "<hr>")
}

func TestBlueskyExtractor_ThreadReplySplit(t *testing.T) {
	t.Parallel()

	doc := parseBlueskyDoc(t, blueskyThreadHTML)
	ext := NewBlueskyExtractor(doc, "https://bsky.app/profile/user1.bsky.social/post/abc", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	// Both thread posts appear before the comments section.
	postSection := result.ContentHTML
	commentsIdx := strings.Index(postSection, `class="comments"`)
	require.Greater(t, commentsIdx, 0, "expected a comments section")

	beforeComments := postSection[:commentsIdx]
	assert.Contains(t, beforeComments, "Hello from Bluesky!")
	assert.Contains(t, beforeComments, "Second post in thread.")

	afterComments := postSection[commentsIdx:]
	assert.Contains(t, afterComments, "A reply from Bob.")
	// The main author's posts must NOT appear in the reply section.
	assert.NotContains(t, afterComments, "Hello from Bluesky!")
}

func TestBlueskyExtractor_HasTopConnector(t *testing.T) {
	t.Parallel()

	doc := parseBlueskyDoc(t, blueskyConnectorHTML)
	ext := NewBlueskyExtractor(doc, "https://bsky.app/profile/user1.bsky.social/post/abc", nil)
	require.True(t, ext.CanExtract())
	require.Len(t, ext.postItems, 3)

	// First item (root): no connector.
	assert.False(t, hasTopConnector(ext.postItems[0]), "root post must not have connector")
	// Second item: has width:2px + background-color connector.
	assert.True(t, hasTopConnector(ext.postItems[1]), "reply with connector div should return true")
	// Third item: no connector.
	assert.False(t, hasTopConnector(ext.postItems[2]), "reply without connector div should return false")
}

func TestBlueskyExtractor_ImageUpgrade(t *testing.T) {
	t.Parallel()

	doc := parseBlueskyDoc(t, blueskyImageHTML)
	ext := NewBlueskyExtractor(doc, "https://bsky.app/profile/user1.bsky.social/post/abc", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.NotContains(t, result.ContentHTML, "/feed_thumbnail/", "thumbnail URL should be upgraded")
	assert.Contains(t, result.ContentHTML, "/feed_fullsize/", "fullsize URL should appear in output")
}

func TestBlueskyExtractor_BidiStrip(t *testing.T) {
	t.Parallel()

	doc := parseBlueskyDoc(t, blueskyBidiHTML)
	ext := NewBlueskyExtractor(doc, "https://bsky.app/profile/user1.bsky.social/post/abc", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	// Bidi markers U+200E and U+200F must not appear in any output.
	assert.NotContains(t, result.ContentHTML, "‎")
	assert.NotContains(t, result.ContentHTML, "‏")
	assert.NotContains(t, result.ContentHTML, "​")
}

func TestBlueskyExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := parseBlueskyDoc(t, blueskyThreadHTML)
	ext := NewBlueskyExtractor(doc, "", nil)
	assert.Equal(t, "BlueskyExtractor", ext.Name())
}

func TestBlueskyExtractor_RegistryEntry(t *testing.T) {
	t.Parallel()

	doc := parseBlueskyDoc(t, blueskyThreadHTML)
	registry := NewRegistry()
	registry.initializeBuiltins()

	ext := registry.FindExtractor(doc, "https://bsky.app/profile/user1.bsky.social/post/abc", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "BlueskyExtractor", ext.Name())
}
