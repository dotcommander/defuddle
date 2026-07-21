package standardize

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
)

var (
	whiteSpacePreRe   = regexp.MustCompile(`white-space\s*:\s*pre`)
	permalinkSymbolRe = regexp.MustCompile(`^[#¶§🔗]$`)
)

// stripUnwantedAttributes removes unwanted attributes from elements
// JavaScript original code:
//
//	function stripUnwantedAttributes(element: Element, debug: boolean): void {
//		let attributeCount = 0;
//
//		const processElement = (el: Element) => {
//			// Skip SVG elements - preserve all their attributes
//			if (el.tagName.toLowerCase() === 'svg' || el.namespaceURI === 'http://www.w3.org/2000/svg') {
//				return;
//			}
//
//			const attributes = Array.from(el.attributes);
//			const tag = el.tagName.toLowerCase();
//
//			attributes.forEach(attr => {
//				const attrName = attr.name.toLowerCase();
//				const attrValue = attr.value;
//
//				// Special cases for preserving specific attributes
//				if (
//					// Preserve footnote IDs
//					(attrName === 'id' && (
//						attrValue.startsWith('fnref:') || // Footnote reference
//						attrValue.startsWith('fn:') || // Footnote content
//						attrValue === 'footnotes' // Footnotes container
//					)) ||
//					// Preserve code block language classes and footnote backref class
//					(attrName === 'class' && (
//						(tag === 'code' && attrValue.startsWith('language-')) ||
//						attrValue === 'footnote-backref'
//					))
//				) {
//					return;
//				}
//
//				// In debug mode, allow debug attributes and data- attributes
//				if (debug) {
//					if (!ALLOWED_ATTRIBUTES.has(attrName) &&
//						!ALLOWED_ATTRIBUTES_DEBUG.has(attrName) &&
//						!attrName.startsWith('data-')) {
//						el.removeAttribute(attr.name);
//						attributeCount++;
//					}
//				} else {
//					// In normal mode, only allow standard attributes
//					if (!ALLOWED_ATTRIBUTES.has(attrName)) {
//						el.removeAttribute(attr.name);
//						attributeCount++;
//					}
//				}
//			});
//		};
//
//		processElement(element);
//		element.querySelectorAll('*').forEach(processElement);
//
//		logDebug('Stripped attributes:', attributeCount);
//	}
func stripUnwantedAttributes(element *goquery.Selection, debug bool) {
	attributeCount := stripElementAttributes(element, debug)
	element.Find("*").Each(func(_ int, el *goquery.Selection) {
		attributeCount += stripElementAttributes(el, debug)
	})

	if debug {
		slog.Debug("Stripped attributes", "count", attributeCount)
	}
}

// stripElementAttributes removes non-allowed attributes from el and returns the
// count removed. It preserves footnote ids, code language / footnote-backref /
// callout classes, plus (in debug mode) data- and debug attributes. SVG elements
// keep all their attributes.
func stripElementAttributes(el *goquery.Selection, debug bool) int {
	if el.Length() == 0 {
		return 0
	}

	node := el.Get(0)

	// Skip SVG elements - preserve all their attributes
	tagName := strings.ToLower(node.Data)
	if tagName == "svg" || node.Namespace == "http://www.w3.org/2000/svg" {
		return 0
	}

	// Get all attributes and process them
	var attributesToRemove []string
	for _, attr := range node.Attr {
		attrName := strings.ToLower(attr.Key)
		attrValue := attr.Val

		// Special cases for preserving specific attributes
		preserveAttribute := false

		// Preserve footnote IDs
		if attrName == "id" && (strings.HasPrefix(attrValue, "fnref:") || // Footnote reference
			strings.HasPrefix(attrValue, "fn:") || // Footnote content
			attrValue == "footnotes") { // Footnotes container
			preserveAttribute = true
		}

		// Preserve code block language classes, footnote backref class, and callout classes
		if attrName == "class" {
			if (tagName == "code" && strings.HasPrefix(attrValue, "language-")) ||
				attrValue == "footnote-backref" ||
				hasCalloutClass(attrValue) {
				preserveAttribute = true
			}
		}

		if preserveAttribute {
			continue
		}

		// In debug mode, allow debug attributes and data- attributes
		if debug {
			if !constants.IsAllowedAttribute(attrName) &&
				!constants.IsAllowedAttributeDebug(attrName) &&
				!strings.HasPrefix(attrName, "data-") {
				attributesToRemove = append(attributesToRemove, attr.Key)
			}
		} else {
			// In normal mode, only allow standard attributes
			if !constants.IsAllowedAttribute(attrName) {
				attributesToRemove = append(attributesToRemove, attr.Key)
			}
		}
	}

	// Remove unwanted attributes
	for _, attrName := range attributesToRemove {
		el.RemoveAttr(attrName)
	}
	return len(attributesToRemove)
}
