package standardize

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
)

// JavaScript original code:
//
//	function removeEmptyElements(element: Element): void {
//		let removedCount = 0;
//		let iterations = 0;
//		let keepRemoving = true;
//
//		while (keepRemoving) {
//			iterations++;
//			keepRemoving = false;
//			// Get all elements without children, working from deepest first
//			const emptyElements = Array.from(element.getElementsByTagName('*')).filter(el => {
//				if (ALLOWED_EMPTY_ELEMENTS.has(el.tagName.toLowerCase())) {
//					return false;
//				}
//
//				// Check if element has only whitespace or &nbsp;
//				const textContent = el.textContent || '';
//				const hasOnlyWhitespace = textContent.trim().length === 0;
//				const hasNbsp = textContent.includes('\u00A0'); // Unicode non-breaking space
//
//				// Check if element has no meaningful children
//				const hasNoChildren = !el.hasChildNodes() ||
//					(Array.from(el.childNodes).every(node => {
//						if (isTextNode(node)) { // TEXT_NODE
//							const nodeText = node.textContent || '';
//							return nodeText.trim().length === 0 && !nodeText.includes('\u00A0');
//						}
//						return false;
//					}));
//
//				// Special case: Check for divs that only contain spans with commas
//				if (el.tagName.toLowerCase() === 'div') {
//					const children = Array.from(el.children);
//					const hasOnlyCommaSpans = children.length > 0 && children.every(child => {
//						if (child.tagName.toLowerCase() !== 'span') return false;
//						const content = child.textContent?.trim() || '';
//						return content === ',' || content === '' || content === ' ';
//					});
//					if (hasOnlyCommaSpans) return true;
//				}
//
//				return hasOnlyWhitespace && !hasNbsp && hasNoChildren;
//			});
//
//			if (emptyElements.length > 0) {
//				emptyElements.forEach(el => {
//					el.remove();
//					removedCount++;
//				});
//				keepRemoving = true;
//			}
//		}
//
//		logDebug('Removed empty elements:', removedCount, 'iterations:', iterations);
//	}
//
// removeEmptyElements removes empty elements that don't contribute content
func removeEmptyElements(element *goquery.Selection, debug bool) {
	removedCount := 0
	iterations := 0
	keepRemoving := true

	for keepRemoving {
		iterations++
		keepRemoving = false

		// Get all elements and filter for empty ones, working from deepest first
		var emptyElements []*goquery.Selection

		element.Find("*").Each(func(_ int, el *goquery.Selection) {
			if isRemovableEmptyElement(el) {
				emptyElements = append(emptyElements, el)
			}
		})

		// Remove empty elements
		if len(emptyElements) > 0 {
			for _, el := range emptyElements {
				el.Remove()
				removedCount++
			}
			keepRemoving = true
		}
	}

	if debug {
		slog.Debug("Removed empty elements",
			"count", removedCount,
			"iterations", iterations)
	}
}

// isRemovableEmptyElement reports whether el should be removed as empty: a
// non-allowed-empty tag that is whitespace-only with no meaningful children, or
// a div containing only comma/blank spans.
func isRemovableEmptyElement(el *goquery.Selection) bool {
	tagName := strings.ToLower(goquery.NodeName(el))

	// Skip allowed empty elements
	if constants.IsAllowedEmptyElement(tagName) {
		return false
	}

	// Check if element has only whitespace or &nbsp;
	textContent := el.Text()
	hasOnlyWhitespace := strings.TrimSpace(textContent) == ""
	hasNbsp := strings.Contains(textContent, "\u00A0") // Unicode non-breaking space

	// Check if element has no meaningful children
	hasNoChildren := true
	el.Contents().Each(func(_ int, child *goquery.Selection) {
		if goquery.NodeName(child) == "#text" {
			nodeText := child.Text()
			if strings.TrimSpace(nodeText) != "" || strings.Contains(nodeText, "\u00A0") {
				hasNoChildren = false
			}
		} else {
			hasNoChildren = false
		}
	})

	// If no child nodes at all, it's definitely empty
	if el.Contents().Length() == 0 {
		hasNoChildren = true
	}

	// Special case: Check for divs that only contain spans with commas
	if tagName == "div" {
		children := el.Children()
		if children.Length() > 0 {
			hasOnlyCommaSpans := true
			children.Each(func(_ int, child *goquery.Selection) {
				childTag := strings.ToLower(goquery.NodeName(child))
				if childTag != "span" {
					hasOnlyCommaSpans = false
					return
				}
				content := strings.TrimSpace(child.Text())
				if content != "," && content != "" && content != " " {
					hasOnlyCommaSpans = false
					return
				}
			})
			if hasOnlyCommaSpans {
				return true
			}
		}
	}

	// Element is empty if it has only whitespace, no &nbsp;, and no meaningful children
	return hasOnlyWhitespace && !hasNbsp && hasNoChildren
}
