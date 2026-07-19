package elements

import (
	"net/url"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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
//
// getLaTeXFromElement extracts LaTeX content from element
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
