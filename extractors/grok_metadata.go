package extractors

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

// getTitle extracts the conversation title
// TypeScript original code:
//
//	private getTitle(): string {
//		// Try to get the page title first (more reliable)
//		const pageTitle = this.document.title?.trim();
//		if (pageTitle && pageTitle !== 'Grok' && !pageTitle.startsWith('Grok by ')) {
//			// Remove ' - Grok' suffix if present
//			return pageTitle.replace(/\s-\s*Grok$/, '').trim();
//		}
//
//		// Fallback: Find the first user message bubble and use its text content
//		// Note: Still relies on 'items-end' class.
//		const firstUserContainer = this.document.querySelector(`${this.messageContainerSelector}.items-end`);
//		if (firstUserContainer) {
//			const messageBubble = firstUserContainer.querySelector('.message-bubble');
//			if (messageBubble) {
//				const text = messageBubble.textContent?.trim() || '';
//				// Truncate to first 50 characters if longer
//				return text.length > 50 ? text.slice(0, 50) + '...' : text;
//			}
//		}
//
//		return 'Grok Conversation'; // Default fallback
//	}
func (g *GrokExtractor) getTitle() string {
	// Try to get the page title first (more reliable)
	pageTitle := strings.TrimSpace(g.document.Find("title").Text())
	if pageTitle != "" && pageTitle != "Grok" && !strings.HasPrefix(pageTitle, "Grok by ") {
		// Remove ' - Grok' suffix if present
		title := strings.TrimSpace(grokTitleSuffixRe.ReplaceAllString(pageTitle, ""))
		if title != "" {
			return title
		}
	}

	// Fallback: Find the first user message bubble and use its text content
	// Note: Still relies on 'items-end' class.
	firstUserContainer := g.document.Find(fmt.Sprintf("%s.items-end", grokMessageContainerSelector)).First()
	if firstUserContainer.Length() > 0 {
		messageBubble := firstUserContainer.Find(".message-bubble").First()
		if messageBubble.Length() > 0 {
			text := strings.TrimSpace(messageBubble.Text())
			if text != "" {
				return TruncateTitle(text, 50)
			}
		}
	}

	return "Grok Conversation" // Default fallback
}

// processFootnotes processes links in content and converts them to footnotes
// TypeScript original code:
//
//	private processFootnotes(content: string): string {
//		// Regex to find <a> tags, capture href and link text
//		const linkPattern = /<a\s+(?:[^>]*?\s+)?href="([^"]*)"[^>]*>(.*?)<\/a>/gi; // Use 'g' and 'i' flags
//
//		return content.replace(linkPattern, (match, url, linkText) => {
//			 // Skip processing for internal anchor links, empty URLs, or non-http(s) protocols
//			if (!url || url.startsWith('#') || !url.match(/^https?:\/\//i)) {
//				return match;
//			}
//
//			// Check if this URL already exists in our footnotes
//			let footnote = this.footnotes.find(fn => fn.url === url);
//			let footnoteIndex: number;
//
//			if (!footnote) {
//				// Create a new footnote if URL doesn't exist
//				this.footnoteCounter++;
//				footnoteIndex = this.footnoteCounter;
//
//				let domainText = url; // Default to full URL if parsing fails
//				try {
//					const domain = new URL(url).hostname.replace(/^www\./, '');
//					domainText = `<a href="${url}" target="_blank" rel="noopener noreferrer">${domain}</a>`;
//				} catch (e) {
//					// Keep domainText as the original URL if parsing fails
//					domainText = `<a href="${url}" target="_blank" rel="noopener noreferrer">${url}</a>`;
//					console.warn(`GrokExtractor: Could not parse URL for footnote: ${url}`);
//				}
//
//				this.footnotes.push({
//					url,
//					text: domainText // Store the link HTML directly
//				});
//			} else {
//				// Find the 1-based index of the existing footnote
//				footnoteIndex = this.footnotes.findIndex(fn => fn.url === url) + 1;
//			}
//
//			// Return the original link text wrapped with a footnote reference
//			// Ensure the link text itself is not clickable again if it was part of the original match
//			return `${linkText}<sup id="fnref:${footnoteIndex}" class="footnote-ref"><a href="#fn:${footnoteIndex}" class="footnote-link">${footnoteIndex}</a></sup>`;
//		});
//	}
func (g *GrokExtractor) processFootnotes(content string) string {
	// Regex to find <a> tags, capture href and link text
	return grokLinkRe.ReplaceAllStringFunc(content, func(match string) string {
		matches := grokLinkRe.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		urlStr := matches[1]
		linkText := matches[2]

		// Skip processing for internal anchor links, empty URLs, or non-http(s) protocols
		if urlStr == "" || strings.HasPrefix(urlStr, "#") {
			return match
		}

		if !grokHTTPRe.MatchString(urlStr) {
			return match
		}

		footnoteIndex := g.footnoteIndexForURL(urlStr)

		// Return the original link text wrapped with a footnote reference
		// Ensure the link text itself is not clickable again if it was part of the original match
		return fmt.Sprintf(`%s<sup id="fnref:%d" class="footnote-ref"><a href="#fn:%d" class="footnote-link">%d</a></sup>`,
			linkText, footnoteIndex, footnoteIndex, footnoteIndex)
	})
}

// footnoteIndexForURL returns the 1-based footnote index for urlStr, creating a
// new footnote (with a domain link) when the URL isn't already registered.
func (g *GrokExtractor) footnoteIndexForURL(urlStr string) int {
	for idx, footnote := range g.footnotes {
		if footnote.URL == urlStr {
			return idx + 1 // 1-based index
		}
	}

	// Create a new footnote if URL doesn't exist
	g.footnoteCounter++

	var domainText string
	if parsedURL, err := url.Parse(urlStr); err == nil {
		domain := strings.TrimPrefix(parsedURL.Hostname(), "www.")
		domainText = fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, urlStr, domain)
	} else {
		// Use full URL if parsing fails
		domainText = fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, urlStr, urlStr)
		slog.Warn("GrokExtractor: Could not parse URL for footnote", "url", urlStr, "error", err)
	}

	g.footnotes = append(g.footnotes, Footnote{
		URL:  urlStr,
		Text: domainText, // Store the link HTML directly
	})
	return g.footnoteCounter
}
