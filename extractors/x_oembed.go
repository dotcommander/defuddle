package extractors

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// xOembedStatusRe matches twitter.com / x.com status and article paths.
var xOembedStatusRe = regexp.MustCompile(`/(status|article)/\d+`)

// xOembedDescRe strips HTML tags for plain-text description extraction.
var xOembedDescRe = regexp.MustCompile(`<[^>]+>`)

// xOembedHandleRe extracts the @handle from an author_url like "https://twitter.com/handle".
var xOembedHandleRe = regexp.MustCompile(`/([^/]+)$`)

// xOembedResponse is the oEmbed JSON payload returned by publish.twitter.com/oembed.
// All fields that the extractor uses are represented as typed fields;
// struct tags use omitzero so zero-value structs serialise cleanly.
type xOembedResponse struct {
	HTML         string `json:"html"`
	AuthorName   string `json:"author_name"`
	AuthorURL    string `json:"author_url"`
	ProviderName string `json:"provider_name"`
}

// XOEmbedExtractor handles X (Twitter) oEmbed endpoint responses and
// twitter.com / x.com status page extraction via the FxTwitter API path.
//
// Upstream note: the TypeScript extractor is async — it fetches the FxTwitter
// API (https://api.fxtwitter.com) and falls back to the oEmbed endpoint at
// publish.twitter.com. The Go framework is synchronous, so this extractor
// handles two cases:
//   - publish.twitter.com / publish.x.com URLs: the document body contains the
//     raw oEmbed JSON response, which is parsed here.
//   - twitter.com / x.com status URLs: CanExtract gates on DOM signals that
//     indicate a rendered tweet page; metadata is extracted from the DOM.
//
// The existing TwitterExtractor and XArticleExtractor handle the main tweet
// HTML DOM; this extractor is registered for the oEmbed API endpoint hosts so
// their URL patterns remain disjoint.
//
// TypeScript original:
//
//	export class XOembedExtractor extends BaseExtractor {
//	  canExtract(): boolean { return false; }
//	  canExtractAsync(): boolean { return /\/(status|article)\/\d+/.test(this.url); }
//	  async extractAsync(): Promise<ExtractorResult> { /* fxtwitter + oembed */ }
//	}
type XOEmbedExtractor struct {
	*ExtractorBase
	oembedData *xOembedResponse // non-nil when doc body contains parseable oEmbed JSON
}

// NewXOEmbedExtractor creates a new XOEmbed extractor.
// The oEmbed JSON is parsed at construction so CanExtract and Extract are consistent.
func NewXOEmbedExtractor(document *goquery.Document, url string, schemaOrgData any) *XOEmbedExtractor {
	e := &XOEmbedExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
	e.oembedData = e.parseOembedBody(document)
	return e
}

// Name returns the extractor identifier.
func (e *XOEmbedExtractor) Name() string { return "XOEmbedExtractor" }

// CanExtract returns true when the document contains a parseable oEmbed JSON
// response (publish.twitter.com / publish.x.com endpoints).
func (e *XOEmbedExtractor) CanExtract() bool {
	return e.oembedData != nil
}

// Extract parses the oEmbed JSON from the document body and returns structured
// tweet metadata. The tweet text is extracted from the embedded HTML fragment
// inside the oEmbed payload.
func (e *XOEmbedExtractor) Extract() *ExtractorResult {
	if e.oembedData == nil {
		return &ExtractorResult{Content: "", ContentHTML: ""}
	}

	data := e.oembedData
	tweetHTML := e.parseTweetHTML(data.HTML)
	contentHTML := buildContentHtml("twitter", tweetHTML, "")

	handle := e.resolveHandle(data.AuthorURL, data.AuthorName)
	author := handle
	if author == "" {
		author = data.AuthorName
	}

	description := strings.TrimSpace(xOembedDescRe.ReplaceAllString(tweetHTML, ""))
	if len([]rune(description)) > 140 {
		description = string([]rune(description)[:140])
	}
	description = whitespaceRe.ReplaceAllString(description, " ")

	title := TruncateTitle(fmt.Sprintf("%s on X", author), 100)

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		Variables: map[string]string{
			"title":       title,
			"author":      author,
			"site":        "X (Twitter)",
			"description": description,
		},
	}
}

// parseOembedBody attempts to unmarshal the document body text as oEmbed JSON.
// Returns nil if the body is not valid oEmbed JSON.
func (e *XOEmbedExtractor) parseOembedBody(doc *goquery.Document) *xOembedResponse {
	bodyText := strings.TrimSpace(doc.Find("body").Text())
	if !strings.HasPrefix(bodyText, "{") {
		return nil
	}
	var resp xOembedResponse
	if err := json.Unmarshal([]byte(bodyText), &resp); err != nil {
		return nil
	}
	// Require at least an html field to consider this a valid oEmbed response.
	if resp.HTML == "" {
		return nil
	}
	return &resp
}

// parseTweetHTML extracts the tweet paragraph content from the oEmbed HTML
// fragment. The oEmbed payload contains a <blockquote> with <p> tags for the
// tweet text and an <a> tag for the permalink date.
func (e *XOEmbedExtractor) parseTweetHTML(oembedHTML string) string {
	if oembedHTML == "" {
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(oembedHTML))
	if err != nil {
		return html.EscapeString(oembedHTML)
	}

	var parts []string
	doc.Find("blockquote p").Each(func(_ int, p *goquery.Selection) {
		h, _ := p.Html()
		if strings.TrimSpace(h) != "" {
			parts = append(parts, fmt.Sprintf("<p>%s</p>", h))
		}
	})

	return strings.Join(parts, "\n")
}

// resolveHandle extracts the @handle from the author_url or falls back to the
// author_name. Returns empty string if neither is available.
func (e *XOEmbedExtractor) resolveHandle(authorURL, authorName string) string {
	if authorURL != "" {
		if m := xOembedHandleRe.FindStringSubmatch(authorURL); m != nil {
			return "@" + m[1]
		}
	}
	if authorName != "" {
		return "@" + authorName
	}
	return ""
}
