// Package standardize provides content standardization functionality for the defuddle content extraction system.
// It converts non-semantic HTML elements to semantic ones and applies standardization rules.
package standardize

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/internal/metadata"
)

// Content standardizes and cleans up the main content element
// JavaScript original code:
//
//	export function standardizeContent(element: Element, metadata: DefuddleMetadata, doc: Document, debug: boolean = false): void {
//		standardizeSpaces(element);
//
//		// Remove HTML comments
//		removeHTMLComments(element);
//
//		// Handle H1 elements - remove first one and convert others to H2
//		standardizeHeadings(element, metadata.title, doc);
//
//		// Standardize footnotes and citations
//		standardizeFootnotes(element);
//
//		// Convert embedded content to standard formats
//		standardizeElements(element, doc);
//
//		// If not debug mode, do the full cleanup
//		if (!debug) {
//			// First pass of div flattening
//			flattenWrapperElements(element, doc);
//
//			// Strip unwanted attributes
//			stripUnwantedAttributes(element, debug);
//
//			// Remove empty elements
//			removeEmptyElements(element);
//
//			// Remove trailing headings
//			removeTrailingHeadings(element);
//
//			// Final pass of div flattening after cleanup operations
//			flattenWrapperElements(element, doc);
//
//			// Standardize consecutive br elements
//			stripExtraBrElements(element);
//
//			// Clean up empty lines
//			removeEmptyLines(element, doc);
//		} else {
//			// In debug mode, still do basic cleanup but preserve structure
//			stripUnwantedAttributes(element, debug);
//			removeTrailingHeadings(element);
//			stripExtraBrElements(element);
//			logDebug('Debug mode: Skipping div flattening to preserve structure');
//		}
//	}
func Content(element *goquery.Selection, metadata *metadata.Metadata, doc *goquery.Document, debug bool) {
	standardizeSpaces(element)

	// Handle H1 elements - remove first one and convert others to H2
	standardizeHeadings(element, metadata.Title, doc)

	// Standardize footnotes and citations
	standardizeFootnotes(element)

	// Convert embedded content to standard formats
	standardizeElements(element, doc, debug)

	// If not debug mode, do the full cleanup
	if !debug {
		// First pass of div flattening
		flattenWrapperElements(element, doc, debug)

		// Strip unwanted attributes
		stripUnwantedAttributes(element, debug)

		// Remove empty elements
		removeEmptyElements(element, debug)

		// Remove trailing headings
		removeTrailingHeadings(element)

		// Final pass of div flattening after cleanup operations
		flattenWrapperElements(element, doc, debug)

		// Standardize consecutive br elements
		stripExtraBrElements(element)

		// Clean up empty lines
		removeEmptyLines(element, doc, debug)
	} else {
		// In debug mode, still do basic cleanup but preserve structure
		stripUnwantedAttributes(element, debug)
		removeTrailingHeadings(element)
		stripExtraBrElements(element)
		// Debug mode: Skipping div flattening to preserve structure
	}
}
