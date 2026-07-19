package scoring

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
	textutil "github.com/dotcommander/defuddle/internal/text"
)

// scoreNonContentBlock scores a block element to determine if it's likely not content
// JavaScript original code:
//
//	private static scoreNonContentBlock(element: Element): number {
//		// Skip footnote list elements
//		if (element.querySelector(FOOTNOTE_LIST_SELECTORS)) {
//			return 0;
//		}
//
//		let score = 0;
//
//		// Get text content
//		const text = element.textContent || '';
//		const words = text.split(/\s+/).length;
//
//		// Skip very small elements
//		if (words < 3) {
//			return 0;
//		}
//
//		for (const indicator of navigationIndicators) {
//			if (text.toLowerCase().includes(indicator)) {
//				score -= 10;
//			}
//		}
//
//		// Check for high link density (navigation)
//		const links = element.getElementsByTagName('a').length;
//		const linkDensity = links / (words || 1);
//		if (linkDensity > 0.5) {
//			score -= 15;
//		}
//
//		// Check for list structure (navigation)
//		const lists = element.getElementsByTagName('ul').length + element.getElementsByTagName('ol').length;
//		if (lists > 0 && links > lists * 3) {
//			score -= 10;
//		}
//
//		// Check for specific class patterns that indicate non-content
//		const className = element.className.toLowerCase();
//		const id = element.id.toLowerCase();
//
//		for (const pattern of nonContentPatterns) {
//			if (className.includes(pattern) || id.includes(pattern)) {
//				score -= 8;
//			}
//		}
//
//		return score;
//	}
func scoreNonContentBlock(_ context.Context, element *goquery.Selection) float64 {
	// Skip footnote list elements and their descendants.
	// FindMatcher: element contains a footnote list (descendant check).
	// ClosestMatcher: element is inside a footnote list (ancestor check).
	// Both guards are needed: a footnote-list parent must score 0, and so
	// must its child block elements that happen to be visited first.
	if element.FindMatcher(constants.FootnoteListMatcher).Length() > 0 ||
		element.ClosestMatcher(constants.FootnoteListMatcher).Length() > 0 {
		return 0
	}

	score := 0.0

	// Get text content
	text := strings.TrimSpace(element.Text())
	words := textutil.CountWords(text)

	// Skip very small elements
	if words < 3 {
		return 0
	}

	// Comma counting — prose has commas, navigation/boilerplate doesn't
	commas := strings.Count(text, ",")
	score += float64(commas)

	// Check for navigation indicators using word-boundary regexes.
	// Fast path: combined alternation is O(1) rejection — most real text has no
	// indicators, so we skip the per-regex loop entirely in the common case.
	// Slow path preserves the count-per-distinct-indicator semantics: each matching
	// regex contributes -10 independently (a block with 3 indicators scores -30, not -10).
	lowerText := strings.ToLower(text)
	indicatorMatches := 0
	if navigationHeadingPattern.MatchString(lowerText) {
		for _, re := range navigationIndicatorRegexes {
			if re.MatchString(lowerText) {
				indicatorMatches++
			}
		}
	}
	score -= float64(indicatorMatches) * 10

	// Collect link count and aggregate link-text length in one pass.
	links := 0
	linkTextLen := 0
	element.Find("a").Each(func(_ int, a *goquery.Selection) {
		links++
		linkTextLen += len(a.Text())
	})

	// Check for high link density (navigation)
	linkDensity := float64(links) / float64(max(words, 1))
	if linkDensity > 0.5 {
		score -= 15
	}

	// Check for high link text ratio (e.g. card groups, nav sections).
	// Requires multiple links to avoid penalizing content paragraphs
	// that happen to be wrapped in a single link.
	if links > 1 && words < 80 {
		totalTextLen := len(text)
		if totalTextLen > 0 && float64(linkTextLen)/float64(totalTextLen) > 0.8 {
			score -= 15
		}
	}

	// Check for list structure (navigation)
	lists := element.Find("ul").Length() + element.Find("ol").Length()
	if lists > 0 && links > lists*3 {
		score -= 10
	}

	// Check for social media profile links (author bios, social widgets)
	if words < 80 && hasSocialProfileLink(element) {
		score -= 15
	}

	// Penalize very small blocks that look like standalone author bylines with dates
	if words < 15 {
		if bylineAuthorRe.MatchString(text) && bylineDateRe.MatchString(text) {
			score -= 10
		}
	}

	// Penalize blocks that look like article card grids
	if isCardGrid(element, words) {
		score -= 15
	}

	// Check for specific class patterns that indicate non-content
	className := strings.ToLower(element.AttrOr("class", ""))
	id := strings.ToLower(element.AttrOr("id", ""))

	for _, pattern := range nonContentPatterns {
		if strings.Contains(className, pattern) || strings.Contains(id, pattern) {
			score -= 8
		}
	}

	return score
}

// isSocialIntentURL returns true if the URL is a social sharing/intent URL
// rather than a profile URL. These should not trigger the social profile penalty.
func isSocialIntentURL(href string) bool {
	return strings.Contains(href, "/intent") ||
		strings.Contains(href, "/share") ||
		strings.Contains(href, "/sharer")
}

// isCardGrid detects article card grids: blocks with 3+ headings and 2+ images
// but very little prose per heading.
func isCardGrid(element *goquery.Selection, words int) bool {
	if words < 3 || words >= 500 {
		return false
	}
	headings := element.Find("h2, h3, h4")
	if headings.Length() < 3 {
		return false
	}
	images := element.Find("img")
	if images.Length() < 2 {
		return false
	}
	headingWordCount := 0
	headings.Each(func(_ int, h *goquery.Selection) {
		headingWordCount += textutil.CountWords(strings.TrimSpace(h.Text()))
	})
	prosePerHeading := float64(words-headingWordCount) / float64(headings.Length())
	return prosePerHeading < 20
}
