package extractors

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// upgradeXImageQuality upgrades a Twitter/X image URL to large quality.
// TypeScript original code:
//
//	if (src.includes('&name=')) { ... } else if (src.includes('?')) { ... } else { ... }
func upgradeXImageQuality(src string) string {
	if strings.Contains(src, "&name=") {
		return xArticleImageNameRe.ReplaceAllString(src, "&name=large")
	}
	if strings.Contains(src, "?") {
		return src + "&name=large"
	}
	return src + "?name=large"
}

// convertDraftParagraphs converts Draft.js block divs into semantic <p> elements,
// preserving inline <strong>, <a>, and <code> formatting.
// TypeScript original code:
//
//	private convertDraftParagraphs(container: HTMLElement, ownerDoc: Document): void {
//	  container.querySelectorAll('.longform-unstyled, .public-DraftStyleDefault-block').forEach(div => { ... });
//	}
func (x *XArticleExtractor) convertDraftParagraphs(container *goquery.Selection) {
	container.Find(".longform-unstyled, .public-DraftStyleDefault-block").Each(func(_ int, div *goquery.Selection) {
		content := buildParagraphContent(div)
		div.ReplaceWithHtml(fmt.Sprintf("<p>%s</p>", content))
	})
}

// buildParagraphContent recursively processes child nodes of a Draft.js block,
// preserving strong, a, and code inline elements.
// TypeScript original code: processNode recursive function inside convertDraftParagraphs
func buildParagraphContent(sel *goquery.Selection) string {
	var sb strings.Builder
	sel.Contents().Each(func(_ int, node *goquery.Selection) {
		switch goquery.NodeName(node) {
		case "#text":
			sb.WriteString(node.Text())
		case "strong":
			fmt.Fprintf(&sb, "<strong>%s</strong>", node.Text())
		case "a":
			href := node.AttrOr("href", "")
			fmt.Fprintf(&sb, `<a href="%s">%s</a>`, href, node.Text())
		case "code":
			fmt.Fprintf(&sb, "<code>%s</code>", node.Text())
		default:
			// recurse into other elements (spans, divs, etc.)
			sb.WriteString(buildParagraphContent(node))
		}
	})
	return sb.String()
}

// convertBoldSpans replaces span[style*="font-weight: bold"] with <strong> elements.
// TypeScript original code:
//
//	private convertBoldSpans(container: HTMLElement, ownerDoc: Document): void {
//	  container.querySelectorAll('span[style*="font-weight: bold"]').forEach(span => { ... });
//	}
func (x *XArticleExtractor) convertBoldSpans(container *goquery.Selection) {
	container.Find(`span[style*="font-weight: bold"]`).Each(func(_ int, span *goquery.Selection) {
		span.ReplaceWithHtml(fmt.Sprintf("<strong>%s</strong>", span.Text()))
	})
}

// removeDraftAttributes strips data-offset-key attributes from all matching elements.
// TypeScript original code:
//
//	private removeDraftAttributes(container: HTMLElement): void {
//	  container.querySelectorAll('[data-offset-key]').forEach(el => { el.removeAttribute('data-offset-key'); });
//	}
func (x *XArticleExtractor) removeDraftAttributes(container *goquery.Selection) {
	container.Find("[data-offset-key]").Each(func(_ int, el *goquery.Selection) {
		el.RemoveAttr("data-offset-key")
	})
}

// createDescription returns up to 140 characters of the article text as a description.
// TypeScript original code:
//
//	private createDescription(): string {
//	  const text = this.articleContainer?.textContent?.trim() || '';
//	  return text.slice(0, 140) + (text.length > 140 ? '...' : '');
//	}
func (x *XArticleExtractor) createDescription() string {
	if !x.CanExtract() {
		return ""
	}
	text := strings.TrimSpace(x.articleContainer.Text())
	if len(text) <= 140 {
		return text
	}
	return text[:140] + "..."
}
