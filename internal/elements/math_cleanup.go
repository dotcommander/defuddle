package elements

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
// // Check for text content that looks like LaTeX
// const textContent = el.textContent?.trim() || ”;
//
//	if (textContent.includes('$') || textContent.includes('\\')) {
//	  return textContent;
//	}
//
// looksLikeLaTeX checks if text content looks like LaTeX
func (p *MathProcessor) looksLikeLaTeX(text string) bool {
	if text == "" {
		return false
	}

	// Basic LaTeX patterns
	latexPatterns := []string{
		`\$.*\$`,                 // Dollar signs
		`\\\w+`,                  // Backslash commands
		`\{.*\}`,                 // Braces
		`\^`,                     // Superscript
		`_`,                      // Subscript
		`\\frac`,                 // Fractions
		`\\sum`,                  // Summation
		`\\int`,                  // Integrals
		`\\alpha|\\beta|\\gamma`, // Greek letters
	}

	for _, pattern := range latexPatterns {
		if matched, _ := regexp.MatchString(pattern, text); matched {
			return true
		}
	}

	return false
}

// ProcessMath processes all mathematical formulas in the document (public interface)
// TypeScript original code:
//
//	export function processMath(doc: Document, options?: MathOptions): void {
//	  const processor = new MathProcessor(doc);
//	  processor.processAllMath(options || defaultOptions);
//	}
func ProcessMath(doc *goquery.Document, options *MathProcessingOptions) {
	processor := NewMathProcessor(doc)
	processor.ProcessMath(options)
}

// ProcessMathInScope processes mathematical formulas within the given container element.
func ProcessMathInScope(scope *goquery.Selection, options *MathProcessingOptions) {
	processor := &MathProcessor{}
	if options == nil {
		options = DefaultMathProcessingOptions()
	}
	combinedSelector := strings.Join([]string{
		"math",
		".MathJax",
		".MathJax_Display",
		".MathJax_Preview",
		".katex",
		".katex-display",
		".katex-block",
		`script[type^="math/"]`,
		`script[type="application/x-tex"]`,
		`script[type="text/latex"]`,
		"[data-math]",
		"[data-latex]",
		"[data-katex]",
		"[data-mathjax]",
	}, ", ")
	scope.Find(combinedSelector).Each(func(_ int, s *goquery.Selection) {
		processor.processMathElement(s, options)
	})
}
