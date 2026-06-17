package standardize

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
)

// StandardizationRule represents element standardization rules
// JavaScript original code:
//
//	interface StandardizationRule {
//		selector: string;
//		element: string;
//		transform?: (el: Element, doc: Document) => Element;
//	}
type StandardizationRule struct {
	Selector  string
	Element   string
	Transform func(el *goquery.Selection, doc *goquery.Document) *goquery.Selection
}

// ELEMENT_STANDARDIZATION_RULES maps selectors to their target HTML element name
// JavaScript original code:
//
//	const ELEMENT_STANDARDIZATION_RULES: StandardizationRule[] = [
//		...mathRules,
//		...codeBlockRules,
//		...headingRules,
//		...imageRules,
//		// Convert divs with paragraph role to actual paragraphs
//		{
//			selector: 'div[data-testid^="paragraph"], div[role="paragraph"]',
//			element: 'p',
//			transform: (el: Element, doc: Document): Element => { ... }
//		},
//		// Convert divs with list roles to actual lists
//		{
//			selector: 'div[role="list"]',
//			element: 'ul',
//			transform: (el: Element, doc: Document): Element => { ... }
//		},
//		{
//			selector: 'div[role="listitem"]',
//			element: 'li',
//			transform: (el: Element, doc: Document): Element => { ... }
//		}
//	];
var elementStandardizationRules = []StandardizationRule{
	// Convert callout asides to blockquotes with data-callout attribute
	{
		Selector: `aside[class*="callout"]`,
		Element:  "blockquote",
		Transform: func(el *goquery.Selection, _ *goquery.Document) *goquery.Selection {
			// Extract callout type from class (e.g., "callout-tip" → "tip")
			calloutType := "note"
			if class, exists := el.Attr("class"); exists {
				for _, c := range strings.Fields(class) {
					if strings.HasPrefix(c, "callout-") {
						calloutType = strings.TrimPrefix(c, "callout-")
						break
					}
				}
			}

			// Get content from .callout-content div, or fall back to whole aside
			content := el.Find(".callout-content").First()
			var inner string
			if content.Length() > 0 {
				inner, _ = content.Html()
			} else {
				inner, _ = el.Html()
			}

			el.ReplaceWithHtml(`<blockquote data-callout="` + calloutType + `">` + inner + `</blockquote>`)
			return nil
		},
	},
	// Convert divs with paragraph role to actual paragraphs
	{
		Selector: `div[data-testid^="paragraph"], div[role="paragraph"]`,
		Element:  "p",
		Transform: func(el *goquery.Selection, _ *goquery.Document) *goquery.Selection {
			// Get the inner HTML and attributes
			html, _ := el.Html()

			// Build new paragraph HTML
			var newHTML strings.Builder
			newHTML.WriteString("<p")

			// Copy allowed attributes (except role)
			if el.Length() > 0 {
				node := el.Get(0)
				for _, attr := range node.Attr {
					if constants.IsAllowedAttribute(attr.Key) && attr.Key != "role" {
						newHTML.WriteString(` ` + attr.Key + `="` + attr.Val + `"`)
					}
				}
			}

			newHTML.WriteString(">" + html + "</p>")

			// Replace the element with the new HTML
			el.ReplaceWithHtml(newHTML.String())

			// Return nil to indicate we handled the replacement
			return nil
		},
	},
	// Convert divs with list roles to actual lists
	{
		Selector:  `div[role="list"]`,
		Element:   "ul",
		Transform: transformListElement,
	},
	{
		Selector:  `div[role="listitem"]`,
		Element:   "li",
		Transform: transformListItemElement,
	},
}

// standardizeElements converts embedded content to standard formats
// JavaScript original code:
//
//	function standardizeElements(element: Element, doc: Document): void {
//		ELEMENT_STANDARDIZATION_RULES.forEach(rule => {
//			const elements = element.querySelectorAll(rule.selector);
//			elements.forEach(el => {
//				try {
//					let newElement: Element;
//					if (rule.transform) {
//						newElement = rule.transform(el, doc);
//					} else {
//						newElement = doc.createElement(rule.element);
//						newElement.innerHTML = el.innerHTML;
//
//						// Copy allowed attributes
//						Array.from(el.attributes).forEach(attr => {
//							if (ALLOWED_ATTRIBUTES.has(attr.name)) {
//								newElement.setAttribute(attr.name, attr.value);
//							}
//						});
//					}
//
//					el.replaceWith(newElement);
//				} catch (e) {
//					console.warn('Failed to standardize element:', e);
//				}
//			});
//		});
//	}
func standardizeElements(element *goquery.Selection, doc *goquery.Document, debug bool) {
	processedCount := 0

	// Process each standardization rule
	for _, rule := range elementStandardizationRules {
		element.Find(rule.Selector).Each(func(_ int, el *goquery.Selection) {
			if rule.Transform != nil {
				// Use custom transform function
				newElement := rule.Transform(el, doc)
				if newElement != nil && newElement.Length() > 0 {
					// Get the HTML of the new element and replace
					newHTML, err := goquery.OuterHtml(newElement)
					if err == nil {
						el.ReplaceWithHtml(newHTML)
						processedCount++
					}
				}
			} else {
				// Default transformation
				html, _ := el.Html()
				var newElementHTML strings.Builder
				newElementHTML.WriteString("<" + rule.Element)

				// Copy allowed attributes
				writeAllowedAttributes(&newElementHTML, el)

				newElementHTML.WriteString(">" + html + "</" + rule.Element + ">")
				el.ReplaceWithHtml(newElementHTML.String())
				processedCount++
			}
		})
	}

	// Convert lite-youtube elements
	element.Find("lite-youtube").Each(func(_ int, el *goquery.Selection) {
		videoID, exists := el.Attr("videoid")
		if !exists || videoID == "" {
			return
		}

		videoTitle, _ := el.Attr("videotitle")
		if videoTitle == "" {
			videoTitle = "YouTube video player"
		}

		iframeHTML := `<iframe width="560" height="315" ` +
			`src="https://www.youtube.com/embed/` + videoID + `" ` +
			`title="` + videoTitle + `" ` +
			`frameborder="0" ` +
			`allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" ` +
			`allowfullscreen></iframe>`

		el.ReplaceWithHtml(iframeHTML)
		processedCount++
	})

	if debug {
		slog.Debug("Converted embedded elements", "count", processedCount)
	}
}

// writeAllowedAttributes appends ` key="val"` to b for each allow-listed attribute
// on the first node of sel. Shared by element and heading standardization, which both
// rebuild a tag and must copy only allow-listed attributes.
func writeAllowedAttributes(b *strings.Builder, sel *goquery.Selection) {
	if sel.Length() == 0 {
		return
	}
	node := sel.Get(0)
	for _, attr := range node.Attr {
		if constants.IsAllowedAttribute(attr.Key) {
			b.WriteString(` ` + attr.Key + `="` + attr.Val + `"`)
		}
	}
}
