package extractors

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// processComments processes the comments with proper nesting
// TypeScript original code:
//
//	private processComments(comments: Element[]): string {
//		let html = '';
//		const processedIds = new Set<string>();
//		let currentDepth = -1;
//		let blockquoteStack: number[] = [];
//
//		for (const comment of comments) {
//			const id = comment.getAttribute('id');
//			if (!id || processedIds.has(id)) continue;
//			processedIds.add(id);
//
//			const indent = comment.querySelector('.ind img')?.getAttribute('width') || '0';
//			const depth = parseInt(indent) / 40;
//			const commentText = comment.querySelector('.commtext');
//			const author = comment.querySelector('.hnuser')?.textContent || '[deleted]';
//			const timeElement = comment.querySelector('.age');
//			const points = comment.querySelector('.score')?.textContent?.trim() || '';
//
//			if (!commentText) continue;
//
//			// Get the comment URL
//			const commentUrl = `https://news.ycombinator.com/item?id=${id}`;
//
//			// Get the timestamp from the title attribute and extract the date portion
//			const timestamp = timeElement?.getAttribute('title') || '';
//			const date = timestamp.split('T')[0] || '';
//
//			// For top-level comments, close all previous blockquotes and start fresh
//			if (depth === 0) {
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
//		<a href="${commentUrl}" class="comment-link">${date}</a>
//		${points ? ` • <span class="comment-points">${points}</span>` : ''}
//	</div>
//	<div class="comment-content">${commentText.innerHTML}</div>
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
func (h *HackerNewsExtractor) processComments(comments []*goquery.Selection) string {
	processedIDs := make(map[string]bool)

	slog.Debug("HackerNews extractor: processing comments", "totalComments", len(comments))

	var data []CommentData
	for _, comment := range comments {
		id, exists := comment.Attr("id")
		if !exists || id == "" || processedIDs[id] {
			continue
		}
		processedIDs[id] = true

		indentImg := comment.Find(".ind img")
		indentWidth, _ := indentImg.Attr("width")
		indent, _ := strconv.Atoi(indentWidth)
		depth := indent / 40

		commentText := comment.Find(".commtext")
		if commentText.Length() == 0 {
			continue
		}

		author := comment.Find(".hnuser").Text()
		if author == "" {
			author = "[deleted]"
		}

		timeElement := comment.Find(".age")
		points := strings.TrimSpace(comment.Find(".score").Text())

		commentURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%s", id)

		date := hnDateFromAge(timeElement)

		extra := ""
		if points != "" {
			extra = fmt.Sprintf(` • <span class="comment-points">%s</span>`, points)
		}

		commentContent, _ := commentText.Html()

		data = append(data, CommentData{
			Depth:    depth,
			Author:   author,
			URL:      commentURL,
			LinkText: date,
			Content:  commentContent,
			Extra:    extra,
		})
	}

	slog.Debug("HackerNews extractor: comments processed", "processedCount", len(processedIDs))
	return renderCommentThread(data)
}

// getPostID extracts the post ID from the URL
// TypeScript original code:
//
//	private getPostId(): string {
//		const match = this.url.match(/id=(\d+)/);
//		return match?.[1] || '';
//	}
func (h *HackerNewsExtractor) getPostID() string {
	matches := hnPostIDRe.FindStringSubmatch(h.url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// getPostTitle extracts the post title
// TypeScript original code:
//
//	private getPostTitle(): string {
//		if (this.isCommentPage && this.mainComment) {
//			const author = this.mainComment.querySelector('.hnuser')?.textContent || '[deleted]';
//			const commentText = this.mainComment.querySelector('.commtext')?.textContent || '';
//			// Use first 50 characters of comment as title
//			const preview = commentText.length > 50 ? commentText.slice(0, 50) + '...' : commentText;
//			return `Comment by ${author}: ${preview}`;
//		}
//		return this.mainPost?.querySelector('.titleline')?.textContent?.trim() || '';
//	}
func (h *HackerNewsExtractor) getPostTitle() string {
	if h.isCommentPage && h.mainComment != nil && h.mainComment.Length() > 0 {
		author := h.mainComment.Find(".hnuser").Text()
		if author == "" {
			author = "[deleted]"
		}

		commentText := strings.TrimSpace(h.mainComment.Find(".commtext").Text())

		// Use first 50 characters of comment as title
		preview := commentText
		if len(commentText) > 50 {
			preview = commentText[:50] + "..."
		}

		return fmt.Sprintf("Comment by %s: %s", author, preview)
	}

	if h.mainPost.Length() == 0 {
		return ""
	}

	return strings.TrimSpace(h.mainPost.Find(".titleline").Text())
}

// getPostAuthor extracts the post author
// TypeScript original code:
//
//	private getPostAuthor(): string {
//		return this.mainPost?.querySelector('.hnuser')?.textContent?.trim() || '';
//	}
func (h *HackerNewsExtractor) getPostAuthor() string {
	if h.mainPost.Length() == 0 {
		return ""
	}

	return strings.TrimSpace(h.mainPost.Find(".hnuser").Text())
}

// createDescription creates a description for the post
// TypeScript original code:
//
//	private createDescription(): string {
//		const title = this.getPostTitle();
//		const author = this.getPostAuthor();
//		if (this.isCommentPage) {
//			return `Comment by ${author} on Hacker News`;
//		}
//		return `${title} - by ${author} on Hacker News`;
//	}
func (h *HackerNewsExtractor) createDescription() string {
	title := h.getPostTitle()
	author := h.getPostAuthor()

	if h.isCommentPage {
		return fmt.Sprintf("Comment by %s on Hacker News", author)
	}

	return fmt.Sprintf("%s - by %s on Hacker News", title, author)
}

// hnDateFromAge returns the YYYY-MM-DD portion of a HackerNews .age element's title.
func hnDateFromAge(timeElement *goquery.Selection) string {
	timestamp, _ := timeElement.Attr("title")
	if timestamp == "" {
		return ""
	}
	parts := strings.Split(timestamp, "T")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// getPostDate extracts the post date
// TypeScript original code:
//
//	private getPostDate(): string {
//		if (!this.mainPost) return '';
//		const timeElement = this.mainPost.querySelector('.age');
//		const timestamp = timeElement?.getAttribute('title') || '';
//		return timestamp.split('T')[0] || '';
//	}
func (h *HackerNewsExtractor) getPostDate() string {
	if h.mainPost.Length() == 0 {
		return ""
	}
	return hnDateFromAge(h.mainPost.Find(".age"))
}
