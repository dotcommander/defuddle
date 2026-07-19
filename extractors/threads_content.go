package extractors

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// threadsUsernameHrefRe extracts a username from an /@username href.
var threadsUsernameHrefRe = regexp.MustCompile(`/@([^/]+)`)

// threadsThreadNumberRe matches trailing "N/M" fraction text in a span.
var threadsThreadNumberRe = regexp.MustCompile(`\s*\d+\s*/\s*\d+\s*$`)

// threadsThreadNumberFullRe matches a div whose entire text is "N/M" (thread numbering).
var threadsThreadNumberFullRe = regexp.MustCompile(`^\d+/\d+$`)

// threadsPostPermalinkRe matches a post permalink path inside an href.
var threadsPostPermalinkRe = regexp.MustCompile(`/@[\w.]+/post/`)

// threadsTimestampRe matches short date strings like "04/04/26" used as link text.
var threadsTimestampRe = regexp.MustCompile(`^\d{2}/\d{2}/\d{2}$`)

// threadsPostAuthorRe extracts /@username from a /post/ href.
var threadsPostAuthorRe = regexp.MustCompile(`/@([^/]+)/post/`)

// extractPostContent builds HTML for a single post container: text, images,
// link card, and any quoted post.
func (e *ThreadsExtractor) extractPostContent(container *goquery.Selection) string {
	var parts []string

	allSpans := container.Find(`span[dir="auto"]`)
	allSpans.Each(func(_ int, span *goquery.Selection) {
		if p := e.spanParagraph(span); p != "" {
			parts = append(parts, p)
		}
	})

	if imgs := e.extractImages(container); imgs != "" {
		parts = append(parts, imgs)
	}
	if card := e.extractLinkCard(container); card != "" {
		parts = append(parts, card)
	}
	if quoted := e.extractQuotedPost(container); quoted != "" {
		parts = append(parts, quoted)
	}

	return strings.Join(parts, "\n")
}

// spanParagraph returns the cleaned <p>…</p> HTML for a content span, or "" when
// the span is navigational/noise (inside a link/time/button, a known label, or a
// bare thread-number) or yields empty content.
func (e *ThreadsExtractor) spanParagraph(span *goquery.Selection) string {
	if span.Closest(`a[href^="/@"], a[href*="/post/"], a[href*="l.threads.com"], time`).Length() > 0 {
		return ""
	}
	if span.Closest(`[role="button"]`).Length() > 0 {
		return ""
	}
	text := strings.TrimSpace(span.Text())
	if text == "" || text == "Author" || text == "·" || text == "Top" || text == "View activity" {
		return ""
	}
	if threadsThreadNumberRe.MatchString(text) && threadsThreadNumberRe.FindString(text) == text {
		return ""
	}
	cleaned := e.cleanText(span)
	if cleaned == "" {
		return ""
	}
	return fmt.Sprintf("<p>%s</p>", cleaned)
}

// cleanText clones a span, removes thread-number divs, rewrites links, then
// unwraps all remaining span/div elements to produce clean inline HTML.
func (e *ThreadsExtractor) cleanText(span *goquery.Selection) string {
	raw, _ := span.Html()
	clone, err := goquery.NewDocumentFromReader(strings.NewReader("<span>" + raw + "</span>"))
	if err != nil {
		return html.EscapeString(strings.TrimSpace(span.Text()))
	}
	root := clone.Find("span").First()

	// Remove thread-number divs (e.g. <div>1/2</div> with ≥2 child spans).
	root.Find("div").Each(func(_ int, d *goquery.Selection) {
		text := strings.TrimSpace(d.Text())
		if threadsThreadNumberFullRe.MatchString(text) && d.Find("span").Length() >= 2 {
			d.Remove()
		}
	})

	// Rewrite links.
	root.Find("a").Each(func(_ int, a *goquery.Selection) {
		href := a.AttrOr("href", "")
		text := strings.TrimSpace(a.Text())

		// Remove post permalink links entirely.
		if threadsPostPermalinkRe.MatchString(href) {
			a.Remove()
			return
		}

		var newHref string
		switch {
		case strings.Contains(href, "l.threads.com"):
			newHref = unwrapRedirectURL(href)
		case strings.HasPrefix(href, "/@"):
			username := strings.TrimPrefix(href, "/@")
			newHref = "https://www.threads.com/@" + username
			text = "@" + username
		case strings.HasPrefix(href, "http"):
			newHref = href
		default:
			newHref = "https://www.threads.com" + href
		}

		a.SetAttr("href", newHref)
		a.SetHtml(html.EscapeString(text))
	})

	// Unwrap spans and divs (keep child content).
	root.Find("span, div").Each(func(_ int, el *goquery.Selection) {
		inner, _ := el.Html()
		el.ReplaceWithHtml(inner)
	})

	result, _ := root.Html()
	result = strings.TrimSpace(whitespaceRe.ReplaceAllString(result, " "))
	return result
}

// extractImages collects content images, excluding profile pictures and tiny icons.
func (e *ThreadsExtractor) extractImages(container *goquery.Selection) string {
	var imgs []string
	container.Find("img").Each(func(_ int, img *goquery.Selection) {
		alt := img.AttrOr("alt", "")
		src := img.AttrOr("src", "")
		if strings.Contains(alt, "profile picture") || src == "" {
			return
		}
		if img.Closest(`a[href*="l.threads.com"]`).Length() > 0 {
			return
		}
		if w := img.AttrOr("width", ""); w != "" {
			width, err := strconv.Atoi(w)
			if err == nil && width > 0 && width <= 48 {
				return
			}
		}
		imgs = append(imgs, fmt.Sprintf(`<img src="%s" alt="%s" />`, html.EscapeString(src), html.EscapeString(alt)))
	})
	return strings.Join(imgs, "\n")
}

// extractLinkCard renders the external link card (l.threads.com redirect with image).
func (e *ThreadsExtractor) extractLinkCard(container *goquery.Selection) string {
	var result string
	container.Find(`a[href*="l.threads.com"]`).EachWithBreak(func(_ int, a *goquery.Selection) bool {
		img := a.Find("img").First()
		if img.Length() == 0 {
			return true
		}
		href := a.AttrOr("href", "")
		actualURL := unwrapRedirectURL(href)
		imgSrc := img.AttrOr("src", "")
		imgAlt := img.AttrOr("alt", "")
		if imgSrc == "" {
			return true
		}
		result = fmt.Sprintf(`<a href="%s"><img src="%s" alt="%s" /></a>`,
			html.EscapeString(actualURL), html.EscapeString(imgSrc), html.EscapeString(imgAlt))
		return false
	})
	return result
}
