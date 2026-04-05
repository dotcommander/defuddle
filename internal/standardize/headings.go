package standardize

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/internal/constants"
)

var (
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// standardizeHeadings handles H1 elements and converts them appropriately
// JavaScript original code:
//
//	function standardizeHeadings(element: Element, title: string, doc: Document): void {
//		const normalizeText = (text: string): string => {
//			return text
//				.replace(/\u00A0/g, ' ') // Convert non-breaking spaces to regular spaces
//				.replace(/\s+/g, ' ') // Normalize all whitespace to single spaces
//				.trim()
//				.toLowerCase();
//		};
//
//		const h1s = element.getElementsByTagName('h1');
//
//		Array.from(h1s).forEach(h1 => {
//			const h2 = doc.createElement('h2');
//			h2.innerHTML = h1.innerHTML;
//			// Copy allowed attributes
//			Array.from(h1.attributes).forEach(attr => {
//				if (ALLOWED_ATTRIBUTES.has(attr.name)) {
//					h2.setAttribute(attr.name, attr.value);
//				}
//			});
//			h1.parentNode?.replaceChild(h2, h1);
//		});
//
//		// Remove first H2 if it matches title
//		const h2s = element.getElementsByTagName('h2');
//		if (h2s.length > 0) {
//			const firstH2 = h2s[0];
//			const firstH2Text = normalizeText(firstH2.textContent || '');
//			const normalizedTitle = normalizeText(title);
//			if (normalizedTitle && normalizedTitle === firstH2Text) {
//				firstH2.remove();
//			}
//		}
//	}
func standardizeHeadings(element *goquery.Selection, title string, _ *goquery.Document) {
	normalizeText := func(text string) string {
		// Convert non-breaking spaces to regular spaces
		text = strings.ReplaceAll(text, "\u00A0", " ")
		// Normalize all whitespace to single spaces
		text = whitespaceRe.ReplaceAllString(text, " ")
		// Trim and convert to lowercase
		return strings.ToLower(strings.TrimSpace(text))
	}

	// Convert all H1s to H2s
	element.Find("h1").Each(func(_ int, h1 *goquery.Selection) {
		html, _ := h1.Html()

		// Create new H2 element
		var newH2 strings.Builder
		newH2.WriteString("<h2")

		// Copy allowed attributes
		if h1.Length() > 0 {
			node := h1.Get(0)
			for _, attr := range node.Attr {
				if constants.IsAllowedAttribute(attr.Key) {
					newH2.WriteString(` ` + attr.Key + `="` + attr.Val + `"`)
				}
			}
		}

		newH2.WriteString(">" + html + "</h2>")
		h1.ReplaceWithHtml(newH2.String())
	})

	// Remove first H2 if it matches title
	firstH2 := element.Find("h2").First()
	if firstH2.Length() > 0 {
		firstH2Text := normalizeText(firstH2.Text())
		normalizedTitle := normalizeText(title)
		if normalizedTitle != "" && normalizedTitle == firstH2Text {
			firstH2.Remove()
		}
	}
}

// standardizeFootnotes processes footnotes and citations
// JavaScript original code:
//
//	export function standardizeFootnotes(element: Element): void {
//		// Remove footnote back-references
//		const backRefs = element.querySelectorAll(FOOTNOTE_BACK_REFERENCES);
//		backRefs.forEach(ref => ref.remove());
//
//		// Process inline footnote references
//		const inlineRefs = element.querySelectorAll(FOOTNOTE_INLINE_REFERENCES);
//		inlineRefs.forEach(ref => {
//			// Convert to superscript if not already
//			if (ref.tagName.toLowerCase() !== 'sup') {
//				const sup = ref.ownerDocument.createElement('sup');
//				sup.innerHTML = ref.innerHTML;
//				ref.replaceWith(sup);
//			}
//		});
//	}
func standardizeFootnotes(element *goquery.Selection) {
	// Remove footnote back-references
	backRefSelectors := []string{
		`a[href^="#"][class*="anchor"]`,
		`a[href^="#"][class*="ref"]`,
		`a[class*="footnote-backref"]`,
		`.footnote-backref`,
	}

	for _, selector := range backRefSelectors {
		element.Find(selector).Remove()
	}

	// Process inline footnote references
	footnoteSelectors := constants.GetFootnoteInlineReferences()
	for _, selector := range footnoteSelectors {
		element.Find(selector).Each(func(_ int, ref *goquery.Selection) {
			// Convert to superscript if not already
			if goquery.NodeName(ref) != "sup" {
				html, _ := ref.Html()
				ref.ReplaceWithHtml("<sup>" + html + "</sup>")
			}
		})
	}
}
