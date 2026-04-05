// Package markdown provides HTML to Markdown conversion functionality.
// It uses the html-to-markdown library with custom plugins for code blocks,
// figures, embeds, footnotes, callouts, highlights, and strikethrough.
package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
)

// Pre-compiled patterns for post-processing.
var (
	leadingTitleRe      = regexp.MustCompile(`^#\s+.+\n+`)
	emptyLinkRe         = regexp.MustCompile(`\n*([^!]|^)\[]\([^)]+\)\n*`)
	tripleNewline       = regexp.MustCompile(`\n{3,}`)
	bangBeforeImageRe   = regexp.MustCompile(`!(!\[|\[!\[)`)
	wbrTagRe            = regexp.MustCompile(`(?i)<wbr\s*/?>`)
	widthDescriptorRe   = regexp.MustCompile(`^(\d+)w,?$`)
	densityDescriptorRe = regexp.MustCompile(`^\d+(?:\.\d+)?x,?$`)
	backLinkRe          = regexp.MustCompile(`\s*↩︎\s*$`)
)

// ConvertHTML converts HTML content to Markdown with custom rules
// matching the TypeScript Defuddle implementation.
func ConvertHTML(htmlContent string) (string, error) {
	// Strip <wbr> tags before conversion — word break opportunity hints
	// that are invisible in browsers but insert unwanted spaces.
	htmlContent = wbrTagRe.ReplaceAllString(htmlContent, "")

	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
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

	// Remove any empty links [](url) but not image links ![](url).
	// Group 1 captures the non-! character before [ so we can restore it.
	md = emptyLinkRe.ReplaceAllString(md, "$1")

	// Add a space between exclamation marks and image syntax ![
	// e.g. "Yey!![IMG](url)" becomes "Yey! ![IMG](url)"
	md = bangBeforeImageRe.ReplaceAllString(md, "! $1")

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

	// ArXiv enumerate lists (ol.ltx_enumerate) + Footnote definitions (ol inside #footnotes)
	conv.Register.RendererFor("ol", converter.TagTypeBlock, renderOrderedList, converter.PriorityEarly)

	// Footnote backlink removal
	conv.Register.RendererFor("a", converter.TagTypeInline, renderLink, converter.PriorityEarly)

	// GitHub Markdown Alert callouts
	conv.Register.RendererFor("div", converter.TagTypeBlock, renderCallout, converter.PriorityEarly)

	// Callout blockquotes with data-callout
	conv.Register.RendererFor("blockquote", converter.TagTypeBlock, renderCalloutBlockquote, converter.PriorityEarly)

	// Standalone images with srcset resolution and title support
	conv.Register.RendererFor("img", converter.TagTypeInline, renderImage, converter.PriorityEarly)

	// Remove button, style, script elements
	conv.Register.RendererFor("button", converter.TagTypeBlock, renderRemove, converter.PriorityEarly)
	conv.Register.RendererFor("style", converter.TagTypeBlock, renderRemove, converter.PriorityEarly)
	conv.Register.RendererFor("script", converter.TagTypeBlock, renderRemove, converter.PriorityEarly)

	// Non-footnote superscripts (footnote refs handled by renderFootnoteRef)
	conv.Register.RendererFor("sup", converter.TagTypeInline, renderSuperscript, converter.PriorityStandard)

	// Math elements → LaTeX ($...$, $$...$$)
	conv.Register.RendererFor("math", converter.TagTypeInline, renderMath, converter.PriorityEarly)

	// KaTeX/MathJax spans → LaTeX
	conv.Register.RendererFor("span", converter.TagTypeInline, renderKaTeX, converter.PriorityEarly)

	// List items with task-list checkbox and OL start attribute support
	conv.Register.RendererFor("li", converter.TagTypeBlock, renderListItem, converter.PriorityEarly)

	// ArXiv equation tables (table.ltx_equation, table.ltx_eqn_table)
	conv.Register.RendererFor("table", converter.TagTypeBlock, renderArXivEquationTable, converter.PriorityEarly)

	// Keep HTML elements that have no markdown equivalent
	conv.Register.RendererFor("video", converter.TagTypeBlock, renderKeepHTML, converter.PriorityEarly)
	conv.Register.RendererFor("audio", converter.TagTypeBlock, renderKeepHTML, converter.PriorityEarly)
	conv.Register.RendererFor("svg", converter.TagTypeBlock, renderKeepHTML, converter.PriorityEarly)
	conv.Register.RendererFor("sub", converter.TagTypeInline, renderKeepHTML, converter.PriorityEarly)

	return nil
}

// --- Renderers ---

func renderCodeBlock(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
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

	// Detect language from data attributes (code, then pre)
	lang := getAttr(codeNode, "data-lang")
	if lang == "" {
		lang = getAttr(codeNode, "data-language")
	}
	if lang == "" {
		lang = getAttr(n, "data-lang")
	}
	if lang == "" {
		lang = getAttr(n, "data-language")
	}
	// Detect from class tokens (code, then pre)
	if lang == "" {
		lang = extractLangFromClass(getAttr(codeNode, "class"))
	}
	if lang == "" {
		lang = extractLangFromClass(getAttr(n, "class"))
	}

	code := extractText(codeNode)
	code = strings.TrimSpace(code)

	// Choose a fence that doesn't conflict with the code content.
	fence := "```"
	if strings.Contains(code, "```") {
		fence = "````"
	}

	w.WriteString("\n" + fence + lang + "\n")
	w.WriteString(code)
	w.WriteString("\n" + fence + "\n")
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
	src := getBestImageSrc(imgNode)

	var caption string
	if captionNode != nil {
		var capBuf bytes.Buffer
		ctx.RenderChildNodes(ctx, &capBuf, captionNode)
		caption = strings.TrimSpace(capBuf.String())
	}

	fmt.Fprintf(w, "\n![%s](%s)\n", alt, src)
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
	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := strings.TrimSpace(buf.String())
	w.WriteString("==" + content + "==")
	return converter.RenderSuccess
}

var (
	youtubeRe    = regexp.MustCompile(`(?:youtube\.com|youtube-nocookie\.com|youtu\.be)/(?:embed/|watch\?v=)?([a-zA-Z0-9_-]+)`)
	tweetRe      = regexp.MustCompile(`(?:twitter\.com|x\.com)/([^/]+)/status/([0-9]+)`)
	tweetEmbedRe = regexp.MustCompile(`(?:platform\.)?twitter\.com/embed/Tweet\.html\?.*?id=([0-9]+)`)
)

func renderEmbed(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "iframe" {
		return converter.RenderTryNext
	}

	src := getAttr(n, "src")
	if src == "" {
		return converter.RenderTryNext
	}

	if m := youtubeRe.FindStringSubmatch(src); m != nil {
		w.WriteString("\n![](https://www.youtube.com/watch?v=" + m[1] + ")\n")
		return converter.RenderSuccess
	}

	// Direct tweet URL: /user/status/id
	if m := tweetRe.FindStringSubmatch(src); m != nil {
		w.WriteString("\n![](https://x.com/" + m[1] + "/status/" + m[2] + ")\n")
		return converter.RenderSuccess
	}

	// Platform embed: ?id=
	if m := tweetEmbedRe.FindStringSubmatch(src); m != nil {
		w.WriteString("\n![](https://x.com/i/status/" + m[1] + ")\n")
		return converter.RenderSuccess
	}

	return converter.RenderTryNext
}

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
	if latex == "" {
		return converter.RenderTryNext
	}

	// Determine display mode
	isBlock := hasExactClass(class, "katex-display") ||
		strings.Contains(class, "mwe-math-fallback-image-display") ||
		hasExactClass(class, "math-display")

	if !isBlock {
		// Check child math element
		walkChildren(n, func(child *html.Node) bool {
			if child.Type == html.ElementNode && child.Data == "math" &&
				getAttr(child, "display") == "block" {
				isBlock = true
				return false
			}
			return true
		})
	}

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

	// Let the default link handler take care of normal links
	return converter.RenderTryNext
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
		w.WriteString("[View original](" + href + ")")
		if title := getAttr(n, "title"); title != "" {
			fmt.Fprintf(w, ` "%s"`, title)
		}
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
				idx := 0
				for c := n.Parent.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "li" {
						idx++
						if c == n {
							break
						}
					}
				}
				customNumber = startNum + idx - 1
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
			num = 0
			for c := n.Parent.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "li" {
					num++
					if c == n {
						break
					}
				}
			}
		}
		prefix = fmt.Sprintf("%d. ", num)
	} else {
		prefix = "- "
	}

	w.WriteString(prefix + checkboxMarker + content + "\n")
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

		if content != "" {
			defs = append(defs, fmt.Sprintf("[^%s]: %s", strings.ToLower(id), content))
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

// renderTableSpecial handles ArXiv equation tables and complex tables (colspan/rowspan).
// ArXiv tables → LaTeX; complex tables → cleaned raw HTML.
func renderArXivEquationTable(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode || n.Data != "table" {
		return converter.RenderTryNext
	}

	class := getAttr(n, "class")

	// ArXiv equation tables → LaTeX
	if hasExactClass(class, "ltx_equation") || hasExactClass(class, "ltx_eqn_table") {
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
	srcset := getAttr(n, "srcset")
	if srcset != "" {
		var bestURL string
		var bestWidth int
		tokens := strings.Fields(strings.TrimSpace(srcset))
		var urlParts []string

		for _, token := range tokens {
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
		if bestURL != "" {
			return bestURL
		}
	}
	return getAttr(n, "src")
}
