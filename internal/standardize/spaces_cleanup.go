package standardize

import (
	"strings"

	"golang.org/x/net/html"
)

// removeEmptyLines removes empty lines and excessive whitespace
// JavaScript original code:
//
//	function removeEmptyLines(element: Element, doc: Document): void {
//		let removedCount = 0;
//		const startTime = Date.now();
//
//		// First pass: remove empty text nodes
//		const removeEmptyTextNodes = (node: Node) => {
//			// Skip if inside pre or code
//			if (isElement(node)) {
//				const tag = (node as Element).tagName.toLowerCase();
//				if (tag === 'pre' || tag === 'code') {
//					return;
//				}
//			}
//
//			// Process children first (depth-first)
//			const children = Array.from(node.childNodes);
//			children.forEach(removeEmptyTextNodes);
//
//			// Then handle this node
//			if (isTextNode(node)) {
//				const text = node.textContent || '';
//				// If it's completely empty or just special characters/whitespace, remove it
//				if (!text || text.match(/^[\u200C\u200B\u200D\u200E\u200F\uFEFF\xA0\s]*$/)) {
//					node.parentNode?.removeChild(node);
//				} else {
//					// Clean up the text content while preserving important spaces
//					const newText = text
//						.replace(/\n{3,}/g, '\n\n') // More than 2 newlines -> 2 newlines
//						.replace(/^[\n\r\t]+/, '') // Remove leading newlines/tabs (preserve spaces)
//						.replace(/[\n\r\t]+$/, '') // Remove trailing newlines/tabs (preserve spaces)
//						.replace(/[ \t]*\n[ \t]*/g, '\n') // Remove spaces around newlines
//						.replace(/[ \t]{3,}/g, ' ') // 3+ spaces -> 1 space
//						.replace(/^[ ]+$/, ' ') // Multiple spaces between elements -> single space
//						.replace(/\s+([,.!?:;])/g, '$1') // Remove spaces before punctuation
//						// Clean up zero-width characters and multiple non-breaking spaces
//						.replace(/[\u200C\u200B\u200D\u200E\u200F\uFEFF]+/g, '')
//						.replace(/(?:\xA0){2,}/g, '\xA0'); // Multiple &nbsp; -> single &nbsp;
//
//					if (newText !== text) {
//						node.textContent = newText;
//					}
//				}
//			}
//		};
//
//		// Second pass: clean up empty elements and normalize spacing
//		const cleanupEmptyElements = (node: Node) => {
//			if (!isElement(node)) return;
//
//			// Skip pre and code elements
//			const tag = node.tagName.toLowerCase();
//			if (tag === 'pre' || tag === 'code') {
//				return;
//			}
//
//			// Process children first (depth-first)
//			Array.from(node.childNodes)
//				.filter(isElement)
//				.forEach(cleanupEmptyElements);
//
//			// Then normalize this element's whitespace
//			node.normalize(); // Combine adjacent text nodes
//
//			// Special handling for block elements
//			const isBlockElement = getComputedStyle(node)?.display === 'block';
//
//			// Only remove empty text nodes at the start and end if they contain just newlines/tabs
//			// For block elements, also remove spaces
//			const startPattern = isBlockElement ? /^[\n\r\t \u200C\u200B\u200D\u200E\u200F\uFEFF\xA0]*$/ : /^[\n\r\t\u200C\u200B\u200D\u200E\u200F\uFEFF]*$/;
//			const endPattern = isBlockElement ? /^[\n\r\t \u200C\u200B\u200D\u200E\u200F\uFEFF\xA0]*$/ : /^[\n\r\t\u200C\u200B\u200D\u200E\u200F\uFEFF]*$/;
//
//			while (node.firstChild &&
//				   isTextNode(node.firstChild) &&
//				   (node.firstChild.textContent || '').match(startPattern)) {
//				node.removeChild(node.firstChild);
//			}
//
//			while (node.lastChild &&
//				   isTextNode(node.lastChild) &&
//				   (node.lastChild.textContent || '').match(endPattern)) {
//				node.removeChild(node.lastChild);
//			}
//
//			// Ensure there's a space between inline elements if needed
//			if (!isBlockElement) {
//				const children = Array.from(node.childNodes);
//				for (let i = 0; i < children.length - 1; i++) {
//					const current = children[i];
//					const next = children[i + 1];
//
//					// Only add space between elements or between element and text
//					if (isElement(current) || isElement(next)) {
//						// Get the text content
//						const nextContent = next.textContent || '';
//						const currentContent = current.textContent || '';
//
//						// Don't add space if:
//						// 1. Next content starts with punctuation or closing parenthesis
//						// 2. Current content ends with punctuation or opening parenthesis
//						// 3. There's already a space
//						const nextStartsWithPunctuation = nextContent.match(/^[,.!?:;)\]]/);
//						const currentEndsWithPunctuation = currentContent.match(/[,.!?:;(\[]\s*$/);
//
//						const hasSpace = (isTextNode(current) &&
//										(current.textContent || '').endsWith(' ')) ||
//										(isTextNode(next) &&
//										(next.textContent || '').startsWith(' '));
//
//						// Only add space if none of the above conditions are true
//						if (!nextStartsWithPunctuation &&
//							!currentEndsWithPunctuation &&
//							!hasSpace) {
//							const space = doc.createTextNode(' ');
//							node.insertBefore(space, next);
//						}
//					}
//				}
//			}
//		};
//
//		// Run both passes
//		removeEmptyTextNodes(element);
//		cleanupEmptyElements(element);
//
//		const endTime = Date.now();
//		logDebug('Removed empty lines:', {
//			charactersRemoved: removedCount,
//			processingTime: `${(endTime - startTime).toFixed(2)}ms`
//		});
//	}
//
// removeEmptyTextNodes is the first pass: remove empty text nodes and clean up
// text content. removedCount accumulates characters/nodes removed (diagnostic only).
func removeEmptyTextNodes(node *html.Node, removedCount *int) {
	// Skip if inside pre or code
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if tag == "pre" || tag == "code" {
			return
		}
	}

	// Process children first (depth-first)
	var children []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, child)
	}
	for _, child := range children {
		removeEmptyTextNodes(child, removedCount)
	}

	// Then handle this node
	if node.Type == html.TextNode {
		text := node.Data
		// If it's completely empty or just special characters/whitespace, remove it
		if text == "" || emptyTextRe.MatchString(text) {
			if node.Parent != nil {
				node.Parent.RemoveChild(node)
				*removedCount++
			}
		} else {
			// Clean up the text content while preserving important spaces
			newText := text

			// More than 2 newlines -> 2 newlines
			newText = threeNewlinesRe.ReplaceAllString(newText, "\n\n")

			// Remove leading newlines/tabs (preserve spaces)
			newText = leadingNewlinesRe.ReplaceAllString(newText, "")

			// Remove trailing newlines/tabs (preserve spaces)
			newText = trailingNewlinesRe.ReplaceAllString(newText, "")

			// Remove spaces around newlines
			newText = spacesAroundNlRe.ReplaceAllString(newText, "\n")

			// 3+ spaces -> 1 space
			newText = threeSpacesRe.ReplaceAllString(newText, " ")

			// Multiple spaces between elements -> single space
			newText = onlySpacesRe.ReplaceAllString(newText, " ")

			// Remove spaces before punctuation
			newText = spaceBeforePunctRe.ReplaceAllString(newText, "$1")

			// Clean up zero-width characters and multiple non-breaking spaces
			newText = zeroWidthCharsRe.ReplaceAllString(newText, "")
			newText = multiNbspRe.ReplaceAllString(newText, "\xA0")

			if newText != text {
				node.Data = newText
				*removedCount += len(text) - len(newText)
			}
		}
	}
}
