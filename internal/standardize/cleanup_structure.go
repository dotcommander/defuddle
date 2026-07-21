package standardize

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// JavaScript original code:
//
//	function stripExtraBrElements(element: Element): void {
//		// Remove more than 2 consecutive br elements
//		const processBrs = () => {
//			const brs = Array.from(element.querySelectorAll('br'));
//			let consecutiveCount = 0;
//			let toRemove: Element[] = [];
//
//			brs.forEach((br, index) => {
//				const nextSibling = br.nextElementSibling;
//				if (nextSibling && nextSibling.tagName.toLowerCase() === 'br') {
//					consecutiveCount++;
//					if (consecutiveCount >= 2) {
//						toRemove.push(br);
//					}
//				} else {
//					consecutiveCount = 0;
//				}
//			});
//
//			toRemove.forEach(br => br.remove());
//		};
//
//		processBrs();
//	}
//
// stripExtraBrElements removes excessive br elements
func stripExtraBrElements(element *goquery.Selection) {
	// Collect all <br> nodes
	var brNodes []*html.Node
	element.Find("br").Each(func(_ int, br *goquery.Selection) {
		brNodes = append(brNodes, br.Get(0))
	})

	// Group consecutive <br>s (only whitespace text nodes allowed between them, matching TS)
	var consecutiveBrs []*html.Node

	processBrs := func() {
		if len(consecutiveBrs) > 2 {
			// Keep only the first two, remove the rest
			for i := 2; i < len(consecutiveBrs); i++ {
				if consecutiveBrs[i].Parent != nil {
					consecutiveBrs[i].Parent.RemoveChild(consecutiveBrs[i])
				}
			}
		}
		consecutiveBrs = nil
	}

	for _, br := range brNodes {
		isConsecutive := len(consecutiveBrs) > 0 && isConsecutiveBr(br, consecutiveBrs[len(consecutiveBrs)-1])

		if isConsecutive {
			consecutiveBrs = append(consecutiveBrs, br)
		} else {
			processBrs()
			consecutiveBrs = []*html.Node{br}
		}
	}
	processBrs()
}

// isConsecutiveBr reports whether br immediately follows lastBr with only
// whitespace-only text nodes between them.
func isConsecutiveBr(br, lastBr *html.Node) bool {
	// Walk backwards from br, skipping whitespace-only text nodes
	node := br.PrevSibling
	for node != nil && node.Type == html.TextNode && strings.TrimSpace(node.Data) == "" {
		node = node.PrevSibling
	}
	return node == lastBr
}

// hasCalloutClass checks if a class attribute value contains a callout class (callout or callout-*).
func hasCalloutClass(classValue string) bool {
	for _, c := range strings.Fields(classValue) {
		if c == "callout" || strings.HasPrefix(c, "callout-") {
			return true
		}
	}
	return false
}

// removeHTMLComments removes all HTML comment nodes from the element tree.
func removeHTMLComments(element *goquery.Selection) {
	if element.Length() == 0 {
		return
	}
	removeCommentsFromNode(element.Get(0))
}

func removeCommentsFromNode(n *html.Node) {
	var toRemove []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.CommentNode {
			toRemove = append(toRemove, c)
		} else {
			removeCommentsFromNode(c)
		}
	}
	for _, c := range toRemove {
		n.RemoveChild(c)
	}
}

// unwrapBareSpans removes attribute-free <span> elements, keeping their children.
// Processes deepest-first so nested bare spans collapse in one pass.
func unwrapBareSpans(element *goquery.Selection) {
	spans := element.Find("span")
	if spans.Length() == 0 {
		return
	}

	// Collect in reverse order (deepest first)
	collected := make([]*html.Node, 0, spans.Length())
	spans.Each(func(_ int, s *goquery.Selection) {
		collected = append(collected, s.Get(0))
	})
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}

	for _, node := range collected {
		if node.Parent == nil {
			continue
		}
		// Skip spans with attributes
		if len(node.Attr) > 0 {
			continue
		}
		// Move children before the span, then remove the span
		parent := node.Parent
		for node.FirstChild != nil {
			child := node.FirstChild
			node.RemoveChild(child)
			parent.InsertBefore(child, node)
		}
		parent.RemoveChild(node)
	}
}
