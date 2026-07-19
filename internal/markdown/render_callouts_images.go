package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

func renderCallout(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "div" {
		return converter.RenderTryNext
	}
	class := getAttr(n, "class")
	if !strings.Contains(class, "markdown-alert") {
		return converter.RenderTryNext
	}

	// Extract alert type from class (e.g. markdown-alert-note → NOTE)
	alertType := "NOTE"
	for _, token := range strings.Fields(class) {
		if strings.HasPrefix(token, "markdown-alert-") && token != "markdown-alert" {
			alertType = strings.ToUpper(strings.TrimPrefix(token, "markdown-alert-"))
			break
		}
	}

	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := strings.TrimSpace(buf.String())
	// Remove the alert title label (GitHub renders it as ".markdown-alert-title")
	// which appears as the first word in the extracted text. Strip case-insensitively
	// since the DOM may contain "Note", "NOTE", etc.
	if strings.HasPrefix(strings.ToUpper(content), alertType) {
		content = strings.TrimSpace(content[len(alertType):])
	}
	content = strings.TrimSpace(content)

	lines := strings.Split(content, "\n")
	w.WriteString("\n> [!" + alertType + "]\n")
	for _, line := range lines {
		w.WriteString("> " + line + "\n")
	}
	w.WriteString("\n")
	return converter.RenderSuccess
}

func renderCalloutBlockquote(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "blockquote" {
		return converter.RenderTryNext
	}
	calloutType := getAttr(n, "data-callout")
	if calloutType == "" {
		return converter.RenderTryNext
	}

	title := strings.ToUpper(calloutType[:1]) + calloutType[1:]
	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := strings.TrimSpace(buf.String())
	lines := strings.Split(content, "\n")

	w.WriteString("\n> [!" + calloutType + "] " + title + "\n")
	for _, line := range lines {
		w.WriteString("> " + line + "\n")
	}
	w.WriteString("\n")
	return converter.RenderSuccess
}

func renderImage(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "img" {
		return converter.RenderTryNext
	}

	alt := getAttr(n, "alt")
	src := getBestImageSrc(n)
	if src == "" {
		return converter.RenderTryNext
	}
	title := getAttr(n, "title")
	titlePart := ""
	if title != "" {
		titlePart = ` "` + title + `"`
	}
	w.WriteString("![" + alt + "](" + src + titlePart + ")")
	return converter.RenderSuccess
}

func renderKeepHTML(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode {
		return converter.RenderTryNext
	}
	var sb strings.Builder
	html.Render(&sb, n)
	w.WriteString(sb.String())
	return converter.RenderSuccess
}

func renderRemove(_ converter.Context, _ converter.Writer, _ *html.Node) converter.RenderStatus {
	return converter.RenderSuccess
}

// renderSuperscript keeps non-footnote <sup> as raw HTML.
// Footnote refs (sup#fnref:X) are handled at PriorityEarly by renderFootnoteRef,
// so only non-footnote sups reach this PriorityStandard handler.
func renderSuperscript(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "sup" {
		return converter.RenderTryNext
	}
	var sb strings.Builder
	html.Render(&sb, n)
	w.WriteString(sb.String())
	return converter.RenderSuccess
}

// --- Helpers ---

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// extractLangFromClass extracts a language identifier from CSS class tokens
// matching "language-*" or "lang-*" patterns.
func extractLangFromClass(class string) string {
	for _, token := range strings.Fields(class) {
		if strings.HasPrefix(token, "language-") {
			return strings.TrimPrefix(token, "language-")
		}
		if strings.HasPrefix(token, "lang-") {
			return strings.TrimPrefix(token, "lang-")
		}
	}
	return ""
}

func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractText(c))
	}
	return sb.String()
}

func walkChildren(n *html.Node, fn func(*html.Node) bool) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !fn(c) {
			return
		}
		walkChildren(c, fn)
	}
}

// getBestImageSrc parses srcset to find the highest-resolution image URL.
// Falls back to src. Handles CDN URLs with commas (e.g. Substack).
func getBestImageSrc(n *html.Node) string {
	if srcset := getAttr(n, "srcset"); srcset != "" {
		if best := bestSrcsetURL(srcset); best != "" {
			return best
		}
	}
	return getAttr(n, "src")
}

// bestSrcsetURL returns the URL with the largest width descriptor in a srcset,
// or "" if none can be parsed.
func bestSrcsetURL(srcset string) string {
	var bestURL string
	var bestWidth int
	var urlParts []string

	for _, token := range strings.Fields(strings.TrimSpace(srcset)) {
		if m := widthDescriptorRe.FindStringSubmatch(token); m != nil {
			width := 0
			fmt.Sscanf(m[1], "%d", &width)
			if len(urlParts) > 0 && width > bestWidth {
				u := strings.TrimLeft(strings.Join(urlParts, " "), ", ")
				if u != "" {
					bestWidth = width
					bestURL = u
				}
			}
			urlParts = nil
		} else if densityDescriptorRe.MatchString(token) {
			// Density descriptor (e.g. 2x) — skip
			urlParts = nil
		} else {
			urlParts = append(urlParts, token)
		}
	}
	return bestURL
}
