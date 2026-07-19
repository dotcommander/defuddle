package extractors

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex pattern for Hacker News extraction.
var hnPostIDRe = regexp.MustCompile(`id=(\d+)`)

// HackerNewsExtractor handles Hacker News content extraction
// TypeScript original code:
// import { BaseExtractor } from './_base';
// import { ExtractorResult } from '../types/extractors';
//
//	export class HackerNewsExtractor extends BaseExtractor {
//		private mainPost: Element | null;
//		private isCommentPage: boolean;
//		private mainComment: Element | null;
//
//		constructor(document: Document, url: string) {
//			super(document, url);
//			this.mainPost = document.querySelector('.fatitem');
//			this.isCommentPage = this.detectCommentPage();
//			this.mainComment = this.isCommentPage ? this.findMainComment() : null;
//		}
//	}
type HackerNewsExtractor struct {
	*ExtractorBase
	mainPost      *goquery.Selection
	isCommentPage bool
	mainComment   *goquery.Selection
}

// NewHackerNewsExtractor creates a new HackerNews extractor
// TypeScript original code:
//
//	constructor(document: Document, url: string) {
//		super(document, url);
//		this.mainPost = document.querySelector('.fatitem');
//		this.isCommentPage = this.detectCommentPage();
//		this.mainComment = this.isCommentPage ? this.findMainComment() : null;
//	}
func NewHackerNewsExtractor(document *goquery.Document, url string, schemaOrgData any) *HackerNewsExtractor {
	extractor := &HackerNewsExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}

	// Find the main post element
	extractor.mainPost = document.Find(".fatitem").First()
	slog.Debug("HackerNews extractor: found main post", "hasMainPost", extractor.mainPost.Length() > 0)

	// Detect if this is a comment page
	extractor.isCommentPage = extractor.detectCommentPage()
	slog.Debug("HackerNews extractor: detected page type", "isCommentPage", extractor.isCommentPage)

	// Find main comment if on a comment page
	if extractor.isCommentPage {
		extractor.mainComment = extractor.findMainComment()
		slog.Debug("HackerNews extractor: found main comment", "hasMainComment", extractor.mainComment != nil && extractor.mainComment.Length() > 0)
	}

	return extractor
}

// detectCommentPage checks if we're on a comment page
// TypeScript original code:
//
//	private detectCommentPage(): boolean {
//		// Check if we're on a comment page by looking for a parent link in the navigation
//		return !!this.mainPost?.querySelector('.navs a[href*="parent"]');
//	}
func (h *HackerNewsExtractor) detectCommentPage() bool {
	if h.mainPost.Length() == 0 {
		return false
	}

	// Check if we're on a comment page by looking for a parent link in the navigation
	parentLink := h.mainPost.Find(`.navs a[href*="parent"]`)
	isCommentPage := parentLink.Length() > 0
	slog.Debug("HackerNews extractor: checking for parent link", "parentLinkFound", isCommentPage)
	return isCommentPage
}

// findMainComment finds the main comment on a comment page
// TypeScript original code:
//
//	private findMainComment(): Element | null {
//		// The main comment is the first comment in the fatitem
//		const comment = this.mainPost?.querySelector('.comment');
//		return comment || null;
//	}
func (h *HackerNewsExtractor) findMainComment() *goquery.Selection {
	if h.mainPost.Length() == 0 {
		slog.Debug("HackerNews extractor: no main post found for comment search")
		return nil
	}

	// The main comment is the first comment in the fatitem
	comment := h.mainPost.Find(".comment").First()
	if comment.Length() > 0 {
		slog.Debug("HackerNews extractor: found main comment")
		return comment
	}

	slog.Debug("HackerNews extractor: no main comment found")
	return nil
}

// CanExtract checks if the extractor can extract content
// TypeScript original code:
//
//	canExtract(): boolean {
//		return !!this.mainPost;
//	}
func (h *HackerNewsExtractor) CanExtract() bool {
	canExtract := h.mainPost.Length() > 0
	slog.Debug("HackerNews extractor can extract check", "canExtract", canExtract)
	return canExtract
}

// Name returns the name of the extractor
func (h *HackerNewsExtractor) Name() string {
	return "HackerNewsExtractor"
}

// Extract extracts the HackerNews content
// TypeScript original code:
//
//	extract(): ExtractorResult {
//		const postContent = this.getPostContent();
//		const comments = this.extractComments();
//
//		const contentHtml = this.createContentHtml(postContent, comments);
//		const postTitle = this.getPostTitle();
//		const postAuthor = this.getPostAuthor();
//		const description = this.createDescription();
//		const published = this.getPostDate();
//
//		return {
//			content: contentHtml,
//			contentHtml: contentHtml,
//			extractedContent: {
//				postId: this.getPostId(),
//				postAuthor,
//			},
//			variables: {
//				title: postTitle,
//				author: postAuthor,
//				site: 'Hacker News',
//				description,
//				published,
//			}
//		};
//	}
func (h *HackerNewsExtractor) Extract() *ExtractorResult {
	slog.Debug("HackerNews extractor starting extraction", "url", h.url)

	postContent := h.getPostContent()
	comments := h.extractComments()

	contentHTML := h.createContentHTML(postContent, comments)
	postTitle := h.getPostTitle()
	postAuthor := h.getPostAuthor()
	description := h.createDescription()
	published := h.getPostDate()
	postID := h.getPostID()

	slog.Debug("HackerNews extraction completed",
		"postTitle", postTitle,
		"postAuthor", postAuthor,
		"postId", postID,
		"hasComments", comments != "",
		"published", published)

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"postId":     postID,
			"postAuthor": postAuthor,
		},
		Variables: map[string]string{
			"title":       postTitle,
			"author":      postAuthor,
			"site":        "Hacker News",
			"description": description,
			"published":   published,
		},
	}
}

// createContentHTML creates the formatted HTML content
// TypeScript original code:
//
//	private createContentHtml(postContent: string, comments: string): string {
//		return `
//			<div class="hackernews-post">
//				<div class="post-content">
//					${postContent}
//				</div>
//				${comments ? `
//					<hr>
//					<h2>Comments</h2>
//					<div class="hackernews-comments">
//						${comments}
//					</div>
//				` : ''}
//			</div>
//		`.trim();
//	}
func (h *HackerNewsExtractor) createContentHTML(postContent, comments string) string {
	var content strings.Builder
	content.WriteString(`<div class="hackernews-post">`)
	content.WriteString(`<div class="post-content">`)
	content.WriteString(postContent)
	content.WriteString(`</div>`)

	if comments != "" {
		content.WriteString(`<hr>`)
		content.WriteString(`<h2>Comments</h2>`)
		content.WriteString(`<div class="hackernews-comments">`)
		content.WriteString(comments)
		content.WriteString(`</div>`)
	}

	content.WriteString(`</div>`)
	return strings.TrimSpace(content.String())
}
