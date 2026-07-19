package elements

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
// // Extract footnote ID based on element type
// // Nature.com
//
//	if (el.matches('a[id^="ref-link"]')) {
//	  footnoteId = el.textContent?.trim() || '';
//
// // Science.org
//
//	} else if (el.matches('a[role="doc-biblioref"]')) {
//	  const xmlRid = el.getAttribute('data-xml-rid');
//	  if (xmlRid) {
//	    footnoteId = xmlRid;
//	  } else {
//	    const href = el.getAttribute('href');
//	    if (href?.startsWith('#core-R')) {
//	      footnoteId = href.replace('#core-R', '');
//	    }
//	  }
//	}
//
// detectTextFootnotes detects footnote patterns in text content
func (p *FootnoteProcessor) detectTextFootnotes(options *FootnoteProcessingOptions) []*Footnote {
	// Skip entirely if no footnote definition sections exist in the document.
	if p.doc.Find(".footnotes, .notes, .references, .endnotes").Length() == 0 {
		return nil
	}

	var footnotes []*Footnote

	// Common footnote patterns
	patterns := []string{
		`\[(\d+)\]`,       // [1], [2], etc.
		`\((\d+)\)`,       // (1), (2), etc.
		`\*(\d+)`,         // *1, *2, etc.
		`†(\d+)`,          // †1, †2, etc.
		`\[([a-zA-Z]+)\]`, // [a], [b], [note], etc.
	}

	compiledPatterns := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		compiledPatterns[i] = regexp.MustCompile(p)
	}

	// Cache the candidate selection once — only elements that plausibly contain
	// footnote references. This avoids scanning every DOM element per pattern.
	candidates := p.doc.Find("p, li, td, dd, span")

	for _, re := range compiledPatterns {
		// Find all text nodes and search for patterns
		candidates.Each(func(_ int, s *goquery.Selection) {
			// Skip elements that are already footnotes
			if s.Is("sup, .footnote, .footnote-ref") {
				return
			}

			text := s.Text()
			matches := re.FindAllStringSubmatch(text, -1)

			for _, match := range matches {
				if len(match) > 1 {
					key := match[1]

					// Try to find definition — only create footnote if definition exists
					// to avoid false positives on math expressions, array notation, etc.
					definition := p.findFootnoteDefinition(key)
					if definition == nil || definition.Length() == 0 {
						continue
					}

					footnotes = append(footnotes, &Footnote{
						ID:         p.generateFootnoteID(key, options),
						RefText:    match[0],
						Definition: definition,
						Content:    strings.TrimSpace(definition.Text()),
					})
				}
			}
		})
	}

	return footnotes
}

// detectWikipediaFootnotes detects Wikipedia-style footnotes
// TypeScript original code:
// // Common format using OL/UL and LI elements
// const items = list.querySelectorAll('li, div[role="listitem"]');
//
//	items.forEach((li: any) => {
//	  let id = '';
//	  let content: any = null;
//
//	  // Handle citations with .citations class
//	  const citationsDiv = li.querySelector('.citations');
//	  if (citationsDiv?.id?.toLowerCase().startsWith('r')) {
//	    id = citationsDiv.id.toLowerCase();
//	    // Look for citation content within the citations div
//	    const citationContent = citationsDiv.querySelector('.citation-content');
//	    if (citationContent) {
//	      content = citationContent;
//	    }
//	  } else {
//	    // Extract ID from various formats
//	    if (li.id.toLowerCase().startsWith('bib.bib')) {
//	      id = li.id.replace('bib.bib', '').toLowerCase();
//	    } else if (li.id.toLowerCase().startsWith('fn:')) {
//	      id = li.id.replace('fn:', '').toLowerCase();
//	    }
//	  }
//	});
func (p *FootnoteProcessor) detectWikipediaFootnotes(_ *FootnoteProcessingOptions) []*Footnote {
	var footnotes []*Footnote

	// Find footnote lists using TS-compatible selector list
	p.doc.Find(FootnoteListSelectors).Each(func(_ int, list *goquery.Selection) {
		// Substack: individual footnote divs with no parent list
		if goquery.NodeName(list) == "div" {
			if _, ok := list.Attr("data-component-name"); ok {
				anchor := list.Find("a.footnote-number").First()
				content := list.Find(".footnote-content").First()
				if anchor.Length() > 0 && content.Length() > 0 {
					id, _ := anchor.Attr("id")
					id = strings.TrimPrefix(id, "footnote-")
					id = strings.ToLower(id)
					if id != "" {
						footnotes = append(footnotes, &Footnote{
							ID:         id,
							Definition: content,
							Content:    strings.TrimSpace(content.Text()),
						})
					}
				}
				return
			}
		}

		// Standard list format: find li items (or div[role="listitem"])
		list.Find("li, div[role='listitem']").Each(func(_ int, li *goquery.Selection) {
			id, hasID := li.Attr("id")
			if !hasID {
				return
			}

			content := strings.TrimSpace(li.Text())

			// Look for backlink
			backlink := li.Find("a[href^='#cite_ref'], a.mw-cite-backlink").First()

			footnote := &Footnote{
				ID:         id,
				Definition: li,
				Content:    content,
			}

			if backlink.Length() > 0 {
				href, _ := backlink.Attr("href")
				refID := strings.TrimPrefix(href, "#")
				if ref := p.doc.Find(fmt.Sprintf(`[id="%s"]`, refID)).First(); ref.Length() > 0 {
					footnote.Reference = ref
				}
			}

			footnotes = append(footnotes, footnote)
		})
	})

	return footnotes
}
