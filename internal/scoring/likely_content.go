package scoring

import (
	"context"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	textutil "github.com/dotcommander/defuddle/internal/text"
)

// isLikelyContent determines if an element is likely to be content
// JavaScript original code:
//
//	private static isLikelyContent(element: Element): boolean {
//		// Check if the element has a role that indicates content
//		const role = element.getAttribute('role');
//		if (role && ['article', 'main', 'contentinfo'].includes(role)) {
//			return true;
//		}
//
//		// Check if the element has a class or id that indicates content
//		const className = element.className.toLowerCase();
//		const id = element.id.toLowerCase();
//
//		for (const indicator of contentIndicators) {
//			if (className.includes(indicator) || id.includes(indicator)) {
//				return true;
//			}
//		}
//
//		// Check if the element has a high text density
//		const text = element.textContent || '';
//		const words = text.split(/\s+/).length;
//		const paragraphs = element.getElementsByTagName('p').length;
//
//		// If the element has a significant amount of text and paragraphs, it's likely content
//		if (words > 50 && paragraphs > 1) {
//			return true;
//		}
//
//		// Check for elements with significant text content, even if they don't have many paragraphs
//		if (words > 100) {
//			return true;
//		}
//
//		// Check for elements with text content and some paragraphs
//		if (words > 30 && paragraphs > 0) {
//			return true;
//		}
//
//		return false;
//	}
func isLikelyContent(_ context.Context, element *goquery.Selection) bool {
	// Check if the element has a role that indicates content
	role, _ := element.Attr("role")
	if role != "" {
		contentRoles := []string{"article", "main", "contentinfo"}
		if slices.Contains(contentRoles, role) {
			return true
		}
	}

	// Check if the element has a class or id that indicates content
	className := strings.ToLower(element.AttrOr("class", ""))
	id := strings.ToLower(element.AttrOr("id", ""))

	for _, indicator := range contentIndicators {
		if strings.Contains(className, indicator) || strings.Contains(id, indicator) {
			return true
		}
	}

	// Elements containing code blocks or tables are likely content
	if element.Find("pre, table").Length() > 0 {
		return true
	}

	text := strings.TrimSpace(element.Text())
	words := textutil.CountWords(text)

	// Navigation heading detection: blocks with headings that match navigation
	// indicators (e.g. "Related Articles", "Popular Posts") are not content
	if words < 1000 && hasNavigationHeading(element) {
		if words < 200 {
			return false
		}
		linkCount := element.Find("a").Length()
		if float64(linkCount)/float64(max(words, 1)) > 0.2 {
			return false
		}
	}

	// Card grids are not content
	if isCardGrid(element, words) {
		return false
	}

	// Social profile links in small blocks indicate author bios, not content
	if words < 80 && hasSocialProfileLink(element) {
		return false
	}

	paragraphs := element.Find("p").Length()
	listItems := element.Find("li").Length()
	contentBlocks := paragraphs + listItems

	// If the element has a significant amount of text and content blocks, it's likely content
	if words > contentMinWordsWithBlocks && contentBlocks > 1 {
		return true
	}

	// Check for elements with significant text content
	if words > contentMinWords {
		return true
	}

	// Check for elements with text content and some content blocks
	if words > contentMinWordsSmall && contentBlocks > 0 {
		return true
	}

	// Prose text with sentence-ending punctuation and low link density
	if words >= contentMinWordsProse && sentenceEndRe.MatchString(text) {
		linkCount := element.Find("a").Length()
		if float64(linkCount)/float64(words) < 0.1 {
			return true
		}
	}

	return false
}

// hasNavigationHeading reports whether element contains a heading whose text
// matches a navigation indicator (e.g. "Related Articles", "Popular Posts").
func hasNavigationHeading(element *goquery.Selection) bool {
	found := false
	element.Find("h1, h2, h3, h4, h5, h6").EachWithBreak(func(_ int, h *goquery.Selection) bool {
		headingText := strings.ToLower(strings.TrimSpace(h.Text()))
		if navigationHeadingPattern.MatchString(headingText) {
			found = true
			return false
		}
		return true
	})
	return found
}

// hasSocialProfileLink reports whether element links to a social profile (an
// author-bio signal), excluding share/intent URLs.
func hasSocialProfileLink(element *goquery.Selection) bool {
	found := false
	element.Find("a").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		href := strings.ToLower(a.AttrOr("href", ""))
		if socialProfileRe.MatchString(href) && !isSocialIntentURL(href) {
			found = true
			return false
		}
		return true
	})
	return found
}
