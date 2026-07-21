package extractors

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// ExtractMessages extracts conversation messages
// TypeScript original code (improved version):
//
//	protected extractMessages(): ConversationMessage[] {
//		const messages: ConversationMessage[] = [];
//		this.footnotes = [];
//		this.footnoteCounter = 0;
//
//		if (!this.articles) return messages;
//
//		this.articles.forEach((article) => {
//			// Get the localized author text from the sr-only heading and clean it
//			const authorElement = article.querySelector('h5.sr-only, h6.sr-only');
//			const authorText = authorElement?.textContent
//				?.trim()
//				?.replace(/:\s*$/, '') // Remove colon and any trailing whitespace
//				|| '';
//
//			let currentAuthorRole = '';
//
//			const authorRole = article.getAttribute('data-message-author-role');
//			if (authorRole) {
//				currentAuthorRole = authorRole;
//			}
//
//			let messageContent = article.innerHTML || '';
//			messageContent = messageContent.replace(/\u200B/g, '');
//
//			// Remove specific elements from the message content
//			const tempDiv = document.createElement('div');
//			tempDiv.innerHTML = messageContent;
//			tempDiv.querySelectorAll('h5.sr-only, h6.sr-only, span[data-state="closed"]').forEach(el => el.remove());
//			messageContent = tempDiv.innerHTML;
//
//			// Process inline references
//			messageContent = this.processFootnotes(messageContent);
//
//			// Clean up any stray empty paragraph tags
//			messageContent = messageContent.replace(/<p[^>]*>\s*<\/p>/g, '');
//
//			messages.push({
//				author: authorText,
//				content: messageContent.trim(),
//				metadata: {
//					role: currentAuthorRole || 'unknown'
//				}
//			});
//		});
//
//		return messages;
//	}
func (c *ChatGPTExtractor) ExtractMessages() []ConversationMessage {
	var messages []ConversationMessage
	c.footnotes = make([]Footnote, 0)
	c.footnoteCounter = 0

	if c.articles.Length() == 0 {
		slog.Debug("No articles found for ChatGPT extraction")
		return messages
	}

	c.articles.Each(func(i int, article *goquery.Selection) {
		// Get the localized author text from the sr-only heading and clean it
		authorElement := article.Find("h5.sr-only, h6.sr-only").First()
		authorText := strings.TrimSpace(authorElement.Text())

		// Remove colon and any trailing whitespace
		authorText = strings.TrimSuffix(strings.TrimSpace(authorText), ":")

		messageEls := c.messageElementsForTurn(article)

		// Get author role from the first message element, falling back to the
		// article for older ChatGPT markup.
		currentAuthorRole := ""
		if len(messageEls) > 0 {
			currentAuthorRole, _ = messageEls[0].Attr("data-message-author-role")
		}
		if currentAuthorRole == "" {
			currentAuthorRole, _ = article.Attr("data-message-author-role")
		}
		if currentAuthorRole == "" {
			currentAuthorRole = "unknown"
		}

		// Get message content. ChatGPT can split one assistant turn into
		// multiple message fragments around Thought sections; merge only the
		// direct fragments for this turn.
		messageContent := c.messageContentHTML(article, messageEls)
		if messageContent == "" {
			slog.Debug("Empty message content found", "index", i)
			return
		}

		// Remove zero-width space characters
		messageContent = strings.ReplaceAll(messageContent, "\u200B", "")

		// Remove specific elements from the message content
		messageContent = c.cleanMessageContent(messageContent)

		// Process inline references using regex to find the containers
		messageContent = c.processFootnotes(messageContent)

		// Clean up any stray empty paragraph tags
		messageContent = chatgptEmptyParagraphRe.ReplaceAllString(messageContent, "")

		if strings.TrimSpace(messageContent) != "" {
			messages = append(messages, ConversationMessage{
				Author:  authorText,
				Content: strings.TrimSpace(messageContent),
				Metadata: map[string]any{
					"role": currentAuthorRole,
				},
			})
		}
	})

	slog.Debug("ChatGPT messages extracted", "messageCount", len(messages), "footnoteCount", len(c.footnotes))
	return messages
}

func (c *ChatGPTExtractor) messageElementsForTurn(article *goquery.Selection) []*goquery.Selection {
	var messageEls []*goquery.Selection
	articleNode := article.Get(0)
	if articleNode == nil {
		return messageEls
	}

	if article.Is(`[data-message-author-role]`) {
		messageEls = append(messageEls, article)
	}

	article.Find(`[data-message-author-role]`).Each(func(_ int, s *goquery.Selection) {
		if closest := s.Closest(`[data-testid^="conversation-turn-"]`); closest.Length() > 0 && closest.Get(0) != articleNode {
			return
		}
		messageEls = append(messageEls, s)
	})

	return messageEls
}

func (c *ChatGPTExtractor) messageContentHTML(article *goquery.Selection, messageEls []*goquery.Selection) string {
	if len(messageEls) == 0 {
		htmlContent, _ := article.Html()
		return htmlContent
	}

	parts := make([]string, 0, len(messageEls))
	for _, messageEl := range messageEls {
		contentEls := c.messageContentElements(messageEl)
		if len(contentEls) == 0 {
			parts = appendOuterHTML(parts, messageEl)
			continue
		}
		for _, contentEl := range contentEls {
			parts = appendOuterHTML(parts, contentEl)
		}
	}

	if len(parts) == 0 {
		htmlContent, _ := article.Html()
		return htmlContent
	}
	return strings.Join(parts, "\n")
}

// appendOuterHTML appends sel's outer HTML to parts when it is non-empty.
func appendOuterHTML(parts []string, sel *goquery.Selection) []string {
	if htmlContent, err := goquery.OuterHtml(sel); err == nil && strings.TrimSpace(htmlContent) != "" {
		parts = append(parts, htmlContent)
	}
	return parts
}

func (c *ChatGPTExtractor) messageContentElements(messageEl *goquery.Selection) []*goquery.Selection {
	var candidates []*goquery.Selection
	if messageEl.Is(".markdown, .whitespace-pre-wrap") {
		candidates = append(candidates, messageEl)
	}
	messageEl.Find(".markdown, .whitespace-pre-wrap").Each(func(_ int, s *goquery.Selection) {
		candidates = append(candidates, s)
	})

	filtered := candidates[:0]
	for _, candidate := range candidates {
		nested := false
		for _, other := range candidates {
			if candidate == other {
				continue
			}
			candidateNode := candidate.Get(0)
			otherNode := other.Get(0)
			if candidateNode != nil && otherNode != nil && nodeContains(otherNode, candidateNode) {
				nested = true
				break
			}
		}
		if !nested {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func nodeContains(parent, child *html.Node) bool {
	for n := child.Parent; n != nil; n = n.Parent {
		if n == parent {
			return true
		}
	}
	return false
}
