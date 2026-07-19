package extractors

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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
