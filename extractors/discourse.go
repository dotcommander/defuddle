package extractors

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// DiscourseExtractor handles forum content extraction from sites running the
// Discourse platform. Detection uses the meta[name="generator"] value rather
// than URL patterns — Discourse is installed at arbitrary domains.
//
// Registration note: must appear BEFORE the Mastodon catch-all in initializeBuiltins.
//
// TypeScript original:
//
//	export class DiscourseExtractor extends BaseExtractor {
//	  private isDiscourse: boolean;
//	  constructor(...) {
//	    const generator = document.querySelector('meta[name="generator"]')?.getAttribute('content') || '';
//	    this.isDiscourse = generator.startsWith('Discourse');
//	  }
//	  canExtract(): boolean { return this.isDiscourse && !!document.querySelector('.topic-post'); }
//	}
type DiscourseExtractor struct {
	*ExtractorBase
	isDiscourse bool
}

// NewDiscourseExtractor creates a new Discourse extractor and sniffs the generator tag.
func NewDiscourseExtractor(document *goquery.Document, rawURL string, schemaOrgData any) *DiscourseExtractor {
	generator, _ := document.Find(`meta[name="generator"]`).First().Attr("content")
	return &DiscourseExtractor{
		ExtractorBase: NewExtractorBase(document, rawURL, schemaOrgData),
		isDiscourse:   strings.HasPrefix(generator, "Discourse"),
	}
}

// Name returns the extractor identifier.
func (e *DiscourseExtractor) Name() string { return "DiscourseExtractor" }

// CanExtract returns true when the page has a Discourse generator meta AND
// at least one .topic-post element. Both conditions must hold to avoid false
// positives on pages that embed a Discourse widget without the full SPA.
func (e *DiscourseExtractor) CanExtract() bool {
	return e.isDiscourse && e.GetDocument().Find(".topic-post").Length() > 0
}

// Extract returns the structured content for a Discourse topic page.
func (e *DiscourseExtractor) Extract() *ExtractorResult {
	doc := e.GetDocument()

	title := e.getTopicTitle(doc)
	siteName, _ := doc.Find(`meta[property="og:site_name"]`).First().Attr("content")
	category := strings.TrimSpace(doc.Find(".badge-category__name").First().Text())
	tags := e.getTags(doc)
	published := e.getPublishedDate(doc)

	posts := doc.Find(".topic-post")
	var op *goquery.Selection
	posts.Each(func(_ int, s *goquery.Selection) {
		if op == nil && s.HasClass("topic-owner") {
			op = s
		}
	})

	postContent := ""
	opAuthor := ""
	if op != nil {
		postContent = e.extractPostContent(op)
		opAuthor = e.getAuthor(op)
	}

	// Replies: all posts except the OP.
	var replyPosts []*goquery.Selection
	posts.Each(func(_ int, s *goquery.Selection) {
		if op == nil || s.Get(0) != op.Get(0) {
			replyPosts = append(replyPosts, s)
		}
	})
	comments := e.extractComments(replyPosts)

	contentHTML := buildContentHtml("discourse", postContent, comments)

	author := opAuthor
	if author == "" && posts.Length() > 0 {
		author = e.getAuthor(posts.First())
	}

	description := ""
	if op != nil {
		text := strings.TrimSpace(op.Find(".cooked").First().Text())
		runes := []rune(text)
		if len(runes) > 140 {
			runes = runes[:140]
		}
		description = whitespaceRe.ReplaceAllString(string(runes), " ")
	}

	topicID, _ := doc.Find("h1[data-topic-id]").First().Attr("data-topic-id")

	vars := map[string]string{
		"title":  title,
		"author": author,
		"site":   discourseSiteLabel(siteName),
	}
	if description != "" {
		vars["description"] = description
	}
	if published != "" {
		vars["published"] = published
	}

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"topicId":  topicID,
			"category": category,
			"tags":     strings.Join(tags, ", "),
		},
		Variables: vars,
	}
}

// discourseSiteLabel returns the site name when available, otherwise "Discourse".
func discourseSiteLabel(siteName string) string {
	if strings.TrimSpace(siteName) != "" {
		return siteName
	}
	return "Discourse"
}

// getTopicTitle resolves the topic title from .fancy-title, then h1[data-topic-id].
// SVG icons and topic-status badges are stripped from the h1 clone.
func (e *DiscourseExtractor) getTopicTitle(doc *goquery.Document) string {
	if fancy := doc.Find(".fancy-title").First(); fancy.Length() > 0 {
		return strings.TrimSpace(fancy.Text())
	}
	h1 := doc.Find("h1[data-topic-id]").First()
	if h1.Length() == 0 {
		return ""
	}
	// Clone and strip visual chrome before reading text.
	clone, _ := goquery.NewDocumentFromReader(strings.NewReader(
		func() string { s, _ := h1.Html(); return "<div>" + s + "</div>" }(),
	))
	clone.Find("svg, .topic-statuses").Remove()
	return strings.TrimSpace(clone.Find("div").First().Text())
}

// getTags returns the list of tag names from Discourse tag links.
func (e *DiscourseExtractor) getTags(doc *goquery.Document) []string {
	var tags []string
	doc.Find("a.discourse-tag").Each(func(_ int, a *goquery.Selection) {
		tag, exists := a.Attr("data-tag-name")
		if !exists || tag == "" {
			tag = strings.TrimSpace(a.Text())
		}
		if tag != "" {
			tags = append(tags, tag)
		}
	})
	return tags
}

// getPublishedDate returns the ISO date from article:published_time meta, or "".
func (e *DiscourseExtractor) getPublishedDate(doc *goquery.Document) string {
	content, _ := doc.Find(`meta[property="article:published_time"]`).First().Attr("content")
	if content == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, content)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// getAuthor returns the username from the post's .names a[data-user-card].
func (e *DiscourseExtractor) getAuthor(post *goquery.Selection) string {
	nameLink := post.Find(".names a[data-user-card]").First()
	if nameLink.Length() == 0 {
		return ""
	}
	if name, exists := nameLink.Attr("data-user-card"); exists && name != "" {
		return name
	}
	return strings.TrimSpace(nameLink.Text())
}

// getPostDate returns the ISO date from the post's relative-date element.
func (e *DiscourseExtractor) getPostDate(post *goquery.Selection) string {
	dateEl := post.Find(".relative-date[data-time]").First()
	if dateEl.Length() == 0 {
		return ""
	}
	rawTime, exists := dateEl.Attr("data-time")
	if !exists || rawTime == "" {
		return ""
	}
	var ms int64
	if _, err := fmt.Sscanf(rawTime, "%d", &ms); err != nil || ms == 0 {
		return ""
	}
	return time.Unix(ms/1000, 0).UTC().Format("2006-01-02")
}

// getPostPermalink returns the absolute URL of a post's permalink anchor.
func (e *DiscourseExtractor) getPostPermalink(post *goquery.Selection) string {
	link := post.Find("a.post-date[href]").First()
	if link.Length() == 0 {
		return ""
	}
	href, exists := link.Attr("href")
	if !exists || href == "" {
		return ""
	}
	base, err := url.Parse(e.GetURL())
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

// getLikeCount returns a formatted like count string, or "" if there are none.
func (e *DiscourseExtractor) getLikeCount(post *goquery.Selection) string {
	count := strings.TrimSpace(post.Find("button.like-count").First().Text())
	if count == "" {
		return ""
	}
	return count + " likes"
}

// extractPostContent serializes the .cooked element after stripping visual noise.
func (e *DiscourseExtractor) extractPostContent(post *goquery.Selection) string {
	cooked := post.Find(".cooked").First()
	if cooked.Length() == 0 {
		return ""
	}
	// Strip selection barriers and heading anchor links (visual noise).
	cooked.Find(".cooked-selection-barrier").Remove()
	cooked.Find("a.anchor").Remove()
	inner, _ := cooked.Html()
	return strings.TrimSpace(inner)
}

// extractComments converts reply posts to a flat CommentData slice and renders
// them via the shared renderCommentThread helper.
func (e *DiscourseExtractor) extractComments(replyPosts []*goquery.Selection) string {
	if len(replyPosts) == 0 {
		return ""
	}
	comments := make([]CommentData, 0, len(replyPosts))
	for _, post := range replyPosts {
		author := e.getAuthor(post)
		content := e.extractPostContent(post)
		date := e.getPostDate(post)
		postURL := e.getPostPermalink(post)
		likes := e.getLikeCount(post)

		score := ""
		if likes != "" {
			score = html.EscapeString(likes)
		}

		cd := CommentData{
			Author:  html.EscapeString(author),
			Content: content,
			Depth:   0,
		}
		if postURL != "" {
			cd.URL = html.EscapeString(postURL)
			cd.LinkText = date // show date as the link text (permalink)
		} else {
			cd.Date = date
		}
		if score != "" {
			cd.Extra = fmt.Sprintf(` <span class="comment-score">%s</span>`, score)
		}
		comments = append(comments, cd)
	}
	return renderCommentThread(comments)
}
