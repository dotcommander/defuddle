package elements

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
//
//	const extractStructuredText = (element: Node): string => {
//	  if (isTextNode(element)) {
//	    return element.textContent || '';
//	  }
//
//	  let text = '';
//	  if (isElement(element)) {
//	    // Handle explicit line breaks
//	    if (element.tagName === 'BR') {
//	      return '\n';
//	    }
//
//	    // Handle common line-based code formats
//	    if (element.matches('div[class*="line"], span[class*="line"], .ec-line, [data-line-number], [data-line]')) {
//	      // Try to find the actual code content in common structures:
//	      // 1. A dedicated code container
//	      const codeContainer = element.querySelector('.code, .content, [class*="code-"], [class*="content-"]');
//	      if (codeContainer) {
//	        return (codeContainer.textContent || '') + '\n';
//	      }
//
//	      // 2. Line number is in a separate element
//	      const lineNumber = element.querySelector('.line-number, .gutter, [class*="line-number"], [class*="gutter"]');
//	      if (lineNumber) {
//	        const withoutLineNum = Array.from(element.childNodes)
//	          .filter(node => !lineNumber.contains(node))
//	          .map(node => extractStructuredText(node))
//	          .join('');
//	        return withoutLineNum + '\n';
//	      }
//
//	      // 3. Fallback to the entire line content
//	      return element.textContent + '\n';
//	    }
//
//	    element.childNodes.forEach(child => {
//	      text += extractStructuredText(child);
//	    });
//	  }
//	  return text;
//	};
//
// extractStructuredText recursively extracts text with structure preservation
func (p *CodeBlockProcessor) extractStructuredText(s *goquery.Selection) string {
	var builder strings.Builder

	s.Contents().Each(func(_ int, node *goquery.Selection) {
		// Handle text nodes
		if goquery.NodeName(node) == "#text" {
			text := node.Text()
			// Skip whitespace-only text nodes between data-line spans
			// (e.g. rehype-pretty-code / Shiki), since data-line handling
			// already appends a newline per line.
			if strings.TrimSpace(text) == "" && node.Parent().Find("[data-line]").Length() > 0 {
				return
			}
			builder.WriteString(text)
			return
		}

		// Handle BR elements
		if node.Is("br") {
			builder.WriteString("\n")
			return
		}

		// Handle common line-based code formats
		lineSelectors := []string{
			"div[class*=\"line\"]",
			"span[class*=\"line\"]",
			".ec-line",
			"[data-line-number]",
			"[data-line]",
		}

		for _, lineSelector := range lineSelectors {
			if node.Is(lineSelector) {
				// Try to find dedicated code container
				codeContainer := node.Find(".code, .content, [class*=\"code-\"], [class*=\"content-\"]")
				if codeContainer.Length() > 0 {
					builder.WriteString(codeContainer.Text())
					builder.WriteString("\n")
					return
				}

				// Handle line numbers in separate element
				lineNumber := node.Find(".line-number, .gutter, [class*=\"line-number\"], [class*=\"gutter\"]")
				if lineNumber.Length() > 0 {
					// Extract content without line numbers
					var lineContent strings.Builder
					node.Contents().Each(func(_ int, child *goquery.Selection) {
						// Check if child is not contained in lineNumber elements
						childNode := child.Get(0)
						isLineNumber := false
						lineNumber.Each(func(_ int, ln *goquery.Selection) {
							if ln.Get(0) == childNode {
								isLineNumber = true
							}
						})
						if !isLineNumber {
							lineContent.WriteString(p.extractStructuredText(child))
						}
					})
					builder.WriteString(lineContent.String())
					builder.WriteString("\n")
					return
				}

				// Fallback to entire line content
				builder.WriteString(node.Text())
				builder.WriteString("\n")
				return
			}
		}

		// Recursively process child elements
		builder.WriteString(p.extractStructuredText(node))
	})

	return builder.String()
}
