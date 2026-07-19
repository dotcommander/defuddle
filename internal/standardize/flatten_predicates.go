package standardize

import (
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
)

// hasDirectInlineContent reports whether el directly contains non-empty text or
// an inline element (i.e. el is not a pure block wrapper).
func hasDirectInlineContent(el *goquery.Selection) bool {
	hasInlineContent := false
	el.Contents().Each(func(_ int, child *goquery.Selection) {
		if goquery.NodeName(child) == "#text" {
			text := strings.TrimSpace(child.Text())
			if text != "" {
				hasInlineContent = true
			}
		} else {
			tagName := goquery.NodeName(child)
			if constants.IsInlineElement(tagName) {
				hasInlineContent = true
			}
		}
	})
	return hasInlineContent
}

// shouldPreserveElement reports whether el must not be flattened: a preserve-tag,
// a semantic role/class, or an element containing preserved children.
func shouldPreserveElement(el *goquery.Selection) bool {
	tagName := goquery.NodeName(el)

	// Check if element should be preserved
	if constants.IsPreserveElement(tagName) {
		return true
	}

	// Check for semantic roles
	role, _ := el.Attr("role")
	semanticRoles := []string{"article", "main", "navigation", "banner", "contentinfo"}
	if slices.Contains(semanticRoles, role) {
		return true
	}

	// Check for semantic classes
	className := strings.ToLower(el.AttrOr("class", ""))
	if semanticClassRe.MatchString(className) {
		return true
	}

	// Check if element contains mixed content types that should be preserved
	hasPreservedElements := false
	el.Children().Each(func(_ int, child *goquery.Selection) {
		childTag := goquery.NodeName(child)
		childRole, _ := child.Attr("role")
		childClass := strings.ToLower(child.AttrOr("class", ""))

		if constants.IsPreserveElement(childTag) ||
			childRole == "article" ||
			semanticClassRe.MatchString(childClass) {
			hasPreservedElements = true
		}
	})

	return hasPreservedElements
}

// isWrapperElement reports whether el is a structural wrapper (no direct inline
// content; empty, only block children, or a known wrapper class) and may be flattened.
func isWrapperElement(el *goquery.Selection, blockElements []string) bool {
	// If it directly contains inline content, it's NOT a wrapper
	if hasDirectInlineContent(el) {
		return false
	}

	// Check if it's just empty space
	text := strings.TrimSpace(el.Text())
	if text == "" {
		return true
	}

	// Check if it only contains other block elements
	children := el.Children()
	if children.Length() == 0 {
		return true
	}

	// Check if all children are block elements
	allBlockElements := true

	children.Each(func(_ int, child *goquery.Selection) {
		tag := goquery.NodeName(child)
		isBlock := slices.Contains(blockElements, tag)

		// Check additional block elements
		if !isBlock {
			if slices.Contains(additionalBlockElements, tag) {
				isBlock = true
			}
		}

		if !isBlock {
			allBlockElements = false
		}
	})

	if allBlockElements {
		return true
	}

	// Check for common wrapper patterns
	className := strings.ToLower(el.AttrOr("class", ""))
	if wrapperClassRe.MatchString(className) {
		return true
	}

	// Check if it has excessive whitespace or empty text nodes
	hasTextContent := false
	el.Contents().Each(func(_ int, child *goquery.Selection) {
		if goquery.NodeName(child) == "#text" {
			childText := strings.TrimSpace(child.Text())
			if childText != "" {
				hasTextContent = true
			}
		}
	})

	if !hasTextContent {
		return true
	}

	// Check if it only contains block elements (different check)
	hasOnlyBlockElements := children.Length() > 0

	children.Each(func(_ int, child *goquery.Selection) {
		tag := goquery.NodeName(child)
		if constants.IsInlineElement(tag) {
			hasOnlyBlockElements = false
		}
	})

	return hasOnlyBlockElements
}
