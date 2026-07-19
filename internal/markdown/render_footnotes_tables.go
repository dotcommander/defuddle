package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

// renderFootnotesList converts <ol> inside a #footnotes container to
// markdown footnote definition syntax: [^id]: content
func renderFootnotesList(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "ol" {
		return converter.RenderTryNext
	}

	// Only match <ol> whose parent has id="footnotes"
	if n.Parent == nil || getAttr(n.Parent, "id") != "footnotes" {
		return converter.RenderTryNext
	}

	var defs []string
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		if def := footnoteDefinition(ctx, li); def != "" {
			defs = append(defs, def)
		}
	}

	if len(defs) == 0 {
		return converter.RenderSuccess
	}

	w.WriteString("\n\n")
	w.WriteString(strings.Join(defs, "\n\n"))
	w.WriteString("\n\n")
	return converter.RenderSuccess
}

// footnoteDefinition renders one footnote <li> as a "[^id]: content" markdown
// definition, deriving the id from fn:/cite_note- conventions and stripping the
// leading sup marker and trailing backlink. Returns "" when the content is empty.
func footnoteDefinition(ctx converter.Context, li *html.Node) string {
	// Extract footnote ID from li's id attribute
	liID := getAttr(li, "id")
	id := liID
	if strings.HasPrefix(liID, "fn:") {
		id = strings.TrimPrefix(liID, "fn:")
	} else if idx := strings.LastIndex(liID, "/"); idx >= 0 {
		// Handle cite_note-style IDs
		tail := liID[idx+1:]
		if strings.HasPrefix(tail, "cite_note-") {
			id = strings.TrimPrefix(tail, "cite_note-")
		}
	}

	// Remove leading <sup> if its text matches the footnote ID
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "sup" {
			if strings.TrimSpace(extractText(c)) == id {
				li.RemoveChild(c)
			}
			break
		}
	}

	// Render li content to markdown
	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, li)
	content := strings.TrimSpace(buf.String())

	// Remove backlink symbol
	content = strings.TrimRight(content, " ")
	content = backLinkRe.ReplaceAllString(content, "")
	content = strings.TrimSpace(content)

	if content == "" {
		return ""
	}
	return fmt.Sprintf("[^%s]: %s", strings.ToLower(id), content)
}

// renderTableSpecial handles ArXiv equation tables and complex tables (colspan/rowspan).
// ArXiv tables → LaTeX; complex tables → cleaned raw HTML.
func renderArXivEquationTable(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "table" {
		return converter.RenderTryNext
	}

	class := getAttr(n, "class")

	// ArXiv equation tables → LaTeX
	if hasExactClass(class, "ltx_equation") || hasExactClass(class, "ltx_eqn_table") {
		equations := collectArXivEquations(n)
		if len(equations) > 0 {
			w.WriteString(strings.Join(equations, "\n\n"))
			return converter.RenderSuccess
		}
		return converter.RenderTryNext
	}

	// Complex tables (colspan/rowspan) → cleaned raw HTML
	if hasComplexTableStructure(n) {
		cleaned := cleanupTableHTML(n)
		w.WriteString("\n\n")
		w.WriteString(cleaned)
		w.WriteString("\n\n")
		return converter.RenderSuccess
	}

	return converter.RenderTryNext
}

// collectArXivEquations returns the LaTeX equations from <math> alttext within an
// ArXiv equation table, each wrapped inline ($...$) or display ($$...$$) per the
// ltx_eqn_inline ancestor class.
func collectArXivEquations(n *html.Node) []string {
	var equations []string
	walkChildren(n, func(child *html.Node) bool {
		if child.Type == html.ElementNode && child.Data == "math" {
			if alttext := getAttr(child, "alttext"); alttext != "" {
				alttext = strings.TrimSpace(alttext)
				isInline := false
				for p := child.Parent; p != nil; p = p.Parent {
					if p.Type == html.ElementNode && hasExactClass(getAttr(p, "class"), "ltx_eqn_inline") {
						isInline = true
						break
					}
				}
				if isInline {
					equations = append(equations, "$"+alttext+"$")
				} else {
					equations = append(equations, "\n$$\n"+alttext+"\n$$")
				}
			}
		}
		return true
	})
	return equations
}

// hasComplexTableStructure checks if any td/th has colspan or rowspan.
func hasComplexTableStructure(n *html.Node) bool {
	found := false
	walkChildren(n, func(child *html.Node) bool {
		if found {
			return false
		}
		if child.Type == html.ElementNode && (child.Data == "td" || child.Data == "th") {
			if getAttr(child, "colspan") != "" || getAttr(child, "rowspan") != "" {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// cleanupTableHTML strips non-essential attributes from a table, preserving layout attrs.
func cleanupTableHTML(n *html.Node) string {
	allowed := map[string]bool{
		"src": true, "href": true, "style": true, "align": true,
		"width": true, "height": true, "rowspan": true, "colspan": true,
		"bgcolor": true, "scope": true, "valign": true, "headers": true,
	}

	// Clean attributes recursively (modifies in place — table is already
	// a working copy from the goquery clone in standardize pipeline)
	var clean func(*html.Node)
	clean = func(node *html.Node) {
		if node.Type == html.ElementNode {
			var kept []html.Attribute
			for _, a := range node.Attr {
				if allowed[a.Key] {
					kept = append(kept, a)
				}
			}
			node.Attr = kept
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			clean(c)
		}
	}
	clean(n)

	var sb strings.Builder
	html.Render(&sb, n)
	return sb.String()
}
