package extractors

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chatgptBasicHTML is a realistic ChatGPT page with two conversation turns.
const chatgptBasicHTML = `<html>
<head><title>What is Go? - ChatGPT</title></head>
<body>
	<article data-testid="conversation-turn-1" data-message-author-role="user">
		<h5 class="sr-only">You:</h5>
		<div class="text-message"><p>What is Go?</p></div>
	</article>
	<article data-testid="conversation-turn-2" data-message-author-role="assistant">
		<h5 class="sr-only">ChatGPT:</h5>
		<div><p>Go is a statically typed, compiled programming language.</p></div>
	</article>
</body>
</html>`

// chatgptNoArticlesHTML has no conversation-turn articles.
const chatgptNoArticlesHTML = `<html>
<head><title>ChatGPT</title></head>
<body><p>No conversation here.</p></body>
</html>`

// chatgptCitationHTML contains an assistant message with a citation link.
const chatgptCitationHTML = `<html>
<head><title>Research - ChatGPT</title></head>
<body>
	<article data-testid="conversation-turn-1" data-message-author-role="user">
		<h5 class="sr-only">You:</h5>
		<div class="text-message"><p>Tell me about Go.</p></div>
	</article>
	<article data-testid="conversation-turn-2" data-message-author-role="assistant">
		<h6 class="sr-only">ChatGPT:</h6>
		<div>
			<p>Go was designed at Google.
			<span><a href="https://go.dev/about" target="_blank" rel="noopener">go.dev</a></span>
			It is fast and simple.</p>
		</div>
	</article>
</body>
</html>`

// chatgptDuplicateCitationHTML has the same URL cited twice — should share a footnote.
const chatgptDuplicateCitationHTML = `<html>
<head><title>Dedup - ChatGPT</title></head>
<body>
	<article data-testid="conversation-turn-1" data-message-author-role="assistant">
		<h5 class="sr-only">ChatGPT:</h5>
		<div>
			<p>First mention
			<span><a href="https://example.com/page" target="_blank" rel="noopener">example.com</a></span>
			and second mention
			<span><a href="https://example.com/page" target="_blank" rel="noopener">example.com</a></span>
			of the same source.</p>
		</div>
	</article>
</body>
</html>`

// chatgptCleanupHTML has sr-only headings and data-state="closed" spans that should be removed.
const chatgptCleanupHTML = `<html>
<head><title>Cleanup - ChatGPT</title></head>
<body>
	<article data-testid="conversation-turn-1" data-message-author-role="user">
		<h5 class="sr-only">You:</h5>
		<div class="text-message">
			<span data-state="closed">hidden tooltip</span>
			<p>Visible content only.</p>
		</div>
	</article>
</body>
</html>`

// chatgptDefaultTitleHTML uses the generic "ChatGPT" title, triggering fallback title logic.
const chatgptDefaultTitleHTML = `<html>
<head><title>ChatGPT</title></head>
<body>
	<article data-testid="conversation-turn-1" data-message-author-role="user">
		<h5 class="sr-only">You:</h5>
		<div class="text-message"><p>Short question</p></div>
	</article>
</body>
</html>`

// chatgptLongFirstMessageHTML triggers the 50-char truncation in getTitle.
const chatgptLongFirstMessageHTML = `<html>
<head><title>ChatGPT</title></head>
<body>
	<article data-testid="conversation-turn-1" data-message-author-role="user">
		<h5 class="sr-only">You:</h5>
		<div class="text-message"><p>This is a very long first message that exceeds fifty characters for sure</p></div>
	</article>
</body>
</html>`

// chatgptNoTitleFallbackHTML has the default title but also no first user message class.
const chatgptNoTitleFallbackHTML = `<html>
<head><title>ChatGPT</title></head>
<body>
	<article data-testid="conversation-turn-1" data-message-author-role="user">
		<h5 class="sr-only">You:</h5>
		<div><p>No text-message class here.</p></div>
	</article>
</body>
</html>`

// chatgptFragmentCitationHTML contains a URL with a #:~:text= fragment.
const chatgptFragmentCitationHTML = `<html>
<head><title>Fragment - ChatGPT</title></head>
<body>
	<article data-testid="conversation-turn-1" data-message-author-role="assistant">
		<h5 class="sr-only">ChatGPT:</h5>
		<div>
			<p>See this excerpt
			<span><a href="https://example.com/doc#:~:text=hello%20world" target="_blank" rel="noopener">example.com</a></span>
			for details.</p>
		</div>
	</article>
</body>
</html>`

// --- CanExtract ---

func TestChatGPTExtractor_CanExtract_True(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)
	assert.True(t, ext.CanExtract())
}

func TestChatGPTExtractor_CanExtract_False_NoArticles(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptNoArticlesHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)
	assert.False(t, ext.CanExtract())
}

func TestChatGPTExtractor_CanExtract_False_EmptyDoc(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/empty", nil)
	assert.False(t, ext.CanExtract())
}

// --- Name ---

func TestChatGPTExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewChatGPTExtractor(doc, "", nil)
	assert.Equal(t, "ChatGPTExtractor", ext.Name())
}

// --- Extract ---

func TestChatGPTExtractor_Extract_BasicConversation(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// Both message authors should appear.
	assert.Contains(t, result.ContentHTML, "You")
	assert.Contains(t, result.ContentHTML, "ChatGPT")

	// Both messages should appear in content.
	assert.Contains(t, result.ContentHTML, "What is Go?")
	assert.Contains(t, result.ContentHTML, "statically typed")

	// Metadata variables.
	assert.Equal(t, "What is Go? - ChatGPT", result.Variables["title"])
	assert.Equal(t, "ChatGPT", result.Variables["site"])
	assert.Equal(t, "2", result.ExtractedContent["messageCount"])
}

func TestChatGPTExtractor_Extract_SeparatorBetweenMessages(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// Consecutive messages are separated by <hr>.
	assert.Contains(t, result.ContentHTML, "<hr>")
}

func TestChatGPTExtractor_Extract_DescriptionIncludesMessageCount(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.Variables["description"], "2 messages")
}

// --- ExtractMessages ---

func TestChatGPTExtractor_ExtractMessages_RoleAssignment(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 2)

	assert.Equal(t, "user", messages[0].Metadata["role"])
	assert.Equal(t, "assistant", messages[1].Metadata["role"])
}

func TestChatGPTExtractor_ExtractMessages_AuthorText(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 2)

	// sr-only headings provide the author text; trailing colon must be stripped.
	assert.Equal(t, "You", messages[0].Author)
	assert.Equal(t, "ChatGPT", messages[1].Author)
}

func TestChatGPTExtractor_ExtractMessages_ContentExtracted(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 2)

	assert.Contains(t, messages[0].Content, "What is Go?")
	assert.Contains(t, messages[1].Content, "statically typed")
}

func TestChatGPTExtractor_ExtractMessages_NoArticles(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptNoArticlesHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/empty", nil)

	messages := ext.ExtractMessages()
	assert.Empty(t, messages)
}

func TestChatGPTExtractor_ExtractMessages_SrOnlyHeadingH6(t *testing.T) {
	t.Parallel()
	// chatgptCitationHTML uses h6.sr-only for the assistant heading.
	doc := newTestDoc(t, chatgptCitationHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 2)
	assert.Equal(t, "ChatGPT", messages[1].Author)
}

func TestChatGPTExtractor_ExtractMessages_CleanupElements(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptCleanupHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 1)

	// data-state="closed" span should be removed.
	assert.NotContains(t, messages[0].Content, "hidden tooltip")
	// sr-only heading should be removed from content too.
	assert.NotContains(t, messages[0].Content, "sr-only")
	// Visible paragraph should remain.
	assert.Contains(t, messages[0].Content, "Visible content only.")
}

func TestChatGPTExtractor_ExtractMessages_UnknownRoleWhenMissing(t *testing.T) {
	t.Parallel()
	html := `<html><body>
		<article data-testid="conversation-turn-1">
			<h5 class="sr-only">Someone:</h5>
			<p>Hello</p>
		</article>
	</body></html>`
	doc := newTestDoc(t, html)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "unknown", messages[0].Metadata["role"])
}

func TestChatGPTExtractor_ExtractMessages_ResetsFootnotesOnEachCall(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptCitationHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	// First call populates footnotes.
	_ = ext.ExtractMessages()
	firstCount := len(ext.footnotes)

	// Second call should reset and produce the same count.
	_ = ext.ExtractMessages()
	secondCount := len(ext.footnotes)

	assert.Equal(t, firstCount, secondCount, "footnote counter should reset between calls")
}

// --- Citations / Footnotes ---

func TestChatGPTExtractor_Extract_CitationBecomesFootnote(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptCitationHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// Footnote reference should be present in the content.
	assert.Contains(t, result.ContentHTML, `href="#fn:1"`)
	// Footnote section should be rendered.
	assert.Contains(t, result.ContentHTML, `id="fn:1"`)
	assert.Contains(t, result.ContentHTML, "go.dev")
	assert.Contains(t, result.ContentHTML, "https://go.dev/about")
}

func TestChatGPTExtractor_GetFootnotes_CitationURL(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptCitationHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	_ = ext.ExtractMessages()
	footnotes := ext.GetFootnotes()

	require.Len(t, footnotes, 1)
	assert.Equal(t, "https://go.dev/about", footnotes[0].URL)
	assert.Contains(t, footnotes[0].Text, "go.dev")
}

func TestChatGPTExtractor_GetFootnotes_DeduplicatesURL(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptDuplicateCitationHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	_ = ext.ExtractMessages()
	footnotes := ext.GetFootnotes()

	// Same URL twice → only one footnote entry.
	require.Len(t, footnotes, 1)
	assert.Equal(t, "https://example.com/page", footnotes[0].URL)
}

func TestChatGPTExtractor_GetFootnotes_DuplicateReusesNumber(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptDuplicateCitationHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 1)

	// Both in-text references should point to fn:1, not fn:1 and fn:2.
	content := messages[0].Content
	assert.Contains(t, content, `href="#fn:1"`)
	assert.NotContains(t, content, `href="#fn:2"`)
}

func TestChatGPTExtractor_Citation_NoCitationLink_NotReplaced(t *testing.T) {
	t.Parallel()
	// A plain link without target="_blank" and rel="noopener" should not become a footnote.
	html := `<html>
<head><title>Plain - ChatGPT</title></head>
<body>
	<article data-testid="conversation-turn-1" data-message-author-role="assistant">
		<h5 class="sr-only">ChatGPT:</h5>
		<div>
			<p>Visit <span><a href="https://example.com">example.com</a></span> for more.</p>
		</div>
	</article>
</body>
</html>`
	doc := newTestDoc(t, html)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	_ = ext.ExtractMessages()
	footnotes := ext.GetFootnotes()

	assert.Empty(t, footnotes, "link without required attributes should not become a footnote")
}

// --- Fragment text in citations ---

func TestChatGPTExtractor_Citation_FragmentTextExtracted(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptFragmentCitationHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	_ = ext.ExtractMessages()
	footnotes := ext.GetFootnotes()

	require.Len(t, footnotes, 1)
	// Fragment text "hello world" should appear with em dash in footnote text.
	assert.Contains(t, footnotes[0].Text, "hello world")
	assert.Contains(t, footnotes[0].Text, "—")
}

// --- GetMetadata ---

func TestChatGPTExtractor_GetMetadata_Site(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	meta := ext.GetMetadata()
	assert.Equal(t, "ChatGPT", meta.Site)
}

func TestChatGPTExtractor_GetMetadata_URL(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	const pageURL = "https://chatgpt.com/c/abc-123"
	ext := NewChatGPTExtractor(doc, pageURL, nil)

	meta := ext.GetMetadata()
	assert.Equal(t, pageURL, meta.URL)
}

func TestChatGPTExtractor_GetMetadata_MessageCount(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	meta := ext.GetMetadata()
	assert.Equal(t, 2, meta.MessageCount)
}

func TestChatGPTExtractor_GetMetadata_DescriptionContainsCount(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	meta := ext.GetMetadata()
	assert.Contains(t, meta.Description, "2 messages")
}

func TestChatGPTExtractor_GetMetadata_TitleFromPageTitle(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptBasicHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	meta := ext.GetMetadata()
	assert.Equal(t, "What is Go? - ChatGPT", meta.Title)
}

func TestChatGPTExtractor_GetMetadata_TitleFallsBackToFirstMessage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptDefaultTitleHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	meta := ext.GetMetadata()
	// Page title is "ChatGPT" (generic), so title should come from first user message.
	assert.Equal(t, "Short question", meta.Title)
}

func TestChatGPTExtractor_GetMetadata_TitleTruncatedAt50Chars(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptLongFirstMessageHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	meta := ext.GetMetadata()
	// Must end with "..." and be no longer than 53 chars (50 + "...").
	assert.True(t, strings.HasSuffix(meta.Title, "..."), "title should end with ellipsis: %q", meta.Title)
	assert.LessOrEqual(t, len(meta.Title), 53)
}

func TestChatGPTExtractor_GetMetadata_TitleFallsBackToDefault(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, chatgptNoTitleFallbackHTML)
	ext := NewChatGPTExtractor(doc, "https://chatgpt.com/c/abc-123", nil)

	meta := ext.GetMetadata()
	// Page title is generic and there is no .text-message element, so the
	// hardcoded default should be used.
	assert.Equal(t, "ChatGPT Conversation", meta.Title)
}

// --- Helper: extractCitationDomain ---

func TestExtractCitationDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		rawURL string
		want   string
	}{
		{"https://www.example.com/path", "example.com"},
		{"https://go.dev/about", "go.dev"},
		{"https://subdomain.site.org/page", "subdomain.site.org"},
		// url.Parse does not error on "not-a-url"; it parses it as a relative
		// path with an empty host, so Hostname() returns "".
		{"not-a-url", ""},
	}

	for _, tc := range cases {
		t.Run(tc.rawURL, func(t *testing.T) {
			t.Parallel()
			got := extractCitationDomain(tc.rawURL)
			assert.Equal(t, tc.want, got)
		})
	}
}

// --- Helper: extractFragmentText ---

func TestExtractFragmentText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "no fragment",
			rawURL: "https://example.com/page",
			want:   "",
		},
		{
			name:   "single word fragment",
			rawURL: "https://example.com/page#:~:text=hello",
			want:   " — hello",
		},
		{
			name:   "comma separated — takes first part",
			rawURL: "https://example.com/page#:~:text=start,end",
			want:   " — start...",
		},
		{
			name:   "url-encoded fragment",
			rawURL: "https://example.com/page#:~:text=hello%20world",
			want:   " — hello world",
		},
		{
			name:   "empty first comma part",
			rawURL: "https://example.com/page#:~:text=,end",
			want:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractFragmentText(tc.rawURL)
			assert.Equal(t, tc.want, got)
		})
	}
}
