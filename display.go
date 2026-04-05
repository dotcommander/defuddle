package defuddle

import (
	"log/slog"
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

	// Check inline styles for hidden elements
	doc.Find("*").Each(func(_ int, element *goquery.Selection) {
		style, exists := element.Attr("style")
		if !exists {
			return
		}

		lowerStyle := strings.ToLower(style)
		if strings.Contains(lowerStyle, "display:none") ||
			strings.Contains(lowerStyle, "display: none") ||
			strings.Contains(lowerStyle, "visibility:hidden") ||
			strings.Contains(lowerStyle, "visibility: hidden") ||
			strings.Contains(lowerStyle, "opacity:0") ||
			strings.Contains(lowerStyle, "opacity: 0") {
			element.Remove()
			count++
		}
	})

	if d.debug {
		slog.Debug("Removed hidden elements", "count", count)
	}
}
