package extractors

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Extract extracts the Twitter content
// TypeScript original code:
//
//	extract(): ExtractorResult {
//		const mainContent = this.extractTweet(this.mainTweet);
//
//		const threadContents = this.threadTweets
//			.map(tweet => this.extractTweet(tweet))
//			.filter(content => content);
//		const threadContent = threadContents.join('\n<hr>\n');
//
//		let contentHtml = '<div class="tweet-thread">';
//		contentHtml += '<div class="main-tweet">' + mainContent + '</div>';
//
//		if (threadContent) {
//			contentHtml += '<hr><div class="thread-tweets">' + threadContent + '</div>';
//		}
//
//		contentHtml += '</div>';
//
//		const tweetId = this.getTweetId();
//		const tweetAuthor = this.getTweetAuthor();
//		const description = this.createDescription(this.mainTweet);
//
//		return {
//			content: contentHtml,
//			contentHtml: contentHtml,
//			extractedContent: {
//				tweetId: tweetId,
//				tweetAuthor: tweetAuthor
//			},
//			variables: {
//				title: `Thread by ${tweetAuthor}`,
//				author: tweetAuthor,
//				site: 'X (Twitter)',
//				description: description
//			}
//		};
//	}
func (t *TwitterExtractor) Extract() *ExtractorResult {
	mainContent := t.extractTweet(t.mainTweet)

	var threadContents []string
	for _, tweet := range t.threadTweets {
		content := t.extractTweet(tweet)
		if content != "" {
			threadContents = append(threadContents, content)
		}
	}
	threadContent := strings.Join(threadContents, "\n<hr>\n")

	var contentHTML strings.Builder
	contentHTML.WriteString(`<div class="tweet-thread">`)
	contentHTML.WriteString(`<div class="main-tweet">`)
	contentHTML.WriteString(mainContent)
	contentHTML.WriteString(`</div>`)

	if threadContent != "" {
		contentHTML.WriteString(`<hr><div class="thread-tweets">`)
		contentHTML.WriteString(threadContent)
		contentHTML.WriteString(`</div>`)
	}

	contentHTML.WriteString(`</div>`)

	tweetID := t.getTweetID()
	tweetAuthor := t.getTweetAuthor()
	description := t.createDescription(t.mainTweet)

	return &ExtractorResult{
		Content:     contentHTML.String(),
		ContentHTML: contentHTML.String(),
		ExtractedContent: map[string]any{
			"tweetId":     tweetID,
			"tweetAuthor": tweetAuthor,
		},
		Variables: map[string]string{
			"title":       fmt.Sprintf("Thread by %s", tweetAuthor),
			"author":      tweetAuthor,
			"site":        "X (Twitter)",
			"description": description,
		},
	}
}

// formatTweetText formats tweet text content
// TypeScript original code:
//
//	private formatTweetText(text: string): string {
//		if (!text) return '';
//
//		// Parse HTML content to clean it
//		const parser = new DOMParser();
//		const doc = parser.parseFromString(text, 'text/html');
//
//		// Convert links to plain text with @ handles
//		doc.querySelectorAll('a').forEach(link => {
//			const handle = link.textContent?.trim() || '';
//			link.replaceWith(handle);
//		});
//
//		// Remove unnecessary spans and divs but keep their content
//		doc.querySelectorAll('span, div').forEach(element => {
//			const content = element.textContent || '';
//			element.replaceWith(content);
//		});
//
//		// Get cleaned text and split into paragraphs
//		const cleanText = doc.body.innerHTML;
//		const paragraphs = cleanText.split('\n').filter(p => p.trim());
//
//		return paragraphs.map(p => `<p>${p.trim()}</p>`).join('\n');
//	}
func (t *TwitterExtractor) formatTweetText(text string) string {
	if text == "" {
		return ""
	}

	// Parse HTML content to clean it
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(text))
	if err != nil {
		return text
	}

	// Convert emoji images to alt text
	doc.Find(`img[src*="/emoji/"]`).Each(func(_ int, img *goquery.Selection) {
		if alt, exists := img.Attr("alt"); exists && alt != "" {
			img.ReplaceWithHtml(alt)
		}
	})

	// Convert links to plain text with @ handles
	doc.Find("a").Each(func(_ int, link *goquery.Selection) {
		handle := strings.TrimSpace(link.Text())
		link.ReplaceWithHtml(handle)
	})

	// Remove unnecessary spans and divs but keep their child nodes
	doc.Find("span, div").Each(func(_ int, element *goquery.Selection) {
		innerHTML, _ := element.Html()
		element.ReplaceWithHtml(innerHTML)
	})

	// Get cleaned text and split into paragraphs
	cleanText, _ := doc.Html()
	paragraphs := strings.Split(cleanText, "\n")

	var formattedParagraphs []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			formattedParagraphs = append(formattedParagraphs, fmt.Sprintf("<p>%s</p>", p))
		}
	}

	return strings.Join(formattedParagraphs, "\n")
}

// extractTweet extracts content from a single tweet
// TypeScript original code:
//
//	private extractTweet(tweet: Element | null): string {
//		if (!tweet) return '';
//
//		// Get tweet text
//		const tweetText = tweet.querySelector('[data-testid="tweetText"]');
//		const tweetHtml = tweetText ? tweetText.innerHTML : '';
//		const formattedText = this.formatTweetText(tweetHtml);
//
//		// Get images
//		const images = this.extractImages(tweet);
//
//		// Get user info and date
//		const userInfo = this.extractUserInfo(tweet);
//
//		// Extract quoted tweet if present
//		const quotedTweet = tweet.querySelector('[aria-labelledby*="id__"]');
//		let quotedContent = '';
//		if (quotedTweet) {
//			const quotedUserName = quotedTweet.querySelector('[data-testid="User-Name"]');
//			if (quotedUserName) {
//				const quotedTweetContainer = quotedUserName.closest('[aria-labelledby*="id__"]');
//				if (quotedTweetContainer) {
//					quotedContent = this.extractTweet(quotedTweetContainer);
//				}
//			}
//		}
//
//		let result = '<div class="tweet">';
//		result += '<div class="tweet-header">';
//		result += `<span class="tweet-author"><strong>${userInfo.fullName}</strong> <span class="tweet-handle">${userInfo.handle}</span></span>`;
//
//		if (userInfo.date) {
//			result += ` <a href="${userInfo.permalink}" class="tweet-date">${userInfo.date}</a>`;
//		}
//
//		result += '</div>';
//
//		if (formattedText) {
//			result += `<div class="tweet-text">${formattedText}</div>`;
//		}
//
//		if (images.length > 0) {
//			result += '<div class="tweet-media">';
//			result += images.join('\n');
//			result += '</div>';
//		}
//
//		if (quotedContent) {
//			result += `<blockquote class="quoted-tweet">${quotedContent}</blockquote>`;
//		}
//
//		result += '</div>';
//		return result.trim();
//	}
//
// quotedTweetContent returns the rendered content of tweet's quoted tweet, or "".
func (t *TwitterExtractor) quotedTweetContent(tweet *goquery.Selection) string {
	quotedTweet := tweet.Find(`[aria-labelledby*="id__"]`).First()
	if quotedTweet.Length() == 0 {
		return ""
	}
	quotedUserName := quotedTweet.Find(`[data-testid="User-Name"]`).First()
	if quotedUserName.Length() == 0 {
		return ""
	}
	// Find the closest parent with aria-labelledby
	quotedTweetContainer := quotedUserName.Closest(`[aria-labelledby*="id__"]`)
	if quotedTweetContainer.Length() == 0 {
		return ""
	}
	return t.extractTweet(quotedTweetContainer)
}
