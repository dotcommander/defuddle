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
//		const messages: ConversationMessage[] = [];
//		this.footnotes = [];
//		this.footnoteCounter = 0;
//
//		if (!this.messageBubbles || this.messageBubbles.length === 0) return messages;
//
//		this.messageBubbles.forEach((container) => {
//			// Note: Relies on layout classes 'items-end' and 'items-start' which might change.
//			const isUserMessage = container.classList.contains('items-end');
//			const isGrokMessage = container.classList.contains('items-start');
//
//			if (!isUserMessage && !isGrokMessage) return; // Skip elements that aren't clearly user or Grok messages
//
//			const messageBubble = container.querySelector('.message-bubble');
//			if (!messageBubble) return; // Skip if the core message bubble isn't found
//
//			let content: string = '';
//			let role: string = '';
//			let author: string = '';
//
//			if (isUserMessage) {
//				// Assume user message bubble's textContent is the desired content.
//				// This is simpler and potentially less brittle than selecting specific spans.
//				content = messageBubble.textContent || '';
//				role = 'user';
//				author = 'You'; // Or potentially extract from an attribute if available later
//			} else if (isGrokMessage) {
//				role = 'assistant';
//				author = 'Grok'; // Or potentially extract from an attribute if available later
//
//				// Clone the bubble to modify it without affecting the original page
//				const clonedBubble = messageBubble.cloneNode(true) as Element;
//
//				// Remove known non-content elements like the DeepSearch artifact
//				clonedBubble.querySelector('.relative.border.border-border-l1.bg-surface-base')?.remove();
//				// Add selectors here for any other known elements to remove (e.g., buttons, toolbars within the bubble)
//
//				content = clonedBubble.innerHTML;
//
//				// Process footnotes/links in the cleaned content
//				content = this.processFootnotes(content);
//			}
//
//			if (content.trim()) {
//				messages.push({
//					author: author,
//					content: content.trim(),
//					metadata: {
//						role: role
//					}
//				});
//			}
//		});
//
//		return messages;
//	}
func (g *GrokExtractor) ExtractMessages() []ConversationMessage {
	var messages []ConversationMessage
	g.footnotes = make([]Footnote, 0)
	g.footnoteCounter = 0

	if g.messageBubbles.Length() == 0 {
		slog.Debug("No message bubbles found for Grok extraction")
		return messages
	}

	g.messageBubbles.Each(func(i int, container *goquery.Selection) {
		role, content, author := g.grokMessageRoleContent(container, i)
		if role == "" {
			return
		}

		if strings.TrimSpace(content) != "" {
			messages = append(messages, ConversationMessage{
				Author:  author,
				Content: strings.TrimSpace(content),
				Metadata: map[string]any{
					"role": role,
				},
			})
			slog.Debug("Grok extractor: extracted message", "index", i, "author", author, "role", role, "contentLength", len(content))
		} else {
			slog.Debug("Grok extractor: empty content found", "index", i, "author", author, "role", role)
		}
	})

	slog.Debug("Grok messages extracted", "messageCount", len(messages), "footnoteCount", len(g.footnotes))
	return messages
}

// grokMessageRoleContent classifies a Grok message bubble into its role, content,
// and author. Returns an empty role for elements to skip (non-message, missing
// bubble, or unparseable HTML).
func (g *GrokExtractor) grokMessageRoleContent(container *goquery.Selection, i int) (role, content, author string) {
	// Note: Relies on layout classes 'items-end' and 'items-start' which might change.
	isUserMessage := container.HasClass("items-end")
	isGrokMessage := container.HasClass("items-start")

	if !isUserMessage && !isGrokMessage {
		slog.Debug("Grok extractor: skipping non-message element", "index", i)
		return "", "", "" // Skip elements that aren't clearly user or Grok messages
	}

	messageBubble := container.Find(".message-bubble").First()
	if messageBubble.Length() == 0 {
		slog.Debug("Grok extractor: no message bubble found", "index", i, "isUserMessage", isUserMessage, "isGrokMessage", isGrokMessage)
		return "", "", "" // Skip if the core message bubble isn't found
	}

	if isUserMessage {
		// Assume user message bubble's textContent is the desired content.
		// This is simpler and potentially less brittle than selecting specific spans.
		return "user", messageBubble.Text(), "You"
	}

	// isGrokMessage: clone the bubble to modify it without affecting the original page
	clonedBubbleHTML, _ := messageBubble.Html()
	clonedDoc, err := goquery.NewDocumentFromReader(strings.NewReader(clonedBubbleHTML))
	if err != nil {
		slog.Warn("Grok extractor: failed to parse message bubble HTML", "error", err, "index", i)
		return "", "", ""
	}

	// Remove known non-content elements like the DeepSearch artifact
	clonedDoc.Find(".relative.border.border-border-l1.bg-surface-base").Remove()

	clonedContent, _ := clonedDoc.Find("body").Html()

	// Process footnotes/links in the cleaned content
	return "assistant", g.processFootnotes(clonedContent), "Grok"
}

// GetFootnotes returns the conversation footnotes
// TypeScript original code:
//
//	protected getFootnotes(): Footnote[] {
//		return this.footnotes;
//	}
func (g *GrokExtractor) GetFootnotes() []Footnote {
	return g.footnotes
}

// GetMetadata returns conversation metadata
// TypeScript original code:
//
//	protected getMetadata(): ConversationMetadata {
//		const title = this.getTitle();
//		const messageCount = this.messageBubbles?.length || 0;
//
//		return {
//			title,
//			site: 'Grok',
//			url: this.url,
//			messageCount: messageCount, // Use estimated count
//			description: `Grok conversation with ${messageCount} messages`
//		};
//	}
func (g *GrokExtractor) GetMetadata() ConversationMetadata {
	title := g.getTitle()
	var messageCount int
	if g.cachedMessages != nil {
		messageCount = len(g.cachedMessages)
	} else {
		messageCount = g.messageBubbles.Length()
	}

	return conversationMetadata("Grok", title, g.url, messageCount)
}
