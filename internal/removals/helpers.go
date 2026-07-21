// Package removals provides content-pattern-based removal for the defuddle extraction pipeline.
package removals

import (
	"strings"

	"golang.org/x/net/html"
)

// nextElementSibling returns the next sibling that is an element node.
func nextElementSibling(n *html.Node) *html.Node {
	for s := n.NextSibling; s != nil; s = s.NextSibling {
		if s.Type == html.ElementNode {
			return s
		}
	}
	return nil
}

// prevElementSibling returns the previous sibling that is an element node.
func prevElementSibling(n *html.Node) *html.Node {
	for s := n.PrevSibling; s != nil; s = s.PrevSibling {
		if s.Type == html.ElementNode {
			return s
		}
	}
	return nil
}

// lastElementChild returns the last child that is an element node.
func lastElementChild(n *html.Node) *html.Node {
	var last *html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			last = c
		}
	}
	return last
}

// elementChildren returns all element-node children of n.
func elementChildren(n *html.Node) []*html.Node {
	var children []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			children = append(children, c)
		}
	}
	return children
}

// nodeText recursively collects text content from a node tree, mirroring
// Element.textContent from the DOM.
func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(nodeText(c))
	}
	return b.String()
}

// removeNode detaches n from its parent.
func removeNode(n *html.Node) {
	if n.Parent != nil {
		n.Parent.RemoveChild(n)
	}
}

// nodeAttr returns the value of attribute key on node n, or "" if absent.
func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// walkUpToWrapper walks from node toward mainNode, ascending while each
// parent's trimmed text equals text. Returns the highest ancestor that still
// matches, so callers can remove the right container level.
func walkUpToWrapper(node, mainNode *html.Node, text string) *html.Node {
	target := node
	for target.Parent != nil && target.Parent != mainNode {
		parentText := strings.TrimSpace(nodeText(target.Parent))
		if parentText != text {
			break
		}
		target = target.Parent
	}
	return target
}

// walkUpIsolated walks from node toward mainNode, ascending while all preceding
// siblings at each level have ≤10 words combined. Returns the highest ancestor
// where the node is effectively isolated (no preceding content siblings).
func walkUpIsolated(node, mainNode *html.Node) *html.Node {
	target := node
	for target.Parent != nil && target.Parent != mainNode {
		precedingWords := 0
		for sib := prevElementSibling(target); sib != nil; sib = prevElementSibling(sib) {
			precedingWords += countWords(strings.TrimSpace(nodeText(sib)))
			if precedingWords > 10 {
				break
			}
		}
		if precedingWords > 10 {
			break
		}
		target = target.Parent
	}
	return target
}
