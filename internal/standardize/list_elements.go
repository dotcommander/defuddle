package standardize

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var orderedListLabelRe = regexp.MustCompile(`^\d+\)`)

// transformListElement converts div[role="list"] to actual lists with recursive nested handling.
// JavaScript original code: (complex transform function from ELEMENT_STANDARDIZATION_RULES)
func transformListElement(el *goquery.Selection, doc *goquery.Document) *goquery.Selection {
	if el.Length() == 0 {
		return nil
	}
	listNode := el.Get(0)

	// Collect only listitems that directly belong to this list (not nested lists).
	var directItems []*goquery.Selection
	el.Find(`div[role="listitem"]`).Each(func(_ int, item *goquery.Selection) {
		closestList := item.ParentsFiltered(`div[role="list"]`).First()
		if closestList.Length() > 0 && closestList.Get(0) == listNode {
			directItems = append(directItems, item)
		}
	})

	// Determine if this is an ordered list from the first direct item's label.
	isOrdered := false
	if len(directItems) > 0 {
		firstLabel := directItems[0].Find(".label").First()
		label := strings.TrimSpace(firstLabel.Text())
		isOrdered = orderedListLabelRe.MatchString(label)
	}

	listTag := "ul"
	if isOrdered {
		listTag = "ol"
	}

	var b strings.Builder
	b.WriteString("<" + listTag + ">")
	for _, item := range directItems {
		b.WriteString("<li>")
		b.WriteString(renderListItemInnerHTML(item, doc))
		b.WriteString("</li>")
	}
	b.WriteString("</" + listTag + ">")

	return parseDetachedElement(b.String(), listTag)
}

// transformListItemElement converts div[role="listitem"] to an li element.
// JavaScript original code: (transform function for listitem)
func transformListItemElement(el *goquery.Selection, doc *goquery.Document) *goquery.Selection {
	if el.Length() == 0 {
		return nil
	}
	liHTML := "<li>" + renderListItemInnerHTML(el, doc) + "</li>"
	return parseDetachedElement(liHTML, "li")
}

// renderListItemInnerHTML extracts and normalizes the inner HTML of a div[role="listitem"],
// preferring its own .content child and falling back to the listitem's own HTML.
func renderListItemInnerHTML(item *goquery.Selection, doc *goquery.Document) string {
	itemNode := item.Get(0)
	target := item

	// Use .content only if it belongs directly to this listitem (not a nested one).
	item.Find(".content").EachWithBreak(func(_ int, c *goquery.Selection) bool {
		closestItem := c.ParentsFiltered(`div[role="listitem"]`).First()
		if closestItem.Length() > 0 && closestItem.Get(0) == itemNode {
			target = c
			return false
		}
		return true
	})

	// Convert direct nested lists recursively before extracting HTML.
	target.Find(`div[role="list"]`).Each(func(_ int, nestedList *goquery.Selection) {
		if nestedList.Length() == 0 || nestedList.Get(0).Parent == nil {
			return
		}
		// Only transform nested lists whose closest enclosing listitem is this item.
		closestItem := nestedList.ParentsFiltered(`div[role="listitem"]`).First()
		if closestItem.Length() > 0 && closestItem.Get(0) != itemNode {
			return
		}
		converted := transformListElement(nestedList, doc)
		if converted != nil && converted.Length() > 0 {
			if nestedHTML, err := goquery.OuterHtml(converted); err == nil {
				nestedList.ReplaceWithHtml(nestedHTML)
			}
		}
	})

	// Convert paragraph divs inside target.
	target.Find(`div[role="paragraph"]`).Each(func(_ int, div *goquery.Selection) {
		pHTML, _ := div.Html()
		div.ReplaceWithHtml("<p>" + pHTML + "</p>")
	})

	innerHTML, _ := target.Html()
	return innerHTML
}

// parseDetachedElement parses a single HTML fragment into a detached Selection
// without mutating the caller's document body.
func parseDetachedElement(fragment, tag string) *goquery.Selection {
	tmpDoc, err := goquery.NewDocumentFromReader(strings.NewReader(fragment))
	if err != nil {
		return nil
	}
	return tmpDoc.Find(tag).First()
}
