// Package markdown provides HTML to Markdown conversion functionality.
// It uses the html-to-markdown library with custom plugins for code blocks,
// figures, embeds, footnotes, callouts, highlights, and strikethrough.
package markdown

import (
	"fmt"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
)

// Pre-compiled patterns for post-processing.
var (
	leadingTitleRe = regexp.MustCompile(`^#\s+.+\n+`)
	emptyLinkRe    = regexp.MustCompile(`\n*(?:^|[^!])\[]\([^)]+\)\n*`)
	tripleNewline  = regexp.MustCompile(`\n{3,}`)
)

// ConvertHTML converts HTML content to Markdown with custom rules
// matching the TypeScript Defuddle implementation.
func ConvertHTML(htmlContent string) (string, error) {
	conv := converter.NewConverter(
		converter.WithPlugins(
			newDefuddlePlugin(),
			table.NewTablePlugin(
				table.WithSpanCellBehavior(table.SpanBehaviorEmpty),
			),
			strikethrough.NewStrikethroughPlugin(),
		),
	)

	md, err := conv.ConvertString(htmlContent)
	if err != nil {
		// Fallback to basic conversion
		md, err = htmltomarkdown.ConvertString(htmlContent)
		if err != nil {
			return "", fmt.Errorf("failed to convert HTML to Markdown: %w", err)
		}
	}

	md = postProcess(md)

	return md, nil
}

// postProcess applies cleanup rules matching TS Defuddle.
func postProcess(md string) string {
	// Remove the title from the beginning of the content if it exists
	md = leadingTitleRe.ReplaceAllString(md, "")

	// Remove any empty links [](url) but not image links ![](url)
	md = emptyLinkRe.ReplaceAllString(md, "")

	// Remove any consecutive newlines more than two
	md = tripleNewline.ReplaceAllString(md, "\n\n")

	return strings.TrimSpace(md)
}

// defuddlePlugin implements converter.Plugin with custom rendering rules.
type defuddlePlugin struct{}

func newDefuddlePlugin() *defuddlePlugin {
	return &defuddlePlugin{}
}

func (p *defuddlePlugin) Name() string { return "defuddle" }

func (p *defuddlePlugin) Init(conv *converter.Converter) error {
	// Code blocks with language detection
	conv.Register.RendererFor("pre", converter.TagTypeBlock, renderCodeBlock, converter.PriorityEarly)

	// Figures with images and captions
	conv.Register.RendererFor("figure", converter.TagTypeBlock, renderFigure, converter.PriorityEarly)

	// Highlight marks
	conv.Register.RendererFor("mark", converter.TagTypeInline, renderHighlight, converter.PriorityEarly)

	// YouTube/Twitter embeds
	conv.Register.RendererFor("iframe", converter.TagTypeBlock, renderEmbed, converter.PriorityEarly)

	// Footnote references (sup#fnref:X)
	conv.Register.RendererFor("sup", converter.TagTypeInline, renderFootnoteRef, converter.PriorityEarly)

	// Footnote backlink removal
	conv.Register.RendererFor("a", converter.TagTypeInline, renderLink, converter.PriorityEarly)

	// GitHub Markdown Alert callouts
	conv.Register.RendererFor("div", converter.TagTypeBlock, renderCallout, converter.PriorityEarly)

	// Callout blockquotes with data-callout
	conv.Register.RendererFor("blockquote", converter.TagTypeBlock, renderCalloutBlockquote, converter.PriorityEarly)

	// Remove button, style, script elements
	conv.Register.RendererFor("button", converter.TagTypeBlock, renderRemove, converter.PriorityEarly)
	conv.Register.RendererFor("style", converter.TagTypeBlock, renderRemove, converter.PriorityEarly)
	conv.Register.RendererFor("script", converter.TagTypeBlock, renderRemove, converter.PriorityEarly)

	return nil
}

// --- Renderers ---

func renderCodeBlock(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "pre" {
		return converter.RenderTryNext
	}

	// Find the <code> child
	var codeNode *html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "code" {
			codeNode = c
			break
		}
	}
	if codeNode == nil {
		return converter.RenderTryNext
	}

	// Detect language from various attributes
	lang := getAttr(codeNode, "data-lang")
	if lang == "" {
		lang = getAttr(codeNode, "data-language")
	}
	if lang == "" {
		lang = getAttr(n, "data-language")
	}
	if lang == "" {
		class := getAttr(codeNode, "class")
		if class != "" {
			for _, token := range strings.Fields(class) {
				if strings.HasPrefix(token, "language-") {
					lang = strings.TrimPrefix(token, "language-")
					break
				}
				if strings.HasPrefix(token, "lang-") {
					lang = strings.TrimPrefix(token, "lang-")
					break
				}
			}
		}
	}

	code := extractText(codeNode)
	code = strings.TrimSpace(code)

	w.WriteString("\n```" + lang + "\n")
	w.WriteString(code)
	w.WriteString("\n```\n")
	return converter.RenderSuccess
}

func renderFigure(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "figure" {
		return converter.RenderTryNext
	}

	var imgNode, captionNode *html.Node
	walkChildren(n, func(child *html.Node) bool {
		if child.Type == html.ElementNode {
			if child.Data == "img" && imgNode == nil {
				imgNode = child
			}
			if child.Data == "figcaption" && captionNode == nil {
				captionNode = child
			}
		}
		return true
	})

	if imgNode == nil {
		return converter.RenderTryNext
	}

	alt := getAttr(imgNode, "alt")
	src := getAttr(imgNode, "src")

	var caption string
	if captionNode != nil {
		caption = strings.TrimSpace(extractText(captionNode))
	}

	w.WriteString(fmt.Sprintf("\n![%s](%s)\n", alt, src))
	if caption != "" {
		w.WriteString("\n" + caption + "\n")
	}
	w.WriteString("\n")
	return converter.RenderSuccess
}

func renderHighlight(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "mark" {
		return converter.RenderTryNext
	}
	content := strings.TrimSpace(extractText(n))
	w.WriteString("==" + content + "==")
	return converter.RenderSuccess
}

var (
	youtubeRe = regexp.MustCompile(`(?:youtube\.com|youtu\.be)/(?:embed/|watch\?v=)?([a-zA-Z0-9_-]+)`)
	tweetRe   = regexp.MustCompile(`(?:twitter\.com|x\.com)/[^/]+/status/([0-9]+)`)
)

func renderEmbed(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "iframe" {
		return converter.RenderTryNext
	}

	src := getAttr(n, "src")
	if src == "" {
		return converter.RenderTryNext
	}

	if m := youtubeRe.FindStringSubmatch(src); m != nil {
		w.WriteString("\n![[ " + m[1] + " ]]\n")
		return converter.RenderSuccess
	}
	if m := tweetRe.FindStringSubmatch(src); m != nil {
		w.WriteString("\n![[ " + m[1] + " ]]\n")
		return converter.RenderSuccess
	}

	return converter.RenderTryNext
}

func renderFootnoteRef(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
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

	// Let the default link handler take care of normal links
	return converter.RenderTryNext
}

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

	content := strings.TrimSpace(extractText(n))
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
		w.WriteString("> " + strings.TrimSpace(line) + "\n")
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
	content := strings.TrimSpace(extractText(n))
	lines := strings.Split(content, "\n")

	w.WriteString("\n> [!" + calloutType + "] " + title + "\n")
	for _, line := range lines {
		w.WriteString("> " + strings.TrimSpace(line) + "\n")
	}
	w.WriteString("\n")
	return converter.RenderSuccess
}

func renderRemove(_ converter.Context, _ converter.Writer, _ *html.Node) converter.RenderStatus {
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
