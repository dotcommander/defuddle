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
//	      // Processing logic for line-based formats
//	    }
//
//	    element.childNodes.forEach(child => {
//	      text += extractStructuredText(child);
//	    });
//	  }
//	  return text;
//	};
//
// extractStructuredContent extracts content using structured approach like TypeScript
func (p *CodeBlockProcessor) extractStructuredContent(s *goquery.Selection) string {
	// First try WordPress syntax highlighter extraction
	if s.HasClass("syntaxhighlighter") || s.HasClass("wp-block-syntaxhighlighter-code") {
		if content := p.extractWordPressContent(s); content != "" {
			return content
		}
	}

	// Use structured text extraction as fallback
	return p.extractStructuredText(s)
}

// extractWordPressContent extracts content from WordPress syntax highlighter
// TypeScript original code:
//
//	const extractWordPressContent = (element: Element): string => {
//	  // Handle WordPress syntax highlighter table format
//	  const codeContainer = element.querySelector('.syntaxhighlighter table .code .container');
//	  if (codeContainer) {
//	    return Array.from(codeContainer.children)
//	      .map(line => {
//	        const codeParts = Array.from(line.querySelectorAll('code'))
//	          .map(code => {
//	            let text = code.textContent || '';
//	            if (code.classList?.contains('spaces')) {
//	              text = ' '.repeat(text.length);
//	            }
//	            return text;
//	          })
//	          .join('');
//	        return codeParts || line.textContent || '';
//	      })
//	      .join('\n');
//	  }
//
//	  // Handle WordPress syntax highlighter non-table format
//	  const codeLines = element.querySelectorAll('.code .line');
//	  if (codeLines.length > 0) {
//	    return Array.from(codeLines)
//	      .map(line => {
//	        const codeParts = Array.from(line.querySelectorAll('code'))
//	          .map(code => code.textContent || '')
//	          .join('');
//	        return codeParts || line.textContent || '';
//	      })
//	      .join('\n');
//	  }
//
//	  return '';
//	};
func (p *CodeBlockProcessor) extractWordPressContent(s *goquery.Selection) string {
	var builder strings.Builder

	// Handle WordPress syntax highlighter table format
	codeContainer := s.Find(".syntaxhighlighter table .code .container")
	if codeContainer.Length() > 0 {
		codeContainer.Children().Each(func(i int, line *goquery.Selection) {
			if i > 0 {
				builder.WriteString("\n")
			}

			var lineBuilder strings.Builder
			line.Find("code").Each(func(_ int, code *goquery.Selection) {
				text := code.Text()
				if code.HasClass("spaces") {
					// Replace with spaces of same length
					lineBuilder.WriteString(strings.Repeat(" ", len(text)))
				} else {
					lineBuilder.WriteString(text)
				}
			})

			if lineContent := lineBuilder.String(); lineContent != "" {
				builder.WriteString(lineContent)
			} else {
				builder.WriteString(line.Text())
			}
		})
		return builder.String()
	}

	// Handle WordPress syntax highlighter non-table format
	codeLines := s.Find(".code .line")
	if codeLines.Length() > 0 {
		codeLines.Each(func(i int, line *goquery.Selection) {
			if i > 0 {
				builder.WriteString("\n")
			}

			var lineBuilder strings.Builder
			line.Find("code").Each(func(_ int, code *goquery.Selection) {
				lineBuilder.WriteString(code.Text())
			})

			if lineContent := lineBuilder.String(); lineContent != "" {
				builder.WriteString(lineContent)
			} else {
				builder.WriteString(line.Text())
			}
		})
		return builder.String()
	}

	return ""
}
