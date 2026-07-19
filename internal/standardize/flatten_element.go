package standardize

import (
	"html"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
)

func flattenElement(el, root *goquery.Selection, blockElements []string) bool {
	// Skip processing if element has been removed or should be preserved
	if el.Length() == 0 || shouldPreserveElement(el) {
		return false
	}

	tagName := goquery.NodeName(el)

	// Case 1: Element is truly empty (no text content, no child elements) and not self-closing
	isAllowedEmpty := constants.IsAllowedEmptyElement(tagName)

	if !isAllowedEmpty && el.Children().Length() == 0 && strings.TrimSpace(el.Text()) == "" {
		el.Remove()
		return true
	}

	// Case 2: Top-level element - be more aggressive
	if el.Parent().Length() > 0 && el.Parent().Get(0) == root.Get(0) {
		children := el.Children()
		hasOnlyBlockElements := children.Length() > 0

		children.Each(func(_ int, child *goquery.Selection) {
			if constants.IsInlineElement(goquery.NodeName(child)) {
				hasOnlyBlockElements = false
			}
		})

		if hasOnlyBlockElements {
			html, _ := el.Html()
			el.ReplaceWithHtml(html)
			return true
		}
	}

	// Case 3: Wrapper element - merge up aggressively
	if isWrapperElement(el, blockElements) {
		// Special case: if element only contains block elements, merge them up
		children := el.Children()
		onlyBlockElements := true

		children.Each(func(_ int, child *goquery.Selection) {
			if constants.IsInlineElement(goquery.NodeName(child)) {
				onlyBlockElements = false
			}
		})

		if onlyBlockElements {
			html, _ := el.Html()
			el.ReplaceWithHtml(html)
			return true
		}

		// Otherwise handle as normal wrapper
		html, _ := el.Html()
		el.ReplaceWithHtml(html)
		return true
	}

	// Case 4: Element only contains text and/or inline elements - convert to paragraph
	hasOnlyInlineOrText := true
	hasContent := false

	el.Contents().Each(func(_ int, child *goquery.Selection) {
		if goquery.NodeName(child) == "#text" {
			text := strings.TrimSpace(child.Text())
			if text != "" {
				hasContent = true
			}
		} else {
			tag := goquery.NodeName(child)
			if !constants.IsInlineElement(tag) {
				hasOnlyInlineOrText = false
			}
		}
	})

	if hasOnlyInlineOrText && hasContent {
		html, _ := el.Html()
		el.ReplaceWithHtml("<p>" + html + "</p>")
		return true
	}

	// Case 5: Element has single child - unwrap only if child is block-level
	children := el.Children()
	if children.Length() == 1 {
		child := children.First()
		childTag := goquery.NodeName(child)

		// Only unwrap if the single child is a block element and not preserved
		isBlockChild := slices.Contains(blockElements, childTag)

		if isBlockChild && !shouldPreserveElement(child) {
			// Build opening tag preserving child's attributes
			var attrStr strings.Builder
			for _, a := range child.Nodes[0].Attr {
				attrStr.WriteString(" " + a.Key + `="` + html.EscapeString(a.Val) + `"`)
			}
			childHTML, _ := child.Html()
			el.ReplaceWithHtml("<" + childTag + attrStr.String() + ">" + childHTML + "</" + childTag + ">")
			return true
		}
	}

	// Case 6: Deeply nested element - merge up
	nestingDepth := 0
	parent := el.Parent()

	for parent.Length() > 0 {
		parentTag := goquery.NodeName(parent)
		if slices.Contains(blockElements, parentTag) {
			nestingDepth++
		}
		parent = parent.Parent()
	}

	// Only unwrap if nested AND does not contain direct inline content
	if nestingDepth > 0 && !hasDirectInlineContent(el) {
		html, _ := el.Html()
		el.ReplaceWithHtml(html)
		return true
	}

	return false
}
