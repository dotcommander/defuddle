package removals

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	textutil "github.com/dotcommander/defuddle/internal/text"
)

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
		if sentencePunctSpaceRe.MatchString(text) {
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
