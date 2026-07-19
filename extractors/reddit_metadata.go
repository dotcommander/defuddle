package extractors

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// getPostID extracts the post ID from URL
// TypeScript original code:
//
//	private getPostId(): string {
//		const match = this.url.match(/comments\/([a-zA-Z0-9]+)/);
//		return match?.[1] || '';
//	}
func (r *RedditExtractor) getPostID() string {
	matches := redditCommentsRe.FindStringSubmatch(r.url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// getSubreddit extracts the subreddit name from URL
// TypeScript original code:
//
//	private getSubreddit(): string {
//		const match = this.url.match(/\/r\/([^/]+)/);
//		return match?.[1] || '';
//	}
func (r *RedditExtractor) getSubreddit() string {
	matches := redditSubredditRe.FindStringSubmatch(r.url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// getPostAuthor extracts the post author
// TypeScript original code:
//
//	private getPostAuthor(): string {
//		return this.shredditPost?.getAttribute('author') || '';
//	}
func (r *RedditExtractor) getPostAuthor() string {
	if r.shredditPost.Length() > 0 {
		author, _ := r.shredditPost.Attr("author")
		return author
	}
	return ""
}

// getPostTitle extracts the post title
// TypeScript original code:
//
//	const postTitle = this.document.querySelector('h1')?.textContent?.trim() || '';
func (r *RedditExtractor) getPostTitle() string {
	// First try to get title from h1 element
	h1Title := strings.TrimSpace(r.document.Find("h1").First().Text())
	if h1Title != "" {
		return h1Title
	}

	// Fallback to page title
	pageTitle := strings.TrimSpace(r.document.Find("title").Text())
	if pageTitle != "" && pageTitle != "Reddit - The heart of the internet" {
		return pageTitle
	}

	return ""
}

// createDescription creates a description from post content
// TypeScript original code:
//
//	private createDescription(postContent: string): string {
//		if (!postContent) return '';
//
//		const tempDiv = document.createElement('div');
//		tempDiv.innerHTML = postContent;
//		return tempDiv.textContent?.trim()
//			.slice(0, 140)
//			.replace(/\s+/g, ' ') || '';
//	}
func (r *RedditExtractor) createDescription(postContent string) string {
	if postContent == "" {
		return ""
	}

	// Create a temporary document to extract text content
	tempDoc, err := goquery.NewDocumentFromReader(strings.NewReader(postContent))
	if err != nil {
		slog.Warn("Reddit extractor: failed to parse post content for description", "error", err)
		return ""
	}

	textContent := strings.TrimSpace(tempDoc.Text())

	// Replace multiple whitespace with single space
	textContent = whitespaceRe.ReplaceAllString(textContent, " ")

	// Limit to 140 characters
	if len(textContent) > 140 {
		return textContent[:140]
	}

	return textContent
}
