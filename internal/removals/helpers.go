// Package removals provides content-pattern-based removal for the defuddle extraction pipeline.
package removals

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
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

// removeTrailingSiblings removes all element siblings after node, then
// optionally removes node itself (removeSelf=true).
func removeTrailingSiblings(node *html.Node, removeSelf, debug bool) {
	sib := nextElementSibling(node)
	for sib != nil {
		next := nextElementSibling(sib)
		if debug {
			_ = strings.TrimSpace(nodeText(sib)) // retain call for side-effect-free debug hook
		}
		removeNode(sib)
		sib = next
	}
	if removeSelf {
		removeNode(node)
	}
}

// removeThinPrecedingSection removes the element immediately before target if it
// has fewer than 50 words and contains no content elements.
func removeThinPrecedingSection(target *html.Node) {
	prev := prevElementSibling(target)
	if prev == nil {
		return
	}
	if countWords(strings.TrimSpace(nodeText(prev))) >= 50 {
		return
	}
	if hasContentElements(prev) {
		return
	}
	removeNode(prev)
}

// hasContentElements returns true if node contains any of the "rich content"
// indicators: math, code, table, img, picture, video, blockquote, figure.
// Matches the TypeScript CONTENT_ELEMENT_SELECTOR list.
func hasContentElements(n *html.Node) bool {
	richTags := map[string]bool{
		"math": true, "pre": true, "code": true,
		"table": true, "img": true, "picture": true,
		"video": true, "blockquote": true, "figure": true,
	}
	return treeContainsTag(n, richTags)
}

func treeContainsTag(n *html.Node, tags map[string]bool) bool {
	if n == nil {
		return false
	}
	if n.Type == html.ElementNode && tags[strings.ToLower(n.Data)] {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if treeContainsTag(c, tags) {
			return true
		}
	}
	return false
}

// countWords counts whitespace-separated words in s (simple Latin approximation).
// Delegates to the shared text package via the package-level shim below.
func countWords(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Fields(s))
}

// hasContentElementsGoquery checks a goquery selection for content elements.
func hasContentElementsGoquery(sel *goquery.Selection) bool {
	return sel.Find("math, [data-mathml], .katex, .MathJax, mjx-container, pre, code, table, img, picture, video, blockquote, figure").Length() > 0
}

// isNewsletterElement returns true if sel is a short element whose text
// matches the newsletter/subscribe pattern and contains no content elements.
func isNewsletterElement(sel *goquery.Selection, maxWords int) bool {
	text := strings.TrimSpace(sel.Text())
	words := countWords(text)
	if words < 2 || words > maxWords {
		return false
	}
	if hasContentElementsGoquery(sel) {
		return false
	}
	// Normalize camelCase boundaries before matching.
	normalized := camelBoundary.ReplaceAllString(text, "$1 $2")
	return newsletterPattern.MatchString(normalized)
}

// allUppercaseFirst returns true if every word starts with an uppercase letter.
func allUppercaseFirst(words []string) bool {
	for _, w := range words {
		if w == "" {
			continue
		}
		if !bylineUppercasePattern.MatchString(w) {
			return false
		}
	}
	return true
}

// isBreadcrumbList returns true when list is a navigation breadcrumb.
func isBreadcrumbList(list *html.Node) bool {
	items := elementChildren(list)
	if len(items) < 2 || len(items) > 8 {
		return false
	}

	var links []*html.Node
	collectByTag(list, "a", &links)
	if len(links) < 1 || len(links) >= len(items) {
		return false
	}

	noContent := map[string]bool{"img": true, "p": true, "figure": true, "blockquote": true}
	if treeContainsTag(list, noContent) {
		return false
	}

	allInternal := true
	hasBreadcrumbLink := false
	shortLinkTexts := true
	for _, a := range links {
		href := nodeAttr(a, "href")
		if strings.HasPrefix(href, "http") || strings.HasPrefix(href, "//") {
			allInternal = false
			break
		}
		if href == "/" || breadcrumbLinkPattern.MatchString(href) {
			hasBreadcrumbLink = true
		}
		linkText := strings.TrimSpace(nodeText(a))
		if len(strings.Fields(linkText)) > 5 {
			shortLinkTexts = false
		}
	}
	return allInternal && hasBreadcrumbLink && shortLinkTexts
}

// countMetadataWords counts words in h1, h2, h3, time children of n,
// deduplicating nested elements so a heading inside a time wrapper isn't double-counted.
func countMetadataWords(n *html.Node) int {
	metaTags := []string{"h1", "h2", "h3", "time"}
	var metaNodes []*html.Node
	for _, tag := range metaTags {
		collectByTag(n, tag, &metaNodes)
	}
	// Deduplicate: skip nodes dominated by an already-counted ancestor.
	counted := 0
outer:
	for _, m := range metaNodes {
		for _, existing := range metaNodes[:counted] {
			if nodeContains(existing, m) {
				continue outer
			}
		}
		metaNodes[counted] = m
		counted++
	}
	total := 0
	for _, m := range metaNodes[:counted] {
		total += countWords(nodeText(m))
	}
	return total
}

// collectByTag collects all descendant element nodes with the given tag into out.
func collectByTag(n *html.Node, tag string, out *[]*html.Node) {
	if n == nil {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.ToLower(c.Data) == tag {
			*out = append(*out, c)
		}
		collectByTag(c, tag, out)
	}
}

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
