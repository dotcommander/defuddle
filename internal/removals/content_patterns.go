package removals

import (
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	textutil "github.com/kaptinlin/defuddle-go/internal/text"
)

// Pre-compiled regex patterns for content-pattern removal.
var (
	contentDatePattern     = regexp.MustCompile(`(?i)(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\s+\d{1,2}`)
	contentReadTimePattern = regexp.MustCompile(`(?i)\d+\s*min(?:ute)?s?\s+read\b`)
	bylineUppercasePattern = regexp.MustCompile(`^\p{Lu}`)
	startsByPattern        = regexp.MustCompile(`(?i)^by\s+\S`)

	boilerplatePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^This (?:article|story|piece) (?:appeared|was published|originally appeared) in\b`),
		regexp.MustCompile(`(?i)^A version of this (?:article|story) (?:appeared|was published) in\b`),
		regexp.MustCompile(`(?i)^Originally (?:published|appeared) (?:in|on|at)\b`),
		regexp.MustCompile(`(?i)^Any re-?use permitted\b`),
		regexp.MustCompile(`(?i)^©\s*(?:Copyright\s+)?\d{4}`),
		regexp.MustCompile(`(?i)^Comments?$`),
		regexp.MustCompile(`(?i)^Leave a (?:comment|reply)$`),
	}

	newsletterPattern = regexp.MustCompile(
		`(?i)\bsubscribe\b[\s\S]{0,40}\bnewsletter\b|\bnewsletter\b[\s\S]{0,40}\bsubscribe\b|\bsign[- ]up\b[\s\S]{0,80}\b(?:newsletter|email alert)`,
	)

	relatedHeadingPattern = regexp.MustCompile(
		`(?i)^(?:related (?:posts?|articles?|content|stories|reads?|reading)|you (?:might|may|could) (?:also )?(?:like|enjoy|be interested in)|read (?:next|more|also)|further reading|see also|more (?:from|articles?|posts?|like this)|more to (?:read|explore)|about (?:the )?author)$`,
	)

	breadcrumbLinkPattern = regexp.MustCompile(`^/[a-zA-Z0-9_-]+/?$`)
	parentIndexPattern    = regexp.MustCompile(`(?i)^index\.(html?|php)$`)
	camelBoundary         = regexp.MustCompile(`([a-z])([A-Z])`)

	// Metadata strip patterns — date/number components.
	metadataStripMonth  = regexp.MustCompile(`(?i)\b(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|Jun(?:e)?|Jul(?:y)?|Aug(?:ust)?|Sep(?:t(?:ember)?)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)\b`)
	metadataStripNumber = regexp.MustCompile(`\b\d+(?:st|nd|rd|th)?\b`)

	// Read-time strip patterns (expect empty residual after applying all).
	readTimeStripMin   = regexp.MustCompile(`(?i)\bmin(?:ute)?s?\b`)
	readTimeStripRead  = regexp.MustCompile(`(?i)\bread\b`)
	readTimeStripPunct = regexp.MustCompile(`[/|·•—–\-,.\s]+`)

	// Byline strip patterns (preserve spaces so name words can be identified).
	bylineStripBy    = regexp.MustCompile(`(?i)\bby\b`)
	bylineStripPunct = regexp.MustCompile(`[/|·•—–\-,]+`)
)

// RemoveByContentPattern detects and removes boilerplate, metadata, and
// navigational fragments from mainContent. It is a faithful port of the
// TypeScript removeByContentPattern function.
func RemoveByContentPattern(mainContent *goquery.Selection, doc *goquery.Document, debug bool, pageURL string) {
	mainNode := mainContent.Nodes[0]

	removeBreadcrumbList(mainContent, mainNode, debug)
	removePromotionalBanners(mainContent, debug)
	removeHeroHeader(mainContent, mainNode, debug)
	removeSinglePassMetadata(mainContent, mainNode, debug)
	removeStandaloneTimeElements(mainContent, debug)
	removeBlogMetadataLists(mainContent, debug)
	removeSectionBreadcrumbs(mainContent, pageURL, debug)
	removeTrailingExternalLinkLists(mainContent, pageURL, debug)
	removeTrailingRelatedPostsBlock(mainContent, mainNode, debug)
	removeTrailingThinSections(mainContent, debug)
	removeBoilerplateSentences(mainContent, mainNode, debug)
	removeRelatedHeadingSections(mainContent, debug)
	removeRelatedPostCardGrids(mainContent, mainNode, debug)
	removeNewsletterSections(mainContent, mainNode, debug)
}

// removeBreadcrumbList removes the first ul/ol if it looks like a breadcrumb.
func removeBreadcrumbList(mainContent *goquery.Selection, mainNode *html.Node, debug bool) {
	firstList := mainContent.Find("ul, ol").First()
	if firstList.Length() == 0 {
		return
	}
	listNode := firstList.Nodes[0]
	if !isBreadcrumbList(listNode) {
		return
	}
	// Walk up while sole child.
	target := listNode
	for target.Parent != nil && target.Parent != mainNode {
		parent := target.Parent
		if len(elementChildren(parent)) != 1 {
			break
		}
		target = parent
	}
	if debug {
		slog.Debug("removeByContentPattern: breadcrumb navigation list", "text", previewNode(target))
	}
	removeNode(target)
}

// removePromotionalBanners removes <a href> blocks appearing before the first h1
// that look like announcement banners (short text, block children, no punctuation).
func removePromotionalBanners(mainContent *goquery.Selection, debug bool) {
	firstH1 := mainContent.Find("h1").First()
	if firstH1.Length() == 0 {
		return
	}
	h1Node := firstH1.Nodes[0]

	mainContent.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
		linkNode := link.Nodes[0]
		if linkNode.Parent == nil {
			return
		}
		// Link must come before h1 in document order.
		if !nodePrecedes(linkNode, h1Node) {
			return
		}
		if link.Find("div").Length() == 0 {
			return
		}
		text := strings.TrimSpace(link.Text())
		if textutil.CountWords(text) > 25 {
			return
		}
		if regexp.MustCompile(`[.!?]\s`).MatchString(text) {
			return
		}
		if debug {
			slog.Debug("removeByContentPattern: promotional banner link", "text", text)
		}
		link.Remove()
	})
}

// removeHeroHeader removes hero header blocks that wrap h1/h2 + time with little prose.
func removeHeroHeader(mainContent *goquery.Selection, mainNode *html.Node, debug bool) {
	timeEls := mainContent.Find("time")
	if timeEls.Length() == 0 {
		return
	}
	contentText := strings.TrimSpace(mainContent.Text())

	timeEls.Each(func(_ int, timeSel *goquery.Selection) {
		timeNode := timeSel.Nodes[0]
		timeText := strings.TrimSpace(timeSel.Text())
		pos := strings.Index(contentText, timeText)
		if pos > 300 {
			return
		}

		var bestBlock *html.Node
		current := timeNode.Parent
		for current != nil && current != mainNode {
			hasSel := goquery.NewDocumentFromNode(current)
			if hasSel.Find("h1, h2").Length() > 0 && hasSel.Find("time").Length() > 0 {
				blockText := strings.TrimSpace(nodeText(current))
				totalWords := textutil.CountWords(blockText)
				metadataWords := countMetadataWords(current)
				proseWords := totalWords - metadataWords
				if proseWords < 30 {
					bestBlock = current
				} else {
					break
				}
			}
			current = current.Parent
		}

		if bestBlock != nil {
			if debug {
				slog.Debug("removeByContentPattern: hero header block", "text", previewNode(bestBlock))
			}
			removeNode(bestBlock)
		}
	})
}

// removeSinglePassMetadata performs a single pass over p/span/div/time elements
// checking for DIV metadata blocks, author bylines, read-time, and author+date combos.
func removeSinglePassMetadata(mainContent *goquery.Selection, mainNode *html.Node, debug bool) {
	contentText := strings.TrimSpace(mainContent.Text())
	bylineFound := false
	authorDateFound := false

	mainContent.Find("p, span, div, time").Each(func(_ int, el *goquery.Selection) {
		node := el.Nodes[0]
		if node.Parent == nil {
			return
		}
		text := strings.TrimSpace(el.Text())
		words := textutil.CountWords(text)
		if words > 15 || words == 0 {
			return
		}
		if el.Closest("pre, code").Length() > 0 {
			return
		}

		tag := strings.ToUpper(node.Data)
		hasDate := contentDatePattern.MatchString(text)

		posCache := -2 // sentinel: not yet computed
		getPos := func() int {
			if posCache == -2 {
				posCache = strings.Index(contentText, text)
			}
			return posCache
		}

		// DIV metadata blocks near top (date but no sentence punctuation).
		if tag == "DIV" && words >= 1 && words <= 10 && hasDate && !regexp.MustCompile(`[.!?]`).MatchString(text) && getPos() <= 400 {
			hasBigPara := false
			el.Find("p, h1, h2, h3, h4, h5, h6").Each(func(_ int, sub *goquery.Selection) {
				if textutil.CountWords(sub.Text()) > 8 {
					hasBigPara = true
				}
			})
			if !hasBigPara {
				if debug {
					slog.Debug("removeByContentPattern: article metadata header block", "text", text)
				}
				el.Remove()
				return
			}
		}

		// Author byline "By Name" near start.
		if !bylineFound && startsByPattern.MatchString(text) && words >= 2 && !regexp.MustCompile(`[.!?]$`).MatchString(text) && getPos() <= 600 {
			target := walkUpToWrapper(node, mainNode, text)
			if debug {
				slog.Debug("removeByContentPattern: author byline", "text", previewNode(target))
			}
			removeNode(target)
			bylineFound = true
			return
		}

		// Read-time metadata (e.g. "Mar 4th | 3 min read").
		if hasDate && contentReadTimePattern.MatchString(text) && el.Find("p, div, section, article").Length() == 0 {
			cleaned := metadataStripMonth.ReplaceAllString(text, "")
			cleaned = metadataStripNumber.ReplaceAllString(cleaned, "")
			cleaned = readTimeStripMin.ReplaceAllString(cleaned, "")
			cleaned = readTimeStripRead.ReplaceAllString(cleaned, "")
			cleaned = readTimeStripPunct.ReplaceAllString(cleaned, "")
			if strings.TrimSpace(cleaned) == "" {
				if debug {
					slog.Debug("removeByContentPattern: read time metadata", "text", text)
				}
				el.Remove()
				return
			}
		}

		// Author + date combo near start.
		if !authorDateFound && words >= 2 && words <= 10 && hasDate && getPos() <= 500 {
			residual := metadataStripMonth.ReplaceAllString(text, "")
			residual = metadataStripNumber.ReplaceAllString(residual, "")
			residual = bylineStripBy.ReplaceAllString(residual, "")
			residual = bylineStripPunct.ReplaceAllString(residual, "")
			residual = strings.TrimSpace(residual)
			if residual != "" {
				nameWords := strings.Fields(residual)
				if len(nameWords) >= 1 && len(nameWords) <= 4 && allUppercaseFirst(nameWords) {
					target := walkUpToWrapper(node, mainNode, text)
					if debug {
						slog.Debug("removeByContentPattern: author date metadata", "text", previewNode(target))
					}
					removeNode(target)
					authorDateFound = true
				}
			}
		}
	})
}

// removeStandaloneTimeElements removes <time> elements near content boundaries
// that are not inline within prose.
func removeStandaloneTimeElements(mainContent *goquery.Selection, debug bool) {
	contentText := strings.TrimSpace(mainContent.Text())

	mainContent.Find("time").Each(func(_ int, timeSel *goquery.Selection) {
		timeNode := timeSel.Nodes[0]
		if timeNode.Parent == nil {
			return
		}
		// Walk up through inline wrappers only.
		target := timeNode
		targetText := strings.TrimSpace(nodeText(target))
		for target.Parent != nil {
			parentTag := strings.ToLower(target.Parent.Data)
			parentText := strings.TrimSpace(nodeText(target.Parent))
			if parentTag == "p" && parentText == targetText {
				target = target.Parent
				break
			}
			inlineWrappers := map[string]bool{"i": true, "em": true, "span": true, "b": true, "strong": true, "small": true}
			if inlineWrappers[parentTag] && parentText == targetText {
				target = target.Parent
				targetText = parentText
				continue
			}
			break
		}

		text := strings.TrimSpace(nodeText(target))
		if textutil.CountWords(text) > 10 {
			return
		}
		pos := strings.Index(contentText, text)
		if pos < 0 {
			return
		}
		distFromEnd := len(contentText) - (pos + len(text))
		if pos > 200 && distFromEnd > 200 {
			return
		}
		if debug {
			slog.Debug("removeByContentPattern: boundary date element", "text", text)
		}
		removeNode(target)
	})
}

// removeBlogMetadataLists removes short ul/ol/dl near content boundaries that
// look like post-metadata blocks (author, date, reading time).
func removeBlogMetadataLists(mainContent *goquery.Selection, debug bool) {
	contentText := strings.TrimSpace(mainContent.Text())

	mainContent.Find("ul, ol, dl").Each(func(_ int, list *goquery.Selection) {
		listNode := list.Nodes[0]
		if listNode.Parent == nil {
			return
		}
		isDL := strings.ToUpper(listNode.Data) == "DL"
		itemTag := "LI"
		if isDL {
			itemTag = "DD"
		}
		var items []*goquery.Selection
		list.Children().Each(func(_ int, child *goquery.Selection) {
			if strings.ToUpper(child.Nodes[0].Data) == itemTag {
				items = append(items, child)
			}
		})

		minItems := 2
		if isDL {
			minItems = 1
		}
		if len(items) < minItems || len(items) > 8 {
			return
		}

		listText := strings.TrimSpace(list.Text())
		listPos := strings.Index(contentText, listText)
		distFromEnd := len(contentText) - (listPos + len(listText))
		if listPos > 500 && distFromEnd > 500 {
			return
		}

		// Skip if previous sibling ends with ":" (content list intro).
		prev := prevElementSibling(listNode)
		if prev != nil {
			prevText := strings.TrimSpace(nodeText(prev))
			if strings.HasSuffix(prevText, ":") {
				return
			}
		}

		// Every item must be short and have no prose punctuation.
		isMetadata := true
		for _, item := range items {
			t := strings.TrimSpace(item.Text())
			if textutil.CountWords(t) > 8 || regexp.MustCompile(`[.!?]$`).MatchString(t) {
				isMetadata = false
				break
			}
		}
		if !isMetadata {
			return
		}
		if textutil.CountWords(listText) > 30 {
			return
		}

		target := walkUpToWrapper(listNode, mainContent.Nodes[0], listText)
		if debug {
			slog.Debug("removeByContentPattern: blog metadata list", "text", previewNode(target))
		}
		removeNode(target)
	})
}

// removeSectionBreadcrumbs removes short elements containing links to parent URL paths.
func removeSectionBreadcrumbs(mainContent *goquery.Selection, pageURL string, debug bool) {
	if pageURL == "" {
		return
	}
	parsed, err := url.Parse(pageURL)
	if err != nil || parsed.Path == "" {
		return
	}
	urlPath := parsed.Path

	firstHeading := mainContent.Find("h1, h2, h3").First()

	mainContent.Find("div, span, p, a[href]").Each(func(_ int, el *goquery.Selection) {
		elNode := el.Nodes[0]
		if elNode.Parent == nil {
			return
		}
		text := strings.TrimSpace(el.Text())
		if textutil.CountWords(text) > 10 {
			return
		}
		if el.Find("p, div, section, article").Length() > 0 {
			return
		}

		// For bare <a>, skip if embedded in flowing prose unless it precedes the heading.
		if el.Is("a[href]") && elNode.Parent != mainContent.Nodes[0] {
			parentText := strings.TrimSpace(goquery.NewDocumentFromNode(elNode.Parent).Text())
			if parentText != text {
				if el.Closest("p").Length() > 0 {
					return
				}
				if firstHeading.Length() == 0 {
					return
				}
				if !nodePrecedes(elNode, firstHeading.Nodes[0]) {
					return
				}
			}
		}

		var linkNode *html.Node
		if el.Is("a[href]") {
			linkNode = elNode
		} else {
			found := el.Find("a[href]")
			if found.Length() == 0 {
				return
			}
			linkNode = found.Nodes[0]
		}

		href := nodeAttr(linkNode, "href")
		linkParsed, err := url.Parse(href)
		if err != nil {
			return
		}
		var linkPath string
		if linkParsed.IsAbs() {
			linkPath = linkParsed.Path
		} else {
			base, _ := url.Parse(pageURL)
			resolved := base.ResolveReference(linkParsed)
			linkPath = resolved.Path
		}

		if linkPath == "" || linkPath == "/" || linkPath == urlPath {
			return
		}

		// Check parent index pattern.
		parts := strings.Split(linkPath, "/")
		lastPart := parts[len(parts)-1]
		linkDir := linkPath[:strings.LastIndex(linkPath, "/")+1]
		isParentIndex := parentIndexPattern.MatchString(lastPart) && strings.HasPrefix(urlPath, linkDir)

		if strings.HasPrefix(urlPath, linkPath) || isParentIndex {
			if debug {
				slog.Debug("removeByContentPattern: section breadcrumb", "text", text)
			}
			el.Remove()
		}
	})
}

// removeTrailingExternalLinkLists removes heading + list of off-site links at
// the end of the article.
func removeTrailingExternalLinkLists(mainContent *goquery.Selection, pageURL string, debug bool) {
	if pageURL == "" {
		return
	}
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return
	}
	pageHost := strings.TrimPrefix(parsed.Hostname(), "www.")
	if pageHost == "" {
		return
	}

	mainContent.Find("h2, h3, h4, h5, h6").Each(func(_ int, heading *goquery.Selection) {
		headingNode := heading.Nodes[0]
		if headingNode.Parent == nil {
			return
		}
		list := heading.Next()
		if list.Length() == 0 {
			return
		}
		listTag := strings.ToUpper(list.Nodes[0].Data)
		if listTag != "UL" && listTag != "OL" {
			return
		}

		// Collect LI children.
		var items []*goquery.Selection
		list.Children().Each(func(_ int, c *goquery.Selection) {
			if strings.ToUpper(c.Nodes[0].Data) == "LI" {
				items = append(items, c)
			}
		})
		if len(items) < 2 {
			return
		}

		// Nothing with content must follow at any ancestor level.
		trailingContent := false
		checkEl := list.Nodes[0]
		for checkEl != nil && checkEl != mainContent.Nodes[0] {
			for sib := nextElementSibling(checkEl); sib != nil; sib = nextElementSibling(sib) {
				if strings.TrimSpace(nodeText(sib)) != "" {
					trailingContent = true
					break
				}
			}
			if trailingContent {
				break
			}
			checkEl = checkEl.Parent
		}
		if trailingContent {
			return
		}

		// All items must link off-site.
		allExternal := true
		for _, item := range items {
			links := item.Find("a[href]")
			if links.Length() == 0 {
				allExternal = false
				break
			}
			itemText := strings.TrimSpace(item.Text())
			linkTextLen := 0
			links.Each(func(_ int, a *goquery.Selection) {
				linkTextLen += len(strings.TrimSpace(a.Text()))
				href, _ := a.Attr("href")
				lp, e := url.Parse(href)
				if e == nil && lp.IsAbs() {
					lh := strings.TrimPrefix(lp.Hostname(), "www.")
					if lh == pageHost {
						allExternal = false
					}
				}
			})
			if !allExternal {
				break
			}
			if float64(linkTextLen) < float64(len(itemText))*0.6 {
				allExternal = false
				break
			}
		}
		if !allExternal {
			return
		}

		if debug {
			slog.Debug("removeByContentPattern: trailing external link list", "text", strings.TrimSpace(heading.Text()))
		}
		list.Remove()
		heading.Remove()
	})
}

// removeTrailingRelatedPostsBlock removes a last-child section/div/aside that
// consists entirely of link-dense paragraphs (related posts pattern).
func removeTrailingRelatedPostsBlock(mainContent *goquery.Selection, mainNode *html.Node, debug bool) {
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
		if regexp.MustCompile(`[.!?]`).MatchString(nonLink) {
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

// removeBoilerplateSentences finds boilerplate patterns and truncates from that
// point, removing the matched element and all following siblings at all levels.
func removeBoilerplateSentences(mainContent *goquery.Selection, mainNode *html.Node, debug bool) {
	fullText := strings.TrimSpace(mainContent.Text())

	mainContent.Find("p, div, span, section").Each(func(_ int, el *goquery.Selection) {
		elNode := el.Nodes[0]
		if elNode.Parent == nil {
			return
		}
		if el.Closest("pre, code").Length() > 0 {
			return
		}
		text := strings.TrimSpace(el.Text())
		words := textutil.CountWords(text)
		if words > 50 || words < 1 {
			return
		}

		for _, pattern := range boilerplatePatterns {
			if !pattern.MatchString(text) {
				continue
			}
			// Walk up to a level that has next siblings.
			target := elNode
			for target.Parent != nil && target.Parent != mainNode {
				if nextElementSibling(target) != nil {
					break
				}
				target = target.Parent
			}

			targetText := nodeText(target)
			targetPos := strings.Index(fullText, strings.TrimSpace(targetText))
			if targetPos < 200 {
				// Walk reached high-level wrapper; remove original only if trailing orphan.
				if target != elNode && nextElementSibling(elNode) == nil {
					if debug {
						slog.Debug("removeByContentPattern: boilerplate text", "text", text)
					}
					removeNode(elNode)
				}
				return
			}

			// Collect ancestors before modifying DOM.
			var ancestors []*html.Node
			for anc := target.Parent; anc != nil && anc != mainNode; anc = anc.Parent {
				ancestors = append(ancestors, anc)
			}

			removeTrailingSiblings(target, true, debug)
			for _, anc := range ancestors {
				removeTrailingSiblings(anc, false, debug)
			}
			return
		}
	})
}

// removeRelatedHeadingSections removes sections whose heading text matches
// "related posts", "about the author", etc.
func removeRelatedHeadingSections(mainContent *goquery.Selection, debug bool) {
	mainNode := mainContent.Nodes[0]
	contentText := strings.TrimSpace(mainContent.Text())

	mainContent.Find("h2, h3, h4, h5, h6").Each(func(_ int, heading *goquery.Selection) {
		headingNode := heading.Nodes[0]
		if headingNode.Parent == nil {
			return
		}
		headingText := strings.TrimSpace(heading.Text())
		if !relatedHeadingPattern.MatchString(headingText) {
			return
		}
		if strings.Index(contentText, headingText) < 500 {
			return
		}

		target := walkUpIsolated(headingNode, mainNode)
		if target == headingNode {
			return
		}

		removeThinPrecedingSection(target)
		if debug {
			slog.Debug("removeByContentPattern: related content section", "text", previewNode(target))
		}
		removeTrailingSiblings(target, true, debug)
	})
}

// removeRelatedPostCardGrids removes div containers whose children are
// predominantly image-bearing cards.
func removeRelatedPostCardGrids(mainContent *goquery.Selection, mainNode *html.Node, debug bool) {
	contentText := strings.TrimSpace(mainContent.Text())

	mainContent.Find("div").Each(func(_ int, el *goquery.Selection) {
		elNode := el.Nodes[0]
		if elNode.Parent == nil {
			return
		}
		children := el.Children()
		if children.Length() < 2 {
			return
		}

		cardCount := 0
		children.Each(func(_ int, c *goquery.Selection) {
			hasImg := c.Find("img, picture").Length() > 0
			hasAnchorOrHeading := c.Find("h2, h3, h4, a[href]").Length() > 0
			if hasImg && hasAnchorOrHeading {
				cardCount++
			}
		})

		total := children.Length()
		if cardCount < 2 || float64(cardCount) < float64(total)*0.7 {
			return
		}

		// Must appear after substantial content.
		firstText := strings.TrimSpace(children.First().Text())
		if len(firstText) > 30 {
			firstText = firstText[:30]
		}
		if len(firstText) < 5 || strings.Index(contentText, firstText) < 500 {
			return
		}

		target := walkUpIsolated(elNode, mainNode)
		if target == elNode {
			return
		}

		removeThinPrecedingSection(target)
		if debug {
			slog.Debug("removeByContentPattern: related post cards", "text", previewNode(target))
		}
		removeTrailingSiblings(target, true, debug)
	})
}

// removeNewsletterSections removes newsletter signup sections identified by text.
func removeNewsletterSections(mainContent *goquery.Selection, mainNode *html.Node, debug bool) {
	// div, section, aside — walk up while parent is not significantly larger.
	mainContent.Find("div, section, aside").Each(func(_ int, el *goquery.Selection) {
		elNode := el.Nodes[0]
		if elNode.Parent == nil {
			return
		}
		if el.Closest("pre, code").Length() > 0 {
			return
		}
		if !isNewsletterElement(el, 60) {
			return
		}

		elWords := textutil.CountWords(strings.TrimSpace(el.Text()))
		target := elNode
		for target.Parent != nil && target.Parent != mainNode {
			parentWords := textutil.CountWords(strings.TrimSpace(nodeText(target.Parent)))
			if parentWords > elWords*2+15 {
				break
			}
			target = target.Parent
		}

		if debug {
			slog.Debug("removeByContentPattern: newsletter signup", "text", previewNode(target))
		}
		removeNode(target)
	})

	// <ul> newsletter lists — remove directly without walking up.
	mainContent.Find("ul").Each(func(_ int, el *goquery.Selection) {
		elNode := el.Nodes[0]
		if elNode.Parent == nil {
			return
		}
		if !isNewsletterElement(el, 30) {
			return
		}
		if debug {
			slog.Debug("removeByContentPattern: newsletter signup list", "text", strings.TrimSpace(el.Text()))
		}
		el.Remove()
	})
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
