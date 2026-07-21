package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

func renderLink(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "a" {
		return converter.RenderTryNext
	}
	href := getAttr(n, "href")

	// Remove footnote backlinks
	if strings.Contains(href, "#fnref") {
		return converter.RenderSuccess
	}
	class := getAttr(n, "class")
	if strings.Contains(class, "footnote-backref") {
		return converter.RenderSuccess
	}

	// Complex link structure: <a> wrapping a heading + other content.
	// Restructure as: heading → remaining content → [View original](url)
	if hasChildHeading(n) {
		return renderComplexLink(ctx, w, n)
	}

	if href == "" {
		return converter.RenderTryNext
	}

	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := strings.TrimSpace(buf.String())
	if content == "" {
		return converter.RenderTryNext
	}

	w.WriteString("[")
	w.WriteString(content)
	w.WriteString("](")
	w.WriteString(formatMarkdownLinkDestination(href))
	w.WriteString(formatMarkdownLinkTitle(getAttr(n, "title")))
	w.WriteString(")")
	return converter.RenderSuccess
}

func formatMarkdownLinkDestination(href string) string {
	if !linkWhitespaceRe.MatchString(href) {
		return strings.NewReplacer("(", `\(`, ")", `\)`).Replace(href)
	}
	return "<" + strings.ReplaceAll(href, ">", `\>`) + ">"
}

func formatMarkdownLinkTitle(title string) string {
	if title == "" {
		return ""
	}
	title = linkTitleNewlineRe.ReplaceAllString(title, "\n")
	title = strings.ReplaceAll(title, `"`, `\"`)
	return ` "` + title + `"`
}

func hasChildHeading(n *html.Node) bool {
	childCount := 0
	hasHeading := false
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			childCount++
			switch c.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				hasHeading = true
			}
		}
	}
	return hasHeading && childCount > 1
}

func renderComplexLink(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	href := getAttr(n, "href")

	// Find and render the heading
	var headingNode *html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			switch c.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				headingNode = c
			}
		}
		if headingNode != nil {
			break
		}
	}

	var headingBuf bytes.Buffer
	if headingNode != nil {
		ctx.RenderChildNodes(ctx, &headingBuf, headingNode)
	}

	// Remove heading from parent temporarily to render remaining content
	if headingNode != nil {
		n.RemoveChild(headingNode)
	}
	var remainBuf bytes.Buffer
	ctx.RenderChildNodes(ctx, &remainBuf, n)

	w.WriteString(strings.TrimSpace(headingBuf.String()))
	remaining := strings.TrimSpace(remainBuf.String())
	if remaining != "" {
		w.WriteString("\n\n")
		w.WriteString(remaining)
	}
	if href != "" {
		w.WriteString("\n\n")
		w.WriteString("[View original](" + formatMarkdownLinkDestination(href) + formatMarkdownLinkTitle(getAttr(n, "title")) + ")")
	}
	return converter.RenderSuccess
}

// renderListItem handles task-list checkboxes and OL start attribute.
// Only intercepts special cases; returns RenderTryNext for normal list items.
func renderListItem(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "li" {
		return converter.RenderTryNext
	}

	// Check for task-list checkbox
	isTaskItem := hasExactClass(getAttr(n, "class"), "task-list-item")
	var checkboxMarker string
	if isTaskItem {
		// Find and remove input[type="checkbox"]
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "input" && getAttr(c, "type") == "checkbox" {
				if getAttr(c, "checked") != "" {
					checkboxMarker = "[x] "
				} else {
					checkboxMarker = "[ ] "
				}
				n.RemoveChild(c)
				break
			}
		}
	}

	// Check OL start attribute
	hasCustomStart := false
	customNumber := 0
	if n.Parent != nil && n.Parent.Type == html.ElementNode && n.Parent.Data == "ol" {
		if start := getAttr(n.Parent, "start"); start != "" {
			startNum := 0
			if _, err := fmt.Sscanf(start, "%d", &startNum); err == nil {
				// Find this li's index among siblings
				customNumber = startNum + liIndexInParent(n) - 1
				hasCustomStart = true
			}
		}
	}

	// If neither special case applies, let default handler take over
	if checkboxMarker == "" && !hasCustomStart {
		return converter.RenderTryNext
	}

	// Render content
	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := strings.TrimSpace(buf.String())

	// Determine prefix
	var prefix string
	if n.Parent != nil && n.Parent.Data == "ol" {
		num := customNumber
		if !hasCustomStart {
			// Count position for regular OL
			num = liIndexInParent(n)
		}
		prefix = fmt.Sprintf("%d. ", num)
	} else {
		prefix = "- "
	}

	w.WriteString(prefix + checkboxMarker + content + "\n")
	return converter.RenderSuccess
}

// liIndexInParent returns the 1-based position of li node n among its parent's
// li children. Callers guard that n.Parent is non-nil.
func liIndexInParent(n *html.Node) int {
	idx := 0
	for c := n.Parent.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "li" {
			idx++
			if c == n {
				break
			}
		}
	}
	return idx
}

// renderOrderedList dispatches to ArXiv enumerate or footnotes list renderers.
func renderOrderedList(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "ol" {
		return converter.RenderTryNext
	}

	// ArXiv enumerate: ol.ltx_enumerate
	if hasExactClass(getAttr(n, "class"), "ltx_enumerate") {
		return renderArXivEnumerate(ctx, w, n)
	}

	// Footnote definitions: ol inside #footnotes
	return renderFootnotesList(ctx, w, n)
}

// renderArXivEnumerate converts ArXiv ol.ltx_enumerate to standard numbered markdown.
// Strips <span class="ltx_tag ltx_tag_item">N.</span> prefix from each item.
func renderArXivEnumerate(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	var items []string
	idx := 0
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		idx++

		// Remove ltx_tag span from the li's children before rendering
		for c := li.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "span" &&
				strings.Contains(getAttr(c, "class"), "ltx_tag") {
				li.RemoveChild(c)
				break
			}
		}

		var buf bytes.Buffer
		ctx.RenderChildNodes(ctx, &buf, li)
		content := strings.TrimSpace(buf.String())
		if content != "" {
			items = append(items, fmt.Sprintf("%d. %s", idx, content))
		}
	}

	if len(items) == 0 {
		return converter.RenderTryNext
	}

	w.WriteString("\n\n")
	w.WriteString(strings.Join(items, "\n\n"))
	w.WriteString("\n\n")
	return converter.RenderSuccess
}
