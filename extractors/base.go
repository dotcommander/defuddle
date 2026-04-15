// Package extractors provides site-specific content extraction functionality.
package extractors

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// genericMessageFallbacks are selector patterns shared by conversation extractors
// when their primary selectors find no elements. Ordered from most- to least-specific.
var genericMessageFallbacks = []string{
	`div[data-testid*="message"]`,
	`.message`,
	`div[class*="message"]`,
	`div[class*="chat"]`,
	`div[role="article"]`,
	`article`,
}

// firstMatchingSelection tries each selector in order against doc and returns the
// first non-empty result. Returns an empty selection if none match.
func firstMatchingSelection(doc *goquery.Document, selectors []string) *goquery.Selection {
	for _, sel := range selectors {
		if found := doc.Find(sel); found.Length() > 0 {
			return found
		}
	}
	return doc.Find("__no_match__") // empty selection
}

// whitespaceRe is a shared pre-compiled regex for collapsing whitespace runs.
var whitespaceRe = regexp.MustCompile(`\s+`)

// ExtractorResult represents the result of content extraction
// Corresponding to TypeScript interface ExtractorResult
type ExtractorResult struct {
	Content          string            `json:"content"`
	ContentHTML      string            `json:"contentHtml"`
	ExtractedContent map[string]any    `json:"extractedContent,omitempty"`
	Variables        map[string]string `json:"variables,omitempty"`
}

// BaseExtractor defines the interface for site-specific extractors
// TypeScript original code:
//
//	export abstract class BaseExtractor {
//		protected document: Document;
//		protected url: string;
//		protected schemaOrgData?: any;
//
//		constructor(document: Document, url: string, schemaOrgData?: any) {
//			this.document = document;
//			this.url = url;
//			this.schemaOrgData = schemaOrgData;
//		}
//
//		abstract canExtract(): boolean;
//		abstract extract(): ExtractorResult;
//		abstract getName(): string;
//	}
type BaseExtractor interface {
	CanExtract() bool
	Extract() *ExtractorResult
	Name() string
}

// ExtractorBase provides common functionality for extractors
// Implementation of the protected properties in TypeScript BaseExtractor
type ExtractorBase struct {
	document      *goquery.Document
	url           string
	schemaOrgData any
}

// NewExtractorBase creates a new base extractor
// TypeScript original code:
//
//	constructor(document: Document, url: string, schemaOrgData?: any) {
//		this.document = document;
//		this.url = url;
//		this.schemaOrgData = schemaOrgData;
//	}
func NewExtractorBase(document *goquery.Document, url string, schemaOrgData any) *ExtractorBase {
	return &ExtractorBase{
		document:      document,
		url:           url,
		schemaOrgData: schemaOrgData,
	}
}

// GetDocument returns the document
func (e *ExtractorBase) GetDocument() *goquery.Document {
	return e.document
}

// GetURL returns the URL
func (e *ExtractorBase) GetURL() string {
	return e.url
}

// GetSchemaOrgData returns the schema.org data
func (e *ExtractorBase) GetSchemaOrgData() any {
	return e.schemaOrgData
}
