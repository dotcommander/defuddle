package elements

import (
	"log/slog"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
//
//	transform: (el: Element, doc: Document): Element => {
//	  if (!hasHTMLElementProps(el)) return el;
//
//	  const mathData = getMathMLFromElement(el);
//	  const latex = getLatexFromElement(el);
//	  const isBlock = isBlockDisplay(el);
//	  const cleanMathEl = createCleanMathEl(doc, mathData, latex, isBlock);
//
//	  // Clean up any associated math scripts after we've extracted their content
//	  if (el.parentElement) {
//	    // Remove all math-related scripts and previews
//	    const mathElements = el.parentElement.querySelectorAll(`
//	      script[type^="math/"],
//	      .MathJax_Preview,
//	      script[type="text/javascript"][src*="mathjax"],
//	      script[type="text/javascript"][src*="katex"]
//	    `);
//	    mathElements.forEach(el => el.remove());
//	  }
//
//	  return cleanMathEl;
//	}
//
// processMathElement processes a single mathematical element
func (p *MathProcessor) processMathElement(s *goquery.Selection, options *MathProcessingOptions) {
	slog.Debug("processing individual math element", "tag", goquery.NodeName(s))

	// Extract mathematical content
	var mathData *MathData
	if options.ExtractMathML {
		mathData = p.getMathMLFromElement(s)
	}

	var latex string
	if options.ExtractLaTeX {
		latex = p.getLaTeXFromElement(s)
	}

	// Determine display type
	isBlock := false
	if options.PreserveDisplay {
		isBlock = p.isBlockDisplay(s)
	}

	// Create clean math element
	cleanMathHTML := p.createCleanMathElement(mathData, latex, isBlock)

	// Replace original element
	s.ReplaceWithHtml(cleanMathHTML)

	// Clean up associated scripts
	if options.CleanupScripts {
		p.cleanupMathScripts(s.Parent())
	}

	slog.Debug("processed math element", "hasLaTeX", latex != "", "hasMathML", mathData != nil && mathData.MathML != "", "isBlock", isBlock)
}
