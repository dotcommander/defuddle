package extractors

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redditPostHTML is a realistic shreddit-post page with text body and an image.
const redditPostHTML = `<html>
<head>
	<title>Why is Go so fast? : r/golang</title>
</head>
<body>
	<h1>Why is Go so fast?</h1>
	<shreddit-post
		author="gopher_dev"
		score="1234"
		permalink="/r/golang/comments/abc123/why_is_go_so_fast/">
		<div slot="text-body">
			<p>Go compiles to native machine code and has a lightweight runtime.</p>
			<p>The garbage collector is designed for low latency.</p>
		</div>
	</shreddit-post>
	<shreddit-comment
		depth="0"
		author="rustacean42"
		score="567"
		permalink="/r/golang/comments/abc123/why_is_go_so_fast/comment1">
		<div slot="comment"><p>Goroutines are really cheap compared to OS threads.</p></div>
		<faceplate-timeago ts="2024-03-15T10:00:00Z"></faceplate-timeago>
	</shreddit-comment>
	<shreddit-comment
		depth="1"
		author="concurrent_dev"
		score="234"
		permalink="/r/golang/comments/abc123/why_is_go_so_fast/comment2">
		<div slot="comment"><p>Exactly, and the scheduler is work-stealing.</p></div>
		<faceplate-timeago ts="2024-03-15T10:05:00Z"></faceplate-timeago>
	</shreddit-comment>
	<shreddit-comment
		depth="0"
		author="systems_programmer"
		score="89"
		permalink="/r/golang/comments/abc123/why_is_go_so_fast/comment3">
		<div slot="comment"><p>Static linking also helps with cold start.</p></div>
		<faceplate-timeago ts="2024-03-15T10:10:00Z"></faceplate-timeago>
	</shreddit-comment>
</body>
</html>`

// redditImagePostHTML has a #post-image element instead of a text body.
const redditImagePostHTML = `<html>
<head><title>Cool graph : r/dataisbeautiful</title></head>
<body>
	<h1>Cool graph</h1>
	<shreddit-post author="chart_maker" score="9999"
		permalink="/r/dataisbeautiful/comments/xyz789/cool_graph/">
		<div id="post-image">
			<img src="https://i.redd.it/cool_graph.png" alt="A cool graph">
		</div>
	</shreddit-post>
</body>
</html>`

// redditFallbackHTML uses old-style Reddit selectors (no shreddit-post).
const redditFallbackHTML = `<html>
<head><title>Old Reddit post</title></head>
<body>
	<div class="usertext-body">
		<div class="md"><p>This is an old-style Reddit post.</p></div>
	</div>
	<div data-testid="comment">
		<p>An old-style comment.</p>
	</div>
</body>
</html>`

// redditNoContentHTML has neither shreddit-post nor fallback selectors.
const redditNoContentHTML = `<html>
<head><title>Just a page</title></head>
<body><p>Nothing Reddit-like here.</p></body>
</html>`

func TestRedditExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewRedditExtractor(doc, "https://reddit.com/r/golang/comments/abc123/title", nil)
	assert.Equal(t, "RedditExtractor", ext.Name())
}

func TestRedditExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		html string
		want bool
	}{
		{
			name: "shreddit-post present",
			html: redditPostHTML,
			want: true,
		},
		{
			name: "fallback usertext-body",
			html: redditFallbackHTML,
			want: true,
		},
		{
			name: "fallback data-testid post-content",
			html: `<html><body><div data-testid="post-content">body</div></body></html>`,
			want: true,
		},
		{
			name: "fallback md class",
			html: `<html><body><div class="md">content</div></body></html>`,
			want: true,
		},
		{
			name: "fallback data-click-id text",
			html: `<html><body><div data-click-id="text">content</div></body></html>`,
			want: true,
		},
		{
			name: "fallback data-click-id body",
			html: `<html><body><div data-click-id="body">content</div></body></html>`,
			want: true,
		},
		{
			name: "fallback thing_t3_ id",
			html: `<html><body><div id="thing_t3_abc123">content</div></body></html>`,
			want: true,
		},
		{
			name: "fallback thing link class",
			html: `<html><body><div class="thing link">content</div></body></html>`,
			want: true,
		},
		{
			name: "no reddit content",
			html: redditNoContentHTML,
			want: false,
		},
		{
			name: "empty body",
			html: `<html><body></body></html>`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, tt.html)
			ext := NewRedditExtractor(doc, "https://reddit.com/r/golang/comments/abc123/title", nil)
			assert.Equal(t, tt.want, ext.CanExtract())
		})
	}
}

func TestRedditExtractor_Extract_PostWithComments(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, redditPostHTML)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/golang/comments/abc123/why_is_go_so_fast/", nil)

	result := ext.Extract()

	require.NotNil(t, result)

	// Post content wrapper structure
	assert.Contains(t, result.ContentHTML, `class="reddit-post"`)
	assert.Contains(t, result.ContentHTML, `class="post-content"`)

	// Text body extracted from [slot="text-body"]
	assert.Contains(t, result.ContentHTML, "Go compiles to native machine code")
	assert.Contains(t, result.ContentHTML, "The garbage collector is designed for low latency")

	// Comments section present
	assert.Contains(t, result.ContentHTML, `<hr>`)
	assert.Contains(t, result.ContentHTML, `<h2>Comments</h2>`)
	assert.Contains(t, result.ContentHTML, `class="reddit-comments"`)

	// Individual comment authors and content
	assert.Contains(t, result.ContentHTML, "rustacean42")
	assert.Contains(t, result.ContentHTML, "Goroutines are really cheap")
	assert.Contains(t, result.ContentHTML, "concurrent_dev")
	assert.Contains(t, result.ContentHTML, "work-stealing")
	assert.Contains(t, result.ContentHTML, "systems_programmer")
	assert.Contains(t, result.ContentHTML, "Static linking")

	// Comments wrapped in blockquotes for nesting
	assert.Contains(t, result.ContentHTML, "<blockquote>")
}

func TestRedditExtractor_Extract_Variables(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, redditPostHTML)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/golang/comments/abc123/why_is_go_so_fast/", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Why is Go so fast?", result.Variables["title"])
	assert.Equal(t, "gopher_dev", result.Variables["author"])
	assert.Equal(t, "r/golang", result.Variables["site"])
	assert.NotEmpty(t, result.Variables["description"])
}

func TestRedditExtractor_Extract_ExtractedContent(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, redditPostHTML)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/golang/comments/abc123/why_is_go_so_fast/", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "abc123", result.ExtractedContent["postId"])
	assert.Equal(t, "golang", result.ExtractedContent["subreddit"])
	assert.Equal(t, "gopher_dev", result.ExtractedContent["postAuthor"])
}

func TestRedditExtractor_Extract_ImagePost(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, redditImagePostHTML)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/dataisbeautiful/comments/xyz789/cool_graph/", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, `id="post-image"`)
	assert.Contains(t, result.ContentHTML, "cool_graph.png")

	// No comments
	assert.NotContains(t, result.ContentHTML, `<h2>Comments</h2>`)
}

func TestRedditExtractor_Extract_NoComments(t *testing.T) {
	t.Parallel()
	html := `<html>
<head><title>A post : r/golang</title></head>
<body>
	<h1>A post</h1>
	<shreddit-post author="someone" score="10"
		permalink="/r/golang/comments/def456/a_post/">
		<div slot="text-body"><p>Post with no comments.</p></div>
	</shreddit-post>
</body>
</html>`

	doc := newTestDoc(t, html)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/golang/comments/def456/a_post/", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "Post with no comments")
	assert.NotContains(t, result.ContentHTML, `<h2>Comments</h2>`)
	assert.NotContains(t, result.ContentHTML, "<blockquote>")
}

func TestRedditExtractor_GetMetadata_Subreddit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		wantSub  string
		wantSite string
	}{
		{
			name:     "golang subreddit",
			url:      "https://reddit.com/r/golang/comments/abc123/title",
			wantSub:  "golang",
			wantSite: "r/golang",
		},
		{
			name:     "programming subreddit",
			url:      "https://www.reddit.com/r/programming/comments/xyz789/other",
			wantSub:  "programming",
			wantSite: "r/programming",
		},
		{
			name:     "no subreddit in URL",
			url:      "https://reddit.com/",
			wantSub:  "",
			wantSite: "r/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, "<html><body></body></html>")
			ext := NewRedditExtractor(doc, tt.url, nil)
			assert.Equal(t, tt.wantSub, ext.getSubreddit())

			result := ext.Extract()
			require.NotNil(t, result)
			assert.Equal(t, tt.wantSite, result.Variables["site"])
		})
	}
}

func TestRedditExtractor_GetMetadata_Author(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		html       string
		wantAuthor string
	}{
		{
			name: "author from shreddit-post attribute",
			html: `<html><body>
				<shreddit-post author="test_user" score="10" permalink="/r/x/comments/1/t/">
					<div slot="text-body"><p>body</p></div>
				</shreddit-post>
			</body></html>`,
			wantAuthor: "test_user",
		},
		{
			name:       "no shreddit-post yields empty author",
			html:       `<html><body><div class="md">old post</div></body></html>`,
			wantAuthor: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, tt.html)
			ext := NewRedditExtractor(doc, "https://reddit.com/r/test/comments/123/title", nil)
			assert.Equal(t, tt.wantAuthor, ext.getPostAuthor())
		})
	}
}

func TestRedditExtractor_GetMetadata_Title(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		html      string
		wantTitle string
	}{
		{
			name: "h1 present",
			html: `<html><head><title>Page Title</title></head>
				<body><h1>Post Title from H1</h1><shreddit-post author="u" score="1" permalink="/r/x/comments/1/t/"><div slot="text-body">x</div></shreddit-post></body>
			</html>`,
			wantTitle: "Post Title from H1",
		},
		{
			name: "no h1, falls back to page title",
			html: `<html><head><title>Interesting Post : r/golang</title></head>
				<body><shreddit-post author="u" score="1" permalink="/r/x/comments/1/t/"><div slot="text-body">x</div></shreddit-post></body>
			</html>`,
			wantTitle: "Interesting Post : r/golang",
		},
		{
			name: "generic Reddit title ignored",
			html: `<html><head><title>Reddit - The heart of the internet</title></head>
				<body></body>
			</html>`,
			wantTitle: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, tt.html)
			ext := NewRedditExtractor(doc, "https://reddit.com/r/golang/comments/abc/title", nil)
			assert.Equal(t, tt.wantTitle, ext.getPostTitle())
		})
	}
}

func TestRedditExtractor_GetPostID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		url    string
		wantID string
	}{
		{"standard comments URL", "https://reddit.com/r/golang/comments/abc123/title", "abc123"},
		{"alphanumeric ID", "https://www.reddit.com/r/programming/comments/XyZ789/title", "XyZ789"},
		{"no comments in URL", "https://reddit.com/r/golang/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, "<html><body></body></html>")
			ext := NewRedditExtractor(doc, tt.url, nil)
			assert.Equal(t, tt.wantID, ext.getPostID())
		})
	}
}

func TestRedditExtractor_TimestampParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ts       string
		wantDate string // empty means date should be empty
	}{
		{
			name:     "ISO 8601 with timezone",
			ts:       "2024-03-15T10:00:00Z",
			wantDate: "2024-03-15",
		},
		{
			name:     "ISO 8601 without timezone",
			ts:       "2024-06-20T14:30:00",
			wantDate: "2024-06-20",
		},
		{
			name:     "Unix seconds",
			ts:       "1710496800", // 2024-03-15 in Unix time
			wantDate: "2024-03-15",
		},
		{
			name:     "Unix milliseconds",
			ts:       "1710496800000", // same date, milliseconds (>1e12)
			wantDate: "2024-03-15",
		},
		{
			name:     "empty timestamp",
			ts:       "",
			wantDate: "",
		},
		{
			name:     "invalid timestamp",
			ts:       "not-a-date",
			wantDate: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tsAttr := ""
			if tt.ts != "" {
				tsAttr = `ts="` + tt.ts + `"`
			}

			html := `<html><body>
				<shreddit-post author="someone" score="5" permalink="/r/x/comments/1/t/">
					<div slot="text-body"><p>content</p></div>
				</shreddit-post>
				<shreddit-comment depth="0" author="user1" score="10"
					permalink="/r/x/comments/1/t/c1">
					<div slot="comment"><p>A comment.</p></div>
					<faceplate-timeago ` + tsAttr + `></faceplate-timeago>
				</shreddit-comment>
			</body></html>`

			doc := newTestDoc(t, html)
			ext := NewRedditExtractor(doc, "https://reddit.com/r/x/comments/1/t/", nil)
			result := ext.Extract()
			require.NotNil(t, result)

			if tt.wantDate == "" {
				// Empty/invalid timestamps produce an empty date span.
				// The comment div is still present.
				assert.Contains(t, result.ContentHTML, `class="comment-date"`)
			} else {
				assert.Contains(t, result.ContentHTML, tt.wantDate)
			}
		})
	}
}

func TestRedditExtractor_CommentNesting(t *testing.T) {
	t.Parallel()

	// Two top-level comments each with one nested reply.
	html := `<html><body>
		<shreddit-post author="op" score="100" permalink="/r/test/comments/n1/post/">
			<div slot="text-body"><p>OP text</p></div>
		</shreddit-post>
		<shreddit-comment depth="0" author="top1" score="50" permalink="/r/test/comments/n1/post/c1">
			<div slot="comment"><p>Top comment 1</p></div>
			<faceplate-timeago ts="2024-01-01T00:00:00Z"></faceplate-timeago>
		</shreddit-comment>
		<shreddit-comment depth="1" author="reply1" score="20" permalink="/r/test/comments/n1/post/c2">
			<div slot="comment"><p>Reply to top 1</p></div>
			<faceplate-timeago ts="2024-01-01T01:00:00Z"></faceplate-timeago>
		</shreddit-comment>
		<shreddit-comment depth="0" author="top2" score="30" permalink="/r/test/comments/n1/post/c3">
			<div slot="comment"><p>Top comment 2</p></div>
			<faceplate-timeago ts="2024-01-01T02:00:00Z"></faceplate-timeago>
		</shreddit-comment>
		<shreddit-comment depth="1" author="reply2" score="10" permalink="/r/test/comments/n1/post/c4">
			<div slot="comment"><p>Reply to top 2</p></div>
			<faceplate-timeago ts="2024-01-01T03:00:00Z"></faceplate-timeago>
		</shreddit-comment>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/test/comments/n1/post/", nil)
	result := ext.Extract()
	require.NotNil(t, result)

	// Blockquotes open and close properly: 2 top-level → 2 opening blockquotes,
	// and replies are nested inside. Count opening vs closing tags.
	openCount := strings.Count(result.ContentHTML, "<blockquote>")
	closeCount := strings.Count(result.ContentHTML, "</blockquote>")
	assert.Equal(t, openCount, closeCount, "blockquote open/close tags must be balanced")

	// All authors appear.
	assert.Contains(t, result.ContentHTML, "top1")
	assert.Contains(t, result.ContentHTML, "reply1")
	assert.Contains(t, result.ContentHTML, "top2")
	assert.Contains(t, result.ContentHTML, "reply2")
}

func TestRedditExtractor_CommentPermalink(t *testing.T) {
	t.Parallel()

	html := `<html><body>
		<shreddit-post author="u" score="1" permalink="/r/x/comments/1/t/">
			<div slot="text-body"><p>x</p></div>
		</shreddit-post>
		<shreddit-comment depth="0" author="commenter" score="77"
			permalink="/r/x/comments/1/t/c99">
			<div slot="comment"><p>comment text</p></div>
			<faceplate-timeago ts="2024-05-10T12:00:00Z"></faceplate-timeago>
		</shreddit-comment>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/x/comments/1/t/", nil)
	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.ContentHTML, "https://reddit.com/r/x/comments/1/t/c99")
	assert.Contains(t, result.ContentHTML, "77 points")
}

func TestRedditExtractor_Description_Truncates(t *testing.T) {
	t.Parallel()

	// Build a post body longer than 140 characters.
	longBody := strings.Repeat("a", 200)
	html := `<html><body>
		<h1>Long post</h1>
		<shreddit-post author="u" score="1" permalink="/r/x/comments/1/t/">
			<div slot="text-body"><p>` + longBody + `</p></div>
		</shreddit-post>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/x/comments/1/t/", nil)
	result := ext.Extract()
	require.NotNil(t, result)

	desc := result.Variables["description"]
	assert.LessOrEqual(t, len(desc), 140)
	assert.NotEmpty(t, desc)
}

func TestRedditExtractor_FallbackComments(t *testing.T) {
	t.Parallel()

	// Old-Reddit-style page: no shreddit-post, uses data-testid="comment".
	doc := newTestDoc(t, redditFallbackHTML)
	ext := NewRedditExtractor(doc, "https://old.reddit.com/r/test/comments/abc/post/", nil)

	assert.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, `class="reddit-post"`)
	// Fallback post content extracted via .usertext-body
	assert.Contains(t, result.ContentHTML, "old-style Reddit post")
}
