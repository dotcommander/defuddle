package elements

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
)

// TypeScript original code:
// // Create new heading of same level
// const newHeading = doc.createElement(el.tagName);
//
// // Copy allowed attributes from original heading
//
//	Array.from(el.attributes).forEach(attr => {
//	  if (ALLOWED_ATTRIBUTES.has(attr.name)) {
//	    newHeading.setAttribute(attr.name, attr.value);
//	  }
//	});
//
// // Set the clean text content
// newHeading.textContent = textContent;
// replaceHeadingContent replaces heading content while preserving structure
func (p *HeadingProcessor) replaceHeadingContent(s *goquery.Selection, textContent string, _ *HeadingProcessingOptions) {
	tagName := goquery.NodeName(s)

	// Build new heading HTML matching TS: iterate element's attributes,
	// keep those in ALLOWED_ATTRIBUTES (does NOT include class or id).
	var headingHTML strings.Builder
	headingHTML.WriteString("<")
	headingHTML.WriteString(tagName)

	if node := s.Get(0); node != nil {
		for _, attr := range node.Attr {
			if constants.IsAllowedAttribute(strings.ToLower(attr.Key)) {
				headingHTML.WriteString(" ")
				headingHTML.WriteString(attr.Key)
				headingHTML.WriteString("=\"")
				escapedValue := strings.ReplaceAll(attr.Val, "\"", "&quot;")
				headingHTML.WriteString(escapedValue)
				headingHTML.WriteString("\"")
			}
		}
	}

	headingHTML.WriteString(">")
	// Escape text content
	escapedContent := strings.ReplaceAll(textContent, "&", "&amp;")
	escapedContent = strings.ReplaceAll(escapedContent, "<", "&lt;")
	escapedContent = strings.ReplaceAll(escapedContent, ">", "&gt;")
	headingHTML.WriteString(escapedContent)
	headingHTML.WriteString("</")
	headingHTML.WriteString(tagName)
	headingHTML.WriteString(">")

	// Replace original heading
	s.ReplaceWithHtml(headingHTML.String())
}

// ProcessHeadings processes all headings in the document (public interface)
// TypeScript original code:
//
//	export function processHeadings(doc: Document, options?: HeadingOptions): void {
//	  const processor = new HeadingProcessor(doc);
//	  processor.processAllHeadings(options || defaultOptions);
//	}
func ProcessHeadings(doc *goquery.Document, options *HeadingProcessingOptions) {
	processor := NewHeadingProcessor(doc)
	processor.ProcessHeadings(options)
}

// ProcessHeadingsInScope processes headings within the given container element.
func ProcessHeadingsInScope(scope *goquery.Selection, options *HeadingProcessingOptions) {
	processor := &HeadingProcessor{}
	if options == nil {
		options = DefaultHeadingProcessingOptions()
	}
	scope.Find("h1, h2, h3, h4, h5, h6").Each(func(_ int, s *goquery.Selection) {
		processor.processHeading(s, options)
	})
}
