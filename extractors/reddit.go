package extractors

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns for Reddit extraction.
var (
	redditCommentsRe  = regexp.MustCompile(`comments/([a-zA-Z0-9]+)`)
	redditSubredditRe = regexp.MustCompile(`/r/([^/]+)`)
)

// RedditExtractor handles Reddit post and comment content extraction
// TypeScript original code:
// import { BaseExtractor } from './_base';
// import { ExtractorResult } from '../types/extractors';
//
//	export class RedditExtractor extends BaseExtractor {
//		private shredditPost: Element | null;
//
//		constructor(document: Document, url: string) {
//			super(document, url);
//			this.shredditPost = document.querySelector('shreddit-post');
//		}
//	}
type RedditExtractor struct {
	*ExtractorBase
	shredditPost *goquery.Selection
}

// NewRedditExtractor creates a new Reddit extractor
// TypeScript original code:
//
//	constructor(document: Document, url: string) {
//		super(document, url);
//		this.shredditPost = document.querySelector('shreddit-post');
//	}
func NewRedditExtractor(document *goquery.Document, url string, schemaOrgData any) *RedditExtractor {
	shredditPost := document.Find("shreddit-post").First()

	slog.Debug("Reddit extractor initialized",
		"hasShredditPost", shredditPost.Length() > 0,
		"url", url)

	return &RedditExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
		shredditPost:  shredditPost,
	}
}

// CanExtract checks if the extractor can extract content
// TypeScript original code:
//
//	canExtract(): boolean {
//		return !!this.shredditPost;
//	}
func (r *RedditExtractor) CanExtract() bool {
	// Primary check: shreddit-post elements
	if r.shredditPost.Length() > 0 {
		slog.Debug("Reddit extractor can extract check", "canExtract", true, "method", "shreddit-post")
		return true
	}

	// Fallback check: alternative selectors for Reddit content
	redditPostFallbacks := []string{
		"[data-testid='post-content']",
		".usertext-body",
		".md",
		"div[data-click-id='text']",
		"div[data-click-id='body']",
		"div[id^='thing_t3_']", // Reddit post format
		".thing.link",          // Old Reddit format
	}

	if sel := firstMatchingSelection(r.document, redditPostFallbacks); sel.Length() > 0 {
		slog.Debug("Reddit extractor can extract check", "canExtract", true, "method", "fallback")
		return true
	}

	slog.Debug("Reddit extractor can extract check", "canExtract", false)
	return false
}

// Name returns the name of the extractor
func (r *RedditExtractor) Name() string {
	return "RedditExtractor"
}

// Extract extracts the Reddit post and comments
// TypeScript original code:
//
//	extract(): ExtractorResult {
//		const postContent = this.getPostContent();
//		const comments = this.extractComments();
//
//		const contentHtml = this.createContentHtml(postContent, comments);
//		const postTitle = this.document.querySelector('h1')?.textContent?.trim() || '';
//		const subreddit = this.getSubreddit();
//		const postAuthor = this.getPostAuthor();
//		const description = this.createDescription(postContent);
//
//		return {
//			content: contentHtml,
//			contentHtml: contentHtml,
//			extractedContent: {
//				postId: this.getPostId(),
//				subreddit,
//				 postAuthor,
//			},
//			variables: {
//				title: postTitle,
//				author: postAuthor,
//				site: `r/${subreddit}`,
//				description,
//			}
//		};
//	}
func (r *RedditExtractor) Extract() *ExtractorResult {
	slog.Debug("Reddit extractor starting extraction", "url", r.url)

	postContent := r.getPostContent()
	comments := r.extractComments()

	contentHTML := r.createContentHTML(postContent, comments)
	postTitle := r.getPostTitle()
	subreddit := r.getSubreddit()
	postAuthor := r.getPostAuthor()
	description := r.createDescription(postContent)
	postID := r.getPostID()

	slog.Debug("Reddit extraction completed",
		"postTitle", postTitle,
		"postAuthor", postAuthor,
		"subreddit", subreddit,
		"postId", postID,
		"hasComments", comments != "")

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"postId":     postID,
			"subreddit":  subreddit,
			"postAuthor": postAuthor,
		},
		Variables: map[string]string{
			"title":       postTitle,
			"author":      postAuthor,
			"site":        fmt.Sprintf("r/%s", subreddit),
			"description": description,
		},
	}
}

// getPostContent extracts the main post content
// TypeScript original code:
//
//	private getPostContent(): string {
//		const textBody = this.shredditPost?.querySelector('[slot="text-body"]')?.innerHTML || '';
//		const mediaBody = this.shredditPost?.querySelector('#post-image')?.outerHTML || '';
//
//		return textBody + mediaBody;
//	}
func (r *RedditExtractor) getPostContent() string {
	var content strings.Builder

	// Primary method: Look for shreddit-post elements
	if r.shredditPost.Length() > 0 {
		slog.Debug("Reddit extractor: using shreddit-post element")

		// Get text body content
		textBody := r.shredditPost.Find(`[slot="text-body"]`).First()
		if textBody.Length() > 0 {
			textBodyHTML, _ := textBody.Html()
			content.WriteString(textBodyHTML)
		}

		// Get media body content
		mediaBody := r.shredditPost.Find("#post-image").First()
		if mediaBody.Length() > 0 {
			mediaBodyHTML, _ := mediaBody.Html()
			// Use innerHTML equivalent since TypeScript uses outerHTML
			fmt.Fprintf(&content, `<div id="post-image">%s</div>`, mediaBodyHTML)
		}
	} else {
		// Fallback method: Look for alternative selectors
		slog.Debug("Reddit extractor: using fallback selectors")
		content.WriteString(r.fallbackPostContent())
	}

	result := content.String()
	slog.Debug("Reddit extractor: extracted post content",
		"hasShredditPost", r.shredditPost.Length() > 0,
		"contentLength", len(result))

	return result
}

// fallbackPostContent builds post content from alternative selectors when no
// shreddit-post element is present: the first content selector that yields HTML,
// plus images from the first matching image selector.
func (r *RedditExtractor) fallbackPostContent() string {
	var content strings.Builder

	// Try to find post content using alternative selectors
	alternativeSelectors := []string{
		"div[data-testid='post-content']",
		".usertext-body",
		".md",
		"div[data-click-id='text']",
		"div[data-click-id='body']",
	}

	for _, selector := range alternativeSelectors {
		postContent := r.document.Find(selector).First()
		if postContent.Length() > 0 {
			if html, err := postContent.Html(); err == nil && html != "" {
				content.WriteString(html)
				slog.Debug("Reddit extractor: found content with selector", "selector", selector)
				break
			}
		}
	}

	// Try to find images separately
	imageSelectors := []string{
		"img[src*='i.redd.it']",
		"img[src*='preview.redd.it']",
		"img[src*='external-preview.redd.it']",
	}

	for _, selector := range imageSelectors {
		images := r.document.Find(selector)
		if images.Length() > 0 {
			images.Each(func(_ int, img *goquery.Selection) {
				if outerHTML, err := img.Clone().Wrap("<div>").Parent().Html(); err == nil {
					content.WriteString(outerHTML)
				}
			})
			break
		}
	}

	return content.String()
}

// createContentHTML creates the formatted HTML content
// TypeScript original code:
//
//	private createContentHtml(postContent: string, comments: string): string {
//		return `
//			<div class="reddit-post">
//				<div class="post-content">
//					${postContent}
//				</div>
//			</div>
//			${comments ? `
//				<hr>
//				<h2>Comments</h2>
//				<div class="reddit-comments">
//					${comments}
//				</div>
//			` : ''}
//		`.trim();
//	}
func (r *RedditExtractor) createContentHTML(postContent, comments string) string {
	var content strings.Builder

	content.WriteString(`<div class="reddit-post">`)
	content.WriteString(`<div class="post-content">`)
	content.WriteString(postContent)
	content.WriteString(`</div>`)
	content.WriteString(`</div>`)

	if comments != "" {
		content.WriteString(`<hr>`)
		content.WriteString(`<h2>Comments</h2>`)
		content.WriteString(`<div class="reddit-comments">`)
		content.WriteString(comments)
		content.WriteString(`</div>`)
	}

	return strings.TrimSpace(content.String())
}
