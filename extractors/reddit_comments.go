package extractors

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// extractComments extracts comments from the page
// TypeScript original code:
//
//	private extractComments(): string {
//		const comments = Array.from(this.document.querySelectorAll('shreddit-comment'));
//		return this.processComments(comments);
//	}
func (r *RedditExtractor) extractComments() string {
	var comments []*goquery.Selection

	// Primary method: Look for shreddit-comment elements
	r.document.Find("shreddit-comment").Each(func(_ int, s *goquery.Selection) {
		comments = append(comments, s)
	})

	// Fallback method: Look for alternative comment selectors
	if len(comments) == 0 {
		slog.Debug("Reddit extractor: using fallback comment selectors")

		redditCommentFallbacks := []string{
			"div[data-testid='comment']",
			".comment",
			".comment-area .comment",
			"div[data-click-id='text']",
			"div[data-click-id='body']",
			"div[id^='thing_t3_']", // Reddit post format
			".thing.link",          // Old Reddit format
		}

		if sel := firstMatchingSelection(r.document, redditCommentFallbacks); sel.Length() > 0 {
			sel.Each(func(_ int, s *goquery.Selection) {
				comments = append(comments, s)
			})
			slog.Debug("Reddit extractor: found comments with fallback", "count", len(comments))
		}
	}

	slog.Debug("Reddit extractor: found comments", "commentCount", len(comments))

	if len(comments) == 0 {
		return ""
	}

	return r.processComments(comments)
}

// processComments processes the comments with proper nesting
// TypeScript original code:
//
//	private processComments(comments: Element[]): string {
//		let html = '';
//		let currentDepth = -1;
//		let blockquoteStack: number[] = []; // Keep track of open blockquotes at each depth
//
//		for (const comment of comments) {
//			const depth = parseInt(comment.getAttribute('depth') || '0');
//			const author = comment.getAttribute('author') || '';
//			const score = comment.getAttribute('score') || '0';
//			const permalink = comment.getAttribute('permalink') || '';
//			const content = comment.querySelector('[slot="comment"]')?.innerHTML || '';
//
//			// Get timestamp from faceplate-timeago element
//			const timeElement = comment.querySelector('faceplate-timeago');
//			const timestamp = timeElement?.getAttribute('ts') || '';
//			const date = timestamp ? new Date(timestamp).toISOString().split('T')[0] : '';
//
//			// For top-level comments, close all previous blockquotes and start fresh
//			if (depth === 0) {
//				// Close all open blockquotes
//				while (blockquoteStack.length > 0) {
//					html += '</blockquote>';
//					blockquoteStack.pop();
//				}
//				html += '<blockquote>';
//				blockquoteStack = [0];
//				currentDepth = 0;
//			}
//			// For nested comments
//			else {
//				// If we're moving back up the tree
//				if (depth < currentDepth) {
//					// Close blockquotes until we reach the current depth
//					while (blockquoteStack.length > 0 && blockquoteStack[blockquoteStack.length - 1] >= depth) {
//						html += '</blockquote>';
//						blockquoteStack.pop();
//					}
//				}
//				// If we're going deeper
//				else if (depth > currentDepth) {
//					html += '<blockquote>';
//					blockquoteStack.push(depth);
//				}
//				// If we're at the same depth, no need to close or open blockquotes
//			}
//
//			html += `<div class="comment">
//	<div class="comment-metadata">
//		<span class="comment-author"><strong>${author}</strong></span> •
//		<a href="https://reddit.com${permalink}" class="comment-link">${score} points</a> •
//		<span class="comment-date">${date}</span>
//	</div>
//	<div class="comment-content">${content}</div>
//
// </div>`;
//
//			currentDepth = depth;
//		}
//
//		// Close any remaining blockquotes
//		while (blockquoteStack.length > 0) {
//			html += '</blockquote>';
//			blockquoteStack.pop();
//		}
//
//		return html;
//	}
func (r *RedditExtractor) processComments(comments []*goquery.Selection) string {
	slog.Debug("Reddit extractor: processing comments", "totalComments", len(comments))

	data := make([]CommentData, 0, len(comments))
	for _, comment := range comments {
		depthStr, _ := comment.Attr("depth")
		depth, _ := strconv.Atoi(depthStr)

		author, _ := comment.Attr("author")
		score, _ := comment.Attr("score")
		permalink, _ := comment.Attr("permalink")

		contentElement := comment.Find(`[slot="comment"]`).First()
		content, _ := contentElement.Html()

		// Get timestamp from faceplate-timeago element.
		timeElement := comment.Find("faceplate-timeago").First()
		timestamp, _ := timeElement.Attr("ts")

		var date string
		if timestamp != "" {
			// Try Unix timestamp (integer milliseconds or seconds).
			if ts, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
				if ts > 1e12 {
					// Milliseconds
					date = time.Unix(ts/1000, 0).Format("2006-01-02")
				} else {
					date = time.Unix(ts, 0).Format("2006-01-02")
				}
			} else if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
				// ISO 8601 string
				date = t.Format("2006-01-02")
			} else if t, err := time.Parse("2006-01-02T15:04:05", timestamp); err == nil {
				// ISO 8601 without timezone
				date = t.Format("2006-01-02")
			}
		}

		data = append(data, CommentData{
			Depth:          depth,
			Author:         author,
			URL:            "https://reddit.com" + permalink,
			LinkText:       score + " points",
			Date:           date,
			RenderDateSpan: true,
			Content:        content,
		})
	}

	slog.Debug("Reddit extractor: comments processed", "processedCount", len(comments))
	return renderCommentThread(data)
}
