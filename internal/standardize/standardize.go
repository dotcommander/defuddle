// Package standardize provides content standardization functionality for the defuddle content extraction system.
// It converts non-semantic HTML elements to semantic ones and applies standardization rules.
package standardize

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/internal/elements"
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

	// Remove HTML comments (TS order: after spaces, before headings)
	removeHTMLComments(element)

	// Handle H1 elements - remove first one and convert others to H2
	standardizeHeadings(element, metadata.Title, doc)

	// Remove permalink anchors from headings
	removeHeadingAnchors(element)

	// Standardize footnotes and citations (full TS-compatible rewrite)
	elements.StandardizeFootnotesInScope(doc, element)

	// Wrap <code> with white-space:pre not inside <pre>
	wrapPreformattedCode(element)

	// Convert embedded content to standard formats
	standardizeElements(element, doc, debug)

	// Process element-specific enhancements within the extracted content scope
	elements.ProcessCodeBlocksInScope(element, nil)
	elements.ProcessMathInScope(element, nil)
	elements.ProcessHeadingsInScope(element, nil)
	elements.ProcessImagesInScope(element, nil)

	// Unwrap special links (javascript:, block-wrapping, section anchors)
	unwrapSpecialLinks(element, doc)

	// If not debug mode, do the full cleanup
	if !debug {
		// First pass of div flattening
		flattenWrapperElements(element, doc, debug)

		// Strip unwanted attributes
		stripUnwantedAttributes(element, debug)

		// Unwrap bare spans (no attributes) after attribute stripping
		unwrapBareSpans(element)

		// Remove empty elements
		removeEmptyElements(element, debug)

		// Remove obsolete elements (object, embed, applet)
		removeObsoleteElements(element)

		// Remove trailing headings
		removeTrailingHeadings(element)

		// Remove orphaned leading/trailing dividers (pass 1)
		removeOrphanedDividers(element)

		// Final pass of div flattening after cleanup operations
		flattenWrapperElements(element, doc, debug)

		// Remove orphaned dividers again after second flatten (pass 2)
		removeOrphanedDividers(element)

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
