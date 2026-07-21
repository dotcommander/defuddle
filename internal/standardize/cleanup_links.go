package standardize

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// unwrapSpecialLinks fixes problematic link structures:
// 1. Removes <a> inside <code> (markdown can't render links in backtick code)
// 2. Unwraps javascript: links (keep text, remove the link)
// 3. Restructures block-wrapping links containing headings into heading-wrapping links
// 4. Unwraps anchor links that wrap headings (clickable section headers)
// restructureHeadingLink moves a block-wrapping <a>'s href onto an inner <a>
// around the heading's content, then unwraps the outer link. No-op when the
// link has no usable href or no heading child.
func restructureHeadingLink(link *goquery.Selection) {
	href, exists := link.Attr("href")
	if !exists || href == "" || strings.HasPrefix(href, "#") {
		return
	}
	// Find a heading child
	var headingNode *html.Node
	link.Children().Each(func(_ int, child *goquery.Selection) {
		tag := strings.ToUpper(goquery.NodeName(child))
		if len(tag) == 2 && tag[0] == 'H' && tag[1] >= '1' && tag[1] <= '6' {
			headingNode = child.Get(0)
		}
	})
	if headingNode == nil {
		return
	}

	// Create inner <a> with the href, move heading's children into it
	innerLink := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{{Key: "href", Val: href}},
	}
	for headingNode.FirstChild != nil {
		child := headingNode.FirstChild
		headingNode.RemoveChild(child)
		innerLink.AppendChild(child)
	}
	headingNode.AppendChild(innerLink)

	// Unwrap the outer <a>
	unwrapSelection(link)
}

func unwrapSpecialLinks(element *goquery.Selection, _ *goquery.Document) {
	// 1. Unwrap links inside inline code
	element.Find("code a").Each(func(_ int, a *goquery.Selection) {
		unwrapSelection(a)
	})

	// 2. Unwrap javascript: links
	element.Find(`a[href^="javascript:"]`).Each(func(_ int, a *goquery.Selection) {
		unwrapSelection(a)
	})

	// 3. Restructure block-wrapping links containing headings
	element.Find("a").Each(func(_ int, link *goquery.Selection) {
		restructureHeadingLink(link)
	})

	// 4. Unwrap anchor links wrapping headings
	element.Find(`a[href^="#"]`).Each(func(_ int, link *goquery.Selection) {
		if link.Find("h1, h2, h3, h4, h5, h6").Length() > 0 {
			unwrapSelection(link)
		}
	})
}

// unwrapSelection replaces a selection with its children (equivalent to TS unwrapElement).
func unwrapSelection(sel *goquery.Selection) {
	if sel.Length() == 0 {
		return
	}
	node := sel.Get(0)
	parent := node.Parent
	if parent == nil {
		return
	}
	for node.FirstChild != nil {
		child := node.FirstChild
		node.RemoveChild(child)
		parent.InsertBefore(child, node)
	}
	parent.RemoveChild(node)
}

// removeHeadingAnchors removes permalink anchors from inside heading elements.
// Handles symbols (#, ¶, §, 🔗), empty links, and class-based anchors.
func removeHeadingAnchors(element *goquery.Selection) {
	element.Find("h1 a, h2 a, h3 a, h4 a, h5 a, h6 a").Each(func(_ int, link *goquery.Selection) {
		if isPermalinkAnchor(link) {
			link.Remove()
		}
	})
}

func isPermalinkAnchor(link *goquery.Selection) bool {
	if goquery.NodeName(link) != "a" {
		return false
	}
	href := link.AttrOr("href", "")
	title := strings.ToLower(link.AttrOr("title", ""))
	className := strings.ToLower(link.AttrOr("class", ""))
	text := strings.TrimSpace(link.Text())

	if strings.HasPrefix(href, "#") || strings.Contains(href, "#") {
		return true
	}
	if strings.Contains(title, "permalink") {
		return true
	}
	if strings.Contains(className, "permalink") || strings.Contains(className, "heading-anchor") || strings.Contains(className, "anchor-link") {
		return true
	}
	if permalinkSymbolRe.MatchString(text) {
		return true
	}
	return false
}

// removeObsoleteElements removes <object>, <embed>, and <applet> elements.
func removeObsoleteElements(element *goquery.Selection) {
	element.Find("object, embed, applet").Remove()
}

// removeOrphanedDividers removes leading and trailing <hr> elements,
// skipping whitespace-only text nodes.
func removeOrphanedDividers(element *goquery.Selection) {
	if element.Length() == 0 {
		return
	}
	node := element.Get(0)

	removeEdgeHRs(node, func(p *html.Node) *html.Node { return p.FirstChild }, func(n *html.Node) *html.Node { return n.NextSibling })
	removeEdgeHRs(node, func(p *html.Node) *html.Node { return p.LastChild }, func(n *html.Node) *html.Node { return n.PrevSibling })
}

// removeEdgeHRs strips <hr> elements from one end of node, skipping
// whitespace-only text nodes between them. edge returns the boundary child
// (FirstChild for leading, LastChild for trailing) and step advances inward
// (NextSibling or PrevSibling).
func removeEdgeHRs(node *html.Node, edge, step func(*html.Node) *html.Node) {
	for {
		n := edge(node)
		for n != nil && n.Type == html.TextNode && strings.TrimSpace(n.Data) == "" {
			n = step(n)
		}
		if n != nil && n.Type == html.ElementNode && strings.EqualFold(n.Data, "hr") {
			node.RemoveChild(n)
		} else {
			break
		}
	}
}

// wrapPreformattedCode wraps <code> elements with white-space:pre style
// in <pre> elements if they aren't already inside one.
func wrapPreformattedCode(element *goquery.Selection) {
	element.Find("code").Each(func(_ int, code *goquery.Selection) {
		// Skip if already inside a <pre>
		if code.Closest("pre").Length() > 0 {
			return
		}
		style := code.AttrOr("style", "")
		if !whiteSpacePreRe.MatchString(style) {
			return
		}
		// Wrap in <pre>
		codeNode := code.Get(0)
		parent := codeNode.Parent
		if parent == nil {
			return
		}
		pre := &html.Node{
			Type: html.ElementNode,
			Data: "pre",
		}
		parent.InsertBefore(pre, codeNode)
		parent.RemoveChild(codeNode)
		pre.AppendChild(codeNode)
	})
}
