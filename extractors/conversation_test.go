package extractors

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConversation implements ConversationExtractor for testing.
type mockConversation struct {
	*ConversationExtractorBase
	messages  []ConversationMessage
	metadata  ConversationMetadata
	footnotes []Footnote
}

func (m *mockConversation) CanExtract() bool                       { return true }
func (m *mockConversation) Name() string                           { return "MockConversation" }
func (m *mockConversation) Extract() *ExtractorResult              { return m.ExtractWithDefuddle(m) }
func (m *mockConversation) ExtractMessages() []ConversationMessage { return m.messages }
func (m *mockConversation) GetMetadata() ConversationMetadata      { return m.metadata }
func (m *mockConversation) GetFootnotes() []Footnote               { return m.footnotes }

func newMockConversation(t *testing.T, msgs []ConversationMessage, meta ConversationMetadata, footnotes []Footnote) *mockConversation {
	t.Helper()
	doc := newTestDoc(t, "<html><body></body></html>")
	base := NewConversationExtractorBase(doc, "https://example.com/chat", nil)
	return &mockConversation{
		ConversationExtractorBase: base,
		messages:                  msgs,
		metadata:                  meta,
		footnotes:                 footnotes,
	}
}

func TestCreateContentHTML_BasicMessages(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	base := NewConversationExtractorBase(doc, "https://example.com", nil)

	msgs := []ConversationMessage{
		{Author: "User", Content: "Hello"},
		{Author: "Assistant", Content: "Hi there"},
	}

	html := base.CreateContentHTML(msgs, nil)

	assert.Contains(t, html, `class="message message-user"`)
	assert.Contains(t, html, `class="message message-assistant"`)
	assert.Contains(t, html, "<strong>User</strong>")
	assert.Contains(t, html, "<strong>Assistant</strong>")
	assert.Contains(t, html, "<p>Hello</p>")
	assert.Contains(t, html, "<p>Hi there</p>")
	assert.Contains(t, html, "<hr>")
}

func TestCreateContentHTML_WithTimestamps(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	base := NewConversationExtractorBase(doc, "https://example.com", nil)

	msgs := []ConversationMessage{
		{Author: "User", Content: "Hello", Timestamp: "2024-01-15 10:30"},
	}

	html := base.CreateContentHTML(msgs, nil)
	assert.Contains(t, html, `class="message-timestamp"`)
	assert.Contains(t, html, "2024-01-15 10:30")
}

func TestCreateContentHTML_WithoutTimestamp(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	base := NewConversationExtractorBase(doc, "https://example.com", nil)

	msgs := []ConversationMessage{
		{Author: "User", Content: "Hello"},
	}

	html := base.CreateContentHTML(msgs, nil)
	assert.NotContains(t, html, `class="message-timestamp"`)
}

func TestCreateContentHTML_ExistingParagraphs(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	base := NewConversationExtractorBase(doc, "https://example.com", nil)

	msgs := []ConversationMessage{
		{Author: "User", Content: "<p>Already wrapped</p>"},
	}

	html := base.CreateContentHTML(msgs, nil)
	assert.NotContains(t, html, "<p><p>")
	assert.Contains(t, html, "<p>Already wrapped</p>")
}

func TestCreateContentHTML_WithFootnotes(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	base := NewConversationExtractorBase(doc, "https://example.com", nil)

	msgs := []ConversationMessage{{Author: "Bot", Content: "See footnote"}}
	footnotes := []Footnote{
		{URL: "https://example.com/ref", Text: "Reference 1"},
		{URL: "https://example.com/ref2", Text: "Reference 2"},
	}

	html := base.CreateContentHTML(msgs, footnotes)
	assert.Contains(t, html, `id="footnotes"`)
	assert.Contains(t, html, `id="fn:1"`)
	assert.Contains(t, html, `id="fn:2"`)
	assert.Contains(t, html, `href="https://example.com/ref"`)
	assert.Contains(t, html, "Reference 1")
	assert.Contains(t, html, `class="footnote-backref"`)
}

func TestCreateContentHTML_WithMetadata(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	base := NewConversationExtractorBase(doc, "https://example.com", nil)

	msgs := []ConversationMessage{
		{Author: "User", Content: "Hello", Metadata: map[string]any{"role": "user", "index": 0}},
	}

	html := base.CreateContentHTML(msgs, nil)
	assert.Contains(t, html, `data-role="user"`)
	assert.Contains(t, html, `data-index="0"`)
}

func TestExtractWithDefuddle_NoProcessor(t *testing.T) {
	t.Parallel()
	mock := newMockConversation(t,
		[]ConversationMessage{{Author: "User", Content: "test content"}},
		ConversationMetadata{Title: "Chat", Site: "TestSite"},
		nil,
	)

	result := mock.Extract()
	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "test content")
	assert.Equal(t, "Chat", result.Variables["title"])
	assert.Equal(t, "TestSite", result.Variables["site"])
	assert.Equal(t, "1", result.ExtractedContent["messageCount"])
}

func TestExtractWithDefuddle_DefaultDescription(t *testing.T) {
	t.Parallel()
	mock := newMockConversation(t,
		[]ConversationMessage{{Author: "A", Content: "x"}, {Author: "B", Content: "y"}},
		ConversationMetadata{Title: "Chat", Site: "TestSite"},
		nil,
	)

	result := mock.Extract()
	assert.Equal(t, "TestSite conversation with 2 messages", result.Variables["description"])
}

func TestExtractWithDefuddle_CustomDescription(t *testing.T) {
	t.Parallel()
	mock := newMockConversation(t,
		[]ConversationMessage{{Author: "A", Content: "x"}},
		ConversationMetadata{Title: "Chat", Site: "TestSite", Description: "Custom desc"},
		nil,
	)

	result := mock.Extract()
	assert.Equal(t, "Custom desc", result.Variables["description"])
}

func TestExtractWithDefuddle_WithProcessor(t *testing.T) {
	t.Parallel()
	mock := newMockConversation(t,
		[]ConversationMessage{{Author: "User", Content: "hello world"}},
		ConversationMetadata{Title: "Chat", Site: "TestSite"},
		nil,
	)

	mock.SetContentProcessor(func(html string) (*ContentProcessResult, error) {
		return &ContentProcessResult{
			Content:   strings.ToUpper(html),
			WordCount: 42,
		}, nil
	})

	result := mock.Extract()
	require.NotNil(t, result)
	assert.Equal(t, result.ContentHTML, strings.ToUpper(result.ContentHTML))
	assert.Equal(t, "42", result.Variables["wordCount"])
}

func TestSetContentProcessor(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	base := NewConversationExtractorBase(doc, "https://example.com", nil)
	assert.Nil(t, base.contentProcessor)

	base.SetContentProcessor(func(html string) (*ContentProcessResult, error) {
		return &ContentProcessResult{Content: html}, nil
	})
	assert.NotNil(t, base.contentProcessor)
}
