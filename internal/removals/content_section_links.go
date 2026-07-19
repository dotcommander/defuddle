package removals

import (
	"log/slog"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	textutil "github.com/dotcommander/defuddle/internal/text"
)

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
	pageHost := parsed.Hostname()
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
					if sameRegisteredDomain(lp.Hostname(), pageHost) {
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
