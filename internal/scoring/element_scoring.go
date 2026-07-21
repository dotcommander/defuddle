package scoring

import (
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
	textutil "github.com/dotcommander/defuddle/internal/text"
)

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
