package standardize

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// JavaScript original code:
//
//	function removeTrailingHeadings(element: Element): void {
//		const hasContentAfter = (el: Element): boolean => {
//			let sibling = el.nextElementSibling;
//			while (sibling) {
//				const text = sibling.textContent?.trim() || '';
//				if (text.length > 0) {
//					return true;
//				}
//				sibling = sibling.nextElementSibling;
//			}
//			return false;
//		};
//
//		const headings = element.querySelectorAll('h1, h2, h3, h4, h5, h6');
//		headings.forEach(heading => {
//			if (!hasContentAfter(heading)) {
//				heading.remove();
//			}
//		});
//	}
//
// removeTrailingHeadings removes headings at the end of content
func removeTrailingHeadings(element *goquery.Selection) {
	// Process headings in reverse order (deepest/last first) and break
	// after finding the first heading with content after it.
	headings := element.Find("h1, h2, h3, h4, h5, h6")
	nodes := make([]*goquery.Selection, 0, headings.Length())
	headings.Each(func(_ int, h *goquery.Selection) {
		nodes = append(nodes, h)
	})
	for i := len(nodes) - 1; i >= 0; i-- {
		if hasContentAfter(nodes[i], element) {
			break
		}
		nodes[i].Remove()
	}
}

// hasContentAfter reports whether el has any non-empty content in a following
// sibling (element or text node), climbing to parents up to (but not including)
// boundary and checking their following siblings too.
func hasContentAfter(el, boundary *goquery.Selection) bool {
	// Check all following sibling nodes (elements AND text nodes)
	if len(el.Nodes) > 0 {
		for sib := el.Nodes[0].NextSibling; sib != nil; sib = sib.NextSibling {
			switch sib.Type {
			case html.ElementNode:
				sibDoc := goquery.NewDocumentFromNode(sib)
				if strings.TrimSpace(sibDoc.Text()) != "" {
					return true
				}
			case html.TextNode:
				if strings.TrimSpace(sib.Data) != "" {
					return true
				}
			default:
				// Ignore comment, doctype, and error nodes
			}
		}
	}
	// Climb to parent and check its following siblings
	parent := el.Parent()
	if parent.Length() > 0 && parent.Nodes[0] != boundary.Nodes[0] {
		return hasContentAfter(parent, boundary)
	}
	return false
}
