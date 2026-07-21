package elements

import (
	"log/slog"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
// // Clean up any associated math scripts after we've extracted their content
//
//	if (el.parentElement) {
//	  // Remove all math-related scripts and previews
//	  const mathElements = el.parentElement.querySelectorAll(`
//	    /* MathJax scripts and previews */
//	    script[type^="math/"],
//	    .MathJax_Preview,
//
//	    /* External math library scripts */
//	    script[type="text/javascript"][src*="mathjax"],
//	    script[type="text/javascript"][src*="katex"]
//	  `);
//	  mathElements.forEach(el => el.remove());
//	}
//
// cleanupMathScripts removes associated math scripts and previews
func (p *MathProcessor) cleanupMathScripts(parent *goquery.Selection) {
	if parent.Length() == 0 {
		return
	}

	// Remove MathJax scripts and previews
	scriptsToRemove := []string{
		"script[type^=\"math/\"]",
		".MathJax_Preview",
		"script[type=\"text/javascript\"][src*=\"mathjax\"]",
		"script[type=\"text/javascript\"][src*=\"katex\"]",
	}

	var removedCount int
	for _, selector := range scriptsToRemove {
		elements := parent.Find(selector)
		removedCount += elements.Length()
		elements.Remove()
	}

	if removedCount > 0 {
		slog.Debug("cleaned up math scripts", "removedCount", removedCount)
	}
}
