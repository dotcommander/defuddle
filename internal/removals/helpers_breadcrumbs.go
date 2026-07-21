package removals

import (
	"strings"

	"golang.org/x/net/html"
)

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

	return linksLookLikeBreadcrumb(links)
}

// linksLookLikeBreadcrumb reports whether links are all internal, include at
// least one breadcrumb-style link, and all have short (at most 5-word) text.
func linksLookLikeBreadcrumb(links []*html.Node) bool {
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
