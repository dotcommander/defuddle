package extractors

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func (t *TwitterExtractor) extractTweet(tweet *goquery.Selection) string {
	if tweet == nil || tweet.Length() == 0 {
		return ""
	}

	// Get tweet text
	tweetText := tweet.Find(`[data-testid="tweetText"]`).First()
	tweetHTML, _ := tweetText.Html()
	formattedText := t.formatTweetText(tweetHTML)

	// Get images
	images := t.extractImages(tweet)

	// Get user info and date
	userInfo := t.extractUserInfo(tweet)

	// Extract quoted tweet if present
	quotedContent := t.quotedTweetContent(tweet)

	var result strings.Builder
	result.WriteString(`<div class="tweet">`)
	result.WriteString(`<div class="tweet-header">`)
	fmt.Fprintf(&result, `<span class="tweet-author"><strong>%s</strong> <span class="tweet-handle">%s</span></span>`,
		userInfo.FullName, userInfo.Handle)

	if userInfo.Date != "" {
		fmt.Fprintf(&result, ` <a href="%s" class="tweet-date">%s</a>`, userInfo.Permalink, userInfo.Date)
	}

	result.WriteString(`</div>`)

	if formattedText != "" {
		fmt.Fprintf(&result, `<div class="tweet-text">%s</div>`, formattedText)
	}

	if len(images) > 0 {
		result.WriteString(`<div class="tweet-media">`)
		for _, img := range images {
			result.WriteString(img)
			result.WriteString("\n")
		}
		result.WriteString(`</div>`)
	}

	if quotedContent != "" {
		fmt.Fprintf(&result, `<blockquote class="quoted-tweet">%s</blockquote>`, quotedContent)
	}

	result.WriteString(`</div>`)
	return strings.TrimSpace(result.String())
}

// extractUserInfo extracts user information from a tweet
// TypeScript original code:
//
//	private extractUserInfo(tweet: Element): UserInfo {
//		const userInfo: UserInfo = {
//			fullName: '',
//			handle: '',
//			date: '',
//			permalink: ''
//		};
//
//		const nameElement = tweet.querySelector('[data-testid="User-Name"]');
//		if (!nameElement) return userInfo;
//
//		// Try to get name and handle from links first (main tweet structure)
//		const links = nameElement.querySelectorAll('a');
//		if (links.length >= 2) {
//			userInfo.fullName = links[0].textContent?.trim() || '';
//			userInfo.handle = links[1].textContent?.trim() || '';
//		}
//
//		// If links don't have the info, try to get from spans (quoted tweet structure)
//		if (!userInfo.fullName || !userInfo.handle) {
//			const fullNameSpan = nameElement.querySelector('span[style*="color: rgb(15, 20, 25)"] span');
//			if (fullNameSpan) {
//				userInfo.fullName = fullNameSpan.textContent?.trim() || '';
//			}
//
//			const handleSpan = nameElement.querySelector('span[style*="color: rgb(83, 100, 113)"]');
//			if (handleSpan) {
//				userInfo.handle = handleSpan.textContent?.trim() || '';
//			}
//		}
//
//		// Get timestamp information
//		const timestamp = tweet.querySelector('time');
//		if (timestamp) {
//			const datetime = timestamp.getAttribute('datetime');
//			if (datetime && datetime.length >= 10) {
//				userInfo.date = datetime.substring(0, 10); // YYYY-MM-DD format
//			}
//
//			// Get permalink from parent link
//			const timestampLink = timestamp.closest('a');
//			if (timestampLink) {
//				userInfo.permalink = timestampLink.getAttribute('href') || '';
//			}
//		}
//
//		return userInfo;
//	}
func (t *TwitterExtractor) extractUserInfo(tweet *goquery.Selection) UserInfo {
	userInfo := UserInfo{
		FullName:  "",
		Handle:    "",
		Date:      "",
		Permalink: "",
	}

	nameElement := tweet.Find(`[data-testid="User-Name"]`).First()
	if nameElement.Length() == 0 {
		return userInfo
	}

	// Try to get name and handle from links first (main tweet structure)
	links := nameElement.Find("a")
	if links.Length() >= 2 {
		userInfo.FullName = strings.TrimSpace(links.Eq(0).Text())
		userInfo.Handle = strings.TrimSpace(links.Eq(1).Text())
	}

	// If links don't have the info, try to get from spans (quoted tweet structure)
	if userInfo.FullName == "" || userInfo.Handle == "" {
		fullNameSpan := nameElement.Find(`span[style*="color: rgb(15, 20, 25)"] span`).First()
		if fullNameSpan.Length() > 0 {
			userInfo.FullName = strings.TrimSpace(fullNameSpan.Text())
		}

		handleSpan := nameElement.Find(`span[style*="color: rgb(83, 100, 113)"]`).First()
		if handleSpan.Length() > 0 {
			userInfo.Handle = strings.TrimSpace(handleSpan.Text())
		}
	}

	// Get timestamp information
	userInfo.Date, userInfo.Permalink = tweetTimestampInfo(tweet)

	return userInfo
}

// tweetTimestampInfo returns the date (YYYY-MM-DD) and permalink from a tweet's
// time element, or empty strings when either is absent.
func tweetTimestampInfo(tweet *goquery.Selection) (date, permalink string) {
	timestamp := tweet.Find("time").First()
	if timestamp.Length() == 0 {
		return "", ""
	}
	if datetime, exists := timestamp.Attr("datetime"); exists && len(datetime) >= 10 {
		date = datetime[:10] // YYYY-MM-DD format
	}
	if timestampLink := timestamp.Closest("a"); timestampLink.Length() > 0 {
		if href, exists := timestampLink.Attr("href"); exists {
			permalink = href
		}
	}
	return date, permalink
}
