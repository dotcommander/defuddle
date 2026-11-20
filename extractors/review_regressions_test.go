package extractors

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// C5-01: XOEmbedExtractor must not be shadowed by x.com / twitter.com.
func TestRegression_C5_01_XOEmbedNotShadowed(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.initializeBuiltins()

	html := `<html><body>{"html":"<blockquote class=\"twitter-tweet\"><p>Hello from oEmbed</p></blockquote>","author_name":"Alice","author_url":"https://twitter.com/alice"}</body></html>`
	for _, u := range []string{
		"https://publish.twitter.com/oembed?url=https://twitter.com/user/status/123",
		"https://publish.x.com/oembed?url=https://x.com/user/status/123",
	} {
		doc := newTestDoc(t, html)
		ext := r.FindExtractor(doc, u, nil)
		require.NotNil(t, ext, "expected XOEmbedExtractor for %s", u)
		assert.Equal(t, "XOEmbedExtractor", ext.Name())
	}
}

// C5-02: Claude & Grok fallback selectors must not claim non-chat pages.
func TestRegression_C5_02_ClaudeAndGrokFallbackGating(t *testing.T) {
	t.Parallel()

	claudeNonChat := newTestDoc(t, `<html><body><div class="message">Status banner</div><article>Marketing copy</article></body></html>`)
	claudeExt := NewClaudeExtractor(claudeNonChat, "https://claude.ai/login", nil)
	assert.False(t, claudeExt.CanExtract(), "ClaudeExtractor must not match generic fallback elements without message markers")

	grokNonChat := newTestDoc(t, `<html><body><div class="bubble">Promo</div><div class="conversation">Landing page</div></body></html>`)
	grokExt := NewGrokExtractor(grokNonChat, "https://grok.com/", nil)
	assert.False(t, grokExt.CanExtract(), "GrokExtractor must not match generic fallback elements without items-start/items-end + .message-bubble")
}

// C5-03: Reddit <img id="post-image"> void element must preserve outer HTML.
func TestRegression_C5_03_RedditPostImageVoidElement(t *testing.T) {
	t.Parallel()

	html := `<html><body>
		<h1>Photo</h1>
		<shreddit-post author="alice" permalink="/r/pics/comments/abc/photo/">
			<img id="post-image" src="https://i.redd.it/photo.jpg" alt="Sunset">
		</shreddit-post>
	</body></html>`
	doc := newTestDoc(t, html)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/pics/comments/abc/photo/", nil)
	res := ext.Extract()
	require.NotNil(t, res)
	assert.Contains(t, res.ContentHTML, `<img id="post-image"`)
	assert.Contains(t, res.ContentHTML, `https://i.redd.it/photo.jpg`)
}

// C5-04: XArticle, Grok, and Twitter must HTML-escape plain text / attributes.
func TestRegression_C5_04_HTMLEscapingInSocialAndGrok(t *testing.T) {
	t.Parallel()

	t.Run("XArticle", func(t *testing.T) {
		t.Parallel()
		html := `<html><body>
			<div data-testid="twitterArticleRichTextView">
				<h2>Title &lt;img src=x onerror=alert(1)&gt;</h2>
				<div class="longform-unstyled">Text &lt;script&gt;alert(1)&lt;/script&gt; <a href="https://example.com/?a=1&quot; onclick=&quot;alert(1)">&lt;b&gt;link&lt;/b&gt;</a></div>
				<div data-testid="markdown-code-block"><span>go&quot; onclick=&quot;x</span><pre><code>if a &lt; b &amp;&amp; c &gt; d {}</code></pre></div>
				<div data-testid="simpleTweet">
					<div data-testid="User-Name"><a href="/a">&lt;b&gt;Bob&lt;/b&gt;</a><a href="/a">@bob</a></div>
					<div data-testid="tweetText">&lt;script&gt;bad()&lt;/script&gt;</div>
				</div>
				<span style="font-weight: bold">&lt;svg onload=1&gt;</span>
			</div>
		</body></html>`
		doc := newTestDoc(t, html)
		ext := NewXArticleExtractor(doc, "https://x.com/user/article/123", nil)
		require.True(t, ext.CanExtract())
		res := ext.Extract()
		assert.NotContains(t, res.ContentHTML, "<script>")
		assert.NotContains(t, res.ContentHTML, "<img src=x onerror=alert(1)>")
		assert.NotContains(t, res.ContentHTML, "<svg onload=1>")
		assert.Contains(t, res.ContentHTML, "&lt;script&gt;")
	})

	t.Run("Grok user message", func(t *testing.T) {
		t.Parallel()
		html := `<html><body>
			<div class="relative group flex flex-col justify-center w-full items-end">
				<div class="message-bubble">&lt;script&gt;alert(1)&lt;/script&gt;</div>
			</div>
		</body></html>`
		doc := newTestDoc(t, html)
		ext := NewGrokExtractor(doc, "https://grok.com/chat/1", nil)
		msgs := ext.ExtractMessages()
		require.Len(t, msgs, 1)
		assert.NotContains(t, msgs[0].Content, "<script>")
		assert.Contains(t, msgs[0].Content, "&lt;script&gt;")
	})

	t.Run("Twitter emoji alt and handle", func(t *testing.T) {
		t.Parallel()
		html := `<html><body>
			<article data-testid="tweet">
				<div data-testid="User-Name"><a href="/u">Alice</a><a href="/u">@alice</a></div>
				<div data-testid="tweetText">Hi <img src="/emoji/1.svg" alt="&lt;script&gt;x&lt;/script&gt;"> <a href="/u">&lt;img src=x onerror=1&gt;</a></div>
			</article>
		</body></html>`
		doc := newTestDoc(t, html)
		ext := NewTwitterExtractor(doc, "https://x.com/alice/status/123", nil)
		res := ext.Extract()
		assert.NotContains(t, res.ContentHTML, "<script>")
		assert.NotContains(t, res.ContentHTML, "<img src=x onerror=1>")
	})
}

// C6-01: YouTube videoID validation and description HTML escaping.
func TestRegression_C6_01_YouTubeSanitization(t *testing.T) {
	t.Parallel()

	badURL := `https://www.youtube.com/watch?v=abc" onload="alert(1)`
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, badURL, nil)
	assert.Empty(t, ext.getVideoID())
	assert.False(t, ext.CanExtract())

	schema := map[string]any{
		"@type":       "VideoObject",
		"description": "Hello <script>alert(1)</script>\nSecond line",
	}
	goodExt := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", schema)
	res := goodExt.Extract()
	assert.NotContains(t, res.ContentHTML, "<script>")
	assert.Contains(t, res.ContentHTML, "&lt;script&gt;alert(1)&lt;/script&gt;<br>Second line")
}

// C6-02 & C6-05: GitHub emits real newlines (not literal \n) and escapes authors.
func TestRegression_C6_02_GitHubNewlinesAndEscaping(t *testing.T) {
	t.Parallel()

	html := `<html>
	<head><meta name="expected-hostname" content="github.com"><title>Test · o/r</title></head>
	<body>
		<div data-testid="issue-title">Test</div>
		<div data-testid="issue-viewer-issue-container">
			<a data-testid="issue-body-header-author" href="/&lt;script&gt;">bad</a>
			<div data-testid="issue-body-viewer"><div class="markdown-body"><p>Issue body</p></div></div>
		</div>
		<div data-wrapper-timeline-id="c1">
			<div class="react-issue-comment">
				<a data-testid="avatar-link" href="/bob&quot;&gt;&lt;b&gt;">bob</a>
				<div class="markdown-body"><p>Comment body</p></div>
			</div>
		</div>
	</body></html>`
	doc := newTestDoc(t, html)
	ext := NewGitHubExtractor(doc, "https://github.com/o/r/issues/1", nil)
	res := ext.Extract()
	assert.NotContains(t, res.ContentHTML, `\n`)
	assert.NotContains(t, res.ContentHTML, "<script>")
	assert.NotContains(t, res.ContentHTML, "<head>")
	assert.NotContains(t, res.ContentHTML, "<body>")
}

// C6-04: Substack preloads JSON.parse("..."), publishedBylines array, and .body.markup fallback.
func TestRegression_C6_04_SubstackPreloadsAndMarkup(t *testing.T) {
	t.Parallel()

	t.Run("JSON.parse with publishedBylines array and trailing JS", func(t *testing.T) {
		t.Parallel()
		html := `<html><body>
			<script>
				window._preloads = JSON.parse("{\"post\":{\"title\":\"Real Substack\",\"subtitle\":\"Sub\",\"post_date\":\"2025-01-02T00:00:00Z\",\"body_html\":\"<p>Hello world</p>\",\"publishedBylines\":[{\"name\":\"Jane Author\"}]}}");
				window._other = 42;
			</script>
		</body></html>`
		doc := newTestDoc(t, html)
		ext := NewSubstackExtractor(doc, "https://example.substack.com/p/real", nil)
		assert.True(t, ext.CanExtract())
		res := ext.Extract()
		assert.Equal(t, "Real Substack", res.Variables["title"])
		assert.Equal(t, "Jane Author", res.Variables["author"])
		assert.Contains(t, res.ContentHTML, "Hello world")
	})

	t.Run("standalone .body.markup", func(t *testing.T) {
		t.Parallel()
		html := `<html><body><div class="body markup"><p>Standalone markup</p></div></body></html>`
		doc := newTestDoc(t, html)
		ext := NewSubstackExtractor(doc, "https://example.substack.com/p/markup", nil)
		assert.True(t, ext.CanExtract())
		res := ext.Extract()
		assert.Contains(t, res.ContentHTML, "Standalone markup")
	})
}

// C5-05: renderCommentThread ascending from depth 2 to depth 1 must keep depth-1 blockquote open.
func TestRegression_C5_05_RenderCommentThreadNesting(t *testing.T) {
	t.Parallel()

	comments := []CommentData{
		{Depth: 0, Author: "a", Content: "<p>d0</p>"},
		{Depth: 1, Author: "b", Content: "<p>d1a</p>"},
		{Depth: 2, Author: "c", Content: "<p>d2</p>"},
		{Depth: 1, Author: "d", Content: "<p>d1b</p>"},
	}
	out := renderCommentThread(comments)
	assert.Equal(t, strings.Count(out, "<blockquote>"), strings.Count(out, "</blockquote>"))
	assert.Equal(t, 3, strings.Count(out, "<blockquote>"))

	// Verify d1b is inside depth-1 blockquote (only 1 </blockquote> between d2 and d1b).
	d2Idx := strings.Index(out, "d2")
	d1bIdx := strings.Index(out, "d1b")
	require.Greater(t, d1bIdx, d2Idx)
	between := out[d2Idx:d1bIdx]
	assert.Equal(t, 1, strings.Count(between, "</blockquote>"), "ascending from depth 2 to 1 must close only depth 2")
}

// C5-06 & C5-15: Reddit fallback does not emit post elements as empty comments, and uses UTC dates.
func TestRegression_C5_06_C5_15_RedditCommentsAndUTC(t *testing.T) {
	t.Parallel()

	postOnlyHTML := `<html><body>
		<div id="thing_t3_abc" class="thing link">
			<div data-click-id="text"><p>Post text only</p></div>
		</div>
	</body></html>`
	doc := newTestDoc(t, postOnlyHTML)
	ext := NewRedditExtractor(doc, "https://reddit.com/r/test/comments/abc/post/", nil)
	res := ext.Extract()
	assert.NotContains(t, res.ContentHTML, `<h2>Comments</h2>`, "post elements must not be rendered as empty comments")

	utcHTML := `<html><body>
		<shreddit-post author="op" permalink="/r/test/comments/1/p/"><div slot="text-body"><p>hi</p></div></shreddit-post>
		<shreddit-comment depth="0" author="u1" score="1" permalink="/r/test/comments/1/p/c1">
			<div slot="comment"><p>comment</p></div>
			<faceplate-timeago ts="1704067200"></faceplate-timeago>
		</shreddit-comment>
	</body></html>`
	utcExt := NewRedditExtractor(newTestDoc(t, utcHTML), "https://reddit.com/r/test/comments/1/p/", nil)
	utcRes := utcExt.Extract()
	assert.Contains(t, utcRes.ContentHTML, "2024-01-01")
}

// C5-07: Threads unwrapRedirectURL must not double-unescape.
func TestRegression_C5_07_ThreadsUnwrapRedirectURL(t *testing.T) {
	t.Parallel()

	href := "https://l.threads.com/?u=https%3A%2F%2Fexample.com%2Fsearch%3Fq%3Da%2520b%26x%3D1%252B2"
	got := unwrapRedirectURL(href)
	assert.Equal(t, "https://example.com/search?q=a%20b&x=1%2B2", got)
}

// C5-08 & C5-12: Case-insensitive hostname matching and user Register before catchall.
func TestRegression_C5_08_C5_12_RegistryCaseAndPrecedence(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.initializeBuiltins()

	doc := newTestDoc(t, `<html><body><div class="detailed-status"><div class="status__content"><p>Toot</p></div></div></body></html>`)
	// Case-insensitive match
	assert.NotNil(t, r.FindExtractor(doc, "HTTPS://WWW.REDDIT.COM/r/golang/comments/1/t/", nil))

	// User-registered extractor must beat catchall (Mastodon) for its domain.
	r.Register(ExtractorMapping{
		Patterns: []any{"MyCustom.Example"},
		Extractor: func(_ *goquery.Document, _ string, _ any) BaseExtractor {
			return testExtractor{name: "CustomUserExtractor"}
		},
	})
	ext := r.FindExtractor(doc, "https://mycustom.example/status/1", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "CustomUserExtractor", ext.Name())
}

// C5-09 / C6-03: No <head></head><body> wrapper leaked by ChatGPT, Gemini, Twitter, GitHub.
func TestRegression_C5_09_NoDocumentWrapperLeak(t *testing.T) {
	t.Parallel()

	t.Run("ChatGPT", func(t *testing.T) {
		t.Parallel()
		doc := newTestDoc(t, chatgptBasicHTML)
		ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/1", nil)
		msgs := ext.ExtractMessages()
		require.NotEmpty(t, msgs)
		assert.NotContains(t, msgs[0].Content, "<head>")
		assert.NotContains(t, msgs[0].Content, "<body>")
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		html := `<html><body><div class="conversation-container">
			<model-response><div class="model-response-text"><div class="markdown"><div class="table-content"><p>Answer</p></div></div></div></model-response>
		</div></body></html>`
		doc := newTestDoc(t, html)
		ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/1", nil)
		msgs := ext.ExtractMessages()
		require.Len(t, msgs, 1)
		assert.NotContains(t, msgs[0].Content, "<head>")
		assert.NotContains(t, msgs[0].Content, "<body>")
	})

	t.Run("Twitter", func(t *testing.T) {
		t.Parallel()
		html := `<html><body>
			<article data-testid="tweet">
				<div data-testid="User-Name"><a href="/u">Alice</a><a href="/u">@alice</a></div>
				<div data-testid="tweetText"><span>Hello world</span></div>
			</article>
		</body></html>`
		doc := newTestDoc(t, html)
		ext := NewTwitterExtractor(doc, "https://x.com/alice/status/1", nil)
		res := ext.Extract()
		assert.NotContains(t, res.ContentHTML, "<head>")
		assert.NotContains(t, res.ContentHTML, "<body>")
	})
}

// C5-10: Extract() must not panic when CanExtract() is false.
func TestRegression_C5_10_NoPanicOnEmptyExtract(t *testing.T) {
	t.Parallel()

	emptyDoc := newTestDoc(t, "<html><body><p>Empty</p></body></html>")

	assert.NotPanics(t, func() {
		res := NewBlueskyExtractor(emptyDoc, "https://bsky.app/profile/x/post/1", nil).Extract()
		require.NotNil(t, res)
	})
	assert.NotPanics(t, func() {
		res := NewThreadsExtractor(emptyDoc, "https://threads.net/@x/post/1", nil).Extract()
		require.NotNil(t, res)
	})
	assert.NotPanics(t, func() {
		res := NewLinkedInExtractor(emptyDoc, "https://linkedin.com/posts/1", nil).Extract()
		require.NotNil(t, res)
	})
	assert.NotPanics(t, func() {
		res := NewNytimesExtractor(emptyDoc, "https://www.nytimes.com/2026/01/01/test.html", nil).Extract()
		require.NotNil(t, res)
	})
}

// C5-11: ChatGPT & Grok footnotes must not produce nested <a><a> tags.
func TestRegression_C5_11_NoNestedAnchorFootnotes(t *testing.T) {
	t.Parallel()

	chatExt := NewChatGPTExtractor(newTestDoc(t, chatgptCitationHTML), "https://chatgpt.com/c/1", nil)
	chatRes := chatExt.Extract()
	assert.NotContains(t, chatRes.ContentHTML, "<a href=\"https://go.dev/about\" target=\"_blank\"><a ")

	grokHTML := `<html><body>
		<div class="relative group flex flex-col justify-center w-full items-start">
			<div class="message-bubble"><a href="https://golang.org">Go</a></div>
		</div>
	</body></html>`
	grokExt := NewGrokExtractor(newTestDoc(t, grokHTML), "https://grok.com/chat/1", nil)
	grokRes := grokExt.Extract()
	assert.NotContains(t, grokRes.ContentHTML, `<a href="https://golang.org" target="_blank"><a `)
	assert.Contains(t, grokRes.ContentHTML, `<a href="https://golang.org" target="_blank">golang.org</a>`)
}

// C6-05 & C6-06: commentHTML escapes metadata without double-escaping and populates LinkText.
func TestRegression_C6_05_C6_06_CommentsEscapingAndLinkText(t *testing.T) {
	t.Parallel()

	out := commentHTML(CommentData{
		Author:   `A & B <script>`,
		URL:      `https://example.com/?a=1&b="2"`,
		LinkText: `2026-01-01`,
		Content:  `<p>ok</p>`,
	})
	assert.Contains(t, out, `<strong>A &amp; B &lt;script&gt;</strong>`)
	assert.Contains(t, out, `href="https://example.com/?a=1&amp;b=&#34;2&#34;"`)
	assert.NotContains(t, out, `</a> •</div>`, "must not emit dangling bullet when no date span follows")

	// Empty LinkText falls back to "link" rather than an empty anchor.
	emptyLink := commentHTML(CommentData{
		Author:  "alice",
		URL:     "https://example.com/c/1",
		Content: "<p>hi</p>",
	})
	assert.Contains(t, emptyLink, `class="comment-link">link</a>`)
	assert.NotContains(t, emptyLink, `class="comment-link"></a>`)

	// LWN comment link uses date as LinkText.
	lwnHTML := `<html><body>
		<div class="PageHeadline"><h1>Kernel News</h1></div>
		<div class="ArticleText"><main>
			<p>Article body</p>
			<details class="CommentBox">
				<summary>
					<h3 class="CommentTitle">Great patch</h3>
					<p class="CommentPoster">Posted January 5, 2026 by <b>Alice &amp; Bob</b> (<a href="/Articles/999/">link</a>)</p>
				</summary>
				<div class="FormattedComment"><p>Nice work!</p></div>
			</details>
		</main></div>
	</body></html>`
	lwnRes := NewLWNExtractor(newTestDoc(t, lwnHTML), "https://lwn.net/Articles/100/", nil).Extract()
	assert.Contains(t, lwnRes.ContentHTML, `class="comment-link">2026-01-05</a>`)
	assert.Contains(t, lwnRes.ContentHTML, `<strong>Alice &amp; Bob</strong>`)
	assert.NotContains(t, lwnRes.ContentHTML, `&amp;amp;`)
}

// C6-07: Medium getDescription returns plain text (no HTML entities) and accepts non-Latin text.
func TestRegression_C6_07_MediumDescriptionUnicodeAndPlainText(t *testing.T) {
	t.Parallel()

	html := `<html>
	<head><meta property="og:site_name" content="Medium"></head>
	<body><article>
		<h1>日本語テスト</h1>
		<p>こんにちは世界 &amp; Go言語の記事です。</p>
	</article></body></html>`
	ext := NewMediumExtractor(newTestDoc(t, html), "https://medium.com/p/1", nil)
	require.True(t, ext.CanExtract())
	res := ext.Extract()
	assert.Equal(t, "こんにちは世界 & Go言語の記事です。", res.Variables["description"])
}

// C5-13 & C6-09: Multi-byte UTF-8 truncation must always produce valid UTF-8.
func TestRegression_C5_13_C6_09_UTF8SafeTruncation(t *testing.T) {
	t.Parallel()

	longCJK := strings.Repeat("日本語テスト", 50) // 300 runes, 900 bytes

	// Twitter
	twDoc := newTestDoc(t, fmt.Sprintf(`<html><body><article data-testid="tweet"><div data-testid="tweetText">%s</div></article></body></html>`, longCJK))
	twDesc := NewTwitterExtractor(twDoc, "https://x.com/u/status/1", nil).Extract().Variables["description"]
	assert.True(t, utf8.ValidString(twDesc))
	assert.LessOrEqual(t, len([]rune(twDesc)), 140)

	// XArticle
	xaDoc := newTestDoc(t, fmt.Sprintf(`<html><body><div data-testid="twitterArticleRichTextView"><p>%s</p></div></body></html>`, longCJK))
	xaDesc := NewXArticleExtractor(xaDoc, "https://x.com/u/article/1", nil).Extract().Variables["description"]
	assert.True(t, utf8.ValidString(xaDesc))

	// Reddit
	rdDoc := newTestDoc(t, fmt.Sprintf(`<html><body><shreddit-post author="u"><div slot="text-body"><p>%s</p></div></shreddit-post></body></html>`, longCJK))
	rdDesc := NewRedditExtractor(rdDoc, "https://reddit.com/r/t/comments/1/p/", nil).Extract().Variables["description"]
	assert.True(t, utf8.ValidString(rdDesc))
	assert.LessOrEqual(t, len([]rune(rdDesc)), 140)

	// YouTube
	ytExt := NewYouTubeExtractor(newTestDoc(t, "<html><body></body></html>"), "https://www.youtube.com/watch?v=abc123", nil)
	ytDesc := ytExt.truncateDescription(longCJK)
	assert.True(t, utf8.ValidString(ytDesc))
	assert.LessOrEqual(t, len([]rune(ytDesc)), 200)

	// GitHub
	ghExt := NewGitHubExtractor(newTestDoc(t, "<html><body></body></html>"), "https://github.com/o/r/issues/1", nil)
	ghDesc := ghExt.createDescription("<p>" + longCJK + "</p>")
	assert.True(t, utf8.ValidString(ghDesc))
	assert.LessOrEqual(t, len([]rune(ghDesc)), 140)

	// HackerNews comment title preview
	hnHTML := fmt.Sprintf(`<html><body><table class="fatitem"><tr class="athing comtr"><td class="default"><span class="comhead"><a class="hnuser">u</a></span><div class="commtext">%s</div></td></tr></table></body></html>`, longCJK)
	hnExt := NewHackerNewsExtractor(newTestDoc(t, hnHTML), "https://news.ycombinator.com/item?id=1", nil)
	hnTitle := hnExt.getPostTitle()
	assert.True(t, utf8.ValidString(hnTitle))
}

// C6-10: Custom-domain Medium and Substack route via catchall without false positives.
func TestRegression_C6_10_CustomDomainMediumAndSubstack(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.initializeBuiltins()

	mediumCustom := newTestDoc(t, `<html><head><meta property="al:android:app_name" content="Medium"></head><body><article><p>Post</p></article></body></html>`)
	ext := r.FindExtractor(mediumCustom, "https://towardsdatascience.com/some-post-123", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "MediumExtractor", ext.Name())

	substackCustom := newTestDoc(t, `<html><head><meta name="generator" content="Substack"></head><body><div class="body markup"><p>Newsletter</p></div></body></html>`)
	ext = r.FindExtractor(substackCustom, "https://www.stratechery-like.example/p/issue-1", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "SubstackExtractor", ext.Name())

	// Arbitrary blog with .post-content must NOT match Substack catchall.
	genericBlog := newTestDoc(t, `<html><body><div class="post-content"><p>WordPress blog</p></div></body></html>`)
	assert.Nil(t, r.FindExtractor(genericBlog, "https://mywordpressblog.example/2026/hello", nil))
}
