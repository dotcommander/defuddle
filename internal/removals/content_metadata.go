package removals

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	textutil "github.com/dotcommander/defuddle/internal/text"
)

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
		if tag == "DIV" && words >= 1 && words <= 10 && hasDate && !metadataLabelPattern.MatchString(text) && !sentencePunctRe.MatchString(text) && getPos() <= 400 {
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
		if !bylineFound && startsByPattern.MatchString(text) && words >= 2 && !sentencePunctEndRe.MatchString(text) && getPos() <= 600 {
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
			cleaned = metadataStripWeekday.ReplaceAllString(cleaned, "")
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
		if !authorDateFound && words >= 2 && words <= 10 && hasDate && !metadataLabelPattern.MatchString(text) && getPos() <= 500 {
			residual := metadataStripMonth.ReplaceAllString(text, "")
			residual = metadataStripWeekday.ReplaceAllString(residual, "")
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
			if textutil.CountWords(t) > 8 || sentencePunctEndRe.MatchString(t) {
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
