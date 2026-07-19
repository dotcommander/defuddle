package elements

import "strings"

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
//
// createCleanMathElement creates a clean math element
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
