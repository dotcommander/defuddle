package standardize

import (
	"cmp"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
)

// flattenWrapperElements removes unnecessary wrapper divs
// JavaScript original code:
//
//	function flattenWrapperElements(element: Element, doc: Document): void {
//		let processedCount = 0;
//		const startTime = Date.now();
//
//		// Process in batches to maintain performance
//		let keepProcessing = true;
//
//		// Helper function to check if an element directly contains inline content
//		// This helps prevent unwrapping divs that visually act as paragraphs.
//		function hasDirectInlineContent(el: Element): boolean {
//			for (const child of el.childNodes) {
//				// Check for non-empty text nodes
//				if (isTextNode(child) && child.textContent?.trim()) {
//					return true;
//				}
//				// Check for element nodes that are considered inline
//				if (isElement(child) && INLINE_ELEMENTS.has(child.nodeName.toLowerCase())) {
//					return true;
//				}
//			}
//			return false;
//		}
//
//		const shouldPreserveElement = (el: Element): boolean => {
//			const tagName = el.tagName.toLowerCase();
//
//			// Check if element should be preserved
//			if (PRESERVE_ELEMENTS.has(tagName)) return true;
//
//			// Check for semantic roles
//			const role = el.getAttribute('role');
//			if (role && ['article', 'main', 'navigation', 'banner', 'contentinfo'].includes(role)) {
//				return true;
//			}
//
//			// Check for semantic classes
//			const className = el.className;
//			if (typeof className === 'string' && className.toLowerCase().match(/(?:article|main|content|footnote|reference|bibliography)/)) {
//				return true;
//			}
//
//			// Check if element contains mixed content types that should be preserved
//			const children = Array.from(el.children);
//			const hasPreservedElements = children.some(child =>
//				PRESERVE_ELEMENTS.has(child.tagName.toLowerCase()) ||
//				child.getAttribute('role') === 'article' ||
//				(child.className && typeof child.className === 'string' &&
//					child.className.toLowerCase().match(/(?:article|main|content|footnote|reference|bibliography)/))
//			);
//			if (hasPreservedElements) return true;
//
//			return false;
//		};
//
//		const isWrapperElement = (el: Element): boolean => {
//			// If it directly contains inline content, it's NOT a wrapper
//			if (hasDirectInlineContent(el)) {
//				return false;
//			}
//
//			// Check if it's just empty space
//			if (!el.textContent?.trim()) return true;
//
//			// Check if it only contains other block elements
//			const children = Array.from(el.children);
//			if (children.length === 0) return true;
//
//			// Check if all children are block elements
//			const allBlockElements = children.every(child => {
//				const tag = child.tagName.toLowerCase();
//				return BLOCK_ELEMENTS.includes(tag) ||
//					   tag === 'p' || tag === 'h1' || tag === 'h2' ||
//					   tag === 'h3' || tag === 'h4' || tag === 'h5' || tag === 'h6' ||
//					   tag === 'ul' || tag === 'ol' || tag === 'pre' || tag === 'blockquote' ||
//					   tag === 'figure';
//			});
//			if (allBlockElements) return true;
//
//			// Check for common wrapper patterns
//			const className = el.className.toLowerCase();
//			const isWrapper = /(?:wrapper|container|layout|row|col|grid|flex|outer|inner|content-area)/i.test(className);
//			if (isWrapper) return true;
//
//			// Check if it has excessive whitespace or empty text nodes
//			const textNodes = Array.from(el.childNodes).filter(node =>
//				isTextNode(node) && node.textContent?.trim()
//			);
//			if (textNodes.length === 0) return true;
//
//			// Check if it only contains block elements
//			const hasOnlyBlockElements = children.length > 0 && !children.some(child => {
//				const tag = child.tagName.toLowerCase();
//				return INLINE_ELEMENTS.has(tag);
//			});
//			if (hasOnlyBlockElements) return true;
//
//			return false;
//		};
//
//		// ... (complex processing logic continues)
//	}
func flattenWrapperElements(element *goquery.Selection, _ *goquery.Document, debug bool) {
	processedCount := 0
	startTime := time.Now()

	// Pre-compute the block selector string used by multiple passes
	blockElements := constants.GetBlockElements()
	blockSelector := strings.Join(blockElements, ",")

	// Process in batches to maintain performance
	keepProcessing := true

	processElement := func(el *goquery.Selection) bool {
		modified := flattenElement(el, element, blockElements)
		if modified {
			processedCount++
		}
		return modified
	}

	// First pass: Process top-level wrapper elements
	processTopLevelElements := func() bool {
		modified := false

		element.Children().Each(func(_ int, el *goquery.Selection) {
			tag := goquery.NodeName(el)
			isBlock := slices.Contains(blockElements, tag)

			if isBlock && processElement(el) {
				modified = true
			}
		})

		return modified
	}

	// Second pass: Process remaining wrapper elements from deepest to shallowest
	processRemainingElements := func() bool {
		modified := false

		// Get all wrapper elements and sort by depth (deepest first)
		var allElements []*goquery.Selection
		element.Find(blockSelector).Each(func(_ int, el *goquery.Selection) {
			allElements = append(allElements, el)
		})

		// Sort by depth descending (deepest first)
		slices.SortFunc(allElements, func(a, b *goquery.Selection) int {
			return cmp.Compare(b.Parents().Length(), a.Parents().Length())
		})

		for _, el := range allElements {
			if processElement(el) {
				modified = true
			}
		}

		return modified
	}

	// Final cleanup pass - aggressively flatten remaining wrapper elements
	finalCleanup := func() bool {
		modified := false

		element.Find(blockSelector).Each(func(_ int, el *goquery.Selection) {
			// Check if element only contains paragraphs
			children := el.Children()
			onlyParagraphs := children.Length() > 0

			children.Each(func(_ int, child *goquery.Selection) {
				if goquery.NodeName(child) != "p" {
					onlyParagraphs = false
				}
			})

			// Unwrap if it only contains paragraphs OR is a non-preserved wrapper element
			if onlyParagraphs || (!shouldPreserveElement(el) && isWrapperElement(el, blockElements)) {
				html, _ := el.Html()
				el.ReplaceWithHtml(html)
				processedCount++
				modified = true
			}
		})

		return modified
	}

	// Execute all passes until no more changes
	for keepProcessing {
		keepProcessing = false
		if processTopLevelElements() {
			keepProcessing = true
		}
		if processRemainingElements() {
			keepProcessing = true
		}
		if finalCleanup() {
			keepProcessing = true
		}
	}

	endTime := time.Now()
	processingTime := float64(endTime.Sub(startTime).Nanoseconds()) / 1e6 // Convert to milliseconds
	if debug {
		slog.Debug("Flattened wrapper elements",
			"count", processedCount,
			"processingTime", processingTime)
	}
}
