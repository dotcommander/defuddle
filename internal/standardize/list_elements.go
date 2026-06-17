package standardize

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var orderedListLabelRe = regexp.MustCompile(`^\d+\)`)

// transformListElement converts div[role="list"] to actual lists with complex nested handling
// JavaScript original code: (complex transform function from ELEMENT_STANDARDIZATION_RULES)
func transformListElement(el *goquery.Selection, doc *goquery.Document) *goquery.Selection {
	// First determine if this is an ordered list
	firstItem := el.Find(`div[role="listitem"] .label`).First()
	label := strings.TrimSpace(firstItem.Text())
	isOrdered := orderedListLabelRe.MatchString(label)

	// Create the appropriate list type
	listTag := "ul"
	if isOrdered {
		listTag = "ol"
	}

	// Create new list element
	newList := doc.Find("body").AppendHtml("<" + listTag + "></" + listTag + ">").Find(listTag).Last()

	// Process each list item
	el.Find(`div[role="listitem"]`).Each(func(_ int, item *goquery.Selection) {
		li := doc.Find("body").AppendHtml("<li></li>").Find("li").Last()
		content := item.Find(".content").First()

		if content.Length() > 0 {
			// Convert any paragraph divs inside content
			content.Find(`div[role="paragraph"]`).Each(func(_ int, div *goquery.Selection) {
				pHTML, _ := div.Html()
				div.ReplaceWithHtml("<p>" + pHTML + "</p>")
			})

			// Convert any nested lists recursively
			content.Find(`div[role="list"]`).Each(func(_ int, nestedList *goquery.Selection) {
				firstNestedItem := nestedList.Find(`div[role="listitem"] .label`).First()
				nestedLabel := strings.TrimSpace(firstNestedItem.Text())
				isNestedOrdered := orderedListLabelRe.MatchString(nestedLabel)

				nestedListTag := "ul"
				if isNestedOrdered {
					nestedListTag = "ol"
				}

				newNestedList := doc.Find("body").AppendHtml("<" + nestedListTag + "></" + nestedListTag + ">").Find(nestedListTag).Last()

				// Process nested items
				nestedList.Find(`div[role="listitem"]`).Each(func(_ int, nestedItem *goquery.Selection) {
					nestedLi := doc.Find("body").AppendHtml("<li></li>").Find("li").Last()
					nestedContent := nestedItem.Find(".content").First()

					if nestedContent.Length() > 0 {
						// Convert paragraph divs in nested items
						nestedContent.Find(`div[role="paragraph"]`).Each(func(_ int, div *goquery.Selection) {
							pHTML, _ := div.Html()
							div.ReplaceWithHtml("<p>" + pHTML + "</p>")
						})
						contentHTML, _ := nestedContent.Html()
						nestedLi.SetHtml(contentHTML)
					}

					newNestedList.AppendSelection(nestedLi)
				})

				nestedList.ReplaceWithSelection(newNestedList)
			})

			contentHTML, _ := content.Html()
			li.SetHtml(contentHTML)
		}

		newList.AppendSelection(li)
	})

	return newList
}

// transformListItemElement converts div[role="listitem"] to li elements
// JavaScript original code: (transform function for listitem)
func transformListItemElement(el *goquery.Selection, _ *goquery.Document) *goquery.Selection {
	content := el.Find(".content").First()
	if content.Length() == 0 {
		return el
	}

	// Convert any paragraph divs inside content
	content.Find(`div[role="paragraph"]`).Each(func(_ int, div *goquery.Selection) {
		pHTML, _ := div.Html()
		div.ReplaceWithHtml("<p>" + pHTML + "</p>")
	})

	return content
}
