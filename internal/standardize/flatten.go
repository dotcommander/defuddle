package standardize

import (
	"cmp"
	"html"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/internal/constants"
)

var (
	semanticClassRe = regexp.MustCompile(`(?:article|main|content|footnote|reference|bibliography)`)
	wrapperClassRe  = regexp.MustCompile(`(?:wrapper|container|layout|row|col|grid|flex|outer|inner|content-area)`)
	// additionalBlockElements supplements constants.GetBlockElements() with heading/list elements
	// that should be treated as block-level during flattening and empty-element cleanup.
	additionalBlockElements = []string{"p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "pre", "blockquote", "figure"}
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

	// Helper function to check if an element directly contains inline content
	hasDirectInlineContent := func(el *goquery.Selection) bool {
		hasInlineContent := false
		el.Contents().Each(func(_ int, child *goquery.Selection) {
			if goquery.NodeName(child) == "#text" {
				text := strings.TrimSpace(child.Text())
				if text != "" {
					hasInlineContent = true
				}
			} else {
				tagName := goquery.NodeName(child)
				if constants.IsInlineElement(tagName) {
					hasInlineContent = true
				}
			}
		})
		return hasInlineContent
	}

	shouldPreserveElement := func(el *goquery.Selection) bool {
		tagName := goquery.NodeName(el)

		// Check if element should be preserved
		if constants.IsPreserveElement(tagName) {
			return true
		}

		// Check for semantic roles
		role, _ := el.Attr("role")
		semanticRoles := []string{"article", "main", "navigation", "banner", "contentinfo"}
		if slices.Contains(semanticRoles, role) {
			return true
		}

		// Check for semantic classes
		className := strings.ToLower(el.AttrOr("class", ""))
		if semanticClassRe.MatchString(className) {
			return true
		}

		// Check if element contains mixed content types that should be preserved
		hasPreservedElements := false
		el.Children().Each(func(_ int, child *goquery.Selection) {
			childTag := goquery.NodeName(child)
			childRole, _ := child.Attr("role")
			childClass := strings.ToLower(child.AttrOr("class", ""))

			if constants.IsPreserveElement(childTag) ||
				childRole == "article" ||
				semanticClassRe.MatchString(childClass) {
				hasPreservedElements = true
			}
		})

		return hasPreservedElements
	}

	isWrapperElement := func(el *goquery.Selection) bool {
		// If it directly contains inline content, it's NOT a wrapper
		if hasDirectInlineContent(el) {
			return false
		}

		// Check if it's just empty space
		text := strings.TrimSpace(el.Text())
		if text == "" {
			return true
		}

		// Check if it only contains other block elements
		children := el.Children()
		if children.Length() == 0 {
			return true
		}

		// Check if all children are block elements
		allBlockElements := true

		children.Each(func(_ int, child *goquery.Selection) {
			tag := goquery.NodeName(child)
			isBlock := slices.Contains(blockElements, tag)

			// Check additional block elements
			if !isBlock {
				if slices.Contains(additionalBlockElements, tag) {
					isBlock = true
				}
			}

			if !isBlock {
				allBlockElements = false
			}
		})

		if allBlockElements {
			return true
		}

		// Check for common wrapper patterns
		className := strings.ToLower(el.AttrOr("class", ""))
		if wrapperClassRe.MatchString(className) {
			return true
		}

		// Check if it has excessive whitespace or empty text nodes
		hasTextContent := false
		el.Contents().Each(func(_ int, child *goquery.Selection) {
			if goquery.NodeName(child) == "#text" {
				childText := strings.TrimSpace(child.Text())
				if childText != "" {
					hasTextContent = true
				}
			}
		})

		if !hasTextContent {
			return true
		}

		// Check if it only contains block elements (different check)
		hasOnlyBlockElements := children.Length() > 0

		children.Each(func(_ int, child *goquery.Selection) {
			tag := goquery.NodeName(child)
			if constants.IsInlineElement(tag) {
				hasOnlyBlockElements = false
			}
		})

		return hasOnlyBlockElements
	}

	// Function to process a single element
	processElement := func(el *goquery.Selection) bool {
		// Skip processing if element has been removed or should be preserved
		if el.Length() == 0 || shouldPreserveElement(el) {
			return false
		}

		tagName := goquery.NodeName(el)

		// Case 1: Element is truly empty (no text content, no child elements) and not self-closing
		isAllowedEmpty := constants.IsAllowedEmptyElement(tagName)

		if !isAllowedEmpty && el.Children().Length() == 0 && strings.TrimSpace(el.Text()) == "" {
			el.Remove()
			processedCount++
			return true
		}

		// Case 2: Top-level element - be more aggressive
		if el.Parent().Length() > 0 && el.Parent().Get(0) == element.Get(0) {
			children := el.Children()
			hasOnlyBlockElements := children.Length() > 0

			children.Each(func(_ int, child *goquery.Selection) {
				if constants.IsInlineElement(goquery.NodeName(child)) {
					hasOnlyBlockElements = false
				}
			})

			if hasOnlyBlockElements {
				html, _ := el.Html()
				el.ReplaceWithHtml(html)
				processedCount++
				return true
			}
		}

		// Case 3: Wrapper element - merge up aggressively
		if isWrapperElement(el) {
			// Special case: if element only contains block elements, merge them up
			children := el.Children()
			onlyBlockElements := true

			children.Each(func(_ int, child *goquery.Selection) {
				if constants.IsInlineElement(goquery.NodeName(child)) {
					onlyBlockElements = false
				}
			})

			if onlyBlockElements {
				html, _ := el.Html()
				el.ReplaceWithHtml(html)
				processedCount++
				return true
			}

			// Otherwise handle as normal wrapper
			html, _ := el.Html()
			el.ReplaceWithHtml(html)
			processedCount++
			return true
		}

		// Case 4: Element only contains text and/or inline elements - convert to paragraph
		hasOnlyInlineOrText := true
		hasContent := false

		el.Contents().Each(func(_ int, child *goquery.Selection) {
			if goquery.NodeName(child) == "#text" {
				text := strings.TrimSpace(child.Text())
				if text != "" {
					hasContent = true
				}
			} else {
				tag := goquery.NodeName(child)
				if !constants.IsInlineElement(tag) {
					hasOnlyInlineOrText = false
				}
			}
		})

		if hasOnlyInlineOrText && hasContent {
			html, _ := el.Html()
			el.ReplaceWithHtml("<p>" + html + "</p>")
			processedCount++
			return true
		}

		// Case 5: Element has single child - unwrap only if child is block-level
		children := el.Children()
		if children.Length() == 1 {
			child := children.First()
			childTag := goquery.NodeName(child)

			// Only unwrap if the single child is a block element and not preserved
			isBlockChild := slices.Contains(blockElements, childTag)

			if isBlockChild && !shouldPreserveElement(child) {
				// Build opening tag preserving child's attributes
				var attrStr strings.Builder
				for _, a := range child.Nodes[0].Attr {
					attrStr.WriteString(" " + a.Key + `="` + html.EscapeString(a.Val) + `"`)
				}
				childHTML, _ := child.Html()
				el.ReplaceWithHtml("<" + childTag + attrStr.String() + ">" + childHTML + "</" + childTag + ">")
				processedCount++
				return true
			}
		}

		// Case 6: Deeply nested element - merge up
		nestingDepth := 0
		parent := el.Parent()

		for parent.Length() > 0 {
			parentTag := goquery.NodeName(parent)
			if slices.Contains(blockElements, parentTag) {
				nestingDepth++
			}
			parent = parent.Parent()
		}

		// Only unwrap if nested AND does not contain direct inline content
		if nestingDepth > 0 && !hasDirectInlineContent(el) {
			html, _ := el.Html()
			el.ReplaceWithHtml(html)
			processedCount++
			return true
		}

		return false
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
			if onlyParagraphs || (!shouldPreserveElement(el) && isWrapperElement(el)) {
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
