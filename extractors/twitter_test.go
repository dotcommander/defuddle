package extractors

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// twitterTweetHTML returns minimal HTML for a single tweet article.
// The caller can embed it in any surrounding context.
func twitterTweetHTML(handle, fullName, tweetID, datetime, text string) string {
	return `<article data-testid="tweet">
  <div data-testid="User-Name">
    <a href="/` + handle + `">` + fullName + `</a>
    <a href="/` + handle + `">@` + handle + `</a>
  </div>
  <div data-testid="tweetText">` + text + `</div>
  <a href="/` + handle + `/status/` + tweetID + `">
    <time datetime="` + datetime + `">` + datetime[:10] + `</time>
  </a>
</article>`
}

// twitterTimelineHTML wraps tweet articles inside a timeline container.
func twitterTimelineHTML(tweetArticles ...string) string {
	return `<html><body>
<div aria-label="Timeline: Conversation">` +
		strings.Join(tweetArticles, "\n") +
		`</div>
</body></html>`
}

// twitterSingleHTML wraps a single tweet outside any timeline container.
func twitterSingleHTML(tweetArticle string) string {
	return `<html><body>` + tweetArticle + `</body></html>`
}

// ─── CanExtract ─────────────────────────────────────────────────────────────

func TestTwitterExtractor_CanExtract_WithTweet(t *testing.T) {
	t.Parallel()
	html := twitterSingleHTML(twitterTweetHTML(
		"testuser", "Test User", "123456789", "2024-03-15T10:00:00Z", "Hello world",
	))
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/testuser/status/123456789", nil)
	assert.True(t, ext.CanExtract())
}

func TestTwitterExtractor_CanExtract_EmptyDocument(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body><p>no tweets here</p></body></html>")
	ext := NewTwitterExtractor(doc, "https://twitter.com/testuser/status/1", nil)
	assert.False(t, ext.CanExtract())
}

func TestTwitterExtractor_CanExtract_XDomain(t *testing.T) {
	t.Parallel()
	html := twitterSingleHTML(twitterTweetHTML(
		"xuser", "X User", "999", "2024-06-01T08:00:00Z", "Post on x.com",
	))
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://x.com/xuser/status/999", nil)
	assert.True(t, ext.CanExtract())
}

// ─── Name ────────────────────────────────────────────────────────────────────

func TestTwitterExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewTwitterExtractor(doc, "", nil)
	assert.Equal(t, "TwitterExtractor", ext.Name())
}

// ─── getTweetID ──────────────────────────────────────────────────────────────

func TestTwitterExtractor_GetTweetID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"twitter.com status URL", "https://twitter.com/user/status/123456789", "123456789"},
		{"x.com status URL", "https://x.com/user/status/987654321", "987654321"},
		{"no status segment", "https://twitter.com/user", ""},
		{"empty URL", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, "<html><body></body></html>")
			ext := NewTwitterExtractor(doc, tt.url, nil)
			assert.Equal(t, tt.want, ext.getTweetID())
		})
	}
}

// ─── formatTweetText ─────────────────────────────────────────────────────────

func TestTwitterExtractor_FormatTweetText_Empty(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewTwitterExtractor(doc, "", nil)
	assert.Equal(t, "", ext.formatTweetText(""))
}

func TestTwitterExtractor_FormatTweetText_PlainText(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewTwitterExtractor(doc, "", nil)

	result := ext.formatTweetText("Hello world")
	assert.Contains(t, result, "<p>")
	assert.Contains(t, result, "Hello world")
}

func TestTwitterExtractor_FormatTweetText_LinkToPlainText(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewTwitterExtractor(doc, "", nil)

	input := `Check out <a href="https://example.com">@exampleuser</a> for more`
	result := ext.formatTweetText(input)

	// The link text should remain; the anchor tag should be removed.
	assert.Contains(t, result, "@exampleuser")
	assert.NotContains(t, result, "<a ")
}

func TestTwitterExtractor_FormatTweetText_EmojiImgToAlt(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewTwitterExtractor(doc, "", nil)

	input := `Hello <img src="https://abs.twimg.com/emoji/v2/svg/1f600.svg" alt="😀"> world`
	result := ext.formatTweetText(input)

	assert.Contains(t, result, "😀")
	assert.NotContains(t, result, "<img ")
}

func TestTwitterExtractor_FormatTweetText_SpanUnwrapped(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewTwitterExtractor(doc, "", nil)

	input := `<span>tweet <span>content</span> here</span>`
	result := ext.formatTweetText(input)

	assert.Contains(t, result, "tweet")
	assert.Contains(t, result, "content")
	assert.Contains(t, result, "here")
}

func TestTwitterExtractor_FormatTweetText_MultipleLines(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewTwitterExtractor(doc, "", nil)

	input := "Line one\nLine two"
	result := ext.formatTweetText(input)

	// goquery wraps the fragment in <html><body>…</body></html>, so the result
	// contains the text split across <p> tags rather than bare lines.
	assert.Contains(t, result, "Line one")
	assert.Contains(t, result, "Line two")
	// The function always wraps non-empty lines in paragraph tags.
	assert.Contains(t, result, "<p>")
}

// ─── extractUserInfo ─────────────────────────────────────────────────────────

func TestTwitterExtractor_ExtractUserInfo_LinksStructure(t *testing.T) {
	t.Parallel()
	html := twitterSingleHTML(twitterTweetHTML(
		"johndoe", "John Doe", "111", "2024-01-20T14:30:00Z", "Some tweet",
	))
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/johndoe/status/111", nil)
	require.NotNil(t, ext.mainTweet)

	info := ext.extractUserInfo(ext.mainTweet)
	assert.Equal(t, "John Doe", info.FullName)
	assert.Equal(t, "@johndoe", info.Handle)
	assert.Equal(t, "2024-01-20", info.Date)
	assert.Contains(t, info.Permalink, "status/111")
}

func TestTwitterExtractor_ExtractUserInfo_NoUserName(t *testing.T) {
	t.Parallel()
	html := `<html><body>
<article data-testid="tweet">
  <div data-testid="tweetText">text only, no user name element</div>
</article>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/user/status/1", nil)
	require.NotNil(t, ext.mainTweet)

	info := ext.extractUserInfo(ext.mainTweet)
	assert.Empty(t, info.FullName)
	assert.Empty(t, info.Handle)
	assert.Empty(t, info.Date)
}

func TestTwitterExtractor_ExtractUserInfo_SpanFallback(t *testing.T) {
	t.Parallel()
	// Quoted tweet structure uses spans instead of links for user name.
	html := `<html><body>
<article data-testid="tweet">
  <div data-testid="User-Name">
    <span style="color: rgb(15, 20, 25)"><span>Jane Smith</span></span>
    <span style="color: rgb(83, 100, 113)">@janesmith</span>
  </div>
  <div data-testid="tweetText">span fallback tweet</div>
</article>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/janesmith/status/2", nil)
	require.NotNil(t, ext.mainTweet)

	info := ext.extractUserInfo(ext.mainTweet)
	assert.Equal(t, "Jane Smith", info.FullName)
	assert.Equal(t, "@janesmith", info.Handle)
}

// ─── Extract (full result) ────────────────────────────────────────────────────

func TestTwitterExtractor_Extract_SingleTweet(t *testing.T) {
	t.Parallel()
	html := twitterSingleHTML(twitterTweetHTML(
		"alice", "Alice", "42", "2024-05-10T09:00:00Z", "This is a tweet",
	))
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/alice/status/42", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.ContentHTML, `class="tweet-thread"`)
	assert.Contains(t, result.ContentHTML, `class="main-tweet"`)
	assert.Contains(t, result.ContentHTML, "This is a tweet")
	assert.Contains(t, result.ContentHTML, "Alice")
	assert.Equal(t, "42", result.ExtractedContent["tweetId"])
	assert.Equal(t, "@alice", result.ExtractedContent["tweetAuthor"])
	assert.Equal(t, "Thread by @alice", result.Variables["title"])
	assert.Equal(t, "@alice", result.Variables["author"])
	assert.Equal(t, "X (Twitter)", result.Variables["site"])
	assert.NotEmpty(t, result.Variables["description"])
}

func TestTwitterExtractor_Extract_TweetThread(t *testing.T) {
	t.Parallel()
	tweet1 := twitterTweetHTML("bob", "Bob", "100", "2024-04-01T10:00:00Z", "First tweet")
	tweet2 := twitterTweetHTML("bob", "Bob", "101", "2024-04-01T10:01:00Z", "Second tweet")
	tweet3 := twitterTweetHTML("bob", "Bob", "102", "2024-04-01T10:02:00Z", "Third tweet")

	html := twitterTimelineHTML(tweet1, tweet2, tweet3)
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/bob/status/100", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.ContentHTML, "First tweet")
	assert.Contains(t, result.ContentHTML, `class="thread-tweets"`)
	assert.Contains(t, result.ContentHTML, "Second tweet")
	assert.Contains(t, result.ContentHTML, "Third tweet")
	assert.Len(t, ext.threadTweets, 2)
}

func TestTwitterExtractor_Extract_NoThreadTweetsOmitsThreadDiv(t *testing.T) {
	t.Parallel()
	html := twitterSingleHTML(twitterTweetHTML(
		"solo", "Solo User", "777", "2024-07-04T00:00:00Z", "standalone",
	))
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/solo/status/777", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	assert.NotContains(t, result.ContentHTML, `class="thread-tweets"`)
	assert.Len(t, ext.threadTweets, 0)
}

func TestTwitterExtractor_Extract_WithMediaImage(t *testing.T) {
	t.Parallel()
	html := `<html><body>
<article data-testid="tweet">
  <div data-testid="User-Name">
    <a href="/picuser">Pic User</a>
    <a href="/picuser">@picuser</a>
  </div>
  <div data-testid="tweetText">Look at this photo</div>
  <div data-testid="tweetPhoto">
    <img src="https://pbs.twimg.com/media/abc123?format=jpg&amp;name=small" alt="A photo" />
  </div>
  <a href="/picuser/status/555">
    <time datetime="2024-08-01T12:00:00Z">2024-08-01</time>
  </a>
</article>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/picuser/status/555", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// Image quality should be upgraded to large.
	assert.Contains(t, result.ContentHTML, "name=large")
	assert.Contains(t, result.ContentHTML, `class="tweet-media"`)
	assert.Contains(t, result.ContentHTML, `alt="A photo"`)
}

func TestTwitterExtractor_Extract_Description_Truncates(t *testing.T) {
	t.Parallel()
	longTweet := strings.Repeat("word ", 40) // well over 140 chars
	html := twitterSingleHTML(twitterTweetHTML(
		"verbose", "Verbose", "888", "2024-09-01T06:00:00Z", longTweet,
	))
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/verbose/status/888", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	assert.LessOrEqual(t, len(result.Variables["description"]), 140)
}

// ─── getTweetAuthor ──────────────────────────────────────────────────────────

func TestTwitterExtractor_GetTweetAuthor_AddsAtSign(t *testing.T) {
	t.Parallel()
	// Handle without leading @.
	html := `<html><body>
<article data-testid="tweet">
  <div data-testid="User-Name">
    <a href="/noat">No At</a>
    <a href="/noat">noat</a>
  </div>
  <div data-testid="tweetText">no at sign</div>
</article>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/noat/status/1", nil)

	assert.Equal(t, "@noat", ext.getTweetAuthor())
}

func TestTwitterExtractor_GetTweetAuthor_PreservesExistingAtSign(t *testing.T) {
	t.Parallel()
	html := twitterSingleHTML(twitterTweetHTML(
		"withatsign", "With At", "2", "2024-01-01T00:00:00Z", "tweet",
	))
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/withatsign/status/2", nil)

	// twitterTweetHTML already adds @ prefix.
	author := ext.getTweetAuthor()
	assert.True(t, strings.HasPrefix(author, "@"))
	// Must not double-add the @ sign.
	assert.False(t, strings.HasPrefix(author, "@@"))
}

func TestTwitterExtractor_GetTweetAuthor_NoMainTweet(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewTwitterExtractor(doc, "https://twitter.com/nobody/status/0", nil)
	assert.Equal(t, "", ext.getTweetAuthor())
}

// ─── Section-boundary cutoff (filterTweetsBeforeBoundary) ───────────────────

func TestTwitterExtractor_SectionBoundary_FiltersRecommended(t *testing.T) {
	t.Parallel()
	// The boundary logic uses sectionBoundary.Parent() as the cutoff node.
	// The section must NOT be a direct child of the timeline div — otherwise
	// its parent IS the timeline root (document position 0) and all tweets
	// would be filtered.  Wrap the "Discover more" section in a sibling div
	// so its parent sits after the thread tweets in document order.
	html := `<html><body>
<div aria-label="Timeline: Conversation">
  <div class="thread-container">
    <article data-testid="tweet">
      <div data-testid="User-Name">
        <a href="/a">Author A</a><a href="/a">@a</a>
      </div>
      <div data-testid="tweetText">first tweet</div>
    </article>
    <article data-testid="tweet">
      <div data-testid="User-Name">
        <a href="/a">Author A</a><a href="/a">@a</a>
      </div>
      <div data-testid="tweetText">second tweet</div>
    </article>
  </div>
  <div class="recommendations-container">
    <section>
      <h2>Discover more</h2>
    </section>
    <article data-testid="tweet">
      <div data-testid="User-Name">
        <a href="/rec">Recommended</a><a href="/rec">@rec</a>
      </div>
      <div data-testid="tweetText">recommended tweet</div>
    </article>
  </div>
</div>
</body></html>`

	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/a/status/1", nil)

	require.True(t, ext.CanExtract())
	// The section's parent (recommendations-container div) sits after the
	// two thread tweets in document order, so the recommended tweet is cut.
	assert.Len(t, ext.threadTweets, 1)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "first tweet")
	assert.Contains(t, result.ContentHTML, "second tweet")
	assert.NotContains(t, result.ContentHTML, "recommended tweet")
}

func TestTwitterExtractor_SectionBoundary_NoSectionKeepsAll(t *testing.T) {
	t.Parallel()
	tweet1 := twitterTweetHTML("c", "C", "10", "2024-01-01T00:00:00Z", "tweet one")
	tweet2 := twitterTweetHTML("c", "C", "11", "2024-01-01T00:00:01Z", "tweet two")
	html := twitterTimelineHTML(tweet1, tweet2)
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/c/status/10", nil)

	// No section present — all tweets should be retained.
	assert.Len(t, ext.threadTweets, 1)
}

// ─── Fallback selectors ───────────────────────────────────────────────────────

func TestTwitterExtractor_FallbackSelector_MainRole(t *testing.T) {
	t.Parallel()
	// Timeline uses main[role="main"] instead of aria-label.
	html := `<html><body>
<main role="main">
  <article data-testid="tweet">
    <div data-testid="User-Name">
      <a href="/fallback">Fallback</a><a href="/fallback">@fallback</a>
    </div>
    <div data-testid="tweetText">fallback timeline tweet</div>
  </article>
</main>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/fallback/status/1", nil)
	assert.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "fallback timeline tweet")
}

// ─── Quoted tweet ─────────────────────────────────────────────────────────────

func TestTwitterExtractor_Extract_QuotedTweet(t *testing.T) {
	t.Parallel()
	html := `<html><body>
<article data-testid="tweet">
  <div data-testid="User-Name">
    <a href="/outer">Outer User</a>
    <a href="/outer">@outer</a>
  </div>
  <div data-testid="tweetText">quoting someone</div>
  <div aria-labelledby="id__quoted" id="id__quoted">
    <div data-testid="User-Name">
      <a href="/inner">Inner User</a>
      <a href="/inner">@inner</a>
    </div>
    <div data-testid="tweetText">the quoted tweet text</div>
  </div>
  <a href="/outer/status/300">
    <time datetime="2024-10-01T10:00:00Z">2024-10-01</time>
  </a>
</article>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewTwitterExtractor(doc, "https://twitter.com/outer/status/300", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "quoting someone")
	assert.Contains(t, result.ContentHTML, `class="quoted-tweet"`)
	assert.Contains(t, result.ContentHTML, "the quoted tweet text")
}
