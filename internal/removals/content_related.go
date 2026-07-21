package removals

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	textutil "github.com/dotcommander/defuddle/internal/text"
)

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
