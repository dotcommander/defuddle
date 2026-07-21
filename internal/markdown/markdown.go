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
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
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
	linkWhitespaceRe    = regexp.MustCompile(`\s`)
	linkTitleNewlineRe  = regexp.MustCompile(`(\n+\s*)+`)
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
