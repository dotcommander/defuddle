package extractors

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// extractQuotedPost renders a nested quoted post, trying the pressable-container
// path first, then a server-HTML fallback using /post/ links with text content.
func (e *ThreadsExtractor) extractQuotedPost(container *goquery.Selection) string {
	// Browser DOM: quoted post is a nested [data-pressable-container].
	nested := container.Find(`[data-pressable-container]`).First()
	if nested.Length() > 0 {
		username := e.getUsername(nested)
		date := e.getDate(nested)

		contentParts := quotedPostParagraphs(nested)

		author := ""
		if username != "" {
			author = "@" + username
		}
		return buildQuotedPost(author, strings.Join(contentParts, "\n"), date, "")
	}

	// Server-rendered HTML fallback: a /post/ link with non-timestamp text.
	var result string
	container.Find(`a[href*="/post/"]`).EachWithBreak(func(_ int, a *goquery.Selection) bool {
		text := strings.TrimSpace(a.Text())
		if threadsTimestampRe.MatchString(text) {
			return true
		}
		href := a.AttrOr("href", "")
		m := threadsPostAuthorRe.FindStringSubmatch(href)
		if len(m) < 2 {
			return true
		}
		username := m[1]
		permalink := href
		if !strings.HasPrefix(permalink, "http") {
			permalink = "https://www.threads.com" + permalink
		}
		result = buildQuotedPost("@"+username, fmt.Sprintf("<p>%s</p>", html.EscapeString(text)), "", permalink)
		return false
	})
	return result
}

// quotedPostParagraphs collects content paragraphs from a quoted post's spans,
// skipping buttons, timestamps, non-post author links, and noise tokens.
func quotedPostParagraphs(nested *goquery.Selection) []string {
	var contentParts []string
	nested.Find(`span[dir="auto"]`).Each(func(_ int, span *goquery.Selection) {
		if span.Closest(`[role="button"], time`).Length() > 0 {
			return
		}
		link := span.Closest(`a[href^="/@"]`)
		if link.Length() > 0 && !strings.Contains(link.AttrOr("href", ""), "/post/") {
			return
		}
		text := strings.TrimSpace(span.Text())
		if text == "" || text == "·" || text == "Author" {
			return
		}
		if threadsThreadNumberRe.MatchString(text) {
			return
		}
		contentParts = append(contentParts, fmt.Sprintf("<p>%s</p>", html.EscapeString(text)))
	})
	return contentParts
}

// getUsername resolves the author handle from /@username links inside a container.
func (e *ThreadsExtractor) getUsername(container *goquery.Selection) string {
	var found string
	container.Find(`a[href^="/@"][role="link"]`).EachWithBreak(func(_ int, a *goquery.Selection) bool {
		text := strings.TrimSpace(a.Text())
		if text != "" && !strings.Contains(text, "profile picture") {
			found = text
			return false
		}
		return true
	})
	if found != "" {
		return found
	}
	href := container.Find(`a[href^="/@"]`).First().AttrOr("href", "")
	if m := threadsUsernameHrefRe.FindStringSubmatch(href); len(m) > 1 {
		return m[1]
	}
	return ""
}

// createDescription returns a plain-text excerpt (≤140 runes) from the first
// relevant span[dir="auto"] in a container.
func (e *ThreadsExtractor) createDescription(container *goquery.Selection) string {
	if container == nil {
		return ""
	}
	var result string
	container.Find(`span[dir="auto"]`).EachWithBreak(func(_ int, span *goquery.Selection) bool {
		if span.Closest(`a[href^="/@"], [role="button"], a[href*="/post/"], time`).Length() > 0 {
			return true
		}
		text := strings.TrimSpace(span.Text())
		if text == "" || text == "Author" || text == "·" || text == "Top" || text == "View activity" {
			return true
		}
		cleaned := threadsThreadNumberRe.ReplaceAllString(text, "")
		cleaned = strings.TrimSpace(whitespaceRe.ReplaceAllString(cleaned, " "))
		if cleaned == "" {
			return true
		}
		runes := []rune(cleaned)
		if len(runes) > 140 {
			cleaned = string(runes[:140])
		}
		result = cleaned
		return false
	})
	return result
}

// unwrapRedirectURL decodes https://l.threads.com/?u=<encoded> → target URL.
// Returns href unchanged on any failure.
func unwrapRedirectURL(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	encoded := u.Query().Get("u")
	if encoded == "" {
		return href
	}
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return href
	}
	return decoded
}
