package extractors

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// cleanMessageContent removes specific elements from message content
// TypeScript original code:
//
//	// Remove specific elements from the message content
//	const tempDiv = document.createElement('div');
//	tempDiv.innerHTML = messageContent;
//	tempDiv.querySelectorAll('h5.sr-only, h6.sr-only, span[data-state="closed"]').forEach(el => el.remove());
//	messageContent = tempDiv.innerHTML;
func (c *ChatGPTExtractor) cleanMessageContent(messageContent string) string {
	// Create a temporary document to manipulate the HTML
	tempDoc, err := goquery.NewDocumentFromReader(strings.NewReader(messageContent))
	if err != nil {
		slog.Warn("Failed to parse message content as HTML", "error", err)
		return messageContent
	}

	// Remove specific elements
	tempDoc.Find(`h5.sr-only, h6.sr-only, span[data-state="closed"]`).Remove()

	// Get the cleaned HTML
	cleanedContent, err := tempDoc.Html()
	if err != nil {
		slog.Warn("Failed to get cleaned HTML content", "error", err)
		return messageContent
	}

	return cleanedContent
}

// GetFootnotes returns the conversation footnotes
// TypeScript original code:
//
//	protected getFootnotes(): Footnote[] {
//		return this.footnotes;
//	}
func (c *ChatGPTExtractor) GetFootnotes() []Footnote {
	return c.footnotes
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
//			site: 'ChatGPT',
//			url: this.url,
//			messageCount: messages.length,
//			description: `ChatGPT conversation with ${messages.length} messages`
//		};
//	}
func (c *ChatGPTExtractor) GetMetadata() ConversationMetadata {
	title := c.getTitle()
	messages := c.cachedMessages
	if messages == nil {
		messages = c.ExtractMessages()
	}

	return conversationMetadata("ChatGPT", title, c.url, len(messages))
}

// getTitle extracts the conversation title
// TypeScript original code:
//
//	private getTitle(): string {
//		// Try to get the page title first
//		const pageTitle = this.document.title?.trim();
//		if (pageTitle && pageTitle !== 'ChatGPT') {
//			return pageTitle;
//		}
//
//		// Fall back to first user message
//		const firstUserTurn = this.articles?.item(0)?.querySelector('.text-message');
//		if (firstUserTurn) {
//			const text = firstUserTurn.textContent || '';
//			// Truncate to first 50 characters if longer
//			return text.length > 50 ? text.slice(0, 50) + '...' : text;
//		}
//
//		return 'ChatGPT Conversation';
//	}
func (c *ChatGPTExtractor) getTitle() string {
	// Try to get the page title first
	pageTitle := strings.TrimSpace(c.document.Find("title").Text())
	if pageTitle != "" && pageTitle != "ChatGPT" {
		return pageTitle
	}

	// Fall back to first user message
	firstUserTurn := c.articles.First().Find(".text-message").First()
	if firstUserTurn.Length() > 0 {
		return TruncateTitle(firstUserTurn.Text(), 50)
	}

	return "ChatGPT Conversation"
}

// processFootnotes processes citation links into footnote references.
// Matches span+a structures with href, target="_blank", and rel="noopener"
// in any attribute order (TS uses lookaheads; Go validates in code).
// Deduplicates footnotes by URL and extracts fragment text from #:~:text= hashes.
func (c *ChatGPTExtractor) processFootnotes(content string) string {
	matches := chatgptCitationRe.FindAllStringSubmatch(content, -1)
	processedContent := content

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		fullMatch := match[0]
		attrs := match[3]

		// Validate required attributes exist (any order)
		if !strings.Contains(attrs, `target="_blank"`) || !strings.Contains(attrs, `rel="noopener"`) {
			continue
		}
		hrefMatch := chatgptHrefRe.FindStringSubmatch(attrs)
		if hrefMatch == nil {
			continue
		}
		citationURL := hrefMatch[1]

		// Extract domain and fragment text
		domain := extractCitationDomain(citationURL)
		fragmentText := extractFragmentText(citationURL)

		// Deduplicate: reuse existing footnote number for same URL
		footnoteNumber := c.footnoteIndexForURL(citationURL, domain, fragmentText)

		// Replace with footnote reference using fn:N format (matching TS)
		replacement := fmt.Sprintf(`<sup id="fnref:%d"><a href="#fn:%d">%d</a></sup>`, footnoteNumber, footnoteNumber, footnoteNumber)
		processedContent = strings.Replace(processedContent, fullMatch, replacement, 1)
	}

	return processedContent
}

// footnoteIndexForURL returns the 1-based footnote number for citationURL,
// creating a new footnote (linking domain + fragment text) if not present.
func (c *ChatGPTExtractor) footnoteIndexForURL(citationURL, domain, fragmentText string) int {
	for i, fn := range c.footnotes {
		if fn.URL == citationURL {
			return i + 1
		}
	}
	c.footnoteCounter++
	c.footnotes = append(c.footnotes, Footnote{
		URL:  citationURL,
		Text: fmt.Sprintf(`<a href="%s">%s</a>%s`, citationURL, domain, fragmentText),
	})
	return c.footnoteCounter
}

// extractCitationDomain extracts the hostname without www. prefix.
func extractCitationDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

// extractFragmentText extracts text from #:~:text= URL fragment.
func extractFragmentText(rawURL string) string {
	parts := strings.SplitN(rawURL, "#:~:text=", 2)
	if len(parts) < 2 {
		return ""
	}
	fragment, err := url.QueryUnescape(parts[1])
	if err != nil {
		return ""
	}
	fragment = strings.ReplaceAll(fragment, "%2C", ",")

	commaParts := strings.SplitN(fragment, ",", 2)
	first := strings.TrimSpace(commaParts[0])
	if first == "" {
		return ""
	}
	if len(commaParts) > 1 {
		return fmt.Sprintf(" — %s...", first)
	}
	return fmt.Sprintf(" — %s", strings.TrimSpace(fragment))
}
