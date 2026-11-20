package standardize

import (
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
	"golang.org/x/net/html"
)

// containerBlockElements are structural table/list container elements that must
// be treated as block-level containers for whitespace normalization so
// ensureInlineSpacing does not inject spaces between their children.
var containerBlockElements = []string{
	"li", "td", "th", "tr", "table", "tbody", "thead", "tfoot", "dl", "dt", "dd",
}

// cleanupEmptyElements is the second pass: clean up empty elements and normalize
// spacing. removedCount accumulates removals (diagnostic only).
func cleanupEmptyElements(node *html.Node, blockElements []string, removedCount *int) {
	if node.Type != html.ElementNode {
		return
	}

	// Skip pre, code, and math elements
	tag := strings.ToLower(node.Data)
	if tag == "pre" || tag == "code" || tag == "math" {
		return
	}

	// Process children first (depth-first)
	var children []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			children = append(children, child)
		}
	}
	for _, child := range children {
		cleanupEmptyElements(child, blockElements, removedCount)
	}

	// Determine if this is a block element (simplified check)
	isBlockElement := slices.Contains(blockElements, tag) ||
		slices.Contains(additionalBlockElements, tag) ||
		slices.Contains(containerBlockElements, tag)

	// Only remove empty text nodes at the start and end if they contain just newlines/tabs
	// For block elements, also remove spaces
	var startPattern, endPattern *regexp.Regexp
	if isBlockElement {
		startPattern = blockStartSpaceRe
		endPattern = blockStartSpaceRe
	} else {
		startPattern = inlineStartSpaceRe
		endPattern = inlineStartSpaceRe
	}

	// Remove empty text nodes at start
	for node.FirstChild != nil &&
		node.FirstChild.Type == html.TextNode &&
		startPattern.MatchString(node.FirstChild.Data) {
		node.RemoveChild(node.FirstChild)
		*removedCount++
	}

	// Remove empty text nodes at end
	for node.LastChild != nil &&
		node.LastChild.Type == html.TextNode &&
		endPattern.MatchString(node.LastChild.Data) {
		node.RemoveChild(node.LastChild)
		*removedCount++
	}

	// Ensure there's a space between inline elements if needed
	if !isBlockElement {
		ensureInlineSpacing(node)
	}
}

// ensureInlineSpacing inserts a single space between adjacent inline children
// of node where one side is an element and no existing space or adjoining
// punctuation would make that space wrong.
func ensureInlineSpacing(node *html.Node) {
	var nodeChildren []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		nodeChildren = append(nodeChildren, child)
	}

	for i := range len(nodeChildren) - 1 {
		current := nodeChildren[i]
		next := nodeChildren[i+1]

		// Only add space between elements or between element and text
		if current.Type == html.ElementNode || next.Type == html.ElementNode {
			// Do not insert spaces before superscripts/subscripts (e.g. footnote refs <a>foo</a><sup>1</sup>)
			if next.Type == html.ElementNode {
				nextTag := strings.ToLower(next.Data)
				if nextTag == "sup" || nextTag == "sub" {
					continue
				}
			}

			// Extract full text content so punctuation/whitespace checks work on ElementNodes too.
			currentContent := extractSubtreeText(current)
			nextContent := extractSubtreeText(next)

			// Don't add space if:
			// 1. Next content starts with punctuation or closing parenthesis
			// 2. Current content ends with punctuation or opening parenthesis
			// 3. There's already whitespace at the boundary
			nextStartsWithPunctuation := startsWithPunctRe.MatchString(nextContent)
			currentEndsWithPunctuation := endsWithPunctRe.MatchString(currentContent)

			hasSpace := endsWithSpace(currentContent) || startsWithSpace(nextContent)

			// Only add space if none of the above conditions are true
			if !nextStartsWithPunctuation &&
				!currentEndsWithPunctuation &&
				!hasSpace {
				space := &html.Node{
					Type: html.TextNode,
					Data: " ",
				}
				node.InsertBefore(space, next)
			}
		}
	}
}

func extractSubtreeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(extractSubtreeText(c))
	}
	return b.String()
}

func startsWithSpace(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsSpace(r)
}

func endsWithSpace(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return unicode.IsSpace(r)
}

// removeEmptyLines runs the two whitespace-normalization passes over element.
func removeEmptyLines(element *goquery.Selection, _ *goquery.Document, debug bool) {
	removedCount := 0
	startTime := time.Now()
	blockElements := constants.GetBlockElements()

	// Run both passes
	element.Each(func(_ int, sel *goquery.Selection) {
		if sel.Length() > 0 {
			removeEmptyTextNodes(sel.Get(0), &removedCount)
		}
	})

	element.Each(func(_ int, sel *goquery.Selection) {
		if sel.Length() > 0 {
			cleanupEmptyElements(sel.Get(0), blockElements, &removedCount)
		}
	})

	endTime := time.Now()
	processingTime := float64(endTime.Sub(startTime).Nanoseconds()) / 1e6 // Convert to milliseconds
	if debug {
		slog.Debug("Removed empty lines",
			"charactersRemoved", removedCount,
			"processingTime", processingTime)
	}
}
