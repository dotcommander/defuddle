package extractors

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// GeminiExtractor handles Gemini conversation content extraction
// TypeScript original code:
// import { ConversationExtractor } from './_conversation';
// import { ConversationMessage, ConversationMetadata, Footnote } from '../types/extractors';
//
//	export class GeminiExtractor extends ConversationExtractor {
//		private conversationContainers: NodeListOf<Element> | null;
//		private footnotes: Footnote[];
//		private messageCount: number | null = null;
//
//		constructor(document: Document, url: string) {
//			super(document, url);
//			this.conversationContainers = document.querySelectorAll('div.conversation-container');
//			this.footnotes = [];
//		}
//
//		canExtract(): boolean {
//			return !!this.conversationContainers && this.conversationContainers.length > 0;
//		}
//
//		protected extractMessages(): ConversationMessage[] {
//			this.messageCount = 0;
//			const messages: ConversationMessage[] = [];
//
//			if (!this.conversationContainers) return messages;
//
//			this.extractSources();
//
//			this.conversationContainers.forEach((container) => {
//				const userQuery = container.querySelector('user-query');
//				if (userQuery) {
//					const queryText = userQuery.querySelector('.query-text');
//					if (queryText) {
//						const content = queryText.innerHTML || '';
//						messages.push({
//							author: 'You',
//							content: content.trim(),
//							metadata: { role: 'user' }
//						});
//					}
//				}
//
//				const modelResponse = container.querySelector('model-response');
//				if (modelResponse) {
//					const regularContent = modelResponse.querySelector('.model-response-text .markdown');
//					const extendedContent = modelResponse.querySelector('#extended-response-markdown-content');
//					const contentElement = extendedContent || regularContent;
//
//					if (contentElement) {
//						let content = contentElement.innerHTML || '';
//
//						const tempDiv = document.createElement('div');
//						tempDiv.innerHTML = content;
//
//						tempDiv.querySelectorAll('.table-content').forEach(el => {
//							// `table-content` is a PARTIAL selector in defuddle (table of contents, will be removed), but a real table in Gemini (should be kept).
//							el.classList.remove('table-content');
//						});
//
//						content = tempDiv.innerHTML;
//
//						messages.push({
//							author: 'Gemini',
//							content: content.trim(),
//							metadata: { role: 'assistant' }
//						});
//					}
//				}
//			});
//			this.messageCount = messages.length;
//			return messages;
//		}
//
//		private extractSources(): void {
//			const browseItems = this.document.querySelectorAll('browse-item');
//
//			if (browseItems && browseItems.length > 0) {
//				browseItems.forEach(item => {
//					const link = item.querySelector('a');
//					if (link instanceof HTMLAnchorElement) {
//						const url = link.href;
//						const domain = link.querySelector('.domain')?.textContent?.trim() || '';
//						const title = link.querySelector('.title')?.textContent?.trim() || '';
//
//						if (url && (domain || title)) {
//							this.footnotes.push({
//								url,
//								text: title ? `${domain}: ${title}` : domain
//							});
//						}
//					}
//				});
//			}
//		}
//
//		protected getFootnotes(): Footnote[] {
//			return this.footnotes;
//		}
//
//		protected getMetadata(): ConversationMetadata {
//			const title = this.getTitle();
//			const messageCount = this.messageCount ?? this.extractMessages().length;
//			return {
//				title,
//				site: 'Gemini',
//				url: this.url,
//				messageCount,
//				description: `Gemini conversation with ${messageCount} messages`
//			};
//		}
//
//		private getTitle(): string {
//			const pageTitle = this.document.title?.trim();
//			if (pageTitle && pageTitle !== 'Gemini' && !pageTitle.includes('Gemini')) {
//				return pageTitle;
//			}
//
//			const researchTitle = this.document.querySelector('.title-text')?.textContent?.trim();
//			if (researchTitle) {
//				return researchTitle;
//			}
//
//			const firstUserQuery = this.conversationContainers?.item(0)?.querySelector('.query-text');
//			if (firstUserQuery) {
//				const text = firstUserQuery.textContent || '';
//				return text.length > 50 ? text.slice(0, 50) + '...' : text;
//			}
//
//			return 'Gemini Conversation';
//		}
//	}
type GeminiExtractor struct {
	*ConversationExtractorBase
	conversationContainers *goquery.Selection
	footnotes              []Footnote
	messageCount           *int
}

// NewGeminiExtractor creates a new Gemini extractor
// TypeScript original code:
//
//	constructor(document: Document, url: string) {
//		super(document, url);
//		this.conversationContainers = document.querySelectorAll('div.conversation-container');
//		this.footnotes = [];
//	}
func NewGeminiExtractor(document *goquery.Document, urlStr string, schemaOrgData any) *GeminiExtractor {
	conversationContainers := document.Find("div.conversation-container")
	slog.Debug("Gemini extractor initialized", "containersFound", conversationContainers.Length(), "url", urlStr)

	return &GeminiExtractor{
		ConversationExtractorBase: NewConversationExtractorBase(document, urlStr, schemaOrgData),
		conversationContainers:    conversationContainers,
		footnotes:                 make([]Footnote, 0),
		messageCount:              nil,
	}
}

// CanExtract checks if the extractor can extract content
// TypeScript original code:
//
//	canExtract(): boolean {
//		return !!this.conversationContainers && this.conversationContainers.length > 0;
//	}
func (g *GeminiExtractor) CanExtract() bool {
	return canExtractFromSelection(g.conversationContainers, "Gemini", "containersCount")
}

// Name returns the name of the extractor
func (g *GeminiExtractor) Name() string {
	return "GeminiExtractor"
}

// Extract extracts the Gemini conversation
func (g *GeminiExtractor) Extract() *ExtractorResult {
	slog.Debug("Gemini extractor starting extraction", "url", g.url)
	return g.ExtractWithDefuddle(g)
}

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

// extractSources extracts browse items as footnotes
// TypeScript original code:
//
//	private extractSources(): void {
//		const browseItems = this.document.querySelectorAll('browse-item');
//
//		if (browseItems && browseItems.length > 0) {
//			browseItems.forEach(item => {
//				const link = item.querySelector('a');
//				if (link instanceof HTMLAnchorElement) {
//					const url = link.href;
//					const domain = link.querySelector('.domain')?.textContent?.trim() || '';
//					const title = link.querySelector('.title')?.textContent?.trim() || '';
//
//					if (url && (domain || title)) {
//						this.footnotes.push({
//							url,
//							text: title ? `${domain}: ${title}` : domain
//						});
//					}
//				}
//			});
//		}
//	}
func (g *GeminiExtractor) extractSources() {
	browseItems := g.document.Find("browse-item")

	if browseItems.Length() > 0 {
		browseItems.Each(func(_ int, item *goquery.Selection) {
			if fn := geminiSourceFootnote(item); fn != nil {
				g.footnotes = append(g.footnotes, *fn)
			}
		})
	}

	slog.Debug("Gemini sources extracted", "footnoteCount", len(g.footnotes))
}

// geminiSourceFootnote builds a Footnote from a browse-item's link (URL + a
// "domain: title" or domain label), or nil when the link or label is missing.
func geminiSourceFootnote(item *goquery.Selection) *Footnote {
	link := item.Find("a").First()
	if link.Length() == 0 {
		return nil
	}
	href, exists := link.Attr("href")
	if !exists || href == "" {
		return nil
	}
	domain := strings.TrimSpace(link.Find(".domain").Text())
	title := strings.TrimSpace(link.Find(".title").Text())
	if domain == "" && title == "" {
		return nil
	}
	text := domain
	if title != "" {
		text = fmt.Sprintf("%s: %s", domain, title)
	}
	return &Footnote{
		URL:  href,
		Text: text,
	}
}

// GetFootnotes returns the conversation footnotes
// TypeScript original code:
//
//	protected getFootnotes(): Footnote[] {
//		return this.footnotes;
//	}
func (g *GeminiExtractor) GetFootnotes() []Footnote {
	return g.footnotes
}

// GetMetadata returns conversation metadata
// TypeScript original code:
//
//	protected getMetadata(): ConversationMetadata {
//		const title = this.getTitle();
//		const messageCount = this.messageCount ?? this.extractMessages().length;
//		return {
//			title,
//			site: 'Gemini',
//			url: this.url,
//			messageCount,
//			description: `Gemini conversation with ${messageCount} messages`
//		};
//	}
func (g *GeminiExtractor) GetMetadata() ConversationMetadata {
	title := g.getTitle()
	var messageCount int
	switch {
	case g.messageCount != nil:
		messageCount = *g.messageCount
	case g.cachedMessages != nil:
		messageCount = len(g.cachedMessages)
	default:
		messageCount = len(g.ExtractMessages())
	}

	return conversationMetadata("Gemini", title, g.url, messageCount)
}

// getTitle extracts the conversation title
// TypeScript original code:
//
//	private getTitle(): string {
//		const pageTitle = this.document.title?.trim();
//		if (pageTitle && pageTitle !== 'Gemini' && !pageTitle.includes('Gemini')) {
//			return pageTitle;
//		}
//
//		const researchTitle = this.document.querySelector('.title-text')?.textContent?.trim();
//		if (researchTitle) {
//			return researchTitle;
//		}
//
//		const firstUserQuery = this.conversationContainers?.item(0)?.querySelector('.query-text');
//		if (firstUserQuery) {
//			const text = firstUserQuery.textContent || '';
//			return text.length > 50 ? text.slice(0, 50) + '...' : text;
//		}
//
//		return 'Gemini Conversation';
//	}
func (g *GeminiExtractor) getTitle() string {
	// Try to get the page title first
	pageTitle := strings.TrimSpace(g.document.Find("title").Text())
	if pageTitle != "" && pageTitle != "Gemini" && !strings.Contains(pageTitle, "Gemini") {
		return pageTitle
	}

	// Try to get research title
	researchTitle := strings.TrimSpace(g.document.Find(".title-text").Text())
	if researchTitle != "" {
		return researchTitle
	}

	// Fall back to first user query
	firstUserQuery := g.conversationContainers.First().Find(".query-text").First()
	if firstUserQuery.Length() > 0 {
		return TruncateTitle(firstUserQuery.Text(), 50)
	}

	return "Gemini Conversation"
}
