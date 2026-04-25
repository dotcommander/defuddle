// Package scoring provides content scoring functionality for the defuddle content extraction system.
// It implements algorithms to score DOM elements based on content quality and relevance.
package scoring

import (
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
	textutil "github.com/dotcommander/defuddle/internal/text"
)

// Pre-compiled regex patterns for content scoring.
var (
	dateRe   = regexp.MustCompile(`(?i)\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\s+\d{1,2},?\s+\d{4}\b`)
	authorRe = regexp.MustCompile(`(?i)\b(?:by|written by|author:)\s+[A-Za-z\s]+\b`)

	// Social media profile URL pattern — used to detect author bios.
	// Go regexp doesn't support lookaheads, so we match broadly here
	// and filter out intent/share URLs in the calling code.
	socialProfileRe = regexp.MustCompile(`(?i)(linkedin\.com/(in|company)/|twitter\.com/\w|x\.com/\w|facebook\.com/\w|instagram\.com/\w|threads\.net/\w|mastodon\.\w)`)

	// Date pattern for detecting standalone bylines (no leading \b because
	// textContent can concatenate adjacent elements without whitespace)
	bylineDateRe = regexp.MustCompile(`(?i)(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\s+\d{1,2}`)

	// Author attribution pattern — case-sensitive "By" + capitalized name
	bylineAuthorRe = regexp.MustCompile(`\bBy\s+[A-Z]`)

	// Sentence-ending punctuation for prose detection
	sentenceEndRe = regexp.MustCompile(`[.?!]`)
)

// Pre-compiled word-boundary regexes for navigation indicator matching.
// Using \b prevents false positives like "share" matching inside "shareholders".
var navigationIndicatorRegexes = compileNavigationRegexes()

// navigationHeadingPattern is a combined regex for heading text matching in isLikelyContent.
var navigationHeadingPattern = compileNavigationHeadingPattern()

func compileNavigationRegexes() []*regexp.Regexp {
	regexes := make([]*regexp.Regexp, len(navigationIndicators))
	for i, indicator := range navigationIndicators {
		escaped := regexp.QuoteMeta(indicator)
		escaped = strings.ReplaceAll(escaped, `\ `, `\s+`)
		regexes[i] = regexp.MustCompile(`(?i)\b` + escaped + `\b`)
	}
	return regexes
}

func compileNavigationHeadingPattern() *regexp.Regexp {
	patterns := make([]string, len(navigationIndicators))
	for i, indicator := range navigationIndicators {
		escaped := regexp.QuoteMeta(indicator)
		escaped = strings.ReplaceAll(escaped, `\ `, `\s+`)
		patterns[i] = `\b` + escaped + `\b`
	}
	return regexp.MustCompile(`(?i)` + strings.Join(patterns, "|"))
}

// Scoring bonus/penalty constants for ScoreElement.
const (
	scoreParagraphBonus       = 10.0 // per paragraph
	scoreImageDensityFactor   = 3.0  // multiplied by image/word density
	scoreRightSideBonus       = 5.0  // right-aligned elements
	scoreDateBonus            = 10.0 // element contains a recognisable date
	scoreAuthorBonus          = 10.0 // element contains an author attribution
	scoreContentClassBonus    = 15.0 // element class includes content/article/post
	scoreFootnoteBonus        = 10.0 // element contains footnote references
	scoreNestedTablePenalty   = 5.0  // per nested table
	scoreCenterCellBonus      = 10.0 // td that is a centre cell in a layout table
	scoreContentTableMinWidth = 400  // pixel width threshold for content-layout tables
	scoreLinkDensityCap       = 0.5  // cap on link-text/total-text ratio
)

// Word-count thresholds used in isLikelyContent.
const (
	contentMinWords           = 100 // sufficient alone to signal content
	contentMinWordsWithBlocks = 50  // sufficient with 2+ content blocks
	contentMinWordsSmall      = 30  // sufficient with 1+ content block
	contentMinWordsProse      = 10  // sufficient with sentence-ending punct + low link density
)

// ContentScore represents a scored element
// JavaScript original code:
//
//	export interface ContentScore {
//	  score: number;
//	  element: Element;
//	}
type ContentScore struct {
	Score   float64
	Element *goquery.Selection
}

// contentIndicators are class/id patterns that indicate content elements
// JavaScript original code:
//
//	const contentIndicators = [
//		'admonition',
//		'article',
//		'content',
//		'entry',
//		'image',
//		'img',
//		'font',
//		'figure',
//		'figcaption',
//		'pre',
//		'main',
//		'post',
//		'story',
//		'table'
//	];
var contentIndicators = []string{
	"admonition",
	"article",
	"content",
	"entry",
	"image",
	"img",
	"font",
	"figure",
	"figcaption",
	"pre",
	"main",
	"post",
	"story",
	"table",
}

// navigationIndicators are text patterns that indicate navigation/non-content
// JavaScript original code:
//
//	const navigationIndicators = [
//		'advertisement',
//		'all rights reserved',
//		'banner',
//		'cookie',
//		'comments',
//		'copyright',
//		'follow me',
//		'follow us',
//		'footer',
//		'header',
//		'homepage',
//		'login',
//		'menu',
//		'more articles',
//		'more like this',
//		'most read',
//		'nav',
//		'navigation',
//		'newsletter',
//		'newsletter',
//		'popular',
//		'privacy',
//		'recommended',
//		'register',
//		'related',
//		'responses',
//		'share',
//		'sidebar',
//		'sign in',
//		'sign up',
//		'signup',
//		'social',
//		'sponsored',
//		'subscribe',
//		'subscribe',
//		'terms',
//		'trending'
//	];
var navigationIndicators = []string{
	"advertisement",
	"all rights reserved",
	"banner",
	"cookie",
	"comments",
	"copyright",
	"follow me",
	"follow us",
	"footer",
	"header",
	"homepage",
	"login",
	"menu",
	"more articles",
	"more like this",
	"most read",
	"nav",
	"navigation",
	"newsletter",
	"popular",
	"privacy",
	"recommended",
	"register",
	"related",
	"responses",
	"share",
	"sidebar",
	"sign in",
	"sign up",
	"signup",
	"social",
	"sponsored",
	"subscribe",
	"terms",
	"trending",
}

// nonContentPatterns are class/id patterns that indicate non-content elements
// JavaScript original code:
//
//	const nonContentPatterns = [
//		'ad',
//		'banner',
//		'cookie',
//		'copyright',
//		'footer',
//		'header',
//		'homepage',
//		'menu',
//		'nav',
//		'newsletter',
//		'popular',
//		'privacy',
//		'recommended',
//		'related',
//		'rights',
//		'share',
//		'sidebar',
//		'social',
//		'sponsored',
//		'subscribe',
//		'terms',
//		'trending',
//		'widget'
//	];
var nonContentPatterns = []string{
	"advert",
	"ad-",
	"ads",
	"banner",
	"cookie",
	"copyright",
	"footer",
	"header",
	"homepage",
	"menu",
	"nav",
	"newsletter",
	"popular",
	"privacy",
	"recommended",
	"related",
	"rights",
	"share",
	"sidebar",
	"social",
	"sponsored",
	"subscribe",
	"terms",
	"trending",
	"widget",
}

// ScoreElement scores an element based on various content indicators
// JavaScript original code:
//
//	static scoreElement(element: Element): number {
//		let score = 0;
//
//		// Text density
//		const text = element.textContent || '';
//		const words = text.split(/\s+/).length;
//		score += words;
//
//		// Paragraph ratio
//		const paragraphs = element.getElementsByTagName('p').length;
//		score += paragraphs * 10;
//
//		// Link density (penalize high link density)
//		const links = element.getElementsByTagName('a').length;
//		const linkDensity = links / (words || 1);
//		score -= linkDensity * 5;
//
//		// Image ratio (penalize high image density)
//		const images = element.getElementsByTagName('img').length;
//		const imageDensity = images / (words || 1);
//		score -= imageDensity * 3;
//
//		// Position bonus (center/right elements)
//		try {
//			const style = element.getAttribute('style') || '';
//			const align = element.getAttribute('align') || '';
//			const isRightSide = style.includes('float: right') ||
//							   style.includes('text-align: right') ||
//							   align === 'right';
//			if (isRightSide) score += 5;
//		} catch (e) {
//			// Ignore position if we can't get style
//		}
//
//		// Content indicators
//		const hasDate = /\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\s+\d{1,2},?\s+\d{4}\b/i.test(text);
//		if (hasDate) score += 10;
//
//		const hasAuthor = /\b(?:by|written by|author:)\s+[A-Za-z\s]+\b/i.test(text);
//		if (hasAuthor) score += 10;
//
//		// Check for common content classes/attributes
//		const className = element.className.toLowerCase();
//		if (className.includes('content') || className.includes('article') || className.includes('post')) {
//			score += 15;
//		}
//
//		// Check for footnotes/references
//		const hasFootnotes = element.querySelector(FOOTNOTE_INLINE_REFERENCES);
//		if (hasFootnotes) score += 10;
//
//		const hasFootnotesList = element.querySelector(FOOTNOTE_LIST_SELECTORS);
//		if (hasFootnotesList) score += 10;
//
//		// Check for nested tables (penalize)
//		const nestedTables = element.getElementsByTagName('table').length;
//		score -= nestedTables * 5;
//
//		// Additional scoring for table cells
//		if (element.tagName.toLowerCase() === 'td') {
//			// Table cells get a bonus for being in the main content area
//			const parentTable = element.closest('table');
//			if (parentTable) {
//				// Only favor cells in tables that look like old-style content layouts
//				const tableWidth = parseInt(parentTable.getAttribute('width') || '0');
//				const tableAlign = parentTable.getAttribute('align') || '';
//				const tableClass = parentTable.className.toLowerCase();
//				const isTableLayout =
//					tableWidth > 400 || // Common width for main content tables
//					tableAlign === 'center' ||
//					tableClass.includes('content') ||
//					tableClass.includes('article');
//
//				if (isTableLayout) {
//					// Additional checks to ensure this is likely the main content cell
//					const allCells = Array.from(parentTable.getElementsByTagName('td'));
//					const cellIndex = allCells.indexOf(element as HTMLTableCellElement);
//					const isCenterCell = cellIndex > 0 && cellIndex < allCells.length - 1;
//
//					if (isCenterCell) {
//						score += 10;
//					}
//				}
//			}
//		}
//
//		return score;
//	}
func ScoreElement(element *goquery.Selection) float64 {
	text := strings.TrimSpace(element.Text())
	words := textutil.CountWords(text)
	className := strings.ToLower(element.AttrOr("class", ""))

	score := scoreTextDensity(element, words)
	score += scoreImagePenalty(element, words)
	score += scorePositionBonus(element)
	score += scoreContentSignals(element, text, className)
	score += scoreTableCellBonus(element)
	score = scoreLinkDensityMultiplier(element, text, score)
	return score
}

// scoreTextDensity returns word count + paragraph bonus + comma bonus.
func scoreTextDensity(element *goquery.Selection, words int) float64 {
	paragraphs := element.Find("p").Length()
	text := element.Text()
	commas := strings.Count(text, ",")
	return float64(words) + float64(paragraphs)*scoreParagraphBonus + float64(commas)
}

// scoreImagePenalty penalises high image density relative to word count.
func scoreImagePenalty(element *goquery.Selection, words int) float64 {
	images := element.Find("img").Length()
	imageDensity := float64(images) / float64(max(words, 1))
	return -(imageDensity * scoreImageDensityFactor)
}

// scorePositionBonus rewards right-aligned elements.
func scorePositionBonus(element *goquery.Selection) float64 {
	style, _ := element.Attr("style")
	align, _ := element.Attr("align")
	isRightSide := strings.Contains(style, "float: right") ||
		strings.Contains(style, "text-align: right") ||
		align == "right"
	if isRightSide {
		return scoreRightSideBonus
	}
	return 0
}

// scoreContentSignals adds bonuses for dates, author attributions, content
// class names, footnotes, and deducts for nested tables.
func scoreContentSignals(element *goquery.Selection, text, className string) float64 {
	score := 0.0

	if dateRe.MatchString(text) {
		score += scoreDateBonus
	}
	if authorRe.MatchString(text) {
		score += scoreAuthorBonus
	}

	if strings.Contains(className, "content") ||
		strings.Contains(className, "article") ||
		strings.Contains(className, "post") {
		score += scoreContentClassBonus
	}

	if element.FindMatcher(constants.FootnoteInlineMatcher).Length() > 0 {
		score += scoreFootnoteBonus
	}

	if element.FindMatcher(constants.FootnoteListMatcher).Length() > 0 {
		score += scoreFootnoteBonus
	}

	nestedTables := element.Find("table").Length()
	score -= float64(nestedTables) * scoreNestedTablePenalty

	return score
}

// scoreTableCellBonus adds a bonus when the element is a centre cell of a
// content-layout table.
func scoreTableCellBonus(element *goquery.Selection) float64 {
	if goquery.NodeName(element) != "td" {
		return 0
	}
	parentTable := element.Closest("table")
	if parentTable.Length() == 0 {
		return 0
	}

	widthStr, _ := parentTable.Attr("width")
	tableWidth := 0
	if widthStr != "" {
		if w, err := strconv.Atoi(widthStr); err == nil {
			tableWidth = w
		}
	}
	tableAlign, _ := parentTable.Attr("align")
	tableClass := strings.ToLower(parentTable.AttrOr("class", ""))

	isTableLayout := tableWidth > scoreContentTableMinWidth ||
		tableAlign == "center" ||
		strings.Contains(tableClass, "content") ||
		strings.Contains(tableClass, "article")

	if !isTableLayout {
		return 0
	}

	allCells := parentTable.Find("td")
	cellIndex := -1
	allCells.Each(func(i int, cell *goquery.Selection) {
		if cell.Get(0) == element.Get(0) {
			cellIndex = i
		}
	})

	isCenterCell := cellIndex > 0 && cellIndex < allCells.Length()-1
	if isCenterCell {
		return scoreCenterCellBonus
	}
	return 0
}

// scoreLinkDensityMultiplier scales score by (1 - link-text density), capped
// at scoreLinkDensityCap. Must be the last scoring step.
func scoreLinkDensityMultiplier(element *goquery.Selection, text string, score float64) float64 {
	linkTextLen := 0
	element.Find("a").Each(func(_ int, a *goquery.Selection) {
		linkTextLen += len(strings.TrimSpace(a.Text()))
	})
	textLen := max(len(text), 1)
	linkDensity := min(float64(linkTextLen)/float64(textLen), scoreLinkDensityCap)
	return score * (1.0 - linkDensity)
}

// FindBestElement finds the best scoring element from a list
// JavaScript original code:
//
//	static findBestElement(elements: Element[], minScore: number = 50): Element | null {
//		let bestElement: Element | null = null;
//		let bestScore = 0;
//
//		elements.forEach(element => {
//			const score = this.scoreElement(element);
//			if (score > bestScore) {
//				bestScore = score;
//				bestElement = element;
//			}
//		});
//
//		return bestScore > minScore ? bestElement : null;
//	}
func FindBestElement(elements []*goquery.Selection, minScore float64) *goquery.Selection {
	var bestElement *goquery.Selection
	bestScore := 0.0

	for _, element := range elements {
		score := ScoreElement(element)
		if score > bestScore {
			bestScore = score
			bestElement = element
		}
	}

	if bestScore > minScore {
		return bestElement
	}
	return nil
}

// NodeContains returns true if ancestor contains descendant in the DOM tree.
func NodeContains(ancestor, descendant *goquery.Selection) bool {
	if ancestor == nil || descendant == nil || ancestor.Length() == 0 || descendant.Length() == 0 {
		return false
	}
	ancestorNode := ancestor.Get(0)
	for n := descendant.Get(0); n != nil; n = n.Parent {
		if n == ancestorNode {
			return true
		}
	}
	return false
}

// IsProtectedNode returns true if el should never be removed:
//   - el is an ancestor of mainContent (removing it would destroy the content)
//   - el is inside a code block (pre or code)
func IsProtectedNode(el *goquery.Selection, mainContent *goquery.Selection) bool {
	if mainContent != nil && NodeContains(el, mainContent) {
		return true
	}
	return el.Closest("pre").Length() > 0 || el.Closest("code").Length() > 0
}

// ScoreAndRemove scores blocks and removes those that are likely not content.
// JavaScript original code:
//
//	public static scoreAndRemove(doc: Document, debug: boolean = false) {
//		const startTime = Date.now();
//		let removedCount = 0;
//
//		// Track all elements to be removed
//		const elementsToRemove = new Set<Element>();
//
//		// Get all block elements
//		const blockElements = Array.from(doc.querySelectorAll(BLOCK_ELEMENTS.join(',')));
//
//		// Process each block element
//		blockElements.forEach(element => {
//			// Skip elements that are already marked for removal
//			if (elementsToRemove.has(element)) {
//				return;
//			}
//
//			// Skip elements that are likely to be content
//			if (ContentScorer.isLikelyContent(element)) {
//				return;
//			}
//
//			// Score the element based on various criteria
//			const score = ContentScorer.scoreNonContentBlock(element);
//
//			// If the score is below the threshold, mark for removal
//			if (score < 0) {
//				elementsToRemove.add(element);
//				removedCount++;
//			}
//		});
//
//		// Remove all collected elements in a single pass
//		elementsToRemove.forEach(el => el.remove());
//
//		const endTime = Date.now();
//		if (debug) {
//			console.log('Defuddle', 'Removed non-content blocks:', {
//				count: removedCount,
//				processingTime: `${(endTime - startTime).toFixed(2)}ms`
//			});
//		}
//	}
func ScoreAndRemove(doc *goquery.Document, debug bool, mainContent *goquery.Selection) {
	startTime := time.Now()
	removedCount := 0

	// Track all elements to be removed
	elementsToRemove := make([]*goquery.Selection, 0, 10) // Pre-allocate with reasonable capacity

	// Get all block elements
	blockElements := constants.GetBlockElements()
	blockSelector := strings.Join(blockElements, ",")

	// Process each block element
	doc.Find(blockSelector).Each(func(_ int, element *goquery.Selection) {
		if IsProtectedNode(element, mainContent) {
			return
		}

		// Skip elements that are likely to be content
		if isLikelyContent(element) {
			return
		}

		// Score the element based on various criteria
		score := scoreNonContentBlock(element)

		// If the score is below the threshold, mark for removal
		if score < 0 {
			elementsToRemove = append(elementsToRemove, element)
			removedCount++
		}
	})

	// Remove all collected elements in a single pass
	for _, el := range elementsToRemove {
		el.Remove()
	}

	endTime := time.Now()
	if debug {
		processingTime := float64(endTime.Sub(startTime).Nanoseconds()) / 1e6 // Convert to milliseconds
		slog.Debug("Removed non-content blocks",
			"count", removedCount,
			"processingTime", processingTime)
	}
}

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
func isLikelyContent(element *goquery.Selection) bool {
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
	if words < 1000 {
		hasNavigationHeading := false
		element.Find("h1, h2, h3, h4, h5, h6").EachWithBreak(func(_ int, h *goquery.Selection) bool {
			headingText := strings.ToLower(strings.TrimSpace(h.Text()))
			if navigationHeadingPattern.MatchString(headingText) {
				hasNavigationHeading = true
				return false
			}
			return true
		})
		if hasNavigationHeading {
			if words < 200 {
				return false
			}
			linkCount := element.Find("a").Length()
			if float64(linkCount)/float64(max(words, 1)) > 0.2 {
				return false
			}
		}
	}

	// Card grids are not content
	if isCardGrid(element, words) {
		return false
	}

	// Social profile links in small blocks indicate author bios, not content
	if words < 80 {
		hasSocialProfile := false
		element.Find("a").EachWithBreak(func(_ int, a *goquery.Selection) bool {
			href := strings.ToLower(a.AttrOr("href", ""))
			if socialProfileRe.MatchString(href) && !isSocialIntentURL(href) {
				hasSocialProfile = true
				return false
			}
			return true
		})
		if hasSocialProfile {
			return false
		}
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
func scoreNonContentBlock(element *goquery.Selection) float64 {
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

	// Check for high link density (navigation)
	links := element.Find("a").Length()
	linkDensity := float64(links) / float64(max(words, 1))
	if linkDensity > 0.5 {
		score -= 15
	}

	// Check for high link text ratio (e.g. card groups, nav sections).
	// Requires multiple links to avoid penalizing content paragraphs
	// that happen to be wrapped in a single link.
	if links > 1 && words < 80 {
		linkTextLen := 0
		element.Find("a").Each(func(_ int, a *goquery.Selection) {
			linkTextLen += len(a.Text())
		})
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
	if words < 80 {
		element.Find("a").EachWithBreak(func(_ int, a *goquery.Selection) bool {
			href := strings.ToLower(a.AttrOr("href", ""))
			if socialProfileRe.MatchString(href) && !isSocialIntentURL(href) {
				score -= 15
				return false // break
			}
			return true
		})
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
