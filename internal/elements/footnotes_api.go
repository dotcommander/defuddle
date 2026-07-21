package elements

import "github.com/PuerkitoBio/goquery"

// ProcessFootnotes processes all footnotes in the document (public interface).
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
