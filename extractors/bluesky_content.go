package extractors

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// bidiStripRe removes Unicode bidi / zero-width markers that Bluesky injects
// around handles: U+200E (LRM), U+200F (RLM), U+200B (ZWSP).
var bidiStripRe = regexp.MustCompile(`[\x{200E}\x{200F}\x{200B}]`)

// blueskyNonNewlineWSRe collapses horizontal whitespace without touching newlines,
// so paragraph splitting on "\n" works correctly after the bidi pass.
var blueskyNonNewlineWSRe = regexp.MustCompile(`[^\S\n]+`)

// extractPostContent builds the HTML for a single post item: text, images,
// link card, and any quoted post.
func (e *BlueskyExtractor) extractPostContent(item *goquery.Selection) string {
	var parts []string
	if text := e.cleanPostText(item); text != "" {
		parts = append(parts, text)
	}
	if imgs := e.extractImages(item); imgs != "" {
		parts = append(parts, imgs)
	}
	if card := e.extractLinkCard(item); card != "" {
		parts = append(parts, card)
	}
	if quoted := e.extractQuotedPost(item); quoted != "" {
		parts = append(parts, quoted)
	}
	return strings.Join(parts, "\n")
}

// cleanPostText extracts and cleans post text from the data-word-wrap div.
// Steps: fix mention/external links → absolute URLs, unwrap spans/divs,
// strip bidi markers, collapse non-newline whitespace, split on "\n" → <p> tags.
func (e *BlueskyExtractor) cleanPostText(item *goquery.Selection) string {
	textDiv := item.Find(`div[data-word-wrap="1"]`).First()
	if textDiv.Length() == 0 {
		return ""
	}
	rawHTML, _ := textDiv.Html()
	if rawHTML == "" {
		return ""
	}

	// Re-parse for DOM manipulation.
	clone, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + rawHTML + "</div>"))
	if err != nil {
		return ""
	}
	root := clone.Find("div").First()

	// Fix mention links: convert DID-based hrefs to readable bsky.app URLs.
	root.Find(`a[href*="/profile/"]`).Each(func(_ int, a *goquery.Selection) {
		text := strings.TrimSpace(a.Text())
		href := a.AttrOr("href", "")
		if strings.HasPrefix(text, "@") {
			a.SetAttr("href", "https://bsky.app/profile/"+text[1:])
			a.SetHtml(text)
		} else if strings.HasPrefix(href, "/profile/") {
			a.SetAttr("href", "https://bsky.app"+href)
		}
	})

	// Normalise external links (already absolute; strip obfuscated class attrs).
	root.Find(`a[href^="http"]`).Each(func(_ int, a *goquery.Selection) {
		href := a.AttrOr("href", "")
		text := strings.TrimSpace(a.Text())
		a.SetAttr("href", href)
		a.SetHtml(html.EscapeString(text))
	})

	// Unwrap spans and divs (keep child content).
	root.Find("span, div").Each(func(_ int, el *goquery.Selection) {
		inner, _ := el.Html()
		el.ReplaceWithHtml(inner)
	})

	combined, _ := root.Html()
	combined = bidiStripRe.ReplaceAllString(combined, "")
	combined = strings.TrimSpace(blueskyNonNewlineWSRe.ReplaceAllString(combined, " "))
	if combined == "" {
		return ""
	}

	var paras []string
	for _, p := range strings.Split(combined, "\n") {
		p = strings.TrimSpace(p)
		if p != "" {
			paras = append(paras, fmt.Sprintf("<p>%s</p>", p))
		}
	}
	return strings.Join(paras, "\n")
}

// extractImages collects content images (feed_thumbnail / feed_fullsize CDN paths).
// Avatar images (avatar_thumbnail) are excluded by selector specificity.
// Thumbnail URLs are upgraded to fullsize in the output.
func (e *BlueskyExtractor) extractImages(item *goquery.Selection) string {
	var imgs []string
	item.Find(`img[src*="/feed_thumbnail/"], img[src*="/feed_fullsize/"]`).Each(func(_ int, img *goquery.Selection) {
		src := img.AttrOr("src", "")
		if src == "" {
			return
		}
		fullSrc := strings.ReplaceAll(src, "/feed_thumbnail/", "/feed_fullsize/")
		imgs = append(imgs, fmt.Sprintf(`<img src="%s" alt="" />`, html.EscapeString(fullSrc)))
	})
	return strings.Join(imgs, "\n")
}

// extractLinkCard renders the external link card attached to a post, if present.
// Bluesky link cards are <a aria-label> elements containing a bordered container.
func (e *BlueskyExtractor) extractLinkCard(item *goquery.Selection) string {
	var result string
	item.Find(`a[aria-label][href^="http"]`).EachWithBreak(func(_ int, a *goquery.Selection) bool {
		if a.Find(`div[style*="border"]`).Length() == 0 {
			return true // not a card
		}
		href := a.AttrOr("href", "")
		title := a.AttrOr("aria-label", "")
		if title == "" {
			return true
		}
		var b strings.Builder
		if img := a.Find("img").First(); img.Length() > 0 {
			src := img.AttrOr("src", "")
			fmt.Fprintf(&b, `<a href="%s"><img src="%s" alt="%s" /></a>`+"\n",
				html.EscapeString(href), html.EscapeString(src), html.EscapeString(title))
		}
		fmt.Fprintf(&b, `<p><a href="%s">%s</a></p>`, html.EscapeString(href), html.EscapeString(title))
		result = b.String()
		return false // stop after first card
	})
	return result
}

// extractQuotedPost renders a nested postThreadItem inside the current item
// as a quoted-post blockquote (first match only).
func (e *BlueskyExtractor) extractQuotedPost(item *goquery.Selection) string {
	var result string
	itemNode := item.Get(0)
	item.Find(`[data-testid^="postThreadItem-by-"]`).EachWithBreak(func(_ int, embed *goquery.Selection) bool {
		if embed.Get(0) == itemNode {
			return true // skip self
		}
		handle := e.getHandle(embed)
		displayName := e.getDisplayName(embed)
		authorLabel := "@" + handle
		if displayName != "" {
			authorLabel = displayName + " @" + handle
		}
		text := e.cleanPostText(embed)
		result = buildQuotedPost(authorLabel, text, "", "")
		return false
	})
	return result
}

// createDescription returns a plain-text excerpt (≤140 runes) from the first
// post item, with bidi markers stripped and whitespace collapsed.
func (e *BlueskyExtractor) createDescription(item *goquery.Selection) string {
	textDiv := item.Find(`div[data-word-wrap="1"]`).First()
	if textDiv.Length() == 0 {
		return ""
	}
	text := bidiStripRe.ReplaceAllString(strings.TrimSpace(textDiv.Text()), "")
	text = whitespaceRe.ReplaceAllString(text, " ")
	runes := []rune(text)
	if len(runes) > 140 {
		text = string(runes[:140])
	}
	return text
}
