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
//
//		if (!this.articles) return messages;
//
//		this.articles.forEach((article) => {
//			let role: string;
//			let content: string;
//
//			if (article.hasAttribute('data-testid')) {
//				// Handle user messages
//				if (article.getAttribute('data-testid') === 'user-message') {
//					role = 'you';
//					content = article.innerHTML;
//				}
//				// Skip non-message elements
//				else {
//					return;
//				}
//			} else if (article.classList.contains('font-claude-message')) {
//				// Handle Claude messages
//				role = 'assistant';
//				content = article.innerHTML;
//			} else {
//				// Skip unknown elements
//				return;
//			}
//
//			if (content) {
//				messages.push({
//					author: role === 'you' ? 'You' : 'Claude',
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
func (c *ClaudeExtractor) ExtractMessages() []ConversationMessage {
	var messages []ConversationMessage

	if c.articles.Length() == 0 {
		slog.Debug("No articles found for Claude extraction")
		return messages
	}

	c.articles.Each(func(i int, article *goquery.Selection) {
		role, content := claudeMessageRoleContent(article)
		if role == "" {
			return
		}

		if strings.TrimSpace(content) != "" {
			var author string
			if role == "you" {
				author = "You"
			} else {
				author = "Claude"
			}

			messages = append(messages, ConversationMessage{
				Author:  author,
				Content: strings.TrimSpace(content),
				Metadata: map[string]any{
					"role": role,
				},
			})
		} else {
			slog.Debug("Empty content found", "role", role, "index", i)
		}
	})

	slog.Debug("Claude messages extracted", "messageCount", len(messages))
	return messages
}

// claudeMessageRoleContent classifies a Claude article into its role ("you" or
// "assistant") and HTML content, returning an empty role for articles to skip.
func claudeMessageRoleContent(article *goquery.Selection) (role, content string) {
	testid, hasTestid := article.Attr("data-testid")

	switch {
	case hasTestid:
		// Only handle user messages via data-testid (TS skips assistant-message testid)
		if testid != "user-message" {
			// Skip non-user-message testid elements
			return "", ""
		}
		content, _ = article.Html()
		return "you", content
	case article.HasClass("font-claude-response"):
		// Claude's response identified by class
		assistantBody := article.Find(".standard-markdown").First()
		if assistantBody.Length() > 0 {
			content, _ = assistantBody.Html()
		} else {
			content, _ = article.Html()
		}
		return "assistant", content
	default:
		return "", ""
	}
}

// GetFootnotes returns the conversation footnotes
// TypeScript original code:
//
//	protected getFootnotes(): Footnote[] {
//		return [];
//	}
func (c *ClaudeExtractor) GetFootnotes() []Footnote {
	// Claude extractor doesn't process footnotes in the TypeScript version
	return []Footnote{}
}

// GetMetadata returns conversation metadata
// TypeScript original code:
//
//	protected getMetadata(): ConversationMetadata {
//		const title = this.getTitle();
//		const messages = this.extractMessages();
//
//		return {
//			title,
//			site: 'Claude',
//			url: this.url,
//			messageCount: messages.length,
//			description: `Claude conversation with ${messages.length} messages`
//		};
//	}
func (c *ClaudeExtractor) GetMetadata() ConversationMetadata {
	title := c.getTitle()
	messages := c.cachedMessages
	if messages == nil {
		messages = c.ExtractMessages()
	}

	return conversationMetadata("Claude", title, c.url, len(messages))
}

// getTitle extracts the conversation title
// TypeScript original code:
//
//	private getTitle(): string {
//		// Try to get the page title first
//		const pageTitle = this.document.title?.trim();
//		if (pageTitle && pageTitle !== 'Claude') {
//			// Remove ' - Claude' suffix if present
//			return pageTitle.replace(/ - Claude$/, '');
//		}
//
//		// Try to get title from header
//		const headerTitle = this.document.querySelector('header .font-tiempos')?.textContent?.trim();
//		if (headerTitle) {
//			return headerTitle;
//		}
//
//		// Fall back to first user message
//		const firstUserMessage = this.articles?.item(0)?.querySelector('[data-testid="user-message"]');
//		if (firstUserMessage) {
//			const text = firstUserMessage.textContent || '';
//			// Truncate to first 50 characters if longer
//			return text.length > 50 ? text.slice(0, 50) + '...' : text;
//		}
//
//		return 'Claude Conversation';
//	}
func (c *ClaudeExtractor) getTitle() string {
	// Try to get the page title first
	pageTitle := strings.TrimSpace(c.document.Find("title").Text())
	if pageTitle != "" && pageTitle != "Claude" {
		// Remove ' - Claude' suffix if present
		return claudeTitleSuffixRe.ReplaceAllString(pageTitle, "")
	}

	// Try to get title from header
	headerTitle := strings.TrimSpace(c.document.Find("header .font-tiempos").Text())
	if headerTitle != "" {
		return headerTitle
	}

	// Fall back to first user message
	firstUserMessage := c.articles.First().Find(`[data-testid="user-message"]`)
	if firstUserMessage.Length() > 0 {
		text := firstUserMessage.Text()
		return TruncateTitle(text, 50)
	}

	// Try to fall back to any first message
	if c.articles.Length() > 0 {
		firstMessage := c.articles.First()
		text := strings.TrimSpace(firstMessage.Text())
		if text != "" {
			return TruncateTitle(text, 50)
		}
	}

	return "Claude Conversation"
}
