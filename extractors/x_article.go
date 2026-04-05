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

// extractContent clones the article container into a new document, cleans it,
// and returns the wrapped HTML string.
// TypeScript original code:
//
//	private extractContent(): string {
//	  if (!this.articleContainer) return '';
//	  const clone = this.articleContainer.cloneNode(true) as HTMLElement;
//	  this.cleanContent(clone);
//	  return `<article class="x-article">${clone.innerHTML}</article>`;
//	}
func (x *XArticleExtractor) extractContent() string {
	if !x.CanExtract() {
		return ""
	}

	// Obtain raw HTML of the container to create a mutable clone.
	outerHTML, err := goquery.OuterHtml(x.articleContainer)
	if err != nil {
		return ""
	}

	cloneDoc, err := goquery.NewDocumentFromReader(strings.NewReader(outerHTML))
	if err != nil {
		return ""
	}

	// The parser wraps content in <html><body>; grab the first child of body.
	container := cloneDoc.Find("body").Children().First()

	x.cleanContent(container)

	innerHTMLStr, err := container.Html()
	if err != nil {
		return ""
	}

	return fmt.Sprintf(`<article class="x-article">%s</article>`, innerHTMLStr)
}

// cleanContent applies all transformations to the cloned container in order.
// TypeScript original code:
//
//	private cleanContent(container: HTMLElement): void {
//	  this.convertEmbeddedTweets(container, ownerDoc);
//	  this.convertCodeBlocks(container, ownerDoc);
//	  this.convertHeaders(container, ownerDoc);
//	  this.unwrapLinkedImages(container, ownerDoc);
//	  this.upgradeImageQuality(container);
//	  this.convertBoldSpans(container, ownerDoc);
//	  this.convertDraftParagraphs(container, ownerDoc);
//	  this.removeDraftAttributes(container);
//	}
func (x *XArticleExtractor) cleanContent(container *goquery.Selection) {
	x.convertEmbeddedTweets(container)
	x.convertCodeBlocks(container)
	x.convertHeaders(container)
	x.unwrapLinkedImages(container)
	x.upgradeImageQuality(container)
	x.convertBoldSpans(container)
	x.convertDraftParagraphs(container)
	x.removeDraftAttributes(container)
}

// convertEmbeddedTweets replaces [data-testid="simpleTweet"] elements with
// semantic <blockquote> elements containing author and text.
// TypeScript original code:
//
//	private convertEmbeddedTweets(container: HTMLElement, ownerDoc: Document): void {
//	  container.querySelectorAll('[data-testid="simpleTweet"]').forEach(tweet => { ... });
//	}
func (x *XArticleExtractor) convertEmbeddedTweets(container *goquery.Selection) {
	container.Find(`[data-testid="simpleTweet"]`).Each(func(_ int, tweet *goquery.Selection) {
		var blockquote strings.Builder
		blockquote.WriteString(`<blockquote class="embedded-tweet">`)

		// extract author info
		userNameEl := tweet.Find(`[data-testid="User-Name"]`).First()
		authorLinks := userNameEl.Find("a")
		fullName := strings.TrimSpace(authorLinks.Eq(0).Text())
		handle := strings.TrimSpace(authorLinks.Eq(1).Text())

		if fullName != "" || handle != "" {
			cite := fullName
			if handle != "" {
				cite = fullName + " " + handle
			}
			fmt.Fprintf(&blockquote, "<cite>%s</cite>", cite)
		}

		// extract tweet text
		tweetTextEl := tweet.Find(`[data-testid="tweetText"]`).First()
		tweetText := strings.TrimSpace(tweetTextEl.Text())
		if tweetText != "" {
			fmt.Fprintf(&blockquote, "<p>%s</p>", tweetText)
		}

		blockquote.WriteString(`</blockquote>`)
		tweet.ReplaceWithHtml(blockquote.String())
	})
}

// convertCodeBlocks replaces [data-testid="markdown-code-block"] with clean <pre><code>.
// TypeScript original code:
//
//	private convertCodeBlocks(container: HTMLElement, ownerDoc: Document): void {
//	  container.querySelectorAll('[data-testid="markdown-code-block"]').forEach(block => { ... });
//	}
func (x *XArticleExtractor) convertCodeBlocks(container *goquery.Selection) {
	container.Find(`[data-testid="markdown-code-block"]`).Each(func(_ int, block *goquery.Selection) {
		pre := block.Find("pre").First()
		code := block.Find("code").First()
		if pre.Length() == 0 || code.Length() == 0 {
			return
		}

		// extract language from class or header span
		language := ""
		codeClass, _ := code.Attr("class")
		if matches := xArticleLangClassRe.FindStringSubmatch(codeClass); len(matches) > 1 {
			language = matches[1]
		} else {
			langSpan := block.Find("span").First()
			language = strings.TrimSpace(langSpan.Text())
		}

		codeText := code.Text()

		var replacement strings.Builder
		replacement.WriteString("<pre><code")
		if language != "" {
			fmt.Fprintf(&replacement, ` data-lang="%s" class="language-%s"`, language, language)
		}
		fmt.Fprintf(&replacement, ">%s</code></pre>", codeText)

		block.ReplaceWithHtml(replacement.String())
	})
}

// convertHeaders simplifies h1-h6 elements to plain text headings.
// TypeScript original code:
//
//	private convertHeaders(container: HTMLElement, ownerDoc: Document): void {
//	  container.querySelectorAll('h1, h2, h3, h4, h5, h6').forEach(header => { ... });
//	}
func (x *XArticleExtractor) convertHeaders(container *goquery.Selection) {
	container.Find("h1, h2, h3, h4, h5, h6").Each(func(_ int, header *goquery.Selection) {
		level := goquery.NodeName(header)
		text := strings.TrimSpace(header.Text())
		if text == "" {
			return
		}
		header.ReplaceWithHtml(fmt.Sprintf("<%s>%s</%s>", level, text, level))
	})
}

// unwrapLinkedImages finds tweetPhoto images inside anchor tags and replaces
// the anchors with clean, quality-upgraded img elements.
// TypeScript original code:
//
//	private unwrapLinkedImages(container: HTMLElement, ownerDoc: Document): void {
//	  container.querySelectorAll('[data-testid="tweetPhoto"] img').forEach(img => { ... });
//	}
func (x *XArticleExtractor) unwrapLinkedImages(container *goquery.Selection) {
	container.Find(`[data-testid="tweetPhoto"] img`).Each(func(_ int, img *goquery.Selection) {
		anchor := img.Closest("a")
		if anchor.Length() == 0 {
			return
		}

		src, _ := img.Attr("src")
		alt := strings.TrimSpace(whitespaceRe.ReplaceAllString(img.AttrOr("alt", ""), " "))
		if alt == "" {
			alt = "Image"
		}

		src = upgradeXImageQuality(src)
		anchor.ReplaceWithHtml(fmt.Sprintf(`<img src="%s" alt="%s" />`, src, alt))
	})
}

// upgradeImageQuality upgrades remaining tweetPhoto image quality in-place.
// TypeScript original code:
//
//	private upgradeImageQuality(container: HTMLElement): void {
//	  container.querySelectorAll('[data-testid="tweetPhoto"] img').forEach(img => { ... });
//	}
func (x *XArticleExtractor) upgradeImageQuality(container *goquery.Selection) {
	container.Find(`[data-testid="tweetPhoto"] img`).Each(func(_ int, img *goquery.Selection) {
		src, exists := img.Attr("src")
		if !exists || src == "" {
			return
		}
		img.SetAttr("src", upgradeXImageQuality(src))
	})
}

// upgradeXImageQuality upgrades a Twitter/X image URL to large quality.
// TypeScript original code:
//
//	if (src.includes('&name=')) { ... } else if (src.includes('?')) { ... } else { ... }
func upgradeXImageQuality(src string) string {
	if strings.Contains(src, "&name=") {
		return xArticleImageNameRe.ReplaceAllString(src, "&name=large")
	}
	if strings.Contains(src, "?") {
		return src + "&name=large"
	}
	return src + "?name=large"
}

// convertDraftParagraphs converts Draft.js block divs into semantic <p> elements,
// preserving inline <strong>, <a>, and <code> formatting.
// TypeScript original code:
//
//	private convertDraftParagraphs(container: HTMLElement, ownerDoc: Document): void {
//	  container.querySelectorAll('.longform-unstyled, .public-DraftStyleDefault-block').forEach(div => { ... });
//	}
func (x *XArticleExtractor) convertDraftParagraphs(container *goquery.Selection) {
	container.Find(".longform-unstyled, .public-DraftStyleDefault-block").Each(func(_ int, div *goquery.Selection) {
		content := buildParagraphContent(div)
		div.ReplaceWithHtml(fmt.Sprintf("<p>%s</p>", content))
	})
}

// buildParagraphContent recursively processes child nodes of a Draft.js block,
// preserving strong, a, and code inline elements.
// TypeScript original code: processNode recursive function inside convertDraftParagraphs
func buildParagraphContent(sel *goquery.Selection) string {
	var sb strings.Builder
	sel.Contents().Each(func(_ int, node *goquery.Selection) {
		switch goquery.NodeName(node) {
		case "#text":
			sb.WriteString(node.Text())
		case "strong":
			fmt.Fprintf(&sb, "<strong>%s</strong>", node.Text())
		case "a":
			href := node.AttrOr("href", "")
			fmt.Fprintf(&sb, `<a href="%s">%s</a>`, href, node.Text())
		case "code":
			fmt.Fprintf(&sb, "<code>%s</code>", node.Text())
		default:
			// recurse into other elements (spans, divs, etc.)
			sb.WriteString(buildParagraphContent(node))
		}
	})
	return sb.String()
}

// convertBoldSpans replaces span[style*="font-weight: bold"] with <strong> elements.
// TypeScript original code:
//
//	private convertBoldSpans(container: HTMLElement, ownerDoc: Document): void {
//	  container.querySelectorAll('span[style*="font-weight: bold"]').forEach(span => { ... });
//	}
func (x *XArticleExtractor) convertBoldSpans(container *goquery.Selection) {
	container.Find(`span[style*="font-weight: bold"]`).Each(func(_ int, span *goquery.Selection) {
		span.ReplaceWithHtml(fmt.Sprintf("<strong>%s</strong>", span.Text()))
	})
}

// removeDraftAttributes strips data-offset-key attributes from all matching elements.
// TypeScript original code:
//
//	private removeDraftAttributes(container: HTMLElement): void {
//	  container.querySelectorAll('[data-offset-key]').forEach(el => { el.removeAttribute('data-offset-key'); });
//	}
func (x *XArticleExtractor) removeDraftAttributes(container *goquery.Selection) {
	container.Find("[data-offset-key]").Each(func(_ int, el *goquery.Selection) {
		el.RemoveAttr("data-offset-key")
	})
}

// createDescription returns up to 140 characters of the article text as a description.
// TypeScript original code:
//
//	private createDescription(): string {
//	  const text = this.articleContainer?.textContent?.trim() || '';
//	  return text.slice(0, 140) + (text.length > 140 ? '...' : '');
//	}
func (x *XArticleExtractor) createDescription() string {
	if !x.CanExtract() {
		return ""
	}
	text := strings.TrimSpace(x.articleContainer.Text())
	if len(text) <= 140 {
		return text
	}
	return text[:140] + "..."
}
