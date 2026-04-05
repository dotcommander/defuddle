package standardize

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/internal/constants"
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
	attributeCount := 0

	processElement := func(el *goquery.Selection) {
		if el.Length() == 0 {
			return
		}

		node := el.Get(0)

		// Skip SVG elements - preserve all their attributes
		tagName := strings.ToLower(node.Data)
		if tagName == "svg" || node.Namespace == "http://www.w3.org/2000/svg" {
			return
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

			// Preserve code block language classes and footnote backref class
			if attrName == "class" && ((tagName == "code" && strings.HasPrefix(attrValue, "language-")) ||
				attrValue == "footnote-backref") {
				preserveAttribute = true
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
					attributeCount++
				}
			} else {
				// In normal mode, only allow standard attributes
				if !constants.IsAllowedAttribute(attrName) {
					attributesToRemove = append(attributesToRemove, attr.Key)
					attributeCount++
				}
			}
		}

		// Remove unwanted attributes
		for _, attrName := range attributesToRemove {
			el.RemoveAttr(attrName)
		}
	}

	processElement(element)
	element.Find("*").Each(func(_ int, el *goquery.Selection) {
		processElement(el)
	})

	if debug {
		slog.Debug("Stripped attributes", "count", attributeCount)
	}
}

// removeEmptyElements removes empty elements that don't contribute content
// JavaScript original code:
//
//	function removeEmptyElements(element: Element): void {
//		let removedCount = 0;
//		let iterations = 0;
//		let keepRemoving = true;
//
//		while (keepRemoving) {
//			iterations++;
//			keepRemoving = false;
//			// Get all elements without children, working from deepest first
//			const emptyElements = Array.from(element.getElementsByTagName('*')).filter(el => {
//				if (ALLOWED_EMPTY_ELEMENTS.has(el.tagName.toLowerCase())) {
//					return false;
//				}
//
//				// Check if element has only whitespace or &nbsp;
//				const textContent = el.textContent || '';
//				const hasOnlyWhitespace = textContent.trim().length === 0;
//				const hasNbsp = textContent.includes('\u00A0'); // Unicode non-breaking space
//
//				// Check if element has no meaningful children
//				const hasNoChildren = !el.hasChildNodes() ||
//					(Array.from(el.childNodes).every(node => {
//						if (isTextNode(node)) { // TEXT_NODE
//							const nodeText = node.textContent || '';
//							return nodeText.trim().length === 0 && !nodeText.includes('\u00A0');
//						}
//						return false;
//					}));
//
//				// Special case: Check for divs that only contain spans with commas
//				if (el.tagName.toLowerCase() === 'div') {
//					const children = Array.from(el.children);
//					const hasOnlyCommaSpans = children.length > 0 && children.every(child => {
//						if (child.tagName.toLowerCase() !== 'span') return false;
//						const content = child.textContent?.trim() || '';
//						return content === ',' || content === '' || content === ' ';
//					});
//					if (hasOnlyCommaSpans) return true;
//				}
//
//				return hasOnlyWhitespace && !hasNbsp && hasNoChildren;
//			});
//
//			if (emptyElements.length > 0) {
//				emptyElements.forEach(el => {
//					el.remove();
//					removedCount++;
//				});
//				keepRemoving = true;
//			}
//		}
//
//		logDebug('Removed empty elements:', removedCount, 'iterations:', iterations);
//	}
func removeEmptyElements(element *goquery.Selection, debug bool) {
	removedCount := 0
	iterations := 0
	keepRemoving := true

	for keepRemoving {
		iterations++
		keepRemoving = false

		// Get all elements and filter for empty ones, working from deepest first
		var emptyElements []*goquery.Selection

		element.Find("*").Each(func(_ int, el *goquery.Selection) {
			tagName := strings.ToLower(goquery.NodeName(el))

			// Skip allowed empty elements
			if constants.IsAllowedEmptyElement(tagName) {
				return
			}

			// Check if element has only whitespace or &nbsp;
			textContent := el.Text()
			hasOnlyWhitespace := strings.TrimSpace(textContent) == ""
			hasNbsp := strings.Contains(textContent, "\u00A0") // Unicode non-breaking space

			// Check if element has no meaningful children
			hasNoChildren := true
			el.Contents().Each(func(_ int, child *goquery.Selection) {
				if goquery.NodeName(child) == "#text" {
					nodeText := child.Text()
					if strings.TrimSpace(nodeText) != "" || strings.Contains(nodeText, "\u00A0") {
						hasNoChildren = false
					}
				} else {
					hasNoChildren = false
				}
			})

			// If no child nodes at all, it's definitely empty
			if el.Contents().Length() == 0 {
				hasNoChildren = true
			}

			// Special case: Check for divs that only contain spans with commas
			if tagName == "div" {
				children := el.Children()
				if children.Length() > 0 {
					hasOnlyCommaSpans := true
					children.Each(func(_ int, child *goquery.Selection) {
						childTag := strings.ToLower(goquery.NodeName(child))
						if childTag != "span" {
							hasOnlyCommaSpans = false
							return
						}
						content := strings.TrimSpace(child.Text())
						if content != "," && content != "" && content != " " {
							hasOnlyCommaSpans = false
							return
						}
					})
					if hasOnlyCommaSpans {
						emptyElements = append(emptyElements, el)
						return
					}
				}
			}

			// Element is empty if it has only whitespace, no &nbsp;, and no meaningful children
			if hasOnlyWhitespace && !hasNbsp && hasNoChildren {
				emptyElements = append(emptyElements, el)
			}
		})

		// Remove empty elements
		if len(emptyElements) > 0 {
			for _, el := range emptyElements {
				el.Remove()
				removedCount++
			}
			keepRemoving = true
		}
	}

	if debug {
		slog.Debug("Removed empty elements",
			"count", removedCount,
			"iterations", iterations)
	}
}

// removeTrailingHeadings removes headings at the end of content
// JavaScript original code:
//
//	function removeTrailingHeadings(element: Element): void {
//		const hasContentAfter = (el: Element): boolean => {
//			let sibling = el.nextElementSibling;
//			while (sibling) {
//				const text = sibling.textContent?.trim() || '';
//				if (text.length > 0) {
//					return true;
//				}
//				sibling = sibling.nextElementSibling;
//			}
//			return false;
//		};
//
//		const headings = element.querySelectorAll('h1, h2, h3, h4, h5, h6');
//		headings.forEach(heading => {
//			if (!hasContentAfter(heading)) {
//				heading.remove();
//			}
//		});
//	}
func removeTrailingHeadings(element *goquery.Selection) {
	hasContentAfter := func(el *goquery.Selection) bool {
		found := false
		el.NextAll().EachWithBreak(func(_ int, sibling *goquery.Selection) bool {
			if strings.TrimSpace(sibling.Text()) != "" {
				found = true
				return false // stop iteration
			}
			return true
		})
		return found
	}

	element.Find("h1, h2, h3, h4, h5, h6").Each(func(_ int, heading *goquery.Selection) {
		if !hasContentAfter(heading) {
			heading.Remove()
		}
	})
}

// stripExtraBrElements removes excessive br elements
// JavaScript original code:
//
//	function stripExtraBrElements(element: Element): void {
//		// Remove more than 2 consecutive br elements
//		const processBrs = () => {
//			const brs = Array.from(element.querySelectorAll('br'));
//			let consecutiveCount = 0;
//			let toRemove: Element[] = [];
//
//			brs.forEach((br, index) => {
//				const nextSibling = br.nextElementSibling;
//				if (nextSibling && nextSibling.tagName.toLowerCase() === 'br') {
//					consecutiveCount++;
//					if (consecutiveCount >= 2) {
//						toRemove.push(br);
//					}
//				} else {
//					consecutiveCount = 0;
//				}
//			});
//
//			toRemove.forEach(br => br.remove());
//		};
//
//		processBrs();
//	}
func stripExtraBrElements(element *goquery.Selection) {
	// Remove more than 2 consecutive br elements
	var toRemove []*goquery.Selection
	consecutiveCount := 0

	element.Find("br").Each(func(_ int, br *goquery.Selection) {
		next := br.Next()
		if next.Length() > 0 && goquery.NodeName(next) == "br" {
			consecutiveCount++
			if consecutiveCount >= 2 {
				toRemove = append(toRemove, br)
			}
		} else {
			consecutiveCount = 0
		}
	})

	for _, br := range toRemove {
		br.Remove()
	}
}
