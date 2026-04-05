package extractors

import (
	"fmt"
	"strings"
)

// CommentData holds the platform-agnostic fields needed to render a single
// comment in the shared blockquote-nesting algorithm.
type CommentData struct {
	Depth          int
	Author         string
	URL            string // href for the <a class="comment-link"> tag; empty skips the link
	LinkText       string // visible text inside the <a> tag (e.g. date for HN, "N points" for Reddit)
	Date           string // rendered in <span class="comment-date">
	RenderDateSpan bool   // when true, always emit <span class="comment-date"> even if Date is empty
	Content        string // raw HTML of the comment body
	Extra          string // optional extra HTML appended after Date in the metadata block (e.g. HN points span)
}

// renderCommentThread converts a flat, depth-annotated slice of CommentData
// into a nested HTML string using blockquotes to represent reply depth.
//
// Algorithm:
//   - Depth 0 resets the blockquote stack (top-level comment → new thread).
//   - Ascending depth (reply) opens a new <blockquote>.
//   - Descending depth closes blockquotes back to the target level.
//   - Same depth leaves the current blockquote unchanged.
func renderCommentThread(comments []CommentData) string {
	var b strings.Builder
	currentDepth := -1
	var blockquoteStack []int

	for _, c := range comments {
		if c.Depth == 0 {
			// Close all open blockquotes and start a fresh thread.
			for len(blockquoteStack) > 0 {
				b.WriteString("</blockquote>")
				blockquoteStack = blockquoteStack[:len(blockquoteStack)-1]
			}
			b.WriteString("<blockquote>")
			blockquoteStack = []int{0}
		} else {
			if c.Depth < currentDepth {
				// Ascending: close blockquotes until we reach the target depth.
				for len(blockquoteStack) > 0 && blockquoteStack[len(blockquoteStack)-1] >= c.Depth {
					b.WriteString("</blockquote>")
					blockquoteStack = blockquoteStack[:len(blockquoteStack)-1]
				}
			} else if c.Depth > currentDepth {
				// Descending: open a new nesting level.
				b.WriteString("<blockquote>")
				blockquoteStack = append(blockquoteStack, c.Depth)
			}
			// Same depth: no blockquote change.
		}

		b.WriteString(`<div class="comment">`)
		b.WriteString(`<div class="comment-metadata">`)
		fmt.Fprintf(&b, `<span class="comment-author"><strong>%s</strong></span> •`, c.Author)
		if c.URL != "" {
			fmt.Fprintf(&b, ` <a href="%s" class="comment-link">%s</a> •`, c.URL, c.LinkText)
		}
		if c.Date != "" || c.RenderDateSpan {
			fmt.Fprintf(&b, ` <span class="comment-date">%s</span>`, c.Date)
		}
		if c.Extra != "" {
			b.WriteString(c.Extra)
		}
		b.WriteString(`</div>`)
		fmt.Fprintf(&b, `<div class="comment-content">%s</div>`, c.Content)
		b.WriteString(`</div>`)

		currentDepth = c.Depth
	}

	// Close any remaining open blockquotes.
	for len(blockquoteStack) > 0 {
		b.WriteString("</blockquote>")
		blockquoteStack = blockquoteStack[:len(blockquoteStack)-1]
	}

	return b.String()
}
