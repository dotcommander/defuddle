package extractors

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// blueskyHandleRe extracts the handle from a postThreadItem data-testid attribute.
var blueskyHandleRe = regexp.MustCompile(`^postThreadItem-by-(.+)$`)

// BlueskyExtractor handles content extraction from bsky.app post/thread pages.
// Detection anchors on data-testid attributes, which are stable across Bluesky
// deploys (unlike obfuscated CSS class names).
type BlueskyExtractor struct {
	*ExtractorBase
	threadScreen *goquery.Selection
	postItems    []*goquery.Selection
}

// NewBlueskyExtractor creates a new Bluesky extractor and caches the thread DOM nodes.
func NewBlueskyExtractor(document *goquery.Document, url string, schemaOrgData any) *BlueskyExtractor {
	e := &BlueskyExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
	e.threadScreen = document.Find(`[data-testid="postThreadScreen"]`).First()
	if e.threadScreen.Length() > 0 {
		e.threadScreen.Find(`[data-testid^="postThreadItem-by-"]`).Each(func(_ int, s *goquery.Selection) {
			e.postItems = append(e.postItems, s)
		})
	}
	return e
}

// Name returns the extractor identifier.
func (e *BlueskyExtractor) Name() string { return "BlueskyExtractor" }

// CanExtract returns true when at least one thread post item was found.
func (e *BlueskyExtractor) CanExtract() bool { return len(e.postItems) > 0 }

// Extract builds the structured content for a Bluesky post/thread page.
func (e *BlueskyExtractor) Extract() *ExtractorResult {
	mainHandle := e.getHandle(e.postItems[0])

	// Consecutive items by the main author form the thread; the first item by
	// a different author (and everything after) becomes replies.
	var threadItems, replyItems []*goquery.Selection
	threadEnded := false
	for _, item := range e.postItems {
		if !threadEnded && e.getHandle(item) == mainHandle {
			threadItems = append(threadItems, item)
		} else {
			threadEnded = true
			replyItems = append(replyItems, item)
		}
	}

	var postParts []string
	for _, item := range threadItems {
		postParts = append(postParts, e.extractPostContent(item))
	}
	postContent := strings.Join(postParts, "\n<hr>\n")

	comments := e.extractComments(replyItems)
	contentHTML := buildContentHtml("bluesky", postContent, comments)

	displayName := e.getDisplayName(e.postItems[0])
	author := "@" + mainHandle
	if displayName != "" {
		author = displayName
	}

	titleName := displayName
	if titleName == "" {
		titleName = "@" + mainHandle
	}
	title := TruncateTitle(fmt.Sprintf("%s on Bluesky", titleName), 100)

	vars := map[string]string{
		"title":  title,
		"author": author,
		"site":   "Bluesky",
	}
	if d := e.createDescription(e.postItems[0]); d != "" {
		vars["description"] = d
	}
	if pub := e.getPublishedDate(); pub != "" {
		vars["published"] = pub
	}

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"postAuthor": mainHandle,
		},
		Variables: vars,
	}
}

// extractComments builds a comment-thread HTML string from reply post items.
// Depth is tracked via hasTopConnector: a connector line means this reply
// continues the thread above it (depth increments), otherwise depth resets to 0.
func (e *BlueskyExtractor) extractComments(replyItems []*goquery.Selection) string {
	if len(replyItems) == 0 {
		return ""
	}
	currentDepth := 0
	var comments []CommentData
	for _, item := range replyItems {
		handle := e.getHandle(item)
		displayName := e.getDisplayName(item)
		authorLabel := "@" + handle
		if displayName != "" {
			authorLabel = displayName + " @" + handle
		}
		if hasTopConnector(item) {
			currentDepth++
		} else {
			currentDepth = 0
		}
		comments = append(comments, CommentData{
			Depth:   currentDepth,
			Author:  authorLabel,
			Content: e.extractPostContent(item),
			Date:    e.getReplyDate(item),
			URL:     e.getPermalink(item),
		})
	}
	return renderCommentThread(comments)
}

// hasTopConnector returns true when the reply item has a thread connector line
// in its top spacer area. Connector lines are 2px-wide divs with a
// background-color inline style — matched by inline style because Bluesky's
// CSS class names are obfuscated and rotate on every deploy.
func hasTopConnector(item *goquery.Selection) bool {
	connector := item.Children().First()
	if connector.Length() == 0 {
		return false
	}
	found := false
	connector.Find("div").Each(func(_ int, d *goquery.Selection) {
		if found {
			return
		}
		style := d.AttrOr("style", "")
		if strings.Contains(style, "width: 2px") && strings.Contains(style, "background-color") {
			found = true
		}
	})
	return found
}

// getHandle extracts the author handle from a postThreadItem's data-testid attribute.
func (e *BlueskyExtractor) getHandle(item *goquery.Selection) string {
	m := blueskyHandleRe.FindStringSubmatch(item.AttrOr("data-testid", ""))
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// getDisplayName resolves the display name from the avatar aria-label or profile links.
func (e *BlueskyExtractor) getDisplayName(item *goquery.Selection) string {
	if link := item.Find(`a[aria-label*="avatar"]`).First(); link.Length() > 0 {
		label := link.AttrOr("aria-label", "")
		if idx := strings.LastIndex(label, "'s avatar"); idx > 0 {
			return label[:idx]
		}
	}
	var name string
	item.Find(`a[href^="/profile/"]`).EachWithBreak(func(_ int, a *goquery.Selection) bool {
		text := strings.TrimSpace(a.Text())
		if text != "" && !strings.HasPrefix(text, "@") && !strings.Contains(text, "avatar") && !strings.Contains(text, "·") {
			name = text
			return false
		}
		return true
	})
	return name
}

// getPublishedDate returns the ISO date from the twitter:value1 meta tag.
func (e *BlueskyExtractor) getPublishedDate() string {
	content := e.GetDocument().Find(`meta[name="twitter:value1"]`).AttrOr("content", "")
	if len(content) >= 10 {
		return content[:10]
	}
	return ""
}

// getReplyDate returns the aria-label of the post permalink link (human date string).
func (e *BlueskyExtractor) getReplyDate(item *goquery.Selection) string {
	return item.Find(`a[href*="/post/"]`).First().AttrOr("aria-label", "")
}

// getPermalink returns the absolute permalink for a post item.
func (e *BlueskyExtractor) getPermalink(item *goquery.Selection) string {
	href := item.Find(`a[href*="/post/"]`).First().AttrOr("href", "")
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http") {
		return href
	}
	return "https://bsky.app" + href
}
