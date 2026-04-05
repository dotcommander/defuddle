package defuddle

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// StyleChange represents a CSS style change for mobile
type StyleChange struct {
	Selector string
	Styles   string
}

// evaluateMediaQueries evaluates mobile styles from CSS media queries
// JavaScript original code:
//
//	private _evaluateMediaQueries(doc: Document): StyleChange[] {
//	  // ... (processes CSS stylesheets and media queries)
//	}
func (d *Defuddle) evaluateMediaQueries() []StyleChange {
	// Note: In Go/server environment, we don't have access to CSS stylesheets
	// This is a placeholder for future implementation if needed
	// Most content extraction doesn't require CSS evaluation
	return []StyleChange{}
}

// applyMobileStyles applies mobile styles to the document
// JavaScript original code:
//
//	private applyMobileStyles(doc: Document, mobileStyles: StyleChange[]) {
//	  let appliedCount = 0;
//	  mobileStyles.forEach(({selector, styles}) => {
//	    try {
//	      const elements = doc.querySelectorAll(selector);
//	      elements.forEach(element => {
//	        element.setAttribute('style',
//	          (element.getAttribute('style') || '') + styles
//	        );
//	        appliedCount++;
//	      });
//	    } catch (e) {
//	      console.error('Defuddle', 'Error applying styles for selector:', selector, e);
//	    }
//	  });
//	}
func (d *Defuddle) applyMobileStyles(doc *goquery.Document, mobileStyles []StyleChange) {
	appliedCount := 0

	for _, change := range mobileStyles {
		doc.Find(change.Selector).Each(func(_ int, element *goquery.Selection) {
			existingStyle, _ := element.Attr("style")
			newStyle := existingStyle + change.Styles
			element.SetAttr("style", newStyle)
			appliedCount++
		})
	}

	if d.debug {
		slog.Debug("Applied mobile styles", "count", appliedCount)
	}
}

// removeHiddenElements removes elements that are hidden via CSS
// JavaScript original code:
//
//	private removeHiddenElements(doc: Document) {
//	  // ... (checks computed styles for display:none, visibility:hidden, opacity:0)
//	}
func (d *Defuddle) removeHiddenElements(doc *goquery.Document) {
	count := 0
	var toRemove []*goquery.Selection

	doc.Find("*").Each(func(_ int, element *goquery.Selection) {
		// Protect math elements from removal
		tag := goquery.NodeName(element)
		if tag == "math" {
			return
		}
		if _, hasMathML := element.Attr("data-mathml"); hasMathML {
			return
		}
		className := element.AttrOr("class", "")
		if strings.Contains(className, "katex-mathml") || strings.Contains(className, "MathJax") {
			return
		}

		// Check inline styles for hidden elements
		if style, exists := element.Attr("style"); exists {
			lowerStyle := strings.ToLower(style)
			if strings.Contains(lowerStyle, "display:none") ||
				strings.Contains(lowerStyle, "display: none") ||
				strings.Contains(lowerStyle, "visibility:hidden") ||
				strings.Contains(lowerStyle, "visibility: hidden") ||
				strings.Contains(lowerStyle, "opacity:0") ||
				strings.Contains(lowerStyle, "opacity: 0") {
				toRemove = append(toRemove, element)
				count++
				return
			}
		}

		// Check class tokens for Tailwind/utility hidden classes
		if className != "" {
			for _, token := range strings.Fields(className) {
				// Exact matches: "hidden", "invisible"
				if token == "hidden" || token == "invisible" {
					toRemove = append(toRemove, element)
					count++
					return
				}
				// Responsive variants: "sm:hidden", "md:hidden", "lg:hidden", etc.
				// Also matches arbitrary prefix:hidden and prefix:invisible
				if strings.HasSuffix(token, ":hidden") || strings.HasSuffix(token, ":invisible") {
					toRemove = append(toRemove, element)
					count++
					return
				}
			}
		}
	})

	for _, el := range toRemove {
		el.Remove()
	}

	if d.debug {
		slog.Debug("Removed hidden elements", "count", count)
	}
}

// resolveReactStreaming resolves React SSR streaming placeholders.
// React's streaming SSR emits <template id="B:X"> as Suspense boundaries,
// then later provides content in <div hidden id="S:X"> with a $RC("B:X","S:X") call.
// This function swaps the templates with their resolved content.
var reactRCPattern = regexp.MustCompile(`\$RC\(\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']\s*\)`)

func resolveReactStreaming(doc *goquery.Document) {
	// Find $RC calls in inline scripts
	doc.Find("script").Each(func(_ int, script *goquery.Selection) {
		text := script.Text()
		matches := reactRCPattern.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			boundaryID, slotID := m[1], m[2]

			boundary := doc.Find(`template[id="` + boundaryID + `"]`)
			slot := doc.Find(`[id="` + slotID + `"]`)

			if boundary.Length() > 0 && slot.Length() > 0 {
				slotHTML, _ := slot.Html()
				if slotHTML != "" {
					boundary.ReplaceWithHtml(slotHTML)
					slot.Remove()
				}
			}
		}
	})
}

// flattenShadowDOM inlines declarative Shadow DOM templates into the main document.
// Browsers use <template shadowrootmode="open"> for SSR shadow DOM; the content
// inside is invisible to goquery without flattening.
func flattenShadowDOM(doc *goquery.Document) {
	doc.Find(`template[shadowrootmode], template[shadowroot]`).Each(func(_ int, tmpl *goquery.Selection) {
		parent := tmpl.Parent()
		if parent.Length() == 0 {
			return
		}
		// Move template children into the parent, replacing the template
		inner, _ := tmpl.Html()
		if inner != "" {
			tmpl.ReplaceWithHtml(inner)
		} else {
			tmpl.Remove()
		}
	})
}
