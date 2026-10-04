package defuddle

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// removeHiddenElements removes elements that are hidden via CSS
// JavaScript original code:
//
//	private removeHiddenElements(doc: Document) {
//	  // ... (checks computed styles for display:none, visibility:hidden, opacity:0)
//	}
func (d *Defuddle) removeHiddenElements(doc *goquery.Document) {
	count := 0
	var toRemove []*goquery.Selection

	doc.Find("*").Each(func(_ int, element *goquery.Selection) {
		if isHiddenElement(element) {
			toRemove = append(toRemove, element)
			count++
		}
	})

	for _, el := range toRemove {
		el.Remove()
	}

	if d.debug {
		slog.Debug("Removed hidden elements", "count", count)
	}
}

// isHiddenElement reports whether element should be removed as hidden: an inline
// display:none / visibility:hidden / opacity:0 style, or a hidden/invisible utility
// class (including responsive prefix:hidden variants). Math elements (math tag,
// data-mathml, katex-mathml/MathJax classes) are never treated as hidden.
func isHiddenElement(element *goquery.Selection) bool {
	// Hidden wrappers must survive until their contained math is processed.
	if containsRecognizedMath(element) {
		return false
	}
	className := element.AttrOr("class", "")

	// Check inline styles for hidden elements
	if style, exists := element.Attr("style"); exists {
		lowerStyle := strings.ToLower(style)
		if strings.Contains(lowerStyle, "display:none") ||
			strings.Contains(lowerStyle, "display: none") ||
			strings.Contains(lowerStyle, "visibility:hidden") ||
			strings.Contains(lowerStyle, "visibility: hidden") ||
			strings.Contains(lowerStyle, "opacity:0") ||
			strings.Contains(lowerStyle, "opacity: 0") {
			return true
		}
	}

	// Check class tokens for Tailwind/utility hidden classes
	if className != "" {
		for _, token := range strings.Fields(className) {
			// Exact matches: "hidden", "invisible"
			if token == "hidden" || token == "invisible" {
				return true
			}
			// Responsive variants: "sm:hidden", "md:hidden", "lg:hidden", etc.
			// Also matches arbitrary prefix:hidden and prefix:invisible
			if strings.HasSuffix(token, ":hidden") || strings.HasSuffix(token, ":invisible") {
				return true
			}
		}
	}

	return false
}

// resolveReactStreaming resolves React SSR streaming placeholders.
// React's streaming SSR emits <template id="B:X"> as Suspense boundaries,
// then later provides content in <div hidden id="S:X"> with a $RC("B:X","S:X") call.
// This function swaps the templates with their resolved content.
var reactRCPattern = regexp.MustCompile(`\$RC\(\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']\s*\)`)

func resolveReactStreaming(doc *goquery.Document) {
	// Find $RC calls in inline scripts
	doc.Find("script").Each(func(_ int, script *goquery.Selection) {
		text := script.Text()
		matches := reactRCPattern.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			boundaryID, slotID := m[1], m[2]

			boundary := elementByID(doc, boundaryID)
			slot := elementByID(doc, slotID)
			if boundary == nil || slot == nil || boundary.Data != "template" || boundary.Parent == nil || slot.Parent == nil {
				continue
			}
			opening := boundary.PrevSibling
			if opening == nil || opening.Type != html.CommentNode || !isSuspenseOpening(opening.Data) {
				continue
			}
			depth := 0
			var closing *html.Node
			for node := boundary.NextSibling; node != nil; node = node.NextSibling {
				if node.Type != html.CommentNode {
					continue
				}
				if isSuspenseOpening(node.Data) {
					depth++
				}
				if node.Data == "/$" {
					if depth == 0 {
						closing = node
						break
					}
					depth--
				}
			}
			if closing == nil {
				continue
			}
			parent := boundary.Parent
			// A malformed slot inside its own fallback cannot be moved safely.
			insideFallback := false
			for node := boundary; node != closing; node = node.NextSibling {
				for ancestor := slot; ancestor != nil; ancestor = ancestor.Parent {
					if ancestor == node {
						insideFallback = true
					}
				}
			}
			if insideFallback {
				continue
			}
			for node := boundary; node != closing; {
				next := node.NextSibling
				parent.RemoveChild(node)
				node = next
			}
			for slot.FirstChild != nil {
				child := slot.FirstChild
				slot.RemoveChild(child)
				parent.InsertBefore(child, closing)
			}
			opening.Data = "$"
			slot.Parent.RemoveChild(slot)
		}
	})
}

// flattenShadowDOM inlines declarative Shadow DOM templates into the main document.
// Browsers use <template shadowrootmode="open"> for SSR shadow DOM; the content
// inside is invisible to goquery without flattening.
func flattenShadowDOM(doc *goquery.Document) {
	doc.Find(`template[shadowrootmode], template[shadowroot]`).Each(func(_ int, tmpl *goquery.Selection) {
		parent := tmpl.Parent()
		if parent.Length() == 0 {
			return
		}
		// Move the nodes themselves so nested templates remain in the original
		// traversal and their children are flattened as well.
		tmpl.ReplaceWithSelection(tmpl.Contents())
	})
}

// containsRecognizedMath identifies a math node or an ancestor containing one.
func containsRecognizedMath(element *goquery.Selection) bool {
	return isRecognizedMath(element) || element.Find("*").FilterFunction(func(_ int, child *goquery.Selection) bool {
		return isRecognizedMath(child)
	}).Length() > 0
}

func isRecognizedMath(element *goquery.Selection) bool {
	if goquery.NodeName(element) == "math" {
		return true
	}
	if _, ok := element.Attr("data-mathml"); ok {
		return true
	}
	className := element.AttrOr("class", "")
	return strings.Contains(className, "katex-mathml") || strings.Contains(className, "MathJax")
}

func elementByID(doc *goquery.Document, id string) *html.Node {
	var found *html.Node
	doc.Find("[id]").EachWithBreak(func(_ int, element *goquery.Selection) bool {
		if element.AttrOr("id", "") == id {
			found = element.Get(0)
			return false
		}
		return true
	})
	return found
}

func isSuspenseOpening(marker string) bool {
	return marker == "$" || marker == "$?" || marker == "$!"
}
