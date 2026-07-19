package extractors

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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
