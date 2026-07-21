// Package elements provides enhanced element processing functionality
// This module handles mathematical formula processing including MathML extraction,
// LaTeX conversion, and math display normalization
package elements

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex for WordPress LaTeX URL extraction.
var wpLatexRe = regexp.MustCompile(`latex\.php\?latex=([^&]+)`)

/*
TypeScript source code (math.core.ts, 68 lines and math.base.ts, 222 lines):

This module provides mathematical formula processing functionality including:
- MathML extraction and normalization
- LaTeX conversion and formatting
- Math display type detection (block vs inline)
- Associated script cleanup
- Mathematical formula standardization

Key TypeScript functions:
- createCleanMathEl(): Creates clean math elements with proper MathML structure
- getMathMLFromElement(): Extracts MathML content from various math libraries
- getBasicLatexFromElement(): Extracts LaTeX content from elements
- isBlockDisplay(): Determines if math should be displayed as block or inline
- mathRules: Transformation rules for math elements
*/

// MathProcessor handles mathematical formula processing and enhancement
// TypeScript original code:
// export const mathRules = [
//
//	{
//	  selector: mathSelectors,
//	  element: 'math',
//	  transform: (el: Element, doc: Document): Element => {
//	    // Processing logic here
//	  }
//	}
//
// ];
type MathProcessor struct {
	doc *goquery.Document
}

// MathData represents extracted mathematical content
// TypeScript original code:
//
//	export interface MathData {
//	  mathml?: string;
//	  latex?: string;
//	  type?: 'katex' | 'mathjax' | 'mathml' | 'latex';
//	  display?: 'block' | 'inline';
//	}
type MathData struct {
	MathML  string `json:"mathml,omitempty"`
	LaTeX   string `json:"latex,omitempty"`
	Type    string `json:"type,omitempty"`
	Display string `json:"display,omitempty"`
}

// MathProcessingOptions contains options for math processing
// TypeScript original code:
//
//	interface MathOptions {
//	  extractMathML?: boolean;
//	  extractLaTeX?: boolean;
//	  cleanupScripts?: boolean;
//	  preserveDisplay?: boolean;
//	}
type MathProcessingOptions struct {
	ExtractMathML   bool
	ExtractLaTeX    bool
	CleanupScripts  bool
	PreserveDisplay bool
}

// DefaultMathProcessingOptions returns default options for math processing
// TypeScript original code:
//
//	const defaultOptions: MathOptions = {
//	  extractMathML: true,
//	  extractLaTeX: true,
//	  cleanupScripts: true,
//	  preserveDisplay: true
//	};
func DefaultMathProcessingOptions() *MathProcessingOptions {
	return &MathProcessingOptions{
		ExtractMathML:   true,
		ExtractLaTeX:    true,
		CleanupScripts:  true,
		PreserveDisplay: true,
	}
}

// NewMathProcessor creates a new math processor
// TypeScript original code:
//
//	class MathProcessor {
//	  constructor(private document: Document) {}
//	}
func NewMathProcessor(doc *goquery.Document) *MathProcessor {
	return &MathProcessor{
		doc: doc,
	}
}

// ProcessMath processes all mathematical formulas in the document
// TypeScript original code:
// export const mathRules = [
//
//	{
//	  selector: mathSelectors,
//	  element: 'math',
//	  transform: (el: Element, doc: Document): Element => {
//	    const mathData = getMathMLFromElement(el);
//	    const latex = getLatexFromElement(el);
//	    const isBlock = isBlockDisplay(el);
//	    const cleanMathEl = createCleanMathEl(doc, mathData, latex, isBlock);
//	    // Cleanup logic...
//	  }
//	}
//
// ];
func (p *MathProcessor) ProcessMath(options *MathProcessingOptions) {
	if options == nil {
		options = DefaultMathProcessingOptions()
	}

	slog.Debug("processing mathematical formulas", "extractMathML", options.ExtractMathML, "extractLaTeX", options.ExtractLaTeX)

	// Math element selectors matching TypeScript mathSelectors
	selectors := []string{
		// WordPress LaTeX images
		`img.latex[src*="latex.php"]`,
		// MathJax elements (v2 and v3)
		"span.MathJax",
		"mjx-container",
		`.MathJax_Preview + script[type="math/tex"]`,
		".MathJax_Display",
		".MathJax_SVG",
		".MathJax_MathML",
		// MediaWiki math elements
		".mwe-math-element",
		".mwe-math-fallback-image-inline",
		".mwe-math-fallback-image-display",
		".mwe-math-mathml-inline",
		".mwe-math-mathml-display",
		// KaTeX elements
		".katex",
		".katex-display",
		".katex-mathml",
		".katex-html",
		"[data-katex]",
		`script[type="math/katex"]`,
		// Generic math elements
		"math",
		"[data-math]",
		"[data-latex]",
		"[data-tex]",
		`script[type^="math/"]`,
		`annotation[encoding="application/x-tex"]`,
	}

	combinedSelector := strings.Join(selectors, ", ")
	slog.Debug("using math selector", "selector", combinedSelector)

	var processedCount int
	p.doc.Find(combinedSelector).Each(func(_ int, s *goquery.Selection) {
		p.processMathElement(s, options)
		processedCount++
	})

	slog.Info("mathematical formulas processed", "count", processedCount)
}
