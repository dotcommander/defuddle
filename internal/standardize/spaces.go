package standardize

import (
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
	"golang.org/x/net/html"
)

var (
	nbspRe             = regexp.MustCompile(`\xA0+`)
	emptyTextRe        = regexp.MustCompile(`^[\x{200C}\x{200B}\x{200D}\x{200E}\x{200F}\x{FEFF}\x{A0}\s]*$`)
	threeNewlinesRe    = regexp.MustCompile(`\n{3,}`)
	leadingNewlinesRe  = regexp.MustCompile(`^[\n\r\t]+`)
	trailingNewlinesRe = regexp.MustCompile(`[\n\r\t]+$`)
	spacesAroundNlRe   = regexp.MustCompile(`[ \t]*\n[ \t]*`)
	threeSpacesRe      = regexp.MustCompile(`[ \t]{3,}`)
	onlySpacesRe       = regexp.MustCompile(`^[ ]+$`)
	spaceBeforePunctRe = regexp.MustCompile(`\s+([,.!?:;])`)
	zeroWidthCharsRe   = regexp.MustCompile(`[\x{200C}\x{200B}\x{200D}\x{200E}\x{200F}\x{FEFF}]+`)
	multiNbspRe        = regexp.MustCompile(`(?:\xA0){2,}`)
	blockStartSpaceRe  = regexp.MustCompile(`^[\n\r\t \x{200C}\x{200B}\x{200D}\x{200E}\x{200F}\x{FEFF}\x{A0}]*$`)
	inlineStartSpaceRe = regexp.MustCompile(`^[\n\r\t\x{200C}\x{200B}\x{200D}\x{200E}\x{200F}\x{FEFF}]*$`)
	startsWithPunctRe  = regexp.MustCompile(`^[,.!?:;)\]]`)
	endsWithPunctRe    = regexp.MustCompile(`[,.!?:;(\[]\s*$`)
)

// isWordChar reports whether s (a single-character string) is an ASCII word
// character, matching Go regex \w semantics: [0-9A-Za-z_].
func isWordChar(s string) bool {
	if len(s) == 0 {
		return false
	}
	b := s[0]
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// standardizeSpaces normalizes whitespace in text content
// JavaScript original code:
//
//	function standardizeSpaces(element: Element): void {
//		const processNode = (node: Node) => {
//			// Skip pre and code elements
//			if (isElement(node)) {
//				const tag = (node as Element).tagName.toLowerCase();
//				if (tag === 'pre' || tag === 'code') {
//					return;
//				}
//			}
//
//			// Process text nodes
//			if (isTextNode(node)) {
//				const text = node.textContent || '';
//				// Replace &nbsp; with regular spaces, except when it's a single &nbsp; between words
//				const newText = text.replace(/\xA0+/g, (match) => {
//					// If it's a single &nbsp; between word characters, preserve it
//					if (match.length === 1) {
//						const prev = node.previousSibling?.textContent?.slice(-1);
//						const next = node.nextSibling?.textContent?.charAt(0);
//						if (prev?.match(/\w/) && next?.match(/\w/)) {
//							return '\xA0';
//						}
//					}
//					return ' '.repeat(match.length);
//				});
//
//				if (newText !== text) {
//					node.textContent = newText;
//				}
//			}
//
//			// Process children recursively
//			if (node.hasChildNodes()) {
//				Array.from(node.childNodes).forEach(processNode);
//			}
//		};
//
//		processNode(element);
//	}
func standardizeSpaces(element *goquery.Selection) {
	var processNode func(node *html.Node)
	processNode = func(node *html.Node) {
		// Skip pre and code elements
		if node.Type == html.ElementNode {
			tag := strings.ToLower(node.Data)
			if tag == "pre" || tag == "code" {
				return
			}
		}

		// Process text nodes
		if node.Type == html.TextNode {
			text := node.Data
			// Replace &nbsp; with regular spaces, except when it's a single &nbsp; between words
			newText := nbspRe.ReplaceAllStringFunc(text, func(match string) string {
				// If it's a single &nbsp; between word characters, preserve it
				if len(match) == 1 {
					// Check previous sibling
					var prev string
					if node.PrevSibling != nil && node.PrevSibling.Type == html.TextNode {
						prevText := node.PrevSibling.Data
						if len(prevText) > 0 {
							prev = string(prevText[len(prevText)-1])
						}
					}

					// Check next sibling
					var next string
					if node.NextSibling != nil && node.NextSibling.Type == html.TextNode {
						nextText := node.NextSibling.Data
						if len(nextText) > 0 {
							next = string(nextText[0])
						}
					}

					// If between word characters, preserve the &nbsp;
					if isWordChar(prev) && isWordChar(next) {
						return "\xA0"
					}
				}
				return strings.Repeat(" ", len(match))
			})

			if newText != text {
				node.Data = newText
			}
		}

		// Process children recursively
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			processNode(child)
		}
	}

	// Process all nodes in the selection
	element.Each(func(_ int, sel *goquery.Selection) {
		if sel.Length() > 0 {
			processNode(sel.Get(0))
		}
	})
}

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
func removeEmptyLines(element *goquery.Selection, _ *goquery.Document, debug bool) {
	removedCount := 0
	startTime := time.Now()
	blockElements := constants.GetBlockElements()

	// First pass: remove empty text nodes and clean up text content
	var removeEmptyTextNodes func(node *html.Node)
	removeEmptyTextNodes = func(node *html.Node) {
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
			removeEmptyTextNodes(child)
		}

		// Then handle this node
		if node.Type == html.TextNode {
			text := node.Data
			// If it's completely empty or just special characters/whitespace, remove it
			if text == "" || emptyTextRe.MatchString(text) {
				if node.Parent != nil {
					node.Parent.RemoveChild(node)
					removedCount++
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
					removedCount += len(text) - len(newText)
				}
			}
		}
	}

	// Second pass: clean up empty elements and normalize spacing
	var cleanupEmptyElements func(node *html.Node)
	cleanupEmptyElements = func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}

		// Skip pre and code elements
		tag := strings.ToLower(node.Data)
		if tag == "pre" || tag == "code" {
			return
		}

		// Process children first (depth-first)
		var children []*html.Node
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode {
				children = append(children, child)
			}
		}
		for _, child := range children {
			cleanupEmptyElements(child)
		}

		// Determine if this is a block element (simplified check)
		isBlockElement := slices.Contains(blockElements, tag)

		// Additional block elements
		if !isBlockElement {
			if slices.Contains(additionalBlockElements, tag) {
				isBlockElement = true
			}
		}

		// Only remove empty text nodes at the start and end if they contain just newlines/tabs
		// For block elements, also remove spaces
		var startPattern, endPattern *regexp.Regexp
		if isBlockElement {
			startPattern = blockStartSpaceRe
			endPattern = blockStartSpaceRe
		} else {
			startPattern = inlineStartSpaceRe
			endPattern = inlineStartSpaceRe
		}

		// Remove empty text nodes at start
		for node.FirstChild != nil &&
			node.FirstChild.Type == html.TextNode &&
			startPattern.MatchString(node.FirstChild.Data) {
			node.RemoveChild(node.FirstChild)
			removedCount++
		}

		// Remove empty text nodes at end
		for node.LastChild != nil &&
			node.LastChild.Type == html.TextNode &&
			endPattern.MatchString(node.LastChild.Data) {
			node.RemoveChild(node.LastChild)
			removedCount++
		}

		// Ensure there's a space between inline elements if needed
		if !isBlockElement {
			var nodeChildren []*html.Node
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				nodeChildren = append(nodeChildren, child)
			}

			for i := range len(nodeChildren) - 1 {
				current := nodeChildren[i]
				next := nodeChildren[i+1]

				// Only add space between elements or between element and text
				if current.Type == html.ElementNode || next.Type == html.ElementNode {
					// Get the text content (simplified)
					var nextContent, currentContent string
					if next.Type == html.TextNode {
						nextContent = next.Data
					}
					if current.Type == html.TextNode {
						currentContent = current.Data
					}

					// Don't add space if:
					// 1. Next content starts with punctuation or closing parenthesis
					// 2. Current content ends with punctuation or opening parenthesis
					// 3. There's already a space
					nextStartsWithPunctuation := startsWithPunctRe.MatchString(nextContent)
					currentEndsWithPunctuation := endsWithPunctRe.MatchString(currentContent)

					hasSpace := (current.Type == html.TextNode && strings.HasSuffix(current.Data, " ")) ||
						(next.Type == html.TextNode && strings.HasPrefix(next.Data, " "))

					// Only add space if none of the above conditions are true
					if !nextStartsWithPunctuation &&
						!currentEndsWithPunctuation &&
						!hasSpace {
						space := &html.Node{
							Type: html.TextNode,
							Data: " ",
						}
						node.InsertBefore(space, next)
					}
				}
			}
		}
	}

	// Run both passes
	element.Each(func(_ int, sel *goquery.Selection) {
		if sel.Length() > 0 {
			removeEmptyTextNodes(sel.Get(0))
		}
	})

	element.Each(func(_ int, sel *goquery.Selection) {
		if sel.Length() > 0 {
			cleanupEmptyElements(sel.Get(0))
		}
	})

	endTime := time.Now()
	processingTime := float64(endTime.Sub(startTime).Nanoseconds()) / 1e6 // Convert to milliseconds
	if debug {
		slog.Debug("Removed empty lines",
			"charactersRemoved", removedCount,
			"processingTime", processingTime)
	}
}
