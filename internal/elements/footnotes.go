// Package elements provides enhanced element processing functionality
// This module handles footnote processing including detection, linking,
// and accessibility improvements
package elements

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// footnoteEntry holds a collected footnote definition and its references.
// TypeScript original code:
//
//	interface FootnoteData {
//	  content: any;
//	  originalId: string;
//	  refs: string[];
//	}
type footnoteEntry struct {
	content    *goquery.Selection
	originalID string
	refs       []string
}

// Pre-compiled regexes used by StandardizeFootnotes.
var (
	reCiteNoteSuffix = regexp.MustCompile(`cite_note-(.+)`)
	reBibBib         = regexp.MustCompile(`bib\.bib(\d+)`)
	reCiteNoteRef    = regexp.MustCompile(`(?:cite_note|cite_ref)-(.+)`)
	reDotTrailing    = regexp.MustCompile(`\.$`)
)

/*
TypeScript source code (footnotes.ts, 387 lines):

This module provides comprehensive footnote processing functionality including:
- Footnote detection and extraction
- Automatic linking between references and definitions
- Footnote numbering and organization
- Accessibility improvements for screen readers
- Footnote popup and tooltip generation

Key functions:
- processFootnotes(): Main processing function for all footnotes
- detectFootnotes(): Footnote detection and extraction
- linkFootnotes(): Linking references to definitions
- improveAccessibility(): Footnote accessibility enhancements
- generateFootnoteSection(): Footnote section generation
*/

// FootnoteProcessor handles footnote processing and enhancement
// TypeScript original code:
//
//	class FootnoteHandler {
//	  private doc: any;
//
//	  constructor(doc: any) {
//	    this.doc = doc;
//	  }
//	}
type FootnoteProcessor struct {
	doc *goquery.Document
}

// FootnoteProcessingOptions contains options for footnote processing
// TypeScript original code:
//
//	interface FootnoteData {
//	  content: any;
//	  originalId: string;
//	  refs: string[];
//	}
//
//	interface FootnoteCollection {
//	  [footnoteNumber: number]: FootnoteData;
//	}
type FootnoteProcessingOptions struct {
	DetectFootnotes      bool
	LinkFootnotes        bool
	ImproveAccessibility bool
	GenerateSection      bool
	NumberFootnotes      bool
	FootnotePrefix       string
	SectionTitle         string
	SectionLocation      string // "end", "after-content", "custom"
}

// Footnote represents a footnote with its reference and definition
// TypeScript original code:
//
//	interface FootnoteData {
//	  content: any;
//	  originalId: string;
//	  refs: string[];
//	}
type Footnote struct {
	ID         string
	Number     int
	Reference  *goquery.Selection
	Definition *goquery.Selection
	Content    string
	RefText    string
	Linked     bool
}

// FootnoteInlineReferences matches inline footnote reference elements.
// Ported from TypeScript FOOTNOTE_INLINE_REFERENCES.
var FootnoteInlineReferences = strings.Join([]string{
	`sup.reference`,
	`cite.ltx_cite`,
	`sup[id^="fnr"]`,
	`span[id^="fnr"]`,
	`span[class*="footnote_ref"]`,
	`span[class*="footnote-ref"]`,
	`span.footnote-link`,
	`a.citation`,
	`a[id^="ref-link"]`,
	`a[href^="#fn"]`,
	`a[href^="#cite"]`,
	`a[href^="#reference"]`,
	`a[href^="#footnote"]`,
	`a[href^="#r"]`,
	`a[href^="#b"]`,
	`a[href*="cite_note"]`,
	`a[href*="cite_ref"]`,
	`a.footnote-anchor`,
	`a.footnote`,
	`a[role="doc-biblioref"]`,
	`a[id^="fnref"]`,
	`.footnote-ref`,
	`sup a[href^="#"]`,
}, ", ")

// FootnoteListSelectors matches footnote definition list containers.
// Ported from TypeScript FOOTNOTE_LIST_SELECTORS.
var FootnoteListSelectors = strings.Join([]string{
	`div.footnote ol`,
	`div.footnotes ol`,
	`div[role="doc-endnotes"]`,
	`div[role="doc-footnotes"]`,
	`ol.footnotes-list`,
	`ol.footnotes`,
	`ol.references`,
	`ol[class*="article-references"]`,
	`section.footnotes ol`,
	`section[role="doc-endnotes"]`,
	`section[role="doc-footnotes"]`,
	`section[role="doc-bibliography"]`,
	`ul.footnotes-list`,
	`ul.ltx_biblist`,
	`div.footnote[data-component-name="FootnoteToDOM"]`,
}, ", ")

// DefaultFootnoteProcessingOptions returns default options for footnote processing
// TypeScript original code:
//
//	const defaultOptions = {
//	  detectFootnotes: true,
//	  linkFootnotes: true,
//	  improveAccessibility: true,
//	  generateSection: true,
//	  numberFootnotes: true
//	};
func DefaultFootnoteProcessingOptions() *FootnoteProcessingOptions {
	return &FootnoteProcessingOptions{
		DetectFootnotes:      true,
		LinkFootnotes:        true,
		ImproveAccessibility: true,
		GenerateSection:      true,
		NumberFootnotes:      true,
		FootnotePrefix:       "fn",
		SectionTitle:         "Footnotes",
		SectionLocation:      "end",
	}
}

// NewFootnoteProcessor creates a new footnote processor
// TypeScript original code:
//
//	constructor(doc: any) {
//	  this.doc = doc;
//	}
func NewFootnoteProcessor(doc *goquery.Document) *FootnoteProcessor {
	return &FootnoteProcessor{
		doc: doc,
	}
}

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
	var footnotes []*Footnote

	// Common footnote patterns
	patterns := []string{
		`\[(\d+)\]`,       // [1], [2], etc.
		`\((\d+)\)`,       // (1), (2), etc.
		`\*(\d+)`,         // *1, *2, etc.
		`†(\d+)`,          // †1, †2, etc.
		`\[([a-zA-Z]+)\]`, // [a], [b], [note], etc.
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)

		// Find all text nodes and search for patterns
		p.doc.Find("*").Each(func(_ int, s *goquery.Selection) {
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

	// Try to find in footnote sections by text content
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
			patterns := []string{
				fmt.Sprintf("^%s\\.", key),
				fmt.Sprintf("^\\[%s\\]", key),
				fmt.Sprintf("^%s\\)", key),
			}

			for _, pattern := range patterns {
				if matched, _ := regexp.MatchString(pattern, text); matched {
					found = el
					return
				}
			}
		})
	})

	return found
}

// generateFootnoteID generates a footnote ID
// TypeScript original code:
//
//	const generateId = (key: string): string => {
//	  return `fn-${key}`;
//	};
func (p *FootnoteProcessor) generateFootnoteID(key string, options *FootnoteProcessingOptions) string {
	prefix := options.FootnotePrefix
	if prefix == "" {
		prefix = "fn"
	}
	return fmt.Sprintf("%s-%s", prefix, key)
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
	sectionHTML.WriteString(fmt.Sprintf(`<div class="footnotes">
<h2>%s</h2>
<ol>`, options.SectionTitle))

	for _, footnote := range footnotes {
		if footnote.Content == "" {
			continue
		}

		defID := fmt.Sprintf("%s:%d", options.FootnotePrefix, footnote.Number)
		refID := fmt.Sprintf("%sref:%d", options.FootnotePrefix, footnote.Number)

		sectionHTML.WriteString(fmt.Sprintf(`
<li id="%s" class="footnote">
<p>%s <a href="#%s" class="footnote-backref" title="return to article">↩</a></p>
</li>`, defID, footnote.Content, refID))
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

// ProcessFootnotes processes all footnotes in the document (public interface)
// TypeScript original code:
//
//	export function standardizeFootnotes(element: any): void {
//	  const handler = new FootnoteHandler(element.ownerDocument);
//	  handler.standardizeFootnotes(element);
//	}
func ProcessFootnotes(doc *goquery.Document, options *FootnoteProcessingOptions) []*Footnote {
	processor := NewFootnoteProcessor(doc)
	return processor.ProcessFootnotes(options)
}

// collectFootnoteDefinitions collects all footnote definitions from a scope.
// It mirrors the TypeScript collectFootnotes method, returning a sequential
// number → footnoteEntry map so callers can rebuild a canonical list.
// TypeScript original code:
//
//	collectFootnotes(element: any): FootnoteCollection {
//	  const footnotes: FootnoteCollection = {};
//	  let footnoteCount = 1;
//	  const processedIds = new Set<string>();
//	  const footnoteLists = element.querySelectorAll(FOOTNOTE_LIST_SELECTORS);
//	  footnoteLists.forEach((list: any) => { ... });
//	  return footnotes;
//	}
func (p *FootnoteProcessor) collectFootnoteDefinitions(scope *goquery.Selection) map[int]*footnoteEntry {
	result := make(map[int]*footnoteEntry)
	count := 1
	processedIDs := make(map[string]bool)

	scope.Find(FootnoteListSelectors).Each(func(_ int, list *goquery.Selection) {
		// Substack: individual footnote divs with no parent list
		if goquery.NodeName(list) == "div" {
			if _, ok := list.Attr("data-component-name"); ok {
				p.collectSubstackFootnote(list, result, processedIDs, &count)
				return
			}
		}
		p.collectListFootnotes(list, result, processedIDs, &count)
	})

	return result
}

// collectSubstackFootnote handles Substack-format footnote divs.
func (p *FootnoteProcessor) collectSubstackFootnote(
	list *goquery.Selection,
	result map[int]*footnoteEntry,
	seen map[string]bool,
	count *int,
) {
	anchor := list.Find("a.footnote-number").First()
	content := list.Find(".footnote-content").First()
	if anchor.Length() == 0 || content.Length() == 0 {
		return
	}
	id, _ := anchor.Attr("id")
	id = strings.ToLower(strings.TrimPrefix(id, "footnote-"))
	if id == "" || seen[id] {
		return
	}
	result[*count] = &footnoteEntry{content: content, originalID: id}
	seen[id] = true
	*count++
}

// collectListFootnotes handles standard OL/UL footnote lists.
func (p *FootnoteProcessor) collectListFootnotes(
	list *goquery.Selection,
	result map[int]*footnoteEntry,
	seen map[string]bool,
	count *int,
) {
	list.Find(`li, div[role="listitem"]`).Each(func(_ int, li *goquery.Selection) {
		id, content := p.extractListItemIDAndContent(li)
		if id == "" || seen[id] {
			return
		}
		if content == nil {
			content = li
		}
		result[*count] = &footnoteEntry{content: content, originalID: id}
		seen[id] = true
		*count++
	})
}

// extractListItemIDAndContent derives the canonical ID and content element from an li.
// TypeScript original code (inside collectFootnotes items.forEach):
//
//	const citationsDiv = li.querySelector('.citations');
//	if (citationsDiv?.id?.toLowerCase().startsWith('r')) { ... }
//	else if (li.id.toLowerCase().startsWith('bib.bib')) { ... }
//	else if (li.id.toLowerCase().startsWith('fn:')) { ... }
//	...
func (p *FootnoteProcessor) extractListItemIDAndContent(li *goquery.Selection) (string, *goquery.Selection) {
	// Citations div (Science.org)
	citDiv := li.Find(".citations").First()
	if citDiv.Length() > 0 {
		citID, _ := citDiv.Attr("id")
		if strings.HasPrefix(strings.ToLower(citID), "r") {
			citContent := citDiv.Find(".citation-content").First()
			if citContent.Length() > 0 {
				return strings.ToLower(citID), citContent
			}
			return strings.ToLower(citID), citDiv
		}
	}

	rawID, _ := li.Attr("id")
	lower := strings.ToLower(rawID)
	var id string

	switch {
	case strings.HasPrefix(lower, "bib.bib"):
		id = strings.TrimPrefix(lower, "bib.bib")
	case strings.HasPrefix(lower, "fn:"):
		id = strings.TrimPrefix(lower, "fn:")
	case strings.HasPrefix(lower, "fn"):
		id = strings.TrimPrefix(lower, "fn")
	default:
		if dc, exists := li.Attr("data-counter"); exists {
			// Nature.com: strip trailing dot
			id = strings.ToLower(reDotTrailing.ReplaceAllString(dc, ""))
		} else {
			// Wikipedia: cite_note-<suffix>
			raw := strings.Split(rawID, "/")
			last := raw[len(raw)-1]
			if m := reCiteNoteSuffix.FindStringSubmatch(last); m != nil {
				id = strings.ToLower(m[1])
			} else {
				id = strings.ToLower(rawID)
			}
		}
	}

	return id, li
}

// findOuterFootnoteContainer walks up from el through span/sup ancestors,
// returning the outermost span or sup encountered.
// TypeScript original code:
//
//	findOuterFootnoteContainer(el: any): any {
//	  let current = el;
//	  let parent = el.parentElement;
//	  while (parent && (parent.tagName === 'span' || parent.tagName === 'sup')) {
//	    current = parent;
//	    parent = parent.parentElement;
//	  }
//	  return current;
//	}
func (p *FootnoteProcessor) findOuterFootnoteContainer(s *goquery.Selection) *goquery.Selection {
	current := s
	for {
		parent := current.Parent()
		if parent.Length() == 0 {
			break
		}
		tag := goquery.NodeName(parent)
		if tag != "span" && tag != "sup" {
			break
		}
		current = parent
	}
	return current
}

// createFootnoteRefHTML returns a standardized inline reference element.
// TypeScript original code:
//
//	createFootnoteReference(footnoteNumber, refId) {
//	  const sup = createElement('sup'); sup.id = refId;
//	  const link = createElement('a'); link.href = `#fn:${footnoteNumber}`;
//	  link.textContent = footnoteNumber;
//	  sup.appendChild(link); return sup;
//	}
func (p *FootnoteProcessor) createFootnoteRefHTML(number int, refID string) string {
	return fmt.Sprintf(`<sup id="%s"><a href="#fn:%d">%d</a></sup>`, refID, number, number)
}

// createFootnoteItemHTML returns a standardized footnote list item.
// TypeScript original code (createFootnoteItem):
//
//	newItem.id = `fn:${footnoteNumber}`;
//	// copy paragraphs from content, append backlinks
func (p *FootnoteProcessor) createFootnoteItemHTML(number int, content *goquery.Selection, refs []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<li id="fn:%d">`, number))

	// Get paragraphs from content element
	paragraphs := content.Find("p")
	if paragraphs.Length() == 0 {
		// Wrap raw innerHTML in a paragraph
		inner, _ := content.Html()
		b.WriteString("<p>")
		b.WriteString(inner)
	} else {
		// Copy first paragraph; others follow after
		paragraphs.Each(func(i int, par *goquery.Selection) {
			inner, _ := par.Html()
			b.WriteString("<p>")
			b.WriteString(inner)
			if i < paragraphs.Length()-1 {
				b.WriteString("</p>")
			}
			// Leave last paragraph open so backlinks are appended inside it
		})
	}

	// Append back-links into the last paragraph
	for i, refID := range refs {
		b.WriteString(fmt.Sprintf(`<a href="#%s" title="return to article" class="footnote-backref">↩</a>`, refID))
		if i < len(refs)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("</p>")
	b.WriteString("</li>")
	return b.String()
}

// extractInlineFootnoteID derives the canonical footnote ID from an inline
// reference element, matching all site-specific patterns from the TypeScript.
// TypeScript original code: the large if-else chain inside standardizeFootnotes.
func (p *FootnoteProcessor) extractInlineFootnoteID(el *goquery.Selection) string {
	// Nature.com: a[id^="ref-link"]
	if id, _ := el.Attr("id"); strings.HasPrefix(id, "ref-link") {
		return strings.TrimSpace(el.Text())
	}

	// Science.org: a[role="doc-biblioref"]
	if role, _ := el.Attr("role"); role == "doc-biblioref" {
		if xmlRid, ok := el.Attr("data-xml-rid"); ok && xmlRid != "" {
			return xmlRid
		}
		if href, ok := el.Attr("href"); ok && strings.HasPrefix(href, "#core-R") {
			return strings.TrimPrefix(href, "#core-")
		}
	}

	// Substack: a.footnote-anchor
	if el.HasClass("footnote-anchor") {
		if id, _ := el.Attr("id"); id != "" {
			return strings.ToLower(strings.TrimPrefix(id, "footnote-anchor-"))
		}
	}

	// ArXiv: cite.ltx_cite
	if goquery.NodeName(el) == "cite" && el.HasClass("ltx_cite") {
		link := el.Find("a").First()
		if link.Length() > 0 {
			if href, ok := link.Attr("href"); ok {
				parts := strings.Split(href, "/")
				last := parts[len(parts)-1]
				if m := reBibBib.FindStringSubmatch(last); m != nil {
					return strings.ToLower(m[1])
				}
			}
		}
		return ""
	}

	// Wikipedia: sup.reference
	if goquery.NodeName(el) == "sup" && el.HasClass("reference") {
		var found string
		el.Find("a").Each(func(_ int, a *goquery.Selection) {
			if found != "" {
				return
			}
			if href, ok := a.Attr("href"); ok {
				parts := strings.Split(href, "/")
				last := parts[len(parts)-1]
				if m := reCiteNoteRef.FindStringSubmatch(last); m != nil {
					found = strings.ToLower(m[1])
				}
			}
		})
		return found
	}

	// sup[id^="fnref:"]
	if tag := goquery.NodeName(el); tag == "sup" {
		if id, _ := el.Attr("id"); strings.HasPrefix(id, "fnref:") {
			return strings.ToLower(strings.TrimPrefix(id, "fnref:"))
		}
		// sup[id^="fnr"]
		if id, _ := el.Attr("id"); strings.HasPrefix(id, "fnr") {
			return strings.ToLower(strings.TrimPrefix(id, "fnr"))
		}
	}

	// LessWrong / span.footnote-reference
	if el.HasClass("footnote-reference") || el.HasClass("footnote_ref") || el.HasClass("footnote-ref") {
		if fnID, ok := el.Attr("data-footnote-id"); ok && fnID != "" {
			return fnID
		}
		if id, _ := el.Attr("id"); strings.HasPrefix(id, "fnref") {
			return strings.ToLower(strings.TrimPrefix(id, "fnref"))
		}
	}

	// span.footnote-link
	if el.HasClass("footnote-link") {
		if fnID, ok := el.Attr("data-footnote-id"); ok && fnID != "" {
			return fnID
		}
	}

	// a.citation
	if goquery.NodeName(el) == "a" && el.HasClass("citation") {
		return strings.TrimSpace(el.Text())
	}

	// a[id^="fnref"]
	if goquery.NodeName(el) == "a" {
		if id, _ := el.Attr("id"); strings.HasPrefix(id, "fnref") {
			return strings.ToLower(strings.TrimPrefix(id, "fnref"))
		}
	}

	// Default: use href
	if href, ok := el.Attr("href"); ok && href != "" {
		id := strings.TrimPrefix(href, "#")
		return strings.ToLower(id)
	}

	return ""
}

// StandardizeFootnotes rewrites all inline references and footnote definitions
// into the canonical form: <sup id="fnref:N"><a href="#fn:N">N</a></sup> for
// references and <div id="footnotes"><ol><li id="fn:N">…</li></ol></div> for
// definitions. It is the Go port of the TypeScript standardizeFootnotes method.
// TypeScript original code:
//
//	standardizeFootnotes(element: any) {
//	  const footnotes = this.collectFootnotes(element);
//	  const refs = element.querySelectorAll(FOOTNOTE_INLINE_REFERENCES);
//	  const supGroups = new Map();
//	  refs.forEach(el => { ... supGroups / replaceWith ... });
//	  supGroups.forEach((refs, container) => { ... });
//	  // rebuild list, remove originals, append new div#footnotes
//	}
func (p *FootnoteProcessor) StandardizeFootnotes(scope *goquery.Selection) {
	footnotes := p.collectFootnoteDefinitions(scope)
	if len(footnotes) == 0 {
		return
	}

	// supGroups maps a *html.Node (shared <sup> container) to grouped refs.
	// We store the Selection alongside to avoid needing a FindNodes API.
	type supRef struct {
		number int
		refID  string
	}
	type supGroup struct {
		sel  *goquery.Selection
		refs []supRef
	}
	supGroups := make(map[*html.Node]*supGroup)
	supOrder := make([]*html.Node, 0) // preserve insertion order

	scope.Find(FootnoteInlineReferences).Each(func(_ int, el *goquery.Selection) {
		node := el.Get(0)
		if node == nil {
			return
		}

		footnoteID := p.extractInlineFootnoteID(el)
		if footnoteID == "" {
			return
		}

		// Find matching collected footnote by originalID
		var matchNum int
		var matchEntry *footnoteEntry
		for num, entry := range footnotes {
			if entry.originalID == strings.ToLower(footnoteID) {
				matchNum = num
				matchEntry = entry
				break
			}
		}
		if matchEntry == nil {
			return
		}

		// Assign ref ID (e.g. fnref:1 or fnref:1-2 for multi-reference)
		var refID string
		if len(matchEntry.refs) > 0 {
			refID = fmt.Sprintf("fnref:%d-%d", matchNum, len(matchEntry.refs)+1)
		} else {
			refID = fmt.Sprintf("fnref:%d", matchNum)
		}
		matchEntry.refs = append(matchEntry.refs, refID)

		container := p.findOuterFootnoteContainer(el)
		containerNode := container.Get(0)

		if goquery.NodeName(container) == "sup" {
			// Group under the shared sup node
			if _, exists := supGroups[containerNode]; !exists {
				supOrder = append(supOrder, containerNode)
				supGroups[containerNode] = &supGroup{sel: container}
			}
			supGroups[containerNode].refs = append(supGroups[containerNode].refs, supRef{matchNum, refID})
		} else {
			// Replace container directly
			container.ReplaceWithHtml(p.createFootnoteRefHTML(matchNum, refID))
		}
	})

	// Replace each grouped sup with individual <sup> elements
	for _, containerNode := range supOrder {
		g := supGroups[containerNode]
		var b strings.Builder
		for _, r := range g.refs {
			b.WriteString(p.createFootnoteRefHTML(r.number, r.refID))
		}
		g.sel.ReplaceWithHtml(b.String())
	}

	// Remove original footnote lists before appending new ones
	scope.Find(FootnoteListSelectors).Remove()

	// Build new canonical list
	var listHTML strings.Builder
	listHTML.WriteString(`<div id="footnotes"><ol>`)
	for i := 1; i <= len(footnotes); i++ {
		entry, ok := footnotes[i]
		if !ok {
			continue
		}
		listHTML.WriteString(p.createFootnoteItemHTML(i, entry.content, entry.refs))
	}
	listHTML.WriteString(`</ol></div>`)

	scope.AppendHtml(listHTML.String())
}

// StandardizeFootnotes is the public entry point that creates a FootnoteProcessor
// and runs StandardizeFootnotes on the document body (or the document root if
// there is no body element).
// TypeScript original code:
//
//	export function standardizeFootnotes(element: any): void {
//	  const doc = element.ownerDocument;
//	  const handler = new FootnoteHandler(doc);
//	  handler.standardizeFootnotes(element);
//	}
func StandardizeFootnotes(doc *goquery.Document) {
	processor := NewFootnoteProcessor(doc)
	scope := doc.Find("body")
	if scope.Length() == 0 {
		scope = doc.Selection
	}
	processor.StandardizeFootnotes(scope)
}

// StandardizeFootnotesInScope runs footnote standardization on a pre-selected
// scope element rather than the entire document body. Use this when content
// has already been extracted to a specific subtree.
func StandardizeFootnotesInScope(doc *goquery.Document, scope *goquery.Selection) {
	if scope == nil || scope.Length() == 0 {
		StandardizeFootnotes(doc)
		return
	}
	processor := NewFootnoteProcessor(doc)
	processor.StandardizeFootnotes(scope)
}
