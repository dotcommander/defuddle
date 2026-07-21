package extractors

import (
	"log/slog"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// extractAuthor extracts author from a container using multiple selectors
// TypeScript original code:
//
//	private extractAuthor(container: Element, selectors: string[]): string {
//		for (const selector of selectors) {
//			const authorLink = container.querySelector(selector);
//			if (authorLink) {
//				const href = authorLink.getAttribute('href');
//				if (href) {
//					if (href.startsWith('/')) {
//						return href.substring(1);
//					} else if (href.includes('github.com/')) {
//						const match = href.match(/github\.com\/([^\/\?#]+)/);
//						if (match && match[1]) {
//							return match[1];
//						}
//					}
//				}
//			}
//		}
//		return 'Unknown';
//	}
func (g *GitHubExtractor) extractAuthor(container *goquery.Selection, selectors []string) string {
	for _, selector := range selectors {
		authorLink := container.Find(selector).First()
		if authorLink.Length() == 0 {
			continue
		}
		href, exists := authorLink.Attr("href")
		if !exists {
			continue
		}
		if strings.HasPrefix(href, "/") {
			return href[1:]
		}
		if strings.Contains(href, "github.com/") {
			matches := githubUserRe.FindStringSubmatch(href)
			if len(matches) > 1 && matches[1] != "" {
				return matches[1]
			}
		}
	}
	return "Unknown"
}

// cleanBodyContent cleans markdown body content by removing buttons and clipboard elements
// TypeScript original code:
//
//	private cleanBodyContent(bodyElement: Element): string {
//		const cleanBody = bodyElement.cloneNode(true) as Element;
//		cleanBody.querySelectorAll('button, [data-testid*="button"], [data-testid*="menu"]').forEach(el => el.remove());
//		cleanBody.querySelectorAll('.js-clipboard-copy, .zeroclipboard-container').forEach(el => el.remove());
//		return cleanBody.innerHTML.trim();
//	}
func (g *GitHubExtractor) cleanBodyContent(bodyElement *goquery.Selection) string {
	// Clone the selection to avoid modifying the original
	htmlContent, err := bodyElement.Html()
	if err != nil {
		return ""
	}

	// Create a new document from the HTML content
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent // Return original if parsing fails
	}

	// Remove buttons and menu elements
	doc.Find(`button, [data-testid*="button"], [data-testid*="menu"]`).Remove()

	// Remove clipboard elements
	doc.Find(".js-clipboard-copy, .zeroclipboard-container").Remove()

	// Get the cleaned HTML
	cleanedHTML, err := doc.Html()
	if err != nil {
		return htmlContent // Return original if extraction fails
	}

	return strings.TrimSpace(cleanedHTML)
}

// extractRepoInfo extracts repository owner and name
// TypeScript original code:
//
//	private extractRepoInfo(): { owner: string; repo: string } {
//		// Try URL first (most reliable)
//		const urlMatch = this.url.match(/github\.com\/([^\/]+)\/([^\/]+)/);
//		if (urlMatch) {
//			return { owner: urlMatch[1], repo: urlMatch[2] };
//		}
//
//		// Fallback to HTML extraction
//		const titleMatch = this.document.title.match(/([^\/\s]+)\/([^\/\s]+)/);
//		return titleMatch ? { owner: titleMatch[1], repo: titleMatch[2] } : { owner: '', repo: '' };
//	}
func (g *GitHubExtractor) extractRepoInfo() map[string]string {
	// Try URL first (most reliable)
	matches := githubRepoRe.FindStringSubmatch(g.url)
	if len(matches) >= 3 {
		return map[string]string{
			"owner": matches[1],
			"repo":  matches[2],
		}
	}

	// Fallback to HTML extraction
	title := g.document.Find("title").Text()
	titleMatches := githubTitleRepoRe.FindStringSubmatch(title)
	if len(titleMatches) >= 3 {
		return map[string]string{
			"owner": titleMatches[1],
			"repo":  titleMatches[2],
		}
	}

	return map[string]string{
		"owner": "",
		"repo":  "",
	}
}

// extractIssueNumber extracts the issue number from URL
// TypeScript original code:
//
//	private extractIssueNumber(): string {
//		const match = this.url.match(/\/issues\/(\d+)/);
//		return match ? match[1] : '';
//	}
func (g *GitHubExtractor) extractIssueNumber() string {
	matches := githubIssueRe.FindStringSubmatch(g.url)
	if len(matches) > 1 {
		issueNumber := matches[1]
		slog.Debug("GitHub extractor: extracted issue number", "issueNumber", issueNumber)
		return issueNumber
	}

	slog.Debug("GitHub extractor: no issue number found in URL", "url", g.url)
	return ""
}

// createDescription creates a description from HTML content
// TypeScript original code:
//
//	private createDescription(content: string): string {
//		if (!content) return '';
//
//		const tempDiv = this.document.createElement('div');
//		tempDiv.innerHTML = content;
//		return tempDiv.textContent?.trim()
//			.slice(0, 140)
//			.replace(/\s+/g, ' ') || '';
//	}
func (g *GitHubExtractor) createDescription(content string) string {
	if content == "" {
		return ""
	}

	// Parse HTML and extract text content
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		slog.Warn("GitHub extractor: failed to parse HTML for description", "error", err)
		return ""
	}

	text := strings.TrimSpace(doc.Text())

	// Truncate to 140 characters to match TypeScript implementation
	if len(text) > 140 {
		text = text[:140]
	}

	// Replace multiple spaces with single space
	text = whitespaceRe.ReplaceAllString(text, " ")

	slog.Debug("GitHub extractor: created description", "descriptionLength", len(text))
	return text
}

// relativeTimeDatetime returns the datetime attribute of the first <relative-time>
// element within sel, or "" if absent.
func relativeTimeDatetime(sel *goquery.Selection) string {
	el := sel.Find("relative-time").First()
	if el.Length() == 0 {
		return ""
	}
	return el.AttrOr("datetime", "")
}

// formatGitHubDate parses an RFC3339 timestamp and formats it as "January 2, 2006",
// or returns "" if ts is empty or unparseable.
func formatGitHubDate(ts string) string {
	date, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	return date.Format("January 2, 2006")
}
