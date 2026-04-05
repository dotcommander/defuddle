package extractors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// geminiConversationHTML is a minimal Gemini conversation page with one
// user query and one model response.
const geminiConversationHTML = `<html>
<head><title>My AI Conversation</title></head>
<body>
  <div class="conversation-container">
    <user-query>
      <div class="query-text">What is the capital of France?</div>
    </user-query>
    <model-response>
      <div class="model-response-text">
        <div class="markdown"><p>The capital of France is Paris.</p></div>
      </div>
    </model-response>
  </div>
</body>
</html>`

const geminiNoContainersHTML = `<html>
<head><title>Gemini</title></head>
<body><p>Nothing here.</p></body>
</html>`

func TestGeminiExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/abc", nil)
	assert.Equal(t, "GeminiExtractor", ext.Name())
}

func TestGeminiExtractor_CanExtract_WithContainers(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, geminiConversationHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/abc123", nil)
	assert.True(t, ext.CanExtract())
}

func TestGeminiExtractor_CanExtract_NoContainers(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, geminiNoContainersHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/xyz", nil)
	assert.False(t, ext.CanExtract())
}

func TestGeminiExtractor_CanExtract_URLVariants(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://gemini.google.com/app/abc123",
		"https://gemini.google.com/app/def456",
	}

	for _, u := range urls {
		t.Run(u, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, geminiConversationHTML)
			ext := NewGeminiExtractor(doc, u, nil)
			// Gating is DOM-based, not URL-based.
			assert.True(t, ext.CanExtract())
		})
	}
}

func TestGeminiExtractor_Extract_BasicConversation(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, geminiConversationHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/abc123", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "What is the capital of France?")
	assert.Contains(t, result.ContentHTML, "Paris")
}

func TestGeminiExtractor_Extract_EmptyPage(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, geminiNoContainersHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/empty", nil)

	result := ext.Extract()
	require.NotNil(t, result)
}

func TestGeminiExtractor_ExtractMessages_UserAndModel(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, geminiConversationHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/abc123", nil)

	messages := ext.ExtractMessages()

	require.Len(t, messages, 2)

	assert.Equal(t, "You", messages[0].Author)
	assert.Equal(t, "user", messages[0].Metadata["role"])
	assert.Contains(t, messages[0].Content, "What is the capital of France?")

	assert.Equal(t, "Gemini", messages[1].Author)
	assert.Equal(t, "assistant", messages[1].Metadata["role"])
	assert.Contains(t, messages[1].Content, "Paris")
}

func TestGeminiExtractor_ExtractMessages_MultipleContainers(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Chat</title></head><body>
	  <div class="conversation-container">
	    <user-query><div class="query-text">Question 1</div></user-query>
	    <model-response>
	      <div class="model-response-text"><div class="markdown"><p>Answer 1</p></div></div>
	    </model-response>
	  </div>
	  <div class="conversation-container">
	    <user-query><div class="query-text">Question 2</div></user-query>
	    <model-response>
	      <div class="model-response-text"><div class="markdown"><p>Answer 2</p></div></div>
	    </model-response>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/multi", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 4)

	assert.Contains(t, messages[0].Content, "Question 1")
	assert.Contains(t, messages[1].Content, "Answer 1")
	assert.Contains(t, messages[2].Content, "Question 2")
	assert.Contains(t, messages[3].Content, "Answer 2")
}

func TestGeminiExtractor_ExtractMessages_ExtendedResponsePreferred(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Chat</title></head><body>
	  <div class="conversation-container">
	    <user-query><div class="query-text">Tell me more</div></user-query>
	    <model-response>
	      <div class="model-response-text">
	        <div class="markdown"><p>Short answer</p></div>
	      </div>
	      <div id="extended-response-markdown-content"><p>Extended answer</p></div>
	    </model-response>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/extended", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 2)

	// Extended content should be preferred over regular content.
	assert.Contains(t, messages[1].Content, "Extended answer")
	assert.NotContains(t, messages[1].Content, "Short answer")
}

func TestGeminiExtractor_ExtractMessages_RemovesTableContentClass(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Chat</title></head><body>
	  <div class="conversation-container">
	    <model-response>
	      <div class="model-response-text">
	        <div class="markdown">
	          <div class="table-content"><p>Real table data</p></div>
	        </div>
	      </div>
	    </model-response>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/table", nil)

	messages := ext.ExtractMessages()
	require.Len(t, messages, 1)

	// table-content class should be removed but the element content kept.
	assert.NotContains(t, messages[0].Content, "table-content")
	assert.Contains(t, messages[0].Content, "Real table data")
}

func TestGeminiExtractor_GetMetadata_Title(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, geminiConversationHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/abc123", nil)

	meta := ext.GetMetadata()

	// Page title doesn't contain "Gemini" so it should be used directly.
	assert.Equal(t, "My AI Conversation", meta.Title)
	assert.Equal(t, "Gemini", meta.Site)
	assert.Equal(t, "https://gemini.google.com/app/abc123", meta.URL)
}

func TestGeminiExtractor_GetMetadata_TitleFallsBackToTitleText(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Gemini</title></head><body>
	  <span class="title-text">Research: Quantum Computing</span>
	  <div class="conversation-container">
	    <user-query><div class="query-text">Query</div></user-query>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/research", nil)

	meta := ext.GetMetadata()
	assert.Equal(t, "Research: Quantum Computing", meta.Title)
}

func TestGeminiExtractor_GetMetadata_TitleFallsBackToFirstQuery(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Gemini</title></head><body>
	  <div class="conversation-container">
	    <user-query><div class="query-text">Short query</div></user-query>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/fallback", nil)

	meta := ext.GetMetadata()
	assert.Equal(t, "Short query", meta.Title)
}

func TestGeminiExtractor_GetMetadata_TitleDefaultWhenEmpty(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, geminiNoContainersHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/none", nil)

	meta := ext.GetMetadata()
	assert.Equal(t, "Gemini Conversation", meta.Title)
}

func TestGeminiExtractor_GetMetadata_TruncatesLongQuery(t *testing.T) {
	t.Parallel()

	longQuery := "This is a very long first user query that definitely exceeds the fifty character truncation limit set in the extractor"
	html := `<html><head><title>Gemini</title></head><body>
	  <div class="conversation-container">
	    <user-query><div class="query-text">` + longQuery + `</div></user-query>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/long", nil)

	meta := ext.GetMetadata()
	assert.LessOrEqual(t, len(meta.Title), 53) // 50 chars + "..."
	assert.Contains(t, meta.Title, "...")
}

func TestGeminiExtractor_GetMetadata_MessageCountAfterExtract(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, geminiConversationHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/count", nil)

	// Call Extract first to set the internal message count.
	result := ext.Extract()
	require.NotNil(t, result)

	meta := ext.GetMetadata()
	assert.Equal(t, 2, meta.MessageCount)
}

func TestGeminiExtractor_GetMetadata_MessageCountBeforeExtract(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, geminiConversationHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/count2", nil)

	// GetMetadata before Extract should still compute the count.
	meta := ext.GetMetadata()
	assert.Equal(t, 2, meta.MessageCount)
}

func TestGeminiExtractor_GetFootnotes_Empty(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, geminiConversationHTML)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/abc", nil)

	footnotes := ext.GetFootnotes()
	assert.Empty(t, footnotes)
}

func TestGeminiExtractor_ExtractSources_BrowseItems(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Research</title></head><body>
	  <browse-item>
	    <a href="https://example.com/article">
	      <span class="domain">example.com</span>
	      <span class="title">A Great Article</span>
	    </a>
	  </browse-item>
	  <div class="conversation-container">
	    <user-query><div class="query-text">Any sources?</div></user-query>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/sources", nil)

	_ = ext.ExtractMessages()
	footnotes := ext.GetFootnotes()

	require.Len(t, footnotes, 1)
	assert.Equal(t, "https://example.com/article", footnotes[0].URL)
	assert.Contains(t, footnotes[0].Text, "A Great Article")
	assert.Contains(t, footnotes[0].Text, "example.com")
}

func TestGeminiExtractor_ExtractSources_DomainOnlyWhenNoTitle(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Research</title></head><body>
	  <browse-item>
	    <a href="https://example.org/page">
	      <span class="domain">example.org</span>
	    </a>
	  </browse-item>
	  <div class="conversation-container">
	    <user-query><div class="query-text">Sources?</div></user-query>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewGeminiExtractor(doc, "https://gemini.google.com/app/sources2", nil)

	_ = ext.ExtractMessages()
	footnotes := ext.GetFootnotes()

	require.Len(t, footnotes, 1)
	assert.Equal(t, "example.org", footnotes[0].Text)
}
