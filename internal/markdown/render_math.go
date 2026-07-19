package markdown

import (
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

func renderFootnoteRef(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "sup" {
		return converter.RenderTryNext
	}
	id := getAttr(n, "id")
	if !strings.HasPrefix(id, "fnref:") {
		return converter.RenderTryNext
	}
	num := strings.TrimPrefix(id, "fnref:")
	num = strings.Split(num, "-")[0]
	w.WriteString("[^" + num + "]")
	return converter.RenderSuccess
}

// renderMath converts <math> elements to LaTeX ($...$, $$...$$).
func renderMath(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "math" {
		return converter.RenderTryNext
	}

	latex := extractLatexFromNode(n)
	if latex == "" {
		// No LaTeX available — keep as raw HTML
		var sb strings.Builder
		html.Render(&sb, n)
		w.WriteString(sb.String())
		return converter.RenderSuccess
	}

	isBlock := getAttr(n, "display") == "block"

	// Never use block math inside tables (breaks layout)
	if isBlock && isInsideTable(n) {
		isBlock = false
	}

	if isBlock {
		w.WriteString("\n$$\n")
		w.WriteString(latex)
		w.WriteString("\n$$\n")
	} else {
		w.WriteString("$")
		w.WriteString(latex)
		w.WriteString("$")
	}
	return converter.RenderSuccess
}

// renderKaTeX converts KaTeX (.katex, .math) and MWE (.mwe-math-element) spans to LaTeX.
func renderKaTeX(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "span" {
		return converter.RenderTryNext
	}

	class := getAttr(n, "class")
	isKatex := hasExactClass(class, "katex") || hasExactClass(class, "math")
	isMweMath := hasExactClass(class, "mwe-math-element") ||
		strings.Contains(class, "mwe-math-fallback-image")

	if !isKatex && !isMweMath {
		return converter.RenderTryNext
	}

	// Extract LaTeX from various sources
	latex := extractKaTeXLatex(n)
	if latex == "" {
		return converter.RenderTryNext
	}

	// Determine display mode
	isBlock := isBlockMath(n, class)

	if isBlock && !isInsideTable(n) {
		w.WriteString("\n$$\n")
		w.WriteString(latex)
		w.WriteString("\n$$\n")
	} else {
		w.WriteString("$")
		w.WriteString(latex)
		w.WriteString("$")
	}
	return converter.RenderSuccess
}

// isBlockMath reports whether a katex/math span renders as display (block) math,
// from its class or a child <math display="block">.
func isBlockMath(n *html.Node, class string) bool {
	if hasExactClass(class, "katex-display") ||
		strings.Contains(class, "mwe-math-fallback-image-display") ||
		hasExactClass(class, "math-display") {
		return true
	}

	isBlock := false
	walkChildren(n, func(child *html.Node) bool {
		if child.Type == html.ElementNode && child.Data == "math" &&
			getAttr(child, "display") == "block" {
			isBlock = true
			return false
		}
		return true
	})
	return isBlock
}

// extractKaTeXLatex pulls LaTeX from a katex/math span: data-latex, then a KaTeX
// annotation (application/x-tex), then a child <math>'s alttext/data-latex, then
// the span's plain text. Returns "" if none found.
func extractKaTeXLatex(n *html.Node) string {
	latex := getAttr(n, "data-latex")
	if latex == "" {
		// KaTeX annotation
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if latex != "" {
				break
			}
			walkChildren(c, func(child *html.Node) bool {
				if child.Type == html.ElementNode && child.Data == "annotation" &&
					getAttr(child, "encoding") == "application/x-tex" {
					latex = strings.TrimSpace(extractText(child))
					return false
				}
				return true
			})
		}
	}
	if latex == "" {
		// Check alttext on child math elements
		walkChildren(n, func(child *html.Node) bool {
			if child.Type == html.ElementNode && child.Data == "math" {
				if alt := getAttr(child, "alttext"); alt != "" {
					latex = strings.TrimSpace(alt)
					return false
				}
				if dl := getAttr(child, "data-latex"); dl != "" {
					latex = strings.TrimSpace(dl)
					return false
				}
			}
			return true
		})
	}
	if latex == "" {
		latex = strings.TrimSpace(extractText(n))
	}
	return latex
}

// extractLatexFromNode extracts LaTeX from a <math> node's data-latex or alttext attributes.
func extractLatexFromNode(n *html.Node) string {
	if latex := getAttr(n, "data-latex"); latex != "" {
		return strings.TrimSpace(latex)
	}
	if alttext := getAttr(n, "alttext"); alttext != "" {
		return strings.TrimSpace(alttext)
	}
	return ""
}

func isInsideTable(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == "table" {
			return true
		}
	}
	return false
}

func hasExactClass(classAttr, className string) bool {
	for _, token := range strings.Fields(classAttr) {
		if token == className {
			return true
		}
	}
	return false
}
