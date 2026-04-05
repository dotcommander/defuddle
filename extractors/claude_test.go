package extractors

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// claudeBasicHTML is a minimal but realistic Claude conversation page with
// a page title, a user message (data-testid="user-message"), and an assistant
// response (class="font-claude-response").
const claudeBasicHTML = `<html>
<head>
	<title>How do I reverse a string in Go? - Claude</title>
</head>
<body>
	<div data-testid="user-message">How do I reverse a string in Go?</div>
	<div class="font-claude-response">
		<div class="standard-markdown">
			<p>You can reverse a string in Go by converting it to a rune slice.</p>
			<pre><code>func reverse(s string) string {
    r := []rune(s)
    for i, j := 0, len(r)-1; i &lt; j; i, j = i+1, j-1 {
        r[i], r[j] = r[j], r[i]
    }
    return string(r)
}</code></pre>
		</div>
	</div>
</body>
</html>`

// claudeMultiTurnHTML is a two-turn conversation to verify correct ordering
// and role assignment across multiple exchanges.
const claudeMultiTurnHTML = `<html>
<head>
	<title>Sorting algorithms explained - Claude</title>
</head>
<body>
	<div data-testid="user-message">Explain quicksort briefly.</div>
	<div class="font-claude-response">
		<div class="standard-markdown"><p>Quicksort picks a pivot and partitions the array around it, then recursively sorts each partition.</p></div>
	</div>
	<div data-testid="user-message">What is its average time complexity?</div>
	<div class="font-claude-response">
		<div class="standard-markdown"><p>On average, quicksort runs in O(n log n) time.</p></div>
	</div>
</body>
</html>`

// claudeHeaderTitleHTML uses a header element with .font-tiempos for the title
// instead of a page title. Exercises the second title-resolution path.
const claudeHeaderTitleHTML = `<html>
<head>
	<title>Claude</title>
</head>
<body>
	<header>
		<span class="font-tiempos">My favourite conversation</span>
	</header>
	<div data-testid="user-message">Hello there.</div>
	<div class="font-claude-response">
		<div class="standard-markdown"><p>Hi! How can I help?</p></div>
	</div>
</body>
</html>`

// claudeNoTitleHTML has neither a useful page title nor a header title, so
// getTitle() falls back to the first message text.
const claudeNoTitleHTML = `<html>
<head>
	<title>Claude</title>
</head>
<body>
	<div data-testid="user-message">What is the capital of France?</div>
	<div class="font-claude-response">
		<div class="standard-markdown"><p>Paris is the capital of France.</p></div>
	</div>
</body>
</html>`

// claudeAssistantTestidHTML exercises the extractor's rule that
// data-testid="assistant-message" elements must be skipped (no content
// extracted) because the TypeScript implementation only processes "user-message"
// via data-testid; assistant content comes from .font-claude-response.
const claudeAssistantTestidHTML = `<html>
<head><title>Skipped assistant-message test - Claude</title></head>
<body>
	<div data-testid="user-message">First question.</div>
	<div data-testid="assistant-message"><p>This should be ignored.</p></div>
	<div class="font-claude-response">
		<div class="standard-markdown"><p>This is the real answer.</p></div>
	</div>
</body>
</html>`

// claudeEmptyHTML has no message elements at all.
const claudeEmptyHTML = `<html><head><title>Claude</title></head><body></body></html>`

// claudeLongFirstMessageHTML triggers title truncation (>50 rune text).
const claudeLongFirstMessageHTML = `<html>
<head><title>Claude</title></head>
<body>
	<div data-testid="user-message">This is a very long user message that exceeds fifty characters total length</div>
	<div class="font-claude-response">
		<div class="standard-markdown"><p>Understood.</p></div>
	</div>
</body>
</html>`

// claudeResponseWithoutStandardMarkdownHTML uses .font-claude-response but
// without an inner .standard-markdown child, so the fallback path (full element
// HTML) is exercised.
const claudeResponseWithoutStandardMarkdownHTML = `<html>
<head><title>No inner div - Claude</title></head>
<body>
	<div data-testid="user-message">Simple question?</div>
	<div class="font-claude-response"><p>Direct answer without standard-markdown wrapper.</p></div>
</body>
</html>`

// ---------------------------------------------------------------------------
// CanExtract
// ---------------------------------------------------------------------------

func TestClaudeExtractor_CanExtract_WithMessages(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeBasicHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/abc123", nil)
	assert.True(t, ext.CanExtract())
}

func TestClaudeExtractor_CanExtract_EmptyPage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeEmptyHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/abc123", nil)
	assert.False(t, ext.CanExtract())
}

func TestClaudeExtractor_CanExtract_URLVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{"chat URL", "https://claude.ai/chat/abc123def"},
		{"share URL", "https://claude.ai/share/xyz789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, claudeBasicHTML)
			ext := NewClaudeExtractor(doc, tt.url, nil)
			assert.True(t, ext.CanExtract())
		})
	}
}

// ---------------------------------------------------------------------------
// Name
// ---------------------------------------------------------------------------

func TestClaudeExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeEmptyHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/x", nil)
	assert.Equal(t, "ClaudeExtractor", ext.Name())
}

// ---------------------------------------------------------------------------
// ExtractMessages
// ---------------------------------------------------------------------------

func TestClaudeExtractor_ExtractMessages_BasicConversation(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeBasicHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/abc123", nil)

	msgs := ext.ExtractMessages()

	require.Len(t, msgs, 2)

	// First message — user
	assert.Equal(t, "You", msgs[0].Author)
	assert.Contains(t, msgs[0].Content, "reverse a string in Go")
	assert.Equal(t, "you", msgs[0].Metadata["role"])

	// Second message — assistant
	assert.Equal(t, "Claude", msgs[1].Author)
	assert.Contains(t, msgs[1].Content, "rune slice")
	assert.Equal(t, "assistant", msgs[1].Metadata["role"])
}

func TestClaudeExtractor_ExtractMessages_MultiTurn(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeMultiTurnHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/multi", nil)

	msgs := ext.ExtractMessages()

	require.Len(t, msgs, 4)
	assert.Equal(t, "You", msgs[0].Author)
	assert.Equal(t, "Claude", msgs[1].Author)
	assert.Equal(t, "You", msgs[2].Author)
	assert.Equal(t, "Claude", msgs[3].Author)

	assert.Contains(t, msgs[0].Content, "quicksort")
	assert.Contains(t, msgs[1].Content, "pivot")
	assert.Contains(t, msgs[2].Content, "time complexity")
	assert.Contains(t, msgs[3].Content, "O(n log n)")
}

func TestClaudeExtractor_ExtractMessages_AssistantTestidSkipped(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeAssistantTestidHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/skip", nil)

	msgs := ext.ExtractMessages()

	// The data-testid="assistant-message" div must be skipped; only the
	// user message and the .font-claude-response are included.
	require.Len(t, msgs, 2)
	assert.Equal(t, "You", msgs[0].Author)
	assert.Equal(t, "Claude", msgs[1].Author)
	assert.NotContains(t, msgs[1].Content, "should be ignored")
	assert.Contains(t, msgs[1].Content, "real answer")
}

func TestClaudeExtractor_ExtractMessages_NoMessages(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeEmptyHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/empty", nil)

	msgs := ext.ExtractMessages()
	assert.Empty(t, msgs)
}

func TestClaudeExtractor_ExtractMessages_RoleMetadata(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeBasicHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/roles", nil)

	msgs := ext.ExtractMessages()
	require.Len(t, msgs, 2)

	assert.Equal(t, "you", msgs[0].Metadata["role"])
	assert.Equal(t, "assistant", msgs[1].Metadata["role"])
}

func TestClaudeExtractor_ExtractMessages_FallbackWithoutStandardMarkdown(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeResponseWithoutStandardMarkdownHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/noinner", nil)

	msgs := ext.ExtractMessages()
	require.Len(t, msgs, 2)
	assert.Equal(t, "Claude", msgs[1].Author)
	assert.Contains(t, msgs[1].Content, "Direct answer")
}

// ---------------------------------------------------------------------------
// GetMetadata
// ---------------------------------------------------------------------------

func TestClaudeExtractor_GetMetadata_BasicFields(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeBasicHTML)
	const u = "https://claude.ai/chat/abc123"
	ext := NewClaudeExtractor(doc, u, nil)

	meta := ext.GetMetadata()

	assert.Equal(t, "Claude", meta.Site)
	assert.Equal(t, u, meta.URL)
	assert.Equal(t, 2, meta.MessageCount)
	assert.Contains(t, meta.Description, "2 messages")
}

func TestClaudeExtractor_GetMetadata_TitleFromPageTitle(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeBasicHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/abc123", nil)

	meta := ext.GetMetadata()

	// The page title is "How do I reverse a string in Go? - Claude".
	// The " - Claude" suffix must be stripped.
	assert.Equal(t, "How do I reverse a string in Go?", meta.Title)
	assert.NotContains(t, meta.Title, "- Claude")
}

func TestClaudeExtractor_GetMetadata_TitleFromHeader(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeHeaderTitleHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/header", nil)

	meta := ext.GetMetadata()

	// Page title is just "Claude" so header .font-tiempos is used.
	assert.Equal(t, "My favourite conversation", meta.Title)
}

func TestClaudeExtractor_GetMetadata_TitleFallbackToFirstMessage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeNoTitleHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/notitle", nil)

	meta := ext.GetMetadata()

	// Neither page title nor header title available — falls back to first message.
	assert.Contains(t, meta.Title, "capital of France")
}

func TestClaudeExtractor_GetMetadata_TitleTruncatedAt50(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeLongFirstMessageHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/long", nil)

	meta := ext.GetMetadata()

	// Title must not exceed 50 characters + "..."
	assert.LessOrEqual(t, len(meta.Title), 53) // 50 + len("...")
	assert.True(t, strings.HasSuffix(meta.Title, "..."))
}

func TestClaudeExtractor_GetMetadata_MessageCount(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeMultiTurnHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/multi", nil)

	meta := ext.GetMetadata()

	assert.Equal(t, 4, meta.MessageCount)
	assert.Contains(t, meta.Description, "4 messages")
}

func TestClaudeExtractor_GetMetadata_EmptyPage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeEmptyHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/empty", nil)

	meta := ext.GetMetadata()

	assert.Equal(t, 0, meta.MessageCount)
	assert.Equal(t, "Claude", meta.Site)
	// Falls back to "Claude Conversation" when there are no messages and no
	// useful page title.
	assert.Equal(t, "Claude Conversation", meta.Title)
}

// ---------------------------------------------------------------------------
// Extract (full pipeline)
// ---------------------------------------------------------------------------

func TestClaudeExtractor_Extract_ReturnsResult(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeBasicHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/abc123", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "reverse a string")
	assert.Contains(t, result.ContentHTML, "rune slice")
}

func TestClaudeExtractor_Extract_Variables(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeBasicHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/abc123", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "How do I reverse a string in Go?", result.Variables["title"])
	assert.Equal(t, "Claude", result.Variables["site"])
	assert.Equal(t, "2", result.ExtractedContent["messageCount"])
}

func TestClaudeExtractor_Extract_MultiTurnContentOrder(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeMultiTurnHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/multi", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// All four messages must appear in document order within the HTML.
	quicksortIdx := strings.Index(result.ContentHTML, "quicksort")
	pivotIdx := strings.Index(result.ContentHTML, "pivot")
	complexityIdx := strings.Index(result.ContentHTML, "time complexity")
	bigOIdx := strings.Index(result.ContentHTML, "O(n log n)")

	require.Greater(t, quicksortIdx, -1, "expected quicksort in output")
	require.Greater(t, pivotIdx, -1, "expected pivot in output")
	require.Greater(t, complexityIdx, -1, "expected time complexity in output")
	require.Greater(t, bigOIdx, -1, "expected O(n log n) in output")

	assert.Less(t, quicksortIdx, pivotIdx)
	assert.Less(t, pivotIdx, complexityIdx)
	assert.Less(t, complexityIdx, bigOIdx)
}

func TestClaudeExtractor_Extract_EmptyPage(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, claudeEmptyHTML)
	ext := NewClaudeExtractor(doc, "https://claude.ai/chat/empty", nil)

	// Extract should not panic on an empty page.
	result := ext.Extract()
	require.NotNil(t, result)
}
