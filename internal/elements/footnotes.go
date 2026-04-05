// Package elements provides enhanced element processing functionality
// This module handles footnote processing including detection, linking,
// and accessibility improvements
package elements

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
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
	fmt.Fprintf(&b, `<li id="fn:%d">`, number)

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
		fmt.Fprintf(&b, `<a href="#%s" title="return to article" class="footnote-backref">↩</a>`, refID)
		if i < len(refs)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("</p>")
	b.WriteString("</li>")
	return b.String()
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
