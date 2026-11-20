package markdown

import (
	"bytes"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

func renderCallout(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "div" {
		return converter.RenderTryNext
	}
	class := getAttr(n, "class")
	if !hasExactClass(class, "markdown-alert") {
		return converter.RenderTryNext
	}

	// Extract alert type from class (e.g. markdown-alert-note → NOTE)
	alertType := "NOTE"
	for _, token := range strings.Fields(class) {
		if strings.HasPrefix(token, "markdown-alert-") && token != "markdown-alert-title" {
			if suffix := strings.TrimPrefix(token, "markdown-alert-"); suffix != "" {
				alertType = strings.ToUpper(suffix)
				break
			}
		}
	}

	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := strings.TrimSpace(buf.String())
	// Remove the alert title label (GitHub renders it as ".markdown-alert-title")
	// which appears as the first word in the extracted text. Strip case-insensitively
	// on a token boundary so words like "Notebook" or multi-byte characters are safe.
	content = stripLeadingAlertTitle(content, alertType)

	lines := strings.Split(content, "\n")
	w.WriteString("\n> [!" + alertType + "]\n")
	for _, line := range lines {
		w.WriteString("> " + line + "\n")
	}
	w.WriteString("\n")
	return converter.RenderSuccess
}

func stripLeadingAlertTitle(content, alertType string) string {
	if content == "" || alertType == "" {
		return content
	}
	end := strings.IndexFunc(content, unicode.IsSpace)
	firstToken := content
	rest := ""
	if end >= 0 {
		firstToken = content[:end]
		rest = content[end:]
	}
	if strings.EqualFold(firstToken, alertType) {
		return strings.TrimSpace(rest)
	}
	return content
}

func renderCalloutBlockquote(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "blockquote" {
		return converter.RenderTryNext
	}
	calloutType := getAttr(n, "data-callout")
	if calloutType == "" {
		return converter.RenderTryNext
	}

	firstRune, size := utf8.DecodeRuneInString(calloutType)
	title := strings.ToUpper(string(firstRune)) + calloutType[size:]
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

	alt := escapeMarkdownAlt(getAttr(n, "alt"))
	src := getBestImageSrc(n)
	if src == "" {
		return converter.RenderTryNext
	}
	title := getAttr(n, "title")
	w.WriteString("![" + alt + "](" + formatMarkdownLinkDestination(src) + formatMarkdownLinkTitle(title) + ")")
	return converter.RenderSuccess
}

func escapeMarkdownAlt(alt string) string {
	if alt == "" {
		return ""
	}
	alt = linkTitleNewlineRe.ReplaceAllString(alt, " ")
	return strings.NewReplacer("[", `\[`, "]", `\]`).Replace(alt)
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

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
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

// bestSrcsetURL parses an HTML srcset candidate list and returns the URL with
// the largest width descriptor, or "" if none can be parsed.
func bestSrcsetURL(srcset string) string {
	var bestURL string
	var bestWidth int

	s := srcset
	for len(s) > 0 {
		s = strings.TrimLeft(s, " \t\n\r\f,")
		if len(s) == 0 {
			break
		}

		wsIdx := strings.IndexAny(s, " \t\n\r\f")
		var rawURL string
		if wsIdx < 0 {
			rawURL = s
			s = ""
		} else {
			rawURL = s[:wsIdx]
			s = s[wsIdx:]
		}

		// A trailing comma ends a descriptor-less candidate; internal commas
		// (e.g. CDN transform paths like w_424,c_limit,f_webp) stay part of rawURL.
		if strings.HasSuffix(rawURL, ",") {
			continue
		}

		s = strings.TrimLeft(s, " \t\n\r\f")
		if len(s) == 0 {
			break
		}

		var desc string
		if commaIdx := strings.IndexByte(s, ','); commaIdx >= 0 {
			desc = strings.TrimSpace(s[:commaIdx])
			s = s[commaIdx+1:]
		} else {
			desc = strings.TrimSpace(s)
			s = ""
		}

		if m := widthDescriptorRe.FindStringSubmatch(desc); m != nil {
			width, err := strconv.Atoi(m[1])
			if err == nil && width > bestWidth {
				bestWidth = width
				bestURL = rawURL
			}
		}
	}

	return bestURL
}
