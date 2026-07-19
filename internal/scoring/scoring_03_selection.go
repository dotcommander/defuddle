package scoring

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
)

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
func ScoreAndRemove(ctx context.Context, doc *goquery.Document, debug bool, mainContent *goquery.Selection) {
	startTime := time.Now()
	removedCount := 0

	// Track all elements to be removed
	elementsToRemove := make([]*goquery.Selection, 0, 10) // Pre-allocate with reasonable capacity

	// Get all block elements
	blockElements := constants.GetBlockElements()
	blockSelector := strings.Join(blockElements, ",")

	// Process each block element. Honor ctx cancellation at the loop head.
	doc.Find(blockSelector).EachWithBreak(func(_ int, element *goquery.Selection) bool {
		if ctx.Err() != nil {
			return false
		}
		if IsProtectedNode(element, mainContent) {
			return true
		}

		// Skip elements that are likely to be content
		if isLikelyContent(ctx, element) {
			return true
		}

		// Score the element based on various criteria
		score := scoreNonContentBlock(ctx, element)

		// If the score is below the threshold, mark for removal
		if score < 0 {
			elementsToRemove = append(elementsToRemove, element)
			removedCount++
		}
		return true
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
