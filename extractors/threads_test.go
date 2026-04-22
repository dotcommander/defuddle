package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// threadsPageletHTML is a minimal Threads pagelet-path page: two posts by alice
// (forming the thread) and one reply pagelet by bob.
const threadsPageletHTML = `<html>
<head><title>@alice on Threads</title></head>
<body>
  <div data-pagelet="threads_post_page_1">
    <div data-pressable-container="true">
      <a href="/@alice" role="link">alice</a>
      <time datetime="2024-05-01T12:00:00Z">May 1</time>
      <a href="/alice/post/abc123" href-after></a>
      <span dir="auto">First post in the thread.</span>
    </div>
  </div>
  <div data-pagelet="threads_post_page_2">
    <div data-pressable-container="true">
      <a href="/@alice" role="link">alice</a>
      <time datetime="2024-05-01T12:01:00Z">May 1</time>
      <span dir="auto">Second post in the thread.</span>
    </div>
  </div>
  <div data-pagelet="threads_post_page_3">
    <div data-pressable-container="true">
      <a href="/@bob" role="link">bob</a>
      <time datetime="2024-05-01T12:05:00Z">May 1</time>
      <span dir="auto">A reply from bob.</span>
    </div>
  </div>
</body>
</html>`

// threadsRegionHTML is a minimal Threads server-rendered region-path page.
const threadsRegionHTML = `<html>
<head><title>@carol on Threads</title></head>
<body>
  <div role="region">
    <a href="/@carol">carol</a>
    <time datetime="2024-06-15T09:30:00Z">Jun 15</time>
    <span dir="auto">Hello from the region path.</span>
  </div>
</body>
</html>`

// threadsNoSignalHTML has no Threads DOM signals — CanExtract must return false.
const threadsNoSignalHTML = `<html>
<head><title>Generic page</title></head>
<body><p>Nothing Threads here.</p></body>
</html>`

// threadsQuotedPostHTML has a server-rendered quoted post via /post/ links.
const threadsQuotedPostHTML = `<html><body>
  <div role="region">
    <a href="/@dave">dave</a>
    <span dir="auto">Check this out.</span>
    <a href="/@eve/post/xyz789">Original post text from eve</a>
  </div>
</body></html>`

func parseThreadsDoc(t *testing.T, rawHTML string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

func TestThreadsExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		html    string
		wantCan bool
	}{
		{"pagelet path", threadsPageletHTML, true},
		{"region path", threadsRegionHTML, true},
		{"no signals", threadsNoSignalHTML, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseThreadsDoc(t, tc.html)
			ext := NewThreadsExtractor(doc, "https://www.threads.com/@alice/post/abc", nil)
			assert.Equal(t, tc.wantCan, ext.CanExtract())
		})
	}
}

func TestThreadsExtractor_Extract_PageletPath(t *testing.T) {
	t.Parallel()

	doc := parseThreadsDoc(t, threadsPageletHTML)
	ext := NewThreadsExtractor(doc, "https://www.threads.com/@alice/post/abc", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "@alice on Threads", result.Variables["title"])
	assert.Equal(t, "@alice", result.Variables["author"])
	assert.Equal(t, "Threads", result.Variables["site"])
	assert.Equal(t, "2024-05-01", result.Variables["published"])
	assert.Contains(t, result.ContentHTML, "First post in the thread.")
	assert.Contains(t, result.ContentHTML, "Second post in the thread.")
	// Reply by bob should appear in the comments section
	assert.Contains(t, result.ContentHTML, "bob")
	assert.Equal(t, "alice", result.ExtractedContent["postAuthor"])
}

func TestThreadsExtractor_Extract_RegionPath(t *testing.T) {
	t.Parallel()

	doc := parseThreadsDoc(t, threadsRegionHTML)
	ext := NewThreadsExtractor(doc, "https://www.threads.com/@carol/post/def", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "@carol on Threads", result.Variables["title"])
	assert.Equal(t, "@carol", result.Variables["author"])
	assert.Equal(t, "2024-06-15", result.Variables["published"])
	assert.Contains(t, result.ContentHTML, "Hello from the region path.")
	assert.Equal(t, "carol", result.ExtractedContent["postAuthor"])
}

func TestThreadsExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := parseThreadsDoc(t, threadsPageletHTML)
	ext := NewThreadsExtractor(doc, "", nil)
	assert.Equal(t, "ThreadsExtractor", ext.Name())
}

func TestThreadsExtractor_RegistryEntry(t *testing.T) {
	t.Parallel()

	doc := parseThreadsDoc(t, threadsPageletHTML)
	registry := NewRegistry()
	registry.initializeBuiltins()
	ext := registry.FindExtractor(doc, "https://www.threads.com/@alice/post/abc", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "ThreadsExtractor", ext.Name())
}

func TestThreadsExtractor_RegistryEntry_ThreadsNet(t *testing.T) {
	t.Parallel()

	doc := parseThreadsDoc(t, threadsRegionHTML)
	registry := NewRegistry()
	registry.initializeBuiltins()
	ext := registry.FindExtractor(doc, "https://www.threads.net/@carol/post/def", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "ThreadsExtractor", ext.Name())
}

func TestThreadsDedupeKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"short text", "hello world", "hello world"},
		{"extra whitespace", "  hello   world  ", "hello world"},
		{"truncates at 80 runes", strings.Repeat("a", 100), strings.Repeat("a", 80)},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, threadsDedupeKey(tc.input))
		})
	}
}

func TestFindPostsInJSON(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"thread_items": []any{
			map[string]any{
				"post": map[string]any{
					"user":    map[string]any{"username": "alice"},
					"caption": map[string]any{"text": "Hello from alice"},
				},
			},
			map[string]any{
				"post": map[string]any{
					"user":    map[string]any{"username": "bob"},
					"caption": map[string]any{"text": "Reply from bob"},
				},
			},
		},
	}

	posts := findPostsInJSON(data, 0)
	require.Len(t, posts, 2)
	assert.Equal(t, "alice", posts[0].username)
	assert.Equal(t, "Hello from alice", posts[0].text)
	assert.Equal(t, "bob", posts[1].username)
	assert.Equal(t, "Reply from bob", posts[1].text)
}

func TestFindPostsInJSON_DepthLimit(t *testing.T) {
	t.Parallel()

	// Calling at depth == limit should return nil immediately.
	result := findPostsInJSON(map[string]any{"user": map[string]any{"username": "x"}}, threadsJSONDepthLimit)
	assert.Nil(t, result)
}

func TestThreadsExtractor_GetDate(t *testing.T) {
	t.Parallel()

	doc := parseThreadsDoc(t, `<html><body>
		<div><time datetime="2024-03-10T15:04:05Z">Mar 10</time></div>
	</body></html>`)
	ext := NewThreadsExtractor(doc, "", nil)
	container := doc.Find("div").First()
	assert.Equal(t, "2024-03-10", ext.getDate(container))
}

func TestThreadsExtractor_GetPermalink(t *testing.T) {
	t.Parallel()

	doc := parseThreadsDoc(t, `<html><body>
		<div><a href="/alice/post/abc123">link</a></div>
	</body></html>`)
	ext := NewThreadsExtractor(doc, "", nil)
	container := doc.Find("div").First()
	assert.Equal(t, "https://www.threads.com/alice/post/abc123", ext.getPermalink(container))
}
