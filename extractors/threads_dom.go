package extractors

import (
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// getDate returns the YYYY-MM-DD date from the first time[datetime] in a container.
func (e *ThreadsExtractor) getDate(container *goquery.Selection) string {
	dt := container.Find(`time[datetime]`).First().AttrOr("datetime", "")
	if dt == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, dt); err == nil {
		return t.UTC().Format("2006-01-02")
	}
	if len(dt) >= 10 {
		return dt[:10]
	}
	return dt
}

// getPermalink returns the absolute URL of the post's permalink.
func (e *ThreadsExtractor) getPermalink(container *goquery.Selection) string {
	href := container.Find(`a[href*="/post/"]`).First().AttrOr("href", "")
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http") {
		return href
	}
	return "https://www.threads.com" + href
}

// getPostsFromPagelet extracts all top-level posts from a single pagelet.
// Nested [data-pressable-container] elements (quoted posts) are skipped.
func (e *ThreadsExtractor) getPostsFromPagelet(pagelet *goquery.Selection) []ThreadsPost {
	var posts []ThreadsPost
	pagelet.Find(`[data-pressable-container]`).Each(func(_ int, container *goquery.Selection) {
		// Skip quoted posts — Closest() includes self, so check the parent's
		// ancestors to detect nesting without matching the element itself.
		if container.Parent().Closest(`[data-pressable-container]`).Length() > 0 {
			return
		}
		username := e.getUsername(container)
		if username == "" {
			return
		}
		posts = append(posts, ThreadsPost{
			username:  username,
			date:      e.getDate(container),
			permalink: e.getPermalink(container),
			content:   e.extractPostContent(container),
			el:        container,
		})
	})
	return posts
}
