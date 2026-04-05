package elements

import (
	"fmt"
	"regexp"
	"strconv"
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

// detectTextFootnotes detects footnote patterns in text content
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

// findFootnoteDefinition finds a footnote definition by key
// TypeScript original code:
// // Try to find definition in common footnote areas
// const footnoteSections = element.querySelectorAll(
//
//	'.footnotes, .notes, .references, .endnotes, [class*="footnote"]'
//
// );
//
//	for (const section of footnoteSections) {
//	  const definition = section.querySelector(`[id*="${key}"], [data-footnote="${key}"]`);
//	  if (definition) {
//	    return definition;
//	  }
//	}
func (p *FootnoteProcessor) findFootnoteDefinition(key string) *goquery.Selection {
	// Try various ID patterns
	selectors := []string{
		"#footnote-" + key,
		"#fn-" + key,
		"#fn:" + key,
		"#note-" + key,
		"#ref-" + key,
		fmt.Sprintf("[data-footnote='%s']", key),
		fmt.Sprintf("[data-note='%s']", key),
	}

	for _, selector := range selectors {
		if def := p.doc.Find(selector).First(); def.Length() > 0 {
			return def
		}
	}

	// Try to find in footnote sections by text content.
	// Compile patterns once per call rather than inside the nested loop.
	keyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\.`),
		regexp.MustCompile(`^\[` + regexp.QuoteMeta(key) + `\]`),
		regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\)`),
	}
	var found *goquery.Selection
	p.doc.Find(".footnotes, .notes, .references, .endnotes").Each(func(_ int, section *goquery.Selection) {
		if found != nil {
			return
		}
		section.Find("li, div, p").Each(func(_ int, el *goquery.Selection) {
			if found != nil {
				return
			}
			text := el.Text()
			for _, re := range keyPatterns {
				if re.MatchString(text) {
					found = el
					return
				}
			}
		})
	})

	return found
}

// linkFootnotes links footnote references to their definitions
// TypeScript original code:
// // Every footnote reference should be a sup element with an anchor inside
// // e.g. <sup id="fnref:1"><a href="#fn:1">1</a></sup>
//
//	createFootnoteReference(footnoteNumber: string, refId: string): any {
//	  const sup = this.doc.createElement('sup');
//	  sup.id = refId;
//	  const link = this.doc.createElement('a');
//	  link.href = `#fn:${footnoteNumber}`;
//	  link.textContent = footnoteNumber;
//	  sup.appendChild(link);
//	  return sup;
//	}
func (p *FootnoteProcessor) linkFootnotes(footnotes []*Footnote, options *FootnoteProcessingOptions) {
	for _, footnote := range footnotes {
		if footnote.Reference == nil || footnote.Definition == nil ||
			footnote.Reference.Length() == 0 || footnote.Definition.Length() == 0 {
			continue
		}

		// Ensure reference has proper structure
		if !footnote.Reference.Parent().Is("sup") {
			// Wrap in sup if not already
			footnote.Reference.WrapHtml("<sup></sup>")
		}

		// Set reference attributes
		refID := fmt.Sprintf("%sref:%d", options.FootnotePrefix, footnote.Number)
		defID := fmt.Sprintf("%s:%d", options.FootnotePrefix, footnote.Number)

		footnote.Reference.Parent().SetAttr("id", refID)
		footnote.Reference.SetAttr("href", "#"+defID)

		// Set definition attributes
		footnote.Definition.SetAttr("id", defID)

		// Add backlink to definition
		backlink := fmt.Sprintf(`<a href="#%s" class="footnote-backref">↩</a>`, refID)
		footnote.Definition.AppendHtml(backlink)

		footnote.Linked = true
	}
}

// numberFootnotes assigns numbers to footnotes
// TypeScript original code:
// let footnoteCount = 1;
//
//	footnotes.forEach((footnote, index) => {
//	  footnote.number = footnoteCount++;
//	  // Update reference text
//	  if (footnote.reference) {
//	    footnote.reference.textContent = footnote.number.toString();
//	  }
//	});
func (p *FootnoteProcessor) numberFootnotes(footnotes []*Footnote, _ *FootnoteProcessingOptions) {
	for i, footnote := range footnotes {
		footnote.Number = i + 1

		// Update reference text
		if footnote.Reference != nil && footnote.Reference.Length() > 0 {
			footnote.Reference.SetText(strconv.Itoa(footnote.Number))
		}
	}
}

// improveAccessibility improves footnote accessibility
// TypeScript original code:
// // Add ARIA attributes for screen readers
// reference.setAttribute('aria-describedby', definitionId);
// reference.setAttribute('role', 'doc-noteref');
// definition.setAttribute('role', 'doc-endnote');
// definition.setAttribute('aria-label', `Footnote ${footnote.number}`);
func (p *FootnoteProcessor) improveAccessibility(footnotes []*Footnote) {
	for _, footnote := range footnotes {
		if footnote.Reference != nil && footnote.Reference.Length() > 0 {
			footnote.Reference.SetAttr("role", "doc-noteref")
			footnote.Reference.SetAttr("aria-describedby", footnote.ID)
		}

		if footnote.Definition != nil && footnote.Definition.Length() > 0 {
			footnote.Definition.SetAttr("role", "doc-endnote")
			footnote.Definition.SetAttr("aria-label", fmt.Sprintf("Footnote %d", footnote.Number))
		}
	}
}

// generateFootnoteSection generates a footnote section
// TypeScript original code:
// createFootnoteItem(
//
//	footnoteNumber: number,
//	content: string | any,
//	refs: string[]
//
//	): any {
//	  const doc = typeof content === 'string' ? this.doc : content.ownerDocument;
//	  const newItem = doc.createElement('li');
//	  newItem.className = 'footnote';
//	  newItem.id = `fn:${footnoteNumber}`;
//
//	  // Handle content
//	  if (typeof content === 'string') {
//	    const paragraph = doc.createElement('p');
//	    paragraph.innerHTML = content;
//	    newItem.appendChild(paragraph);
//	  }
//
//	  // Add backlink(s) to the last paragraph
//	  const lastParagraph = newItem.querySelector('p:last-of-type') || newItem;
//	  refs.forEach((refId, index) => {
//	    const backlink = doc.createElement('a');
//	    backlink.href = `#${refId}`;
//	    backlink.title = 'return to article';
//	    backlink.className = 'footnote-backref';
//	    backlink.innerHTML = '↩';
//	    lastParagraph.appendChild(backlink);
//	  });
//
//	  return newItem;
//	}
func (p *FootnoteProcessor) generateFootnoteSection(footnotes []*Footnote, options *FootnoteProcessingOptions) {
	if len(footnotes) == 0 {
		return
	}

	// Create footnote section HTML
	var sectionHTML strings.Builder
	fmt.Fprintf(&sectionHTML, `<div class="footnotes">
<h2>%s</h2>
<ol>`, options.SectionTitle)

	for _, footnote := range footnotes {
		if footnote.Content == "" {
			continue
		}

		defID := fmt.Sprintf("%s:%d", options.FootnotePrefix, footnote.Number)
		refID := fmt.Sprintf("%sref:%d", options.FootnotePrefix, footnote.Number)

		fmt.Fprintf(&sectionHTML, `
<li id="%s" class="footnote">
<p>%s <a href="#%s" class="footnote-backref" title="return to article">↩</a></p>
</li>`, defID, footnote.Content, refID)
	}

	sectionHTML.WriteString(`
</ol>
</div>`)

	// Insert the section
	p.insertFootnoteSection(sectionHTML.String(), options)
}

// insertFootnoteSection inserts the footnote section into the document
// TypeScript original code:
// // Insert footnote section at appropriate location
// const insertLocation = options.sectionLocation || 'end';
//
//	switch (insertLocation) {
//	  case 'end':
//	    document.body.appendChild(footnoteSection);
//	    break;
//	  case 'after-content':
//	    const content = document.querySelector('main, article, .content');
//	    if (content) {
//	      content.insertAdjacentElement('afterend', footnoteSection);
//	    }
//	    break;
//	}
func (p *FootnoteProcessor) insertFootnoteSection(html string, options *FootnoteProcessingOptions) {
	switch options.SectionLocation {
	case "end":
		// Append to body
		p.doc.Find("body").AppendHtml(html)
	case "after-content":
		// Insert after main content
		contentArea := p.doc.Find("main, article, .content").First()
		if contentArea.Length() > 0 {
			contentArea.AfterHtml(html)
		} else {
			p.doc.Find("body").AppendHtml(html)
		}
	default:
		// Default to end
		p.doc.Find("body").AppendHtml(html)
	}
}

// GetFootnotes returns all footnotes found in the document
// TypeScript original code:
//
//	getFootnotes(): Footnote[] {
//	  return this.footnotes;
//	}
func (p *FootnoteProcessor) GetFootnotes() []*Footnote {
	return p.ProcessFootnotes(DefaultFootnoteProcessingOptions())
}

// HasFootnotes checks if the document has footnotes
// TypeScript original code:
//
//	hasFootnotes(): boolean {
//	  return this.footnotes.length > 0;
//	}
func (p *FootnoteProcessor) HasFootnotes() bool {
	footnotes := p.GetFootnotes()
	return len(footnotes) > 0
}

// CleanupFootnotes removes duplicate and invalid footnotes
// TypeScript original code:
//
//	cleanupFootnotes(footnotes: Footnote[]): Footnote[] {
//	  const uniqueFootnotes = new Map();
//	  const cleaned = [];
//
//	  for (const footnote of footnotes) {
//	    if (!uniqueFootnotes.has(footnote.id) && footnote.isValid()) {
//	      uniqueFootnotes.set(footnote.id, footnote);
//	      cleaned.push(footnote);
//	    }
//	  }
//
//	  return cleaned;
//	}
func (p *FootnoteProcessor) CleanupFootnotes(footnotes []*Footnote) []*Footnote {
	seen := make(map[string]bool)
	cleaned := make([]*Footnote, 0, len(footnotes))

	for _, footnote := range footnotes {
		// Skip duplicates and invalid footnotes
		if seen[footnote.ID] || footnote.ID == "" {
			continue
		}

		seen[footnote.ID] = true
		cleaned = append(cleaned, footnote)
	}

	return cleaned
}
