package extractors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grokConversationHTML is a minimal Grok conversation page with one user
// message and one assistant message using the CSS classes the extractor relies on.
const grokConversationHTML = `<html>
<head><title>How do I learn Go? - Grok</title></head>
<body>
  <div class="relative group flex flex-col justify-center w-full items-end">
    <div class="message-bubble">How do I learn Go?</div>
  </div>
  <div class="relative group flex flex-col justify-center w-full items-start">
    <div class="message-bubble"><p>Start with the official tour at golang.org.</p></div>
  </div>
</body>
</html>`

const grokNoMessagesHTML = `<html>
<head><title>Grok</title></head>
<body><p>No conversation here.</p></body>
</html>`

func TestGrokExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewGrokExtractor(doc, "https://grok.com/chat/123", nil)
	assert.Equal(t, "GrokExtractor", ext.Name())
}

func TestGrokExtractor_CanExtract_WithMessages(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, grokConversationHTML)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/123", nil)
	assert.True(t, ext.CanExtract())
}

func TestGrokExtractor_CanExtract_NoMessages(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, grokNoMessagesHTML)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/abc", nil)
	assert.False(t, ext.CanExtract())
}

func TestGrokExtractor_CanExtract_URLVariants(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://grok.com/chat/123",
		"https://grok.x.ai/conversation/abc",
		"https://x.ai/grok/share/xyz",
	}

	for _, u := range urls {
		u := u
		t.Run(u, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, grokConversationHTML)
			ext := NewGrokExtractor(doc, u, nil)
			// The extractor gates on DOM presence, not URL — so all should extract
			// when the HTML contains message bubbles.
			assert.True(t, ext.CanExtract())
		})
	}
}

func TestGrokExtractor_Extract_BasicConversation(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, grokConversationHTML)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/123", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "How do I learn Go?")
	assert.Contains(t, result.ContentHTML, "golang.org")
}

func TestGrokExtractor_Extract_NoMessages(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, grokNoMessagesHTML)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/empty", nil)

	result := ext.Extract()
	// Extract returns a result even with no messages — just empty content.
	require.NotNil(t, result)
}

func TestGrokExtractor_ExtractMessages_Roles(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, grokConversationHTML)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/123", nil)

	messages := ext.ExtractMessages()

	require.Len(t, messages, 2)

	assert.Equal(t, "You", messages[0].Author)
	assert.Equal(t, "user", messages[0].Metadata["role"])
	assert.Contains(t, messages[0].Content, "How do I learn Go?")

	assert.Equal(t, "Grok", messages[1].Author)
	assert.Equal(t, "assistant", messages[1].Metadata["role"])
	assert.Contains(t, messages[1].Content, "golang.org")
}

func TestGrokExtractor_ExtractMessages_SkipsNonMessages(t *testing.T) {
	t.Parallel()

	// Container with neither items-end nor items-start should be skipped.
	html := `<html><body>
	  <div class="relative group flex flex-col justify-center w-full">
	    <div class="message-bubble">Should be skipped</div>
	  </div>
	  <div class="relative group flex flex-col justify-center w-full items-end">
	    <div class="message-bubble">Valid user message</div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/skip", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].Content, "Valid user message")
}

func TestGrokExtractor_ExtractMessages_SkipsMissingBubble(t *testing.T) {
	t.Parallel()

	// Container with items-end but no .message-bubble child should be skipped.
	html := `<html><body>
	  <div class="relative group flex flex-col justify-center w-full items-end">
	    <div class="other-class">No bubble here</div>
	  </div>
	  <div class="relative group flex flex-col justify-center w-full items-end">
	    <div class="message-bubble">Real message</div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/bubble", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].Content, "Real message")
}

func TestGrokExtractor_GetMetadata_Title(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, grokConversationHTML)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/123", nil)

	meta := ext.GetMetadata()

	// Title should strip the " - Grok" suffix.
	assert.Equal(t, "How do I learn Go?", meta.Title)
	assert.Equal(t, "Grok", meta.Site)
	assert.Equal(t, "https://grok.com/chat/123", meta.URL)
}

func TestGrokExtractor_GetMetadata_DefaultTitle_WhenPageTitleIsGrok(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Grok</title></head><body>
	  <div class="relative group flex flex-col justify-center w-full items-end">
	    <div class="message-bubble">Short query</div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/123", nil)

	meta := ext.GetMetadata()
	// Falls back to first user bubble text.
	assert.Equal(t, "Short query", meta.Title)
}

func TestGrokExtractor_GetMetadata_DefaultTitle_FallbackWhenNoTitle(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Grok</title></head><body>
	  <div class="relative group flex flex-col justify-center w-full items-start">
	    <div class="message-bubble">Only assistant</div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/fall", nil)

	meta := ext.GetMetadata()
	assert.Equal(t, "Grok Conversation", meta.Title)
}

func TestGrokExtractor_GetMetadata_TruncatesLongUserMessage(t *testing.T) {
	t.Parallel()

	longMsg := "This is a very long first user message that exceeds the fifty character limit for title truncation"
	html := `<html><head><title>Grok</title></head><body>
	  <div class="relative group flex flex-col justify-center w-full items-end">
	    <div class="message-bubble">` + longMsg + `</div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/long", nil)

	meta := ext.GetMetadata()
	assert.LessOrEqual(t, len(meta.Title), 53) // 50 chars + "..."
	assert.Contains(t, meta.Title, "...")
}

func TestGrokExtractor_GetMetadata_MessageCount(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, grokConversationHTML)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/123", nil)

	meta := ext.GetMetadata()
	// MessageCount is the count of message container elements found, not parsed messages.
	assert.Equal(t, 2, meta.MessageCount)
}

func TestGrokExtractor_GetFootnotes_Empty(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, grokConversationHTML)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/123", nil)

	footnotes := ext.GetFootnotes()
	assert.Empty(t, footnotes)
}

func TestGrokExtractor_ProcessFootnotes_CreatesFootnoteForHTTPLink(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div class="relative group flex flex-col justify-center w-full items-start">
	    <div class="message-bubble">
	      <a href="https://golang.org">Go website</a>
	    </div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/links", nil)

	messages := ext.ExtractMessages()

	// The link should be replaced with a footnote superscript reference.
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].Content, `class="footnote-ref"`)

	footnotes := ext.GetFootnotes()
	require.Len(t, footnotes, 1)
	assert.Equal(t, "https://golang.org", footnotes[0].URL)
}

func TestGrokExtractor_ProcessFootnotes_SkipsAnchorLinks(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div class="relative group flex flex-col justify-center w-full items-start">
	    <div class="message-bubble">
	      See <a href="#section1">section 1</a> for details.
	    </div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/anchor", nil)

	_ = ext.ExtractMessages()
	footnotes := ext.GetFootnotes()
	assert.Empty(t, footnotes)
}

func TestGrokExtractor_ProcessFootnotes_DeduplicatesURL(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div class="relative group flex flex-col justify-center w-full items-start">
	    <div class="message-bubble">
	      Visit <a href="https://golang.org">Go</a> and again <a href="https://golang.org">Go again</a>.
	    </div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGrokExtractor(doc, "https://grok.com/chat/dup", nil)

	_ = ext.ExtractMessages()
	footnotes := ext.GetFootnotes()
	// Same URL should appear only once.
	assert.Len(t, footnotes, 1)
}
