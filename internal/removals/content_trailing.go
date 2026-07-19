package removals

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	textutil "github.com/dotcommander/defuddle/internal/text"
)

// removeTrailingRelatedPostsBlock removes a last-child section/div/aside that
// consists entirely of link-dense paragraphs (related posts pattern).
func removeTrailingRelatedPostsBlock(_ *goquery.Selection, mainNode *html.Node, debug bool) {
	lastChild := lastElementChild(mainNode)
	for lastChild != nil {
		tag := strings.ToUpper(lastChild.Data)
		if tag != "HR" && tag != "BR" {
			break
		}
		lastChild = prevElementSibling(lastChild)
	}
	if lastChild == nil {
		return
	}
	tag := strings.ToUpper(lastChild.Data)
	if tag != "SECTION" && tag != "DIV" && tag != "ASIDE" {
		return
	}

	var paras []*html.Node
	hasNonPara := false
	for c := lastChild.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		text := strings.TrimSpace(nodeText(c))
		if text == "" {
			continue
		}
		if strings.ToUpper(c.Data) == "P" {
			paras = append(paras, c)
		} else if strings.ToUpper(c.Data) != "BR" {
			hasNonPara = true
			break
		}
	}
	if len(paras) < 2 || hasNonPara {
		return
	}

	allLinkDense := true
	for _, p := range paras {
		text := strings.Join(strings.Fields(nodeText(p)), " ")
		var links []*html.Node
		collectByTag(p, "a", &links)
		if len(links) == 0 {
			allLinkDense = false
			break
		}
		linkTextLen := 0
		for _, a := range links {
			linkTextLen += len(strings.TrimSpace(nodeText(a)))
		}
		if float64(linkTextLen)/float64(max(1, len(text))) <= 0.6 {
			allLinkDense = false
			break
		}
		// No sentence punctuation outside of links.
		nonLink := text
		for _, a := range links {
			nonLink = strings.Replace(nonLink, strings.TrimSpace(nodeText(a)), "", 1)
		}
		if sentencePunctRe.MatchString(nonLink) {
			allLinkDense = false
			break
		}
	}
	if !allLinkDense {
		return
	}

	if debug {
		slog.Debug("removeByContentPattern: trailing related posts block", "text", previewNode(lastChild))
	}
	removeNode(lastChild)
}

// removeTrailingThinSections removes trailing direct children of mainContent
// that form a heading + thin CTA/promo block.
func removeTrailingThinSections(mainContent *goquery.Selection, debug bool) {
	totalWords := textutil.CountWords(strings.TrimSpace(mainContent.Text()))
	if totalWords <= 300 {
		return
	}

	var trailingEls []*html.Node
	trailingWords := 0
	mainNode := mainContent.Nodes[0]
	child := lastElementChild(mainNode)
	for child != nil {
		svgWords := 0
		var svgs []*html.Node
		collectByTag(child, "svg", &svgs)
		for _, svg := range svgs {
			svgWords += textutil.CountWords(nodeText(svg))
		}
		words := textutil.CountWords(strings.TrimSpace(nodeText(child))) - svgWords
		if words > 25 {
			break
		}
		trailingWords += words
		trailingEls = append(trailingEls, child)
		child = prevElementSibling(child)
	}

	if len(trailingEls) < 1 || trailingWords >= totalWords/7 { // 15% ≈ 1/7
		return
	}

	hasHeading := false
	for _, el := range trailingEls {
		tag := strings.ToUpper(el.Data)
		if tag == "H1" || tag == "H2" || tag == "H3" || tag == "H4" || tag == "H5" || tag == "H6" {
			hasHeading = true
			break
		}
		var headings []*html.Node
		collectByTag(el, "h1", &headings)
		collectByTag(el, "h2", &headings)
		collectByTag(el, "h3", &headings)
		if len(headings) > 0 {
			hasHeading = true
			break
		}
	}
	if !hasHeading {
		return
	}

	hasContent := false
	for _, el := range trailingEls {
		if hasContentElements(el) {
			hasContent = true
			break
		}
	}
	if hasContent {
		return
	}

	proseParagraphs := 0
	for _, el := range trailingEls {
		if strings.ToUpper(el.Data) == "P" && textutil.CountWords(nodeText(el)) > 5 {
			proseParagraphs++
		}
	}
	if proseParagraphs >= 2 {
		return
	}

	for _, el := range trailingEls {
		if debug {
			slog.Debug("removeByContentPattern: trailing thin section", "text", previewNode(el))
		}
		removeNode(el)
	}
}
