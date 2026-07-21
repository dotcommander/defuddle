package elements

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ProcessFootnotes processes all footnotes in the document
// TypeScript original code:
//
//	standardizeFootnotes(element: any) {
//	  const footnotes = this.collectFootnotes(element);
//	  // Standardize inline footnotes using the collected IDs
//	  const footnoteInlineReferences = element.querySelectorAll(FOOTNOTE_INLINE_REFERENCES);
//	  // Process all footnote references and definitions
//	}
func (p *FootnoteProcessor) ProcessFootnotes(options *FootnoteProcessingOptions) []*Footnote {
	if options == nil {
		options = DefaultFootnoteProcessingOptions()
	}

	var footnotes []*Footnote

	// Detect footnotes if enabled
	if options.DetectFootnotes {
		footnotes = p.detectFootnotes(options)
	}

	// Link footnotes if enabled
	if options.LinkFootnotes {
		p.linkFootnotes(footnotes, options)
	}

	// Number footnotes if enabled
	if options.NumberFootnotes {
		p.numberFootnotes(footnotes, options)
	}

	// Improve accessibility if enabled
	if options.ImproveAccessibility {
		p.improveAccessibility(footnotes)
	}

	// Generate footnote section if enabled
	if options.GenerateSection && len(footnotes) > 0 {
		p.generateFootnoteSection(footnotes, options)
	}

	return footnotes
}

// detectFootnotes detects footnotes in the document
// TypeScript original code:
//
//	collectFootnotes(element: any): FootnoteCollection {
//	  const footnotes: FootnoteCollection = {};
//	  let footnoteCount = 1;
//	  const processedIds = new Set<string>();
//
//	  // Collect all footnotes and their IDs from footnote lists
//	  const footnoteLists = element.querySelectorAll(FOOTNOTE_LIST_SELECTORS);
//	  footnoteLists.forEach((list: any) => {
//	    // Process different footnote formats
//	  });
//
//	  return footnotes;
//	}
func (p *FootnoteProcessor) detectFootnotes(options *FootnoteProcessingOptions) []*Footnote {
	footnotes := make([]*Footnote, 0, 10)

	// Detect existing footnote elements
	existingFootnotes := p.detectExistingFootnotes(options)
	footnotes = append(footnotes, existingFootnotes...)

	// Detect footnote patterns in text
	textFootnotes := p.detectTextFootnotes(options)
	footnotes = append(footnotes, textFootnotes...)

	// Detect Wikipedia-style footnotes
	wikiFootnotes := p.detectWikipediaFootnotes(options)
	footnotes = append(footnotes, wikiFootnotes...)

	return footnotes
}

// detectExistingFootnotes detects existing footnote elements
// TypeScript original code:
// // Substack has individual footnote divs with no parent
//
//	if (list.matches('div.footnote[data-component-name="FootnoteToDOM"]')) {
//	  const anchor = list.querySelector('a.footnote-number');
//	  const content = list.querySelector('.footnote-content');
//	  if (anchor && content) {
//	    const id = anchor.id.replace('footnote-', '').toLowerCase();
//	    if (id && !processedIds.has(id)) {
//	      footnotes[footnoteCount] = {
//	        content: content,
//	        originalId: id,
//	        refs: []
//	      };
//	      processedIds.add(id);
//	      footnoteCount++;
//	    }
//	  }
//	  return;
//	}
func (p *FootnoteProcessor) detectExistingFootnotes(_ *FootnoteProcessingOptions) []*Footnote {
	var footnotes []*Footnote

	// Find footnote references using TS-compatible selector list
	p.doc.Find(FootnoteInlineReferences).Each(func(_ int, s *goquery.Selection) {
		var footnoteID string

		// Science.org: a[role="doc-biblioref"] with data-xml-rid
		if role, _ := s.Attr("role"); role == "doc-biblioref" {
			if xmlRid, exists := s.Attr("data-xml-rid"); exists && xmlRid != "" {
				footnoteID = xmlRid
			}
		}

		// Nature.com: a[id^="ref-link"] — ID from text content
		if footnoteID == "" {
			if id, _ := s.Attr("id"); strings.HasPrefix(id, "ref-link") {
				footnoteID = strings.TrimSpace(s.Text())
			}
		}

		// LessWrong: span.footnote-reference with data-footnote-id
		if footnoteID == "" {
			if fnID, exists := s.Attr("data-footnote-id"); exists && fnID != "" {
				footnoteID = fnID
			}
		}

		// Default: extract from href
		if footnoteID == "" {
			href, hasHref := s.Attr("href")
			if !hasHref || !strings.HasPrefix(href, "#") {
				// Try fnref ID pattern (a[id^="fnref"])
				if id, _ := s.Attr("id"); strings.HasPrefix(id, "fnref") {
					footnoteID = strings.TrimPrefix(id, "fnref")
					footnoteID = strings.TrimPrefix(footnoteID, ":")
					footnoteID = strings.TrimPrefix(footnoteID, "-")
				}
				if footnoteID == "" {
					return
				}
			} else {
				footnoteID = strings.TrimPrefix(href, "#")
			}
		}

		if footnoteID == "" {
			return
		}

		// Find corresponding definition (use attribute selector for IDs with special chars like colons)
		definition := p.doc.Find(fmt.Sprintf(`[id="%s"]`, footnoteID)).First()

		footnote := &Footnote{
			ID:         footnoteID,
			Reference:  s,
			Definition: definition,
			RefText:    strings.TrimSpace(s.Text()),
		}

		if definition.Length() > 0 {
			footnote.Content = strings.TrimSpace(definition.Text())
		}

		footnotes = append(footnotes, footnote)
	})

	return footnotes
}
