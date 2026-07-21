package extractors

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns for XArticle extraction.
// TypeScript original code: selectors and patterns defined as constants
var (
	xArticleURLAuthorRe = regexp.MustCompile(`/([a-zA-Z][a-zA-Z0-9_]{0,14})/article/\d+`)
	xArticleOgTitleRe   = regexp.MustCompile(`^(?:\(\d+\)\s+)?(.+?)\s+on\s+X\s*:`)
	xArticleIDRe        = regexp.MustCompile(`article/(\d+)`)
	xArticleImageNameRe = regexp.MustCompile(`&name=\w+`)
	xArticleLangClassRe = regexp.MustCompile(`language-(\w+)`)
)

// XArticleExtractor handles X (Twitter) long-form article content extraction.
// TypeScript original code:
//
//	export class XArticleExtractor extends BaseExtractor {
//	  private articleContainer: Element | null;
//	  constructor(document: Document, url: string, schemaOrgData?: any) {
//	    super(document, url, schemaOrgData);
//	    this.articleContainer = document.querySelector('[data-testid="twitterArticleRichTextView"]');
//	  }
//	}
type XArticleExtractor struct {
	*ExtractorBase
	articleContainer *goquery.Selection
}

// NewXArticleExtractor creates a new XArticle extractor.
// TypeScript original code:
//
//	constructor(document: Document, url: string, schemaOrgData?: any) {
//	  super(document, url, schemaOrgData);
//	  this.articleContainer = document.querySelector('[data-testid="twitterArticleRichTextView"]');
//	}
func NewXArticleExtractor(document *goquery.Document, url string, schemaOrgData any) *XArticleExtractor {
	container := document.Find(`[data-testid="twitterArticleRichTextView"]`).First()
	return &XArticleExtractor{
		ExtractorBase:    NewExtractorBase(document, url, schemaOrgData),
		articleContainer: container,
	}
}

// CanExtract returns true when an article container was found.
// TypeScript original code:
//
//	canExtract(): boolean { return !!this.articleContainer; }
func (x *XArticleExtractor) CanExtract() bool {
	return x.articleContainer != nil && x.articleContainer.Length() > 0
}

// Name returns the extractor name.
func (x *XArticleExtractor) Name() string {
	return "XArticleExtractor"
}

// Extract extracts the article content and metadata.
// TypeScript original code:
//
//	extract(): ExtractorResult {
//	  const title = this.extractTitle();
//	  const author = this.extractAuthor();
//	  const contentHtml = this.extractContent();
//	  const description = this.createDescription();
//	  return { content: contentHtml, contentHtml, extractedContent: { articleId: this.getArticleId() },
//	    variables: { title, author, site: 'X (Twitter)', description } };
//	}
func (x *XArticleExtractor) Extract() *ExtractorResult {
	title := x.extractTitle()
	author := x.extractAuthor()
	contentHTML := x.extractContent()
	description := x.createDescription()

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"articleId": x.getArticleID(),
		},
		Variables: map[string]string{
			"title":       title,
			"author":      author,
			"site":        "X (Twitter)",
			"description": description,
		},
	}
}

// extractTitle returns the article title text or a fallback.
// TypeScript original code:
//
//	private extractTitle(): string {
//	  const titleEl = this.document.querySelector('[data-testid="twitter-article-title"]');
//	  return titleEl?.textContent?.trim() || 'Untitled X Article';
//	}
func (x *XArticleExtractor) extractTitle() string {
	titleEl := x.document.Find(`[data-testid="twitter-article-title"]`).First()
	if titleEl.Length() == 0 {
		return "Untitled X Article"
	}
	title := strings.TrimSpace(titleEl.Text())
	if title == "" {
		return "Untitled X Article"
	}
	return title
}

// extractAuthor resolves the author from structured metadata, URL, or og:title.
// TypeScript original code:
//
//	private extractAuthor(): string {
//	  const authorContainer = this.document.querySelector('[itemprop="author"]');
//	  if (!authorContainer) return this.getAuthorFromUrl();
//	  const name = authorContainer.querySelector('meta[itemprop="name"]')?.getAttribute('content');
//	  const handle = authorContainer.querySelector('meta[itemprop="additionalName"]')?.getAttribute('content');
//	  if (name && handle) return `${name} (@${handle})`;
//	  return name || handle || this.getAuthorFromUrl();
//	}
func (x *XArticleExtractor) extractAuthor() string {
	authorContainer := x.document.Find(`[itemprop="author"]`).First()
	if authorContainer.Length() == 0 {
		return x.getAuthorFromURL()
	}

	name, _ := authorContainer.Find(`meta[itemprop="name"]`).First().Attr("content")
	handle, _ := authorContainer.Find(`meta[itemprop="additionalName"]`).First().Attr("content")

	name = strings.TrimSpace(name)
	handle = strings.TrimSpace(handle)

	if name != "" && handle != "" {
		return fmt.Sprintf("%s (@%s)", name, handle)
	}
	if name != "" {
		return name
	}
	if handle != "" {
		return handle
	}
	return x.getAuthorFromURL()
}

// getAuthorFromURL extracts the username from the article URL.
// TypeScript original code:
//
//	private getAuthorFromUrl(): string {
//	  const match = this.url.match(/\/([a-zA-Z][a-zA-Z0-9_]{0,14})\/article\/\d+/);
//	  return match ? `@${match[1]}` : this.getAuthorFromOgTitle();
//	}
func (x *XArticleExtractor) getAuthorFromURL() string {
	matches := xArticleURLAuthorRe.FindStringSubmatch(x.url)
	if len(matches) > 1 {
		return "@" + matches[1]
	}
	return x.getAuthorFromOgTitle()
}

// getAuthorFromOgTitle parses the og:title meta tag for the author name.
// TypeScript original code:
//
//	private getAuthorFromOgTitle(): string {
//	  const ogTitle = this.document.querySelector('meta[property="og:title"]')?.getAttribute('content') || '';
//	  const match = ogTitle.match(/^(?:\(\d+\)\s+)?(.+?)\s+on\s+X\s*:/);
//	  return match ? match[1].trim() : 'Unknown';
//	}
func (x *XArticleExtractor) getAuthorFromOgTitle() string {
	ogTitle, _ := x.document.Find(`meta[property="og:title"]`).First().Attr("content")
	matches := xArticleOgTitleRe.FindStringSubmatch(ogTitle)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return "Unknown"
}

// getArticleID extracts the numeric article ID from the URL.
// TypeScript original code:
//
//	private getArticleId(): string {
//	  const match = this.url.match(/article\/(\d+)/);
//	  return match ? match[1] : '';
//	}
func (x *XArticleExtractor) getArticleID() string {
	matches := xArticleIDRe.FindStringSubmatch(x.url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
