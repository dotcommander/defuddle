package removals

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	textutil "github.com/dotcommander/defuddle/internal/text"
)

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
