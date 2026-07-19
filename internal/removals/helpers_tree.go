package removals

import (
	"strings"

	"golang.org/x/net/html"
)

// nodePrecedes returns true when a appears before b in document order.
func nodePrecedes(a, b *html.Node) bool {
	aChain := make(map[*html.Node]int)
	for n, depth := a, 0; n != nil; n, depth = n.Parent, depth+1 {
		aChain[n] = depth
	}
	for n := b; n != nil; n = n.Parent {
		if _, ok := aChain[n]; ok {
			aChild := ancestorChildAt(a, n)
			bChild := ancestorChildAt(b, n)
			if aChild == nil || bChild == nil || aChild == bChild {
				return false
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c == aChild {
					return true
				}
				if c == bChild {
					return false
				}
			}
			return false
		}
	}
	return false
}

// ancestorChildAt returns the child of parent that is an ancestor of (or equal to) node.
func ancestorChildAt(node, parent *html.Node) *html.Node {
	for n := node; n != nil; n = n.Parent {
		if n.Parent == parent {
			return n
		}
	}
	return nil
}

// nodeContains returns true if ancestor contains descendant.
func nodeContains(ancestor, descendant *html.Node) bool {
	for n := descendant; n != nil; n = n.Parent {
		if n == ancestor {
			return true
		}
	}
	return false
}

// previewNode returns the first 80 characters of a node's text for debug logging.
func previewNode(n *html.Node) string {
	text := strings.TrimSpace(nodeText(n))
	if len(text) > 80 {
		return text[:80] + "…"
	}
	return text
}
