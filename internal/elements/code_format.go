package elements

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
// codeContent = codeContent
//
//	.replace(/^\s+|\s+$/g, '')      // Trim start/end whitespace
//	.replace(/\t/g, '    ')         // Convert tabs to spaces
//	.replace(/\n{3,}/g, '\n\n')     // Normalize multiple newlines
//	.replace(/\u00a0/g, ' ')        // Replace non-breaking spaces
//	.replace(/^\n+/, '')            // Remove extra newlines at start
//	.replace(/\n+$/, '');           // Remove extra newlines at end
//
// normalizeCodeContent normalizes and cleans up code content
func (p *CodeBlockProcessor) normalizeCodeContent(content string) string {
	// Trim whitespace
	content = strings.TrimSpace(content)

	// Convert tabs to spaces
	content = strings.ReplaceAll(content, "\t", "    ")

	// Replace non-breaking spaces
	content = strings.ReplaceAll(content, "\u00a0", " ")

	// Normalize multiple newlines
	content = codeThreeNewlinesRe.ReplaceAllString(content, "\n\n")

	// Remove extra newlines at start and end
	content = codeLeadingNlRe.ReplaceAllString(content, "")
	content = codeTrailingNlRe.ReplaceAllString(content, "")

	return content
}

// formatCodeBlock formats a code block with language and options
// TypeScript original code:
// // Create new pre element
// const newPre = doc.createElement('pre');
//
// // Create code element
// const code = doc.createElement('code');
//
//	if (language) {
//	  code.setAttribute('data-lang', language);
//	  code.setAttribute('class', `language-${language}`);
//	}
//
// code.textContent = codeContent;
//
// newPre.appendChild(code);
// return newPre;
func (p *CodeBlockProcessor) formatCodeBlock(s *goquery.Selection, language, content string, _ *CodeBlockProcessingOptions) {
	// Create new pre and code structure using HTML strings (simpler approach)
	var preHTML strings.Builder
	preHTML.WriteString("<pre>")
	preHTML.WriteString("<code")

	if language != "" {
		fmt.Fprintf(&preHTML, ` data-lang="%s" class="language-%s"`, language, language)
	}

	preHTML.WriteString(">")
	// Escape HTML content
	escapedContent := strings.ReplaceAll(content, "&", "&amp;")
	escapedContent = strings.ReplaceAll(escapedContent, "<", "&lt;")
	escapedContent = strings.ReplaceAll(escapedContent, ">", "&gt;")
	preHTML.WriteString(escapedContent)
	preHTML.WriteString("</code>")
	preHTML.WriteString("</pre>")

	// Replace original element with new structure
	s.ReplaceWithHtml(preHTML.String())

	slog.Debug("formatted code block", "language", language, "contentLength", len(content))
}
