// Package elements provides enhanced element processing functionality
// This module handles mathematical formula processing including MathML extraction,
// LaTeX conversion, and math display normalization
package elements

import (
	"log/slog"
	"net/url"
	"regexp"
	"slices"
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

// processMathElement processes a single mathematical element
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

// getMathMLFromElement extracts MathML content from element
// TypeScript original code:
//
//	export const getMathMLFromElement = (el: Element): MathData | null => {
//	  // Try to extract MathML from various math libraries
//	  const mathElement = el.querySelector('math');
//	  if (mathElement) {
//	    return {
//	      mathml: mathElement.outerHTML,
//	      type: 'mathml',
//	      display: mathElement.getAttribute('display') || 'inline'
//	    };
//	  }
//
//	  // Check for KaTeX
//	  if (el.classList?.contains('katex')) {
//	    const annotation = el.querySelector('annotation[encoding="application/x-tex"]');
//	    if (annotation) {
//	      return {
//	        latex: annotation.textContent?.trim() || '',
//	        type: 'katex'
//	      };
//	    }
//	  }
//
//	  // Check for MathJax
//	  if (el.classList?.contains('MathJax')) {
//	    const script = el.querySelector('script[type^="math/"]');
//	    if (script) {
//	      return {
//	        latex: script.textContent?.trim() || '',
//	        type: 'mathjax'
//	      };
//	    }
//	  }
//
//	  return null;
//	};
func (p *MathProcessor) getMathMLFromElement(s *goquery.Selection) *MathData {
	// 1. Try to extract MathML directly
	mathElement := s.Find("math").First()
	if mathElement.Length() > 0 {
		outerHTML, err := goquery.OuterHtml(mathElement)
		if err == nil {
			display := mathElement.AttrOr("display", "inline")
			return &MathData{
				MathML:  outerHTML,
				Type:    "mathml",
				Display: display,
			}
		}
	}

	// 2. MathJax v2: data-mathml attribute
	if mathmlStr, exists := s.Attr("data-mathml"); exists && mathmlStr != "" {
		tempDoc, err := goquery.NewDocumentFromReader(strings.NewReader(mathmlStr))
		if err == nil {
			mathEl := tempDoc.Find("math").First()
			if mathEl.Length() > 0 {
				outerHTML, _ := goquery.OuterHtml(mathEl)
				display := mathEl.AttrOr("display", "inline")
				return &MathData{
					MathML:  outerHTML,
					Type:    "mathjax",
					Display: display,
				}
			}
		}
	}

	// 3. MathJax v3: assistive MathML (.MJX_Assistive_MathML, mjx-assistive-mml)
	assistive := s.Find(".MJX_Assistive_MathML, mjx-assistive-mml").First()
	if assistive.Length() > 0 {
		mathEl := assistive.Find("math").First()
		if mathEl.Length() > 0 {
			outerHTML, _ := goquery.OuterHtml(mathEl)
			display := mathEl.AttrOr("display", "inline")
			return &MathData{
				MathML:  outerHTML,
				Type:    "mathjax",
				Display: display,
			}
		}
	}

	// 4. Check for KaTeX
	if s.HasClass("katex") {
		annotation := s.Find("annotation[encoding=\"application/x-tex\"]").First()
		if annotation.Length() > 0 {
			latex := strings.TrimSpace(annotation.Text())
			return &MathData{
				LaTeX: latex,
				Type:  "katex",
			}
		}
	}

	// 5. Check for MathJax script
	if s.HasClass("MathJax") {
		script := s.Find("script[type^=\"math/\"]").First()
		if script.Length() > 0 {
			latex := strings.TrimSpace(script.Text())
			return &MathData{
				LaTeX: latex,
				Type:  "mathjax",
			}
		}
	}

	return nil
}

// getLaTeXFromElement extracts LaTeX content from element
// TypeScript original code:
//
//	export const getBasicLatexFromElement = (el: Element): string | null => {
//	  // Check for data attributes
//	  const dataLatex = el.getAttribute('data-latex') || el.getAttribute('data-tex');
//	  if (dataLatex) {
//	    return dataLatex;
//	  }
//
//	  // Check for script elements with LaTeX content
//	  const scripts = el.querySelectorAll('script[type^="math/"], script[type="application/x-tex"], script[type="text/latex"]');
//	  for (const script of scripts) {
//	    const content = script.textContent?.trim();
//	    if (content) {
//	      return content;
//	    }
//	  }
//
//	  // Check for KaTeX annotation
//	  const annotation = el.querySelector('annotation[encoding="application/x-tex"]');
//	  if (annotation) {
//	    return annotation.textContent?.trim() || null;
//	  }
//
//	  // Check for text content that looks like LaTeX
//	  const textContent = el.textContent?.trim() || '';
//	  if (textContent.includes('$') || textContent.includes('\\')) {
//	    return textContent;
//	  }
//
//	  return null;
//	};
func (p *MathProcessor) getLaTeXFromElement(s *goquery.Selection) string {
	// Check for data attributes
	if dataLatex, hasDataLatex := s.Attr("data-latex"); hasDataLatex && dataLatex != "" {
		return dataLatex
	}
	if dataTex, hasDataTex := s.Attr("data-tex"); hasDataTex && dataTex != "" {
		return dataTex
	}

	// WordPress LaTeX images: extract from src URL
	if src, exists := s.Attr("src"); exists && strings.Contains(src, "latex.php") {
		if m := wpLatexRe.FindStringSubmatch(src); m != nil {
			decoded, err := url.QueryUnescape(m[1])
			if err == nil {
				decoded = strings.ReplaceAll(decoded, "+", " ")
				return decoded
			}
		}
	}

	// KaTeX .math wrapper: check data-latex on parent
	if s.HasClass("katex") {
		parent := s.Parent()
		if parent.Length() > 0 {
			if dataLatex, exists := parent.Attr("data-latex"); exists && dataLatex != "" {
				return dataLatex
			}
		}
	}

	// Check for script elements with LaTeX content
	scriptSelectors := []string{
		"script[type^=\"math/\"]",
		"script[type=\"application/x-tex\"]",
		"script[type=\"text/latex\"]",
	}

	for _, selector := range scriptSelectors {
		script := s.Find(selector).First()
		if script.Length() > 0 {
			content := strings.TrimSpace(script.Text())
			if content != "" {
				return content
			}
		}
	}

	// Check for KaTeX annotation
	annotation := s.Find("annotation[encoding=\"application/x-tex\"]").First()
	if annotation.Length() > 0 {
		content := strings.TrimSpace(annotation.Text())
		if content != "" {
			return content
		}
	}

	// Check for text content that looks like LaTeX
	textContent := strings.TrimSpace(s.Text())
	if p.looksLikeLaTeX(textContent) {
		return textContent
	}

	return ""
}

// isBlockDisplay determines if math should be displayed as block
// TypeScript original code:
//
//	export const isBlockDisplay = (el: Element): boolean => {
//	  // Check explicit display attribute
//	  const mathEl = el.querySelector('math');
//	  if (mathEl) {
//	    const display = mathEl.getAttribute('display');
//	    if (display === 'block') return true;
//	    if (display === 'inline') return false;
//	  }
//
//	  // Check CSS classes
//	  const blockClasses = ['MathJax_Display', 'katex-display', 'katex-block'];
//	  for (const className of blockClasses) {
//	    if (el.classList?.contains(className)) return true;
//	  }
//
//	  // Check if it's in a display context
//	  const parent = el.parentElement;
//	  if (parent) {
//	    const style = getComputedStyle(parent);
//	    if (style.display === 'block' && style.textAlign === 'center') {
//	      return true;
//	    }
//	  }
//
//	  return false;
//	};
func (p *MathProcessor) isBlockDisplay(s *goquery.Selection) bool {
	// Check explicit display attribute in math element
	mathEl := s.Find("math").First()
	if mathEl.Length() > 0 {
		if display, hasDisplay := mathEl.Attr("display"); hasDisplay {
			return display == "block"
		}
	}

	// Check CSS classes on the element itself
	blockClasses := []string{"MathJax_Display", "katex-display", "katex-block", "mwe-math-mathml-display", "mwe-math-fallback-image-display"}
	if slices.ContainsFunc(blockClasses, s.HasClass) {
		return true
	}

	// Check ancestor context for block display containers
	if s.Closest(".katex-display, .MathJax_Display, .mwe-math-mathml-display").Length() > 0 {
		return true
	}

	// Check parent context
	parent := s.Parent()
	if parent.Length() > 0 {
		if parent.Is("div") && parent.HasClass("math-display") {
			return true
		}
		if style, hasStyle := parent.Attr("style"); hasStyle {
			if strings.Contains(strings.ToLower(style), "text-align") && strings.Contains(strings.ToLower(style), "center") {
				return true
			}
		}
	}

	return false
}

// createCleanMathElement creates a clean math element
// TypeScript original code:
//
//	export const createCleanMathEl = (doc: Document, mathData: MathData | null, latex: string | null, isBlock: boolean): Element => {
//	  const cleanMathEl = doc.createElement('math');
//
//	  cleanMathEl.setAttribute('xmlns', 'http://www.w3.org/1998/Math/MathML');
//	  cleanMathEl.setAttribute('display', isBlock ? 'block' : 'inline');
//	  cleanMathEl.setAttribute('data-latex', latex || '');
//
//	  // First try to use existing MathML content
//	  if (mathData?.mathml) {
//	    const tempDiv = doc.createElement('div');
//	    tempDiv.innerHTML = mathData.mathml;
//	    const mathContent = tempDiv.querySelector('math');
//	    if (mathContent) {
//	      cleanMathEl.innerHTML = mathContent.innerHTML;
//	    }
//	  }
//	  // If no MathML content but we have LaTeX, store it as text content
//	  else if (latex) {
//	    cleanMathEl.textContent = latex;
//	  }
//
//	  return cleanMathEl;
//	};
func (p *MathProcessor) createCleanMathElement(mathData *MathData, latex string, isBlock bool) string {
	var mathHTML strings.Builder

	mathHTML.WriteString("<math")
	mathHTML.WriteString(" xmlns=\"http://www.w3.org/1998/Math/MathML\"")

	if isBlock {
		mathHTML.WriteString(" display=\"block\"")
	} else {
		mathHTML.WriteString(" display=\"inline\"")
	}

	if latex != "" {
		mathHTML.WriteString(" data-latex=\"")
		// Escape attribute value
		escapedLatex := strings.ReplaceAll(latex, "\"", "&quot;")
		escapedLatex = strings.ReplaceAll(escapedLatex, "&", "&amp;")
		mathHTML.WriteString(escapedLatex)
		mathHTML.WriteString("\"")
	}

	mathHTML.WriteString(">")

	// First try to use existing MathML content
	if mathData != nil && mathData.MathML != "" {
		// Extract inner content from MathML if it's a complete math element
		mathML := mathData.MathML
		if strings.HasPrefix(mathML, "<math") {
			// Extract inner content
			start := strings.Index(mathML, ">")
			end := strings.LastIndex(mathML, "</math>")
			if start != -1 && end != -1 && start < end {
				mathHTML.WriteString(mathML[start+1 : end])
			} else {
				mathHTML.WriteString(mathML)
			}
		} else {
			mathHTML.WriteString(mathML)
		}
	} else if latex != "" {
		// Escape text content
		escapedContent := strings.ReplaceAll(latex, "&", "&amp;")
		escapedContent = strings.ReplaceAll(escapedContent, "<", "&lt;")
		escapedContent = strings.ReplaceAll(escapedContent, ">", "&gt;")
		mathHTML.WriteString(escapedContent)
	}

	mathHTML.WriteString("</math>")

	return mathHTML.String()
}

// cleanupMathScripts removes associated math scripts and previews
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

// looksLikeLaTeX checks if text content looks like LaTeX
// TypeScript original code:
// // Check for text content that looks like LaTeX
// const textContent = el.textContent?.trim() || ”;
//
//	if (textContent.includes('$') || textContent.includes('\\')) {
//	  return textContent;
//	}
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
