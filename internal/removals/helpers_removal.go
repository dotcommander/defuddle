package removals

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

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
