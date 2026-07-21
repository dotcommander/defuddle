// Package elements provides enhanced element processing functionality
// This module handles footnote processing including detection, linking,
// and accessibility improvements
package elements

import (
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
