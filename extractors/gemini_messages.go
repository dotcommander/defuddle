package extractors

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractMessages extracts conversation messages
// TypeScript original code:
//
//	protected extractMessages(): ConversationMessage[] {
//		this.messageCount = 0;
//		const messages: ConversationMessage[] = [];
//
//		if (!this.conversationContainers) return messages;
//
//		this.extractSources();
//
//		this.conversationContainers.forEach((container) => {
//			const userQuery = container.querySelector('user-query');
//			if (userQuery) {
//				const queryText = userQuery.querySelector('.query-text');
//				if (queryText) {
//					const content = queryText.innerHTML || '';
//					messages.push({
//						author: 'You',
//						content: content.trim(),
//						metadata: { role: 'user' }
//					});
//				}
//			}
//
//			const modelResponse = container.querySelector('model-response');
//			if (modelResponse) {
//				const regularContent = modelResponse.querySelector('.model-response-text .markdown');
//				const extendedContent = modelResponse.querySelector('#extended-response-markdown-content');
//				const contentElement = extendedContent || regularContent;
//
//				if (contentElement) {
//					let content = contentElement.innerHTML || '';
//
//					const tempDiv = document.createElement('div');
//					tempDiv.innerHTML = content;
//
//					tempDiv.querySelectorAll('.table-content').forEach(el => {
//						// `table-content` is a PARTIAL selector in defuddle (table of contents, will be removed), but a real table in Gemini (should be kept).
//						el.classList.remove('table-content');
//					});
//
//					content = tempDiv.innerHTML;
//
//					messages.push({
//						author: 'Gemini',
//						content: content.trim(),
//						metadata: { role: 'assistant' }
//					});
//				}
//			}
//		});
//		this.messageCount = messages.length;
//		return messages;
//	}
func (g *GeminiExtractor) ExtractMessages() []ConversationMessage {
	messageCount := 0
	g.messageCount = &messageCount
	var messages []ConversationMessage

	if g.conversationContainers.Length() == 0 {
		slog.Debug("No conversation containers found for Gemini extraction")
		return messages
	}

	// Extract sources first (for footnotes)
	g.extractSources()

	g.conversationContainers.Each(func(_ int, container *goquery.Selection) {
		if msg := geminiUserMessage(container); msg != nil {
			messages = append(messages, *msg)
		}
		if msg := g.geminiModelMessage(container); msg != nil {
			messages = append(messages, *msg)
		}
	})

	*g.messageCount = len(messages)
	slog.Debug("Gemini messages extracted", "messageCount", len(messages), "footnoteCount", len(g.footnotes))
	return messages
}

// geminiUserMessage extracts the user-query message from a conversation container,
// or nil when absent or empty.
func geminiUserMessage(container *goquery.Selection) *ConversationMessage {
	userQuery := container.Find("user-query").First()
	if userQuery.Length() == 0 {
		return nil
	}
	queryText := userQuery.Find(".query-text").First()
	if queryText.Length() == 0 {
		return nil
	}
	content, _ := queryText.Html()
	if strings.TrimSpace(content) == "" {
		return nil
	}
	return &ConversationMessage{
		Author:  "You",
		Content: strings.TrimSpace(content),
		Metadata: map[string]any{
			"role": "user",
		},
	}
}

// geminiModelMessage extracts the model-response message from a conversation
// container (preferring extended content over regular), or nil when absent or empty.
func (g *GeminiExtractor) geminiModelMessage(container *goquery.Selection) *ConversationMessage {
	modelResponse := container.Find("model-response").First()
	if modelResponse.Length() == 0 {
		return nil
	}
	// Try extended content first, then regular content
	extendedContent := modelResponse.Find("#extended-response-markdown-content").First()
	regularContent := modelResponse.Find(".model-response-text .markdown").First()
	contentElement := regularContent
	if extendedContent.Length() > 0 {
		contentElement = extendedContent
	}
	if contentElement.Length() == 0 {
		return nil
	}
	content, _ := contentElement.Html()
	if strings.TrimSpace(content) == "" {
		return nil
	}
	// Clean up content - remove table-content class but keep the content
	// `table-content` is a PARTIAL selector in defuddle (table of contents, will be removed), but a real table in Gemini (should be kept).
	cleanedContent := g.cleanGeminiContent(content)
	return &ConversationMessage{
		Author:  "Gemini",
		Content: strings.TrimSpace(cleanedContent),
		Metadata: map[string]any{
			"role": "assistant",
		},
	}
}

// cleanGeminiContent cleans up Gemini response content
// TypeScript original code:
//
//	const tempDiv = document.createElement('div');
//	tempDiv.innerHTML = content;
//
//	tempDiv.querySelectorAll('.table-content').forEach(el => {
//		// `table-content` is a PARTIAL selector in defuddle (table of contents, will be removed), but a real table in Gemini (should be kept).
//		el.classList.remove('table-content');
//	});
//
//	content = tempDiv.innerHTML;
func (g *GeminiExtractor) cleanGeminiContent(content string) string {
	// Create a temporary document to manipulate the HTML
	tempDoc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		slog.Warn("Failed to parse Gemini content as HTML", "error", err)
		return content
	}

	// Remove table-content class but keep the element
	// `table-content` is a PARTIAL selector in defuddle (table of contents, will be removed), but a real table in Gemini (should be kept).
	tempDoc.Find(".table-content").RemoveClass("table-content")

	// Get the cleaned HTML
	cleanedContent, err := tempDoc.Html()
	if err != nil {
		slog.Warn("Failed to get cleaned Gemini HTML content", "error", err)
		return content
	}

	return cleanedContent
}
