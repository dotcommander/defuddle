package elements

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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
//
// getMathMLFromElement extracts MathML content from element
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
			if md := mathMLData(tempDoc.Find("math").First(), "mathjax"); md != nil {
				return md
			}
		}
	}

	// 3. MathJax v3: assistive MathML (.MJX_Assistive_MathML, mjx-assistive-mml)
	assistive := s.Find(".MJX_Assistive_MathML, mjx-assistive-mml").First()
	if assistive.Length() > 0 {
		if md := mathMLData(assistive.Find("math").First(), "mathjax"); md != nil {
			return md
		}
	}

	// 4. Check for KaTeX
	if s.HasClass("katex") {
		if md := laTeXData(s.Find("annotation[encoding=\"application/x-tex\"]").First(), "katex"); md != nil {
			return md
		}
	}

	// 5. Check for MathJax script
	if s.HasClass("MathJax") {
		if md := laTeXData(s.Find("script[type^=\"math/\"]").First(), "mathjax"); md != nil {
			return md
		}
	}

	return nil
}

// mathMLData builds a MathML MathData from mathEl (a <math> selection), or nil
// when mathEl is empty. Shared by the MathJax v2/v3 detection strategies.
func mathMLData(mathEl *goquery.Selection, mathType string) *MathData {
	if mathEl.Length() == 0 {
		return nil
	}
	outerHTML, _ := goquery.OuterHtml(mathEl)
	display := mathEl.AttrOr("display", "inline")
	return &MathData{
		MathML:  outerHTML,
		Type:    mathType,
		Display: display,
	}
}

// laTeXData builds a LaTeX MathData from sel's trimmed text, or nil when sel is
// empty. Shared by the KaTeX and MathJax-script detection strategies.
func laTeXData(sel *goquery.Selection, mathType string) *MathData {
	if sel.Length() == 0 {
		return nil
	}
	return &MathData{
		LaTeX: strings.TrimSpace(sel.Text()),
		Type:  mathType,
	}
}
