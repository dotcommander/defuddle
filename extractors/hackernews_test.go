package extractors

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hnStoryHTML is a minimal but realistic HN story page.
//
// Important: tr.comtr rows must be direct children of the outer table, not
// nested inside a <td>. The HTML5 parser drops <tr> elements that appear
// directly inside a <td>. On real HN all comment rows live as siblings in
// <table id="hnmain"> — the fatitem row and comment rows are all at the same
// table level, separated only by the <td> wrapper trick.
const hnStoryHTML = `<html><head><title>Show HN: Some Cool Project | Hacker News</title></head>
<body>
<table id="hnmain">
<tr><td>
  <table class="fatitem">
    <tr class="athing" id="12345">
      <td class="title">
        <span class="titleline">
          <a href="https://example.com/cool-project">Show HN: Some Cool Project</a>
        </span>
      </td>
    </tr>
    <tr>
      <td class="subtext">
        <span class="score">142 points</span> by
        <a class="hnuser" href="/user?id=testauthor">testauthor</a>
        <span class="age" title="2024-03-15T10:00:00">3 hours ago</span>
        | <a href="item?id=12345">58 comments</a>
      </td>
    </tr>
  </table>
</td></tr>

<tr class="comtr" id="100001">
  <td>
    <table>
      <tr>
        <td class="ind"><img width="0"></td>
        <td class="comhead">
          <a class="hnuser" href="/user?id=alice">alice</a>
          <span class="age" title="2024-03-15T11:00:00">2 hours ago</span>
          <span class="score">42 points</span>
        </td>
      </tr>
      <tr>
        <td class="commtext c00">This is a top-level comment.</td>
      </tr>
    </table>
  </td>
</tr>

<tr class="comtr" id="100002">
  <td>
    <table>
      <tr>
        <td class="ind"><img width="40"></td>
        <td class="comhead">
          <a class="hnuser" href="/user?id=bob">bob</a>
          <span class="age" title="2024-03-15T11:30:00">90 minutes ago</span>
        </td>
      </tr>
      <tr>
        <td class="commtext c00">This is a nested reply to alice.</td>
      </tr>
    </table>
  </td>
</tr>

<tr class="comtr" id="100003">
  <td>
    <table>
      <tr>
        <td class="ind"><img width="80"></td>
        <td class="comhead">
          <a class="hnuser" href="/user?id=carol">carol</a>
          <span class="age" title="2024-03-15T12:00:00">1 hour ago</span>
        </td>
      </tr>
      <tr>
        <td class="commtext c00">Deeply nested reply.</td>
      </tr>
    </table>
  </td>
</tr>

<tr class="comtr" id="100004">
  <td>
    <table>
      <tr>
        <td class="ind"><img width="0"></td>
        <td class="comhead">
          <a class="hnuser" href="/user?id=dave">dave</a>
          <span class="age" title="2024-03-15T12:30:00">30 minutes ago</span>
        </td>
      </tr>
      <tr>
        <td class="commtext c00">Another top-level comment.</td>
      </tr>
    </table>
  </td>
</tr>

</table>
</body></html>`

// hnCommentPageHTML is a minimal HN single-comment page (has a "parent" nav link).
const hnCommentPageHTML = `<html><head><title>Comment by carol | Hacker News</title></head>
<body>
<table class="fatitem">
  <tr>
    <td class="navs">
      <a href="item?id=12345">root</a> |
      <a href="item?id=100002&parent=true">parent</a> |
      <a href="item?id=100003">next</a>
    </td>
  </tr>
  <tr>
    <td>
      <div class="comment">
        <a class="hnuser" href="/user?id=carol">carol</a>
        <span class="age" title="2024-03-15T12:00:00">1 hour ago</span>
        <span class="score">7 points</span>
        <div class="commtext">This is the main comment on a comment page with more than fifty characters of text for the title truncation test.</div>
      </div>
    </td>
  </tr>
</table>
</body></html>`

// hnDeletedCommentHTML has a comment row with no .hnuser (deleted author).
// Both the fatitem and tr.comtr must live inside the same outer table so that
// the HTML5 parser preserves the tr elements.
const hnDeletedCommentHTML = `<html><body>
<table id="hnmain">
<tr><td>
  <table class="fatitem">
    <tr class="athing" id="99999">
      <td class="title">
        <span class="titleline"><a href="https://example.com">Deleted author post</a></span>
      </td>
    </tr>
    <tr>
      <td class="subtext">
        <span class="age" title="2024-01-01T00:00:00">1 day ago</span>
      </td>
    </tr>
  </table>
</td></tr>

<tr class="comtr" id="200001">
  <td>
    <table>
      <tr>
        <td class="ind"><img width="0"></td>
        <td class="comhead"></td>
      </tr>
      <tr>
        <td class="commtext c00">Comment from deleted user.</td>
      </tr>
    </table>
  </td>
</tr>

</table>
</body></html>`

func TestHackerNewsExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=1", nil)
	assert.Equal(t, "HackerNewsExtractor", ext.Name())
}

// CanExtract tests — the extractor checks for a .fatitem element.

func TestHackerNewsExtractor_CanExtract_WithFatitem(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, hnStoryHTML)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=12345", nil)
	assert.True(t, ext.CanExtract())
}

func TestHackerNewsExtractor_CanExtract_WithoutFatitem(t *testing.T) {
	t.Parallel()
	html := `<html><head></head><body><p>No HN structure here.</p></body></html>`
	doc := newTestDoc(t, html)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/news", nil)
	assert.False(t, ext.CanExtract())
}

// Registry integration — the registry pattern matches news.ycombinator.com/item?id=…

func TestHackerNewsExtractor_RegistryRouting_MatchesItemURL(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.initializeBuiltins()
	doc := newTestDoc(t, hnStoryHTML)

	ext := r.FindExtractor(doc, "https://news.ycombinator.com/item?id=12345", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "HackerNewsExtractor", ext.Name())
}

func TestHackerNewsExtractor_RegistryRouting_NoMatchForNewsPage(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.initializeBuiltins()
	doc := newTestDoc(t, "<html><body></body></html>")

	// /news does not contain "item?id=" so the pattern should not match.
	ext := r.FindExtractor(doc, "https://news.ycombinator.com/news", nil)
	// The registry returns nil (no matching pattern) or a non-HN extractor.
	if ext != nil {
		assert.NotEqual(t, "HackerNewsExtractor", ext.Name())
	}
}

// GetPostTitle tests — reads from .titleline on story pages.

func TestHackerNewsExtractor_GetPostTitle_StoryPage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, hnStoryHTML)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=12345", nil)
	title := ext.getPostTitle()
	assert.Contains(t, title, "Show HN: Some Cool Project")
}

func TestHackerNewsExtractor_GetPostTitle_CommentPage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, hnCommentPageHTML)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=100003", nil)

	// On a comment page the title is "Comment by <author>: <preview…>"
	title := ext.getPostTitle()
	assert.True(t, strings.HasPrefix(title, "Comment by carol: "), "title should start with 'Comment by carol: ', got: %q", title)
	// Long text should be truncated to 50 chars + "..."
	assert.True(t, strings.HasSuffix(title, "..."), "title should end with '...', got: %q", title)
}

func TestHackerNewsExtractor_GetPostTitle_CommentPage_Short(t *testing.T) {
	t.Parallel()
	html := `<html><body>
<table class="fatitem">
  <tr><td class="navs"><a href="item?id=1&parent=true">parent</a></td></tr>
  <tr><td><div class="comment"><a class="hnuser">alice</a><span class="age" title="2024-01-01T00:00:00"></span><div class="commtext">Short comment.</div></div></td></tr>
</table>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=1", nil)

	title := ext.getPostTitle()
	// Short text must NOT be truncated with ellipsis.
	assert.Equal(t, "Comment by alice: Short comment.", title)
}

// Extract tests — story page with comments.

func TestHackerNewsExtractor_Extract_StoryPage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, hnStoryHTML)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=12345", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// Content wrappers
	assert.Contains(t, result.ContentHTML, `class="hackernews-post"`)
	assert.Contains(t, result.ContentHTML, `class="post-content"`)

	// The story link should appear in post content
	assert.Contains(t, result.ContentHTML, "https://example.com/cool-project")

	// Comments section
	assert.Contains(t, result.ContentHTML, "<h2>Comments</h2>")
	assert.Contains(t, result.ContentHTML, `class="hackernews-comments"`)

	// Comment authors
	assert.Contains(t, result.ContentHTML, "alice")
	assert.Contains(t, result.ContentHTML, "bob")
	assert.Contains(t, result.ContentHTML, "carol")
	assert.Contains(t, result.ContentHTML, "dave")

	// Comment text
	assert.Contains(t, result.ContentHTML, "This is a top-level comment.")
	assert.Contains(t, result.ContentHTML, "This is a nested reply to alice.")
	assert.Contains(t, result.ContentHTML, "Deeply nested reply.")

	// Variables
	assert.Equal(t, "Hacker News", result.Variables["site"])
	assert.Equal(t, "testauthor", result.Variables["author"])
	assert.Contains(t, result.Variables["title"], "Show HN: Some Cool Project")
	assert.Equal(t, "2024-03-15", result.Variables["published"])

	// ExtractedContent
	assert.Equal(t, "12345", result.ExtractedContent["postId"])
	assert.Equal(t, "testauthor", result.ExtractedContent["postAuthor"])
}

func TestHackerNewsExtractor_Extract_NoComments(t *testing.T) {
	t.Parallel()
	html := `<html><body>
<table class="fatitem">
  <tr class="athing" id="99999">
    <td class="title">
      <span class="titleline"><a href="https://example.com/link">Link Post</a></span>
    </td>
  </tr>
  <tr>
    <td class="subtext">
      by <a class="hnuser">author</a>
      <span class="age" title="2024-06-01T09:00:00">5 hours ago</span>
    </td>
  </tr>
</table>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=99999", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// No comments means no comments section
	assert.NotContains(t, result.ContentHTML, "<h2>Comments</h2>")
	assert.Contains(t, result.ContentHTML, `class="hackernews-post"`)
}

func TestHackerNewsExtractor_Extract_CommentPage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, hnCommentPageHTML)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=100003", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// Comment page content wraps the main comment
	assert.Contains(t, result.ContentHTML, `class="comment main-comment"`)
	assert.Contains(t, result.ContentHTML, "<strong>carol</strong>")
	assert.Contains(t, result.ContentHTML, "This is the main comment")

	// Parent link must appear in the metadata
	assert.Contains(t, result.ContentHTML, `class="parent-link"`)

	// Variables
	assert.Equal(t, "carol", result.Variables["author"])
	assert.Equal(t, "Hacker News", result.Variables["site"])
	assert.Equal(t, "2024-03-15", result.Variables["published"])
}

// Comment nesting — verify blockquote depth from img width attribute.

func TestHackerNewsExtractor_CommentNesting_IndentLevels(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, hnStoryHTML)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=12345", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	html := result.ContentHTML

	// The nesting logic opens <blockquote> for each new level and closes when
	// ascending back up. With widths 0, 40, 80, 0 we should see:
	//   depth-0 alice  → open blockquote (level 0)
	//   depth-1 bob    → open blockquote (level 1)
	//   depth-2 carol  → open blockquote (level 2)
	//   depth-0 dave   → close 3 blockquotes, open new one (level 0)
	//
	// There must be at least 3 opening <blockquote> tags in total.
	openCount := strings.Count(html, "<blockquote>")
	assert.GreaterOrEqual(t, openCount, 3, "expected at least 3 <blockquote> opens for 3 nesting levels")

	// All blockquotes opened must also be closed (balanced).
	closeCount := strings.Count(html, "</blockquote>")
	assert.Equal(t, openCount, closeCount, "blockquote tags must be balanced")
}

func TestHackerNewsExtractor_CommentNesting_IndentCalculation(t *testing.T) {
	t.Parallel()
	// img width="40" → indent 40 / 40 = depth 1
	// img width="80" → indent 80 / 40 = depth 2
	html := `<html><body>
<table class="fatitem">
  <tr class="athing" id="1"><td class="title"><span class="titleline"><a href="http://x.com">T</a></span></td></tr>
</table>
<table class="comment-tree">
  <tr class="comtr" id="c1">
    <td><table>
      <tr><td class="ind"><img width="40"></td></tr>
      <tr><td class="commtext c00">indent-1 comment</td></tr>
    </table></td>
  </tr>
  <tr class="comtr" id="c2">
    <td><table>
      <tr><td class="ind"><img width="80"></td></tr>
      <tr><td class="commtext c00">indent-2 comment</td></tr>
    </table></td>
  </tr>
</table>
</body></html>`

	doc := newTestDoc(t, html)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=1", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// Two nested comments starting at depth 1 and 2 should produce 2 blockquotes.
	assert.Equal(t, 2, strings.Count(result.ContentHTML, "<blockquote>"))
	assert.Equal(t, 2, strings.Count(result.ContentHTML, "</blockquote>"))
}

// GetPostID tests — extracts from URL query string.

func TestHackerNewsExtractor_GetPostID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
		want string
	}{
		{"standard", "https://news.ycombinator.com/item?id=12345", "12345"},
		{"comment page", "https://news.ycombinator.com/item?id=99999", "99999"},
		{"no id", "https://news.ycombinator.com/news", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, "<html><body></body></html>")
			ext := NewHackerNewsExtractor(doc, tc.url, nil)
			assert.Equal(t, tc.want, ext.getPostID())
		})
	}
}

// Deleted author — falls back to "[deleted]".

func TestHackerNewsExtractor_DeletedAuthor_FallsBackToDeleted(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, hnDeletedCommentHTML)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=99999", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// The comment text should still appear.
	assert.Contains(t, result.ContentHTML, "Comment from deleted user.")
	// Deleted author label in metadata div.
	assert.Contains(t, result.ContentHTML, "[deleted]")
}

// Duplicate comment IDs — must not be processed twice.

func TestHackerNewsExtractor_DuplicateCommentIDs_NotProcessedTwice(t *testing.T) {
	t.Parallel()
	html := `<html><body>
<table class="fatitem">
  <tr class="athing" id="1"><td class="title"><span class="titleline"><a href="http://x.com">T</a></span></td></tr>
</table>
<table class="comment-tree">
  <tr class="comtr" id="dup1">
    <td><table>
      <tr><td class="ind"><img width="0"></td></tr>
      <tr><td class="commtext c00">Only once.</td></tr>
    </table></td>
  </tr>
  <tr class="comtr" id="dup1">
    <td><table>
      <tr><td class="ind"><img width="0"></td></tr>
      <tr><td class="commtext c00">Only once.</td></tr>
    </table></td>
  </tr>
</table>
</body></html>`

	doc := newTestDoc(t, html)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=1", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// "Only once." should appear exactly once.
	count := strings.Count(result.ContentHTML, "Only once.")
	assert.Equal(t, 1, count, "duplicate comment IDs should be de-duplicated")
}

// CreateDescription tests.

func TestHackerNewsExtractor_CreateDescription_StoryPage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, hnStoryHTML)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=12345", nil)

	desc := ext.createDescription()
	assert.Contains(t, desc, "on Hacker News")
	assert.Contains(t, desc, "testauthor")
	// Story page description includes title and author.
	assert.NotEqual(t, "Comment by testauthor on Hacker News", desc)
}

func TestHackerNewsExtractor_CreateDescription_CommentPage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, hnCommentPageHTML)
	ext := NewHackerNewsExtractor(doc, "https://news.ycombinator.com/item?id=100003", nil)

	desc := ext.createDescription()
	assert.Equal(t, "Comment by carol on Hacker News", desc)
}
