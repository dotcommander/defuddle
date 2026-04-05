package standardize

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/internal/constants"
	"golang.org/x/net/html"
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
	// hasContentAfter checks siblings (including text nodes) and climbs to parent,
	// matching the TS implementation that walks all sibling nodes and recurses up.
	var hasContentAfter func(el *goquery.Selection) bool
	hasContentAfter = func(el *goquery.Selection) bool {
		// Check all following sibling nodes (elements AND text nodes)
		if len(el.Nodes) > 0 {
			for sib := el.Nodes[0].NextSibling; sib != nil; sib = sib.NextSibling {
				if sib.Type == html.ElementNode {
					sibDoc := goquery.NewDocumentFromNode(sib)
					if strings.TrimSpace(sibDoc.Text()) != "" {
						return true
					}
				} else if sib.Type == html.TextNode {
					if strings.TrimSpace(sib.Data) != "" {
						return true
					}
				}
			}
		}
		// Climb to parent and check its following siblings
		parent := el.Parent()
		if parent.Length() > 0 && parent.Nodes[0] != element.Nodes[0] {
			return hasContentAfter(parent)
		}
		return false
	}

	// Process headings in reverse order (deepest/last first) and break
	// after finding the first heading with content after it.
	headings := element.Find("h1, h2, h3, h4, h5, h6")
	nodes := make([]*goquery.Selection, 0, headings.Length())
	headings.Each(func(_ int, h *goquery.Selection) {
		nodes = append(nodes, h)
	})
	for i := len(nodes) - 1; i >= 0; i-- {
		if hasContentAfter(nodes[i]) {
			break
		}
		nodes[i].Remove()
	}
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

// hasCalloutClass checks if a class attribute value contains a callout class (callout or callout-*).
func hasCalloutClass(classValue string) bool {
	for _, c := range strings.Fields(classValue) {
		if c == "callout" || strings.HasPrefix(c, "callout-") {
			return true
		}
	}
	return false
}

// removeHtmlComments removes all HTML comment nodes from the element tree.
func removeHtmlComments(element *goquery.Selection) {
	if element.Length() == 0 {
		return
	}
	removeCommentsFromNode(element.Get(0))
}

func removeCommentsFromNode(n *html.Node) {
	var toRemove []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.CommentNode {
			toRemove = append(toRemove, c)
		} else {
			removeCommentsFromNode(c)
		}
	}
	for _, c := range toRemove {
		n.RemoveChild(c)
	}
}

// unwrapBareSpans removes attribute-free <span> elements, keeping their children.
// Processes deepest-first so nested bare spans collapse in one pass.
func unwrapBareSpans(element *goquery.Selection) {
	spans := element.Find("span")
	if spans.Length() == 0 {
		return
	}

	// Collect in reverse order (deepest first)
	collected := make([]*html.Node, 0, spans.Length())
	spans.Each(func(_ int, s *goquery.Selection) {
		collected = append(collected, s.Get(0))
	})
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}

	for _, node := range collected {
		if node.Parent == nil {
			continue
		}
		// Skip spans with attributes
		if len(node.Attr) > 0 {
			continue
		}
		// Move children before the span, then remove the span
		parent := node.Parent
		for node.FirstChild != nil {
			child := node.FirstChild
			node.RemoveChild(child)
			parent.InsertBefore(child, node)
		}
		parent.RemoveChild(node)
	}
}

// unwrapSpecialLinks fixes problematic link structures:
// 1. Removes <a> inside <code> (markdown can't render links in backtick code)
// 2. Unwraps javascript: links (keep text, remove the link)
// 3. Restructures block-wrapping links containing headings into heading-wrapping links
// 4. Unwraps anchor links that wrap headings (clickable section headers)
func unwrapSpecialLinks(element *goquery.Selection, doc *goquery.Document) {
	// 1. Unwrap links inside inline code
	element.Find("code a").Each(func(_ int, a *goquery.Selection) {
		unwrapSelection(a)
	})

	// 2. Unwrap javascript: links
	element.Find(`a[href^="javascript:"]`).Each(func(_ int, a *goquery.Selection) {
		unwrapSelection(a)
	})

	// 3. Restructure block-wrapping links containing headings
	element.Find("a").Each(func(_ int, link *goquery.Selection) {
		href, exists := link.Attr("href")
		if !exists || href == "" || strings.HasPrefix(href, "#") {
			return
		}
		// Find a heading child
		var headingNode *html.Node
		link.Children().Each(func(_ int, child *goquery.Selection) {
			tag := strings.ToUpper(goquery.NodeName(child))
			if len(tag) == 2 && tag[0] == 'H' && tag[1] >= '1' && tag[1] <= '6' {
				headingNode = child.Get(0)
			}
		})
		if headingNode == nil {
			return
		}

		// Create inner <a> with the href, move heading's children into it
		innerLink := &html.Node{
			Type: html.ElementNode,
			Data: "a",
			Attr: []html.Attribute{{Key: "href", Val: href}},
		}
		for headingNode.FirstChild != nil {
			child := headingNode.FirstChild
			headingNode.RemoveChild(child)
			innerLink.AppendChild(child)
		}
		headingNode.AppendChild(innerLink)

		// Unwrap the outer <a>
		unwrapSelection(link)
	})

	// 4. Unwrap anchor links wrapping headings
	element.Find(`a[href^="#"]`).Each(func(_ int, link *goquery.Selection) {
		if link.Find("h1, h2, h3, h4, h5, h6").Length() > 0 {
			unwrapSelection(link)
		}
	})
}

// unwrapSelection replaces a selection with its children (equivalent to TS unwrapElement).
func unwrapSelection(sel *goquery.Selection) {
	if sel.Length() == 0 {
		return
	}
	node := sel.Get(0)
	parent := node.Parent
	if parent == nil {
		return
	}
	for node.FirstChild != nil {
		child := node.FirstChild
		node.RemoveChild(child)
		parent.InsertBefore(child, node)
	}
	parent.RemoveChild(node)
}

// removeHeadingAnchors removes permalink anchors from inside heading elements.
// Handles symbols (#, ¶, §, 🔗), empty links, and class-based anchors.
func removeHeadingAnchors(element *goquery.Selection) {
	element.Find("h1 a, h2 a, h3 a, h4 a, h5 a, h6 a").Each(func(_ int, link *goquery.Selection) {
		if isPermalinkAnchor(link) {
			link.Remove()
		}
	})
}

func isPermalinkAnchor(link *goquery.Selection) bool {
	if goquery.NodeName(link) != "a" {
		return false
	}
	href := link.AttrOr("href", "")
	title := strings.ToLower(link.AttrOr("title", ""))
	className := strings.ToLower(link.AttrOr("class", ""))
	text := strings.TrimSpace(link.Text())

	if strings.HasPrefix(href, "#") || strings.Contains(href, "#") {
		return true
	}
	if strings.Contains(title, "permalink") {
		return true
	}
	if strings.Contains(className, "permalink") || strings.Contains(className, "heading-anchor") || strings.Contains(className, "anchor-link") {
		return true
	}
	if permalinkSymbolRe.MatchString(text) {
		return true
	}
	return false
}

// removeObsoleteElements removes <object>, <embed>, and <applet> elements.
func removeObsoleteElements(element *goquery.Selection) {
	element.Find("object, embed, applet").Remove()
}

// removeOrphanedDividers removes leading and trailing <hr> elements,
// skipping whitespace-only text nodes.
func removeOrphanedDividers(element *goquery.Selection) {
	if element.Length() == 0 {
		return
	}
	node := element.Get(0)

	// Remove leading <hr> elements
	for {
		n := node.FirstChild
		for n != nil && n.Type == html.TextNode && strings.TrimSpace(n.Data) == "" {
			n = n.NextSibling
		}
		if n != nil && n.Type == html.ElementNode && strings.EqualFold(n.Data, "hr") {
			node.RemoveChild(n)
		} else {
			break
		}
	}

	// Remove trailing <hr> elements
	for {
		n := node.LastChild
		for n != nil && n.Type == html.TextNode && strings.TrimSpace(n.Data) == "" {
			n = n.PrevSibling
		}
		if n != nil && n.Type == html.ElementNode && strings.EqualFold(n.Data, "hr") {
			node.RemoveChild(n)
		} else {
			break
		}
	}
}

// wrapPreformattedCode wraps <code> elements with white-space:pre style
// in <pre> elements if they aren't already inside one.
func wrapPreformattedCode(element *goquery.Selection) {
	element.Find("code").Each(func(_ int, code *goquery.Selection) {
		// Skip if already inside a <pre>
		if code.Closest("pre").Length() > 0 {
			return
		}
		style := code.AttrOr("style", "")
		if !whiteSpacePreRe.MatchString(style) {
			return
		}
		// Wrap in <pre>
		codeNode := code.Get(0)
		parent := codeNode.Parent
		if parent == nil {
			return
		}
		pre := &html.Node{
			Type: html.ElementNode,
			Data: "pre",
		}
		parent.InsertBefore(pre, codeNode)
		parent.RemoveChild(codeNode)
		pre.AppendChild(codeNode)
	})
}
