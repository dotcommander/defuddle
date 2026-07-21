package extractors

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// getPostContent extracts the main post content
// TypeScript original code:
//
//	private getPostContent(): string {
//		if (!this.mainPost) return '';
//
//		// If this is a comment page, use the comment as the main content
//		if (this.isCommentPage && this.mainComment) {
//			const author = this.mainComment.querySelector('.hnuser')?.textContent || '[deleted]';
//			const commentText = this.mainComment.querySelector('.commtext')?.innerHTML || '';
//			const timeElement = this.mainComment.querySelector('.age');
//			const timestamp = timeElement?.getAttribute('title') || '';
//			const date = timestamp.split('T')[0] || '';
//			const points = this.mainComment.querySelector('.score')?.textContent?.trim() || '';
//			const parentUrl = this.mainPost.querySelector('.navs a[href*="parent"]')?.getAttribute('href') || '';
//
//			return `
//				<div class="comment main-comment">
//					<div class="comment-metadata">
//						<span class="comment-author"><strong>${author}</strong></span> •
//						<span class="comment-date">${date}</span>
//						${points ? ` • <span class="comment-points">${points}</span>` : ''}
//						${parentUrl ? ` • <a href="https://news.ycombinator.com/${parentUrl}" class="parent-link">parent</a>` : ''}
//					</div>
//					<div class="comment-content">${commentText}</div>
//				</div>
//			`.trim();
//		}
//
//		// Otherwise handle regular post content
//		const titleRow = this.mainPost.querySelector('tr.athing');
//		const subRow = titleRow?.nextElementSibling;
//		const url = titleRow?.querySelector('.titleline a')?.getAttribute('href') || '';
//
//		let content = '';
//		if (url) {
//			content += `<p><a href="${url}" target="_blank">${url}</a></p>`;
//		}
//
//		const text = this.mainPost.querySelector('.toptext');
//		if (text) {
//			content += `<div class="post-text">${text.innerHTML}</div>`;
//		}
//
//		return content;
//	}
func (h *HackerNewsExtractor) getPostContent() string {
	if h.mainPost.Length() == 0 {
		slog.Debug("HackerNews extractor: no main post for content extraction")
		return ""
	}

	// If this is a comment page, use the comment as the main content
	if h.isCommentPage && h.mainComment != nil && h.mainComment.Length() > 0 {
		return h.commentPageContent()
	}

	// Otherwise handle regular post content
	titleRow := h.mainPost.Find("tr.athing").First()
	url, _ := titleRow.Find(".titleline a").Attr("href")

	var content strings.Builder
	if url != "" {
		fmt.Fprintf(&content, `<p><a href="%s" target="_blank">%s</a></p>`, url, url)
	}

	text := h.mainPost.Find(".toptext")
	if text.Length() > 0 {
		textHTML, _ := text.Html()
		fmt.Fprintf(&content, `<div class="post-text">%s</div>`, textHTML)
	}

	slog.Debug("HackerNews extractor: extracted regular post content", "hasUrl", url != "", "hasText", text.Length() > 0)
	return content.String()
}

// commentPageContent renders the main comment as the post content for a
// HackerNews comment page.
func (h *HackerNewsExtractor) commentPageContent() string {
	author := h.mainComment.Find(".hnuser").Text()
	if author == "" {
		author = "[deleted]"
	}

	commentText, _ := h.mainComment.Find(".commtext").Html()

	timeElement := h.mainComment.Find(".age")
	date := hnDateFromAge(timeElement)

	points := strings.TrimSpace(h.mainComment.Find(".score").Text())
	parentURL, _ := h.mainPost.Find(`.navs a[href*="parent"]`).Attr("href")

	var content strings.Builder
	content.WriteString(`<div class="comment main-comment">`)
	content.WriteString(`<div class="comment-metadata">`)
	fmt.Fprintf(&content, `<span class="comment-author"><strong>%s</strong></span> •`, author)
	fmt.Fprintf(&content, ` <span class="comment-date">%s</span>`, date)

	if points != "" {
		fmt.Fprintf(&content, ` • <span class="comment-points">%s</span>`, points)
	}

	if parentURL != "" {
		fmt.Fprintf(&content, ` • <a href="https://news.ycombinator.com/%s" class="parent-link">parent</a>`, parentURL)
	}

	content.WriteString(`</div>`)
	fmt.Fprintf(&content, `<div class="comment-content">%s</div>`, commentText)
	content.WriteString(`</div>`)

	slog.Debug("HackerNews extractor: extracted comment page content", "author", author, "hasPoints", points != "", "hasParentURL", parentURL != "")
	return content.String()
}

// extractComments extracts all comments
// TypeScript original code:
//
//	private extractComments(): string {
//		const comments = Array.from(this.document.querySelectorAll('tr.comtr'));
//		return this.processComments(comments);
//	}
func (h *HackerNewsExtractor) extractComments() string {
	var comments []*goquery.Selection
	h.document.Find("tr.comtr").Each(func(_ int, s *goquery.Selection) {
		comments = append(comments, s)
	})

	slog.Debug("HackerNews extractor: found comments", "commentCount", len(comments))
	return h.processComments(comments)
}
