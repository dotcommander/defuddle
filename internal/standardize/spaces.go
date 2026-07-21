package standardize

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
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
	// Process all nodes in the selection
	element.Each(func(_ int, sel *goquery.Selection) {
		if sel.Length() > 0 {
			standardizeSpacesNode(sel.Get(0))
		}
	})
}

// standardizeSpacesNode normalizes &nbsp; runs in node's text recursively,
// preserving a single &nbsp; between word characters and skipping pre/code subtrees.
func standardizeSpacesNode(node *html.Node) {
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
		standardizeSpacesNode(child)
	}
}
