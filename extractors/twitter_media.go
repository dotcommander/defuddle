package extractors

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// extractImages extracts images from a tweet
// TypeScript original code:
//
//	private extractImages(tweet: Element): string[] {
//		const images: string[] = [];
//
//		// Look for images in different containers
//		const imageSelectors = [
//			'[data-testid="tweetPhoto"]',
//			'[data-testid="tweet-image"]',
//			'img[src*="media"]'
//		];
//
//		// Skip images that are inside quoted tweets
//		const quotedTweet = tweet.querySelector('[aria-labelledby*="id__"]');
//		let quotedTweetContainer: Element | null = null;
//		if (quotedTweet) {
//			const quotedUserName = quotedTweet.querySelector('[data-testid="User-Name"]');
//			if (quotedUserName) {
//				quotedTweetContainer = quotedUserName.closest('[aria-labelledby*="id__"]');
//			}
//		}
//
//		for (const selector of imageSelectors) {
//			tweet.querySelectorAll(selector).forEach(img => {
//				// Skip if the image is inside a quoted tweet
//				if (quotedTweetContainer && quotedTweetContainer.contains(img)) {
//					return;
//				}
//
//				if (img.tagName === 'IMG') {
//					const src = img.getAttribute('src');
//					if (src) {
//						// Improve image quality
//						const highQualitySrc = src.replace(/&name=\w+$/, '&name=large');
//
//						const alt = img.getAttribute('alt') || '';
//						const cleanAlt = alt.trim().replace(/\s+/g, ' ');
//
//						images.push(`<img src="${highQualitySrc}" alt="${cleanAlt}" />`);
//					}
//				}
//			});
//		}
//
//		return images;
//	}
func (t *TwitterExtractor) extractImages(tweet *goquery.Selection) []string {
	var images []string

	// Look for images in different containers
	imageSelectors := []string{
		`[data-testid="tweetPhoto"]`,
		`[data-testid="tweet-image"]`,
		`img[src*="media"]`,
	}

	// Skip images that are inside quoted tweets
	quotedTweet := tweet.Find(`[aria-labelledby*="id__"]`).First()
	var quotedTweetContainer *goquery.Selection
	if quotedTweet.Length() > 0 {
		quotedUserName := quotedTweet.Find(`[data-testid="User-Name"]`).First()
		if quotedUserName.Length() > 0 {
			quotedTweetContainer = quotedUserName.Closest(`[aria-labelledby*="id__"]`)
		}
	}

	for _, selector := range imageSelectors {
		tweet.Find(selector).Each(func(_ int, img *goquery.Selection) {
			// Skip if the image is inside a quoted tweet
			if quotedTweetContainer != nil && quotedTweetContainer.Length() > 0 {
				// Check if img is contained within quotedTweetContainer
				isInQuoted := false
				quotedTweetContainer.Find("*").Each(func(_ int, el *goquery.Selection) {
					if el.Get(0) == img.Get(0) {
						isInQuoted = true
						return
					}
				})
				if isInQuoted {
					return
				}
			}

			if h := tweetImageHTML(img); h != "" {
				images = append(images, h)
			}
		})
	}

	return images
}

// tweetImageHTML returns the high-quality <img> HTML for an img selection
// (forcing &name=large and cleaning the alt text), or "" if it is not an img
// element or has no src.
func tweetImageHTML(img *goquery.Selection) string {
	if goquery.NodeName(img) != "img" {
		return ""
	}
	src, exists := img.Attr("src")
	if !exists {
		return ""
	}
	// Improve image quality
	highQualitySrc := twitterImageNameRe.ReplaceAllString(src, "&name=large")
	cleanAlt := strings.TrimSpace(whitespaceRe.ReplaceAllString(img.AttrOr("alt", ""), " "))
	return fmt.Sprintf(`<img src="%s" alt="%s" />`, highQualitySrc, cleanAlt)
}

// getTweetID extracts the tweet ID from the URL
// TypeScript original code:
//
//	private getTweetId(): string {
//		const match = this.url.match(/status\/(\d+)/);
//		return match ? match[1] : '';
//	}
func (t *TwitterExtractor) getTweetID() string {
	matches := twitterStatusRe.FindStringSubmatch(t.url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// getTweetAuthor extracts the author handle from the main tweet
// TypeScript original code:
//
//	private getTweetAuthor(): string {
//		if (!this.mainTweet) return '';
//
//		const nameElement = this.mainTweet.querySelector('[data-testid="User-Name"]');
//		if (!nameElement) return '';
//
//		const links = nameElement.querySelectorAll('a');
//		if (links.length >= 2) {
//			let handle = links[1].textContent?.trim() || '';
//			if (!handle.startsWith('@')) {
//				handle = '@' + handle;
//			}
//			return handle;
//		}
//
//		return '';
//	}
func (t *TwitterExtractor) getTweetAuthor() string {
	if t.mainTweet == nil {
		return ""
	}

	nameElement := t.mainTweet.Find(`[data-testid="User-Name"]`).First()
	if nameElement.Length() == 0 {
		return ""
	}

	links := nameElement.Find("a")
	if links.Length() >= 2 {
		handle := strings.TrimSpace(links.Eq(1).Text())
		if !strings.HasPrefix(handle, "@") {
			handle = "@" + handle
		}
		return handle
	}

	return ""
}

// createDescription creates a description from the main tweet
// TypeScript original code:
//
//	private createDescription(tweet: Element | null): string {
//		if (!tweet) return '';
//
//		const tweetText = tweet.querySelector('[data-testid="tweetText"]');
//		if (!tweetText) return '';
//
//		let text = tweetText.textContent?.trim() || '';
//		if (text.length > 140) {
//			text = text.substring(0, 140);
//		}
//
//		// Replace multiple spaces with single space
//		return text.replace(/\s+/g, ' ');
//	}
func (t *TwitterExtractor) createDescription(tweet *goquery.Selection) string {
	if tweet == nil || tweet.Length() == 0 {
		return ""
	}

	tweetText := tweet.Find(`[data-testid="tweetText"]`).First()
	if tweetText.Length() == 0 {
		return ""
	}

	text := strings.TrimSpace(tweetText.Text())
	if len(text) > 140 {
		text = text[:140]
	}

	// Replace multiple spaces with single space
	text = whitespaceRe.ReplaceAllString(text, " ")
	return text
}
