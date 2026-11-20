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

	var headingText string
	if headingNode != nil {
		var headingBuf bytes.Buffer
		ctx.RenderChildNodes(ctx, &headingBuf, headingNode)
		if text := strings.TrimSpace(headingBuf.String()); text != "" {
			level := int(headingNode.Data[1] - '0')
			headingText = strings.Repeat("#", level) + " " + text
		}
	}

	// Remove heading from parent temporarily to render remaining content
	if headingNode != nil {
		next := headingNode.NextSibling
		n.RemoveChild(headingNode)
		defer n.InsertBefore(headingNode, next)
	}
	var remainBuf bytes.Buffer
	ctx.RenderChildNodes(ctx, &remainBuf, n)

	remaining := strings.TrimSpace(remainBuf.String())
	if headingText != "" {
		w.WriteString(headingText)
		if remaining != "" {
			w.WriteString("\n\n")
		}
	}
	if remaining != "" {
		w.WriteString(remaining)
	}
	if href != "" {
		if headingText != "" || remaining != "" {
			w.WriteString("\n\n")
		}
		w.WriteString("[View original](" + formatMarkdownLinkDestination(href) + formatMarkdownLinkTitle(getAttr(n, "title")) + ")")
	}
	return converter.RenderSuccess
}

// unwrapComplexLinks restructures <a> elements containing a heading and sibling
// content before commonmark's pre-renderer downgrades headings inside links to
// bold text.
func unwrapComplexLinks(_ converter.Context, doc *html.Node) {
	var links []*html.Node
	walkChildren(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "a" && hasChildHeading(n) {
			links = append(links, n)
			return false
		}
		return true
	})

	for _, a := range links {
		parent := a.Parent
		if parent == nil {
			continue
		}
		href := getAttr(a, "href")
		title := getAttr(a, "title")

		var heading *html.Node
		for c := a.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				switch c.Data {
				case "h1", "h2", "h3", "h4", "h5", "h6":
					heading = c
				}
			}
			if heading != nil {
				break
			}
		}

		wrapper := &html.Node{Type: html.ElementNode, Data: "div"}
		if heading != nil {
			a.RemoveChild(heading)
			wrapper.AppendChild(heading)
		}
		for a.FirstChild != nil {
			child := a.FirstChild
			a.RemoveChild(child)
			wrapper.AppendChild(child)
		}
		if href != "" {
			p := &html.Node{Type: html.ElementNode, Data: "p"}
			link := &html.Node{
				Type: html.ElementNode,
				Data: "a",
				Attr: []html.Attribute{{Key: "href", Val: href}},
			}
			if title != "" {
				link.Attr = append(link.Attr, html.Attribute{Key: "title", Val: title})
			}
			link.AppendChild(&html.Node{Type: html.TextNode, Data: "View original"})
			p.AppendChild(link)
			wrapper.AppendChild(p)
		}

		parent.InsertBefore(wrapper, a)
		parent.RemoveChild(a)
	}
}

const taskCheckboxAttr = "data-defuddle-task-checkbox"

// preserveTaskListCheckboxes records checkbox state on task-list <li> elements
// and removes the <input type="checkbox"> before base.preRenderRemove strips
// all <input> elements from the DOM.
func preserveTaskListCheckboxes(_ converter.Context, doc *html.Node) {
	walkChildren(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "li" {
			return true
		}
		if checkbox := findTaskCheckbox(n); checkbox != nil {
			state := "unchecked"
			if hasAttr(checkbox, "checked") {
				state = "checked"
			}
			n.Attr = append(n.Attr, html.Attribute{Key: taskCheckboxAttr, Val: state})
			if checkbox.Parent != nil {
				checkbox.Parent.RemoveChild(checkbox)
			}
		}
		return true
	})
}

// findTaskCheckbox locates a task-list <input type="checkbox"> inside an <li>,
// either as a direct child or as the first element child of a leading <p>.
func findTaskCheckbox(li *html.Node) *html.Node {
	isTaskClass := hasExactClass(getAttr(li, "class"), "task-list-item")
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
			continue
		}
		if c.Type == html.ElementNode && c.Data == "input" && strings.EqualFold(getAttr(c, "type"), "checkbox") {
			return c
		}
		if c.Type == html.ElementNode && c.Data == "p" {
			for pc := c.FirstChild; pc != nil; pc = pc.NextSibling {
				if pc.Type == html.TextNode && strings.TrimSpace(pc.Data) == "" {
					continue
				}
				if pc.Type == html.ElementNode && pc.Data == "input" && strings.EqualFold(getAttr(pc, "type"), "checkbox") {
					return pc
				}
				break
			}
		}
		if !isTaskClass {
			break
		}
	}
	return nil
}

// renderListItem handles task-list checkboxes.
// Normal list items and <ol start="N"> fall through to commonmark's list
// container renderer, which emits the bullet/number prefix and handles nesting.
func renderListItem(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "li" {
		return converter.RenderTryNext
	}

	var checkboxMarker string
	switch getAttr(n, taskCheckboxAttr) {
	case "checked":
		checkboxMarker = "[x] "
	case "unchecked":
		checkboxMarker = "[ ] "
	default:
		if checkbox := findTaskCheckbox(n); checkbox != nil {
			if hasAttr(checkbox, "checked") {
				checkboxMarker = "[x] "
			} else {
				checkboxMarker = "[ ] "
			}
			if checkbox.Parent != nil {
				checkbox.Parent.RemoveChild(checkbox)
			}
		}
	}

	if checkboxMarker == "" {
		return converter.RenderTryNext
	}

	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := strings.TrimSpace(buf.String())

	// Inside <ul>/<ol>, commonmark's renderListContainer supplies the list marker
	// and multi-line/nested indentation, so emit only the checkbox + content.
	if n.Parent != nil && n.Parent.Type == html.ElementNode && (n.Parent.Data == "ul" || n.Parent.Data == "ol") {
		w.WriteString(checkboxMarker + content)
		return converter.RenderSuccess
	}

	w.WriteString("- " + checkboxMarker + content + "\n")
	return converter.RenderSuccess
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
