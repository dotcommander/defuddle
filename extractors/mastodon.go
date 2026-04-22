package extractors

import (
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// mastodonStatusURLRe matches Mastodon status URLs of the form /@user/<digits>.
var mastodonStatusURLRe = regexp.MustCompile(`/@[^/]+/\d+`)

// MastodonExtractor handles Mastodon status page content extraction.
// Mastodon renders the primary toot inside .detailed-status or
// article.activity-stream, with replies in .activity-stream .entry items.
// Quoted posts appear as .quote-inline elements.
type MastodonExtractor struct {
	*ExtractorBase
}

// NewMastodonExtractor creates a new Mastodon extractor.
func NewMastodonExtractor(document *goquery.Document, url string, schemaOrgData any) *MastodonExtractor {
	slog.Debug("Mastodon extractor initialized", "url", url)
	return &MastodonExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
}

// Name returns the extractor identifier.
func (e *MastodonExtractor) Name() string { return "MastodonExtractor" }

// CanExtract returns true when the page looks like a Mastodon status or profile.
// Mastodon pages share no unique meta generator tag, so we match on DOM structure.
func (e *MastodonExtractor) CanExtract() bool {
	doc := e.GetDocument()

	// Mastodon's detailed status view
	if doc.Find(".detailed-status, .detailed-status__wrapper").Length() > 0 {
		return true
	}
	// Activity stream (older Mastodon / Glitch-soc)
	if doc.Find("article.activity-stream, .activity-stream").Length() > 0 {
		return true
	}
	// Mastodon sets a characteristic app meta tag
	if gen := mastodonMetaAttr(doc, `meta[name="application-name"]`, "content"); strings.EqualFold(gen, "Mastodon") {
		return true
	}
	// og:url containing /@user/digits is a strong Mastodon status signal
	if ogURL := mastodonMetaAttr(doc, `meta[property="og:url"]`, "content"); mastodonStatusURLRe.MatchString(ogURL) {
		return true
	}
	return false
}

// Extract returns the structured content for a Mastodon status page.
func (e *MastodonExtractor) Extract() *ExtractorResult {
	doc := e.GetDocument()

	author := e.extractAuthor(doc)
	published := e.extractPublished(doc)
	title := e.buildTitle(doc, author, published)
	postHTML := e.extractPostHTML(doc)
	comments := e.extractReplies(doc)
	contentHTML := buildContentHtml("mastodon", postHTML, comments)

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		Variables: map[string]string{
			"title":     title,
			"author":    author,
			"published": published,
			"site":      "Mastodon",
		},
	}
}

// extractAuthor resolves the post author from structured data or DOM.
// Priority: og:article:author → meta author → DOM display name → og:site_name.
func (e *MastodonExtractor) extractAuthor(doc *goquery.Document) string {
	if a := mastodonMetaAttr(doc, `meta[property="og:article:author"]`, "content"); a != "" {
		return a
	}
	if a := mastodonMetaAttr(doc, `meta[name="author"]`, "content"); a != "" {
		return a
	}
	if a := strings.TrimSpace(doc.Find(".detailed-status__display-name strong, .account__display-name strong").First().Text()); a != "" {
		return a
	}
	if a := mastodonMetaAttr(doc, `meta[property="og:site_name"]`, "content"); a != "" {
		return a
	}
	return ""
}

// extractPublished returns the ISO datetime of the status.
func (e *MastodonExtractor) extractPublished(doc *goquery.Document) string {
	if t, exists := doc.Find(".detailed-status__datetime[datetime], .status__relative-time[datetime]").First().Attr("datetime"); exists && t != "" {
		return t
	}
	if t := mastodonMetaAttr(doc, `meta[property="article:published_time"]`, "content"); t != "" {
		return t
	}
	return ""
}

// buildTitle constructs a human-readable title for the status.
func (e *MastodonExtractor) buildTitle(doc *goquery.Document, author, published string) string {
	if t := mastodonMetaAttr(doc, `meta[property="og:title"]`, "content"); t != "" {
		return t
	}
	// Compose from author + formatted date when og:title is absent.
	date := ""
	if published != "" {
		if parsed, err := time.Parse(time.RFC3339, published); err == nil {
			date = parsed.Format("Jan 2, 2006")
		} else {
			date = published
		}
	}
	switch {
	case author != "" && date != "":
		return fmt.Sprintf("%s on %s", author, date)
	case author != "":
		return author
	default:
		return "Mastodon Status"
	}
}

// extractPostHTML builds the HTML for the primary toot body.
// Quoted posts are extracted via buildQuotedPost and appended inline.
func (e *MastodonExtractor) extractPostHTML(doc *goquery.Document) string {
	content := doc.Find(".detailed-status .status__content, .detailed-status__content").First()
	if content.Length() == 0 {
		content = doc.Find(".status__content").First()
	}
	if content.Length() == 0 {
		if d := mastodonMetaAttr(doc, `meta[property="og:description"]`, "content"); d != "" {
			return fmt.Sprintf("<p>%s</p>", html.EscapeString(d))
		}
		return ""
	}

	// Extract any quoted post before stripping the wrapper.
	var quotedHTML strings.Builder
	content.Find(".quote-inline, [data-quote]").Each(func(_ int, q *goquery.Selection) {
		qAuthor := strings.TrimSpace(q.Find(".account__display-name, .display-name strong").First().Text())
		qContent, _ := q.Find(".status__content__text, .status__content p").First().Html()
		qDate, _ := q.Find("time[datetime]").First().Attr("datetime")
		qURL, _ := q.Find("a.status__relative-time, a[href*='/@']").First().Attr("href")
		if qContent != "" {
			quotedHTML.WriteString(buildQuotedPost(qAuthor, qContent, qDate, qURL))
		}
		q.Remove()
	})

	bodyHTML, _ := content.Html()
	return bodyHTML + quotedHTML.String()
}

// extractReplies builds comment HTML from the reply statuses in the thread.
// Mastodon renders context as .entry or .status items in .activity-stream.
func (e *MastodonExtractor) extractReplies(doc *goquery.Document) string {
	var comments []CommentData

	doc.Find(".activity-stream .entry, .thread-wrapper .status, .status__wrapper").Each(func(_ int, s *goquery.Selection) {
		// Skip the primary detailed-status — already extracted above.
		if s.Find(".detailed-status").Length() > 0 {
			return
		}

		author := strings.TrimSpace(s.Find(".account__display-name strong, .display-name__html").First().Text())
		if author == "" {
			return
		}
		bodyHTML, _ := s.Find(".status__content, .status__content__text").First().Html()
		if strings.TrimSpace(bodyHTML) == "" {
			return
		}

		date, _ := s.Find("time[datetime]").First().Attr("datetime")
		linkURL, _ := s.Find("a.status__relative-time, a[href*='/@']").First().Attr("href")

		comments = append(comments, CommentData{
			Depth:    0,
			Author:   html.EscapeString(author),
			URL:      linkURL,
			LinkText: date,
			Content:  bodyHTML,
		})
	})

	if len(comments) == 0 {
		return ""
	}
	return renderCommentThread(comments)
}

// mastodonMetaAttr returns a named attribute from the first element matching
// selector, or "" when absent. Scoped to mastodon to avoid collision with any
// future package-level helper of the same name.
func mastodonMetaAttr(doc *goquery.Document, selector, attr string) string {
	v, _ := doc.Find(selector).First().Attr(attr)
	return strings.TrimSpace(v)
}
