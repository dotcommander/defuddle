package extractors

import (
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// SubstackExtractor handles Substack newsletter content extraction.
// Substack embeds post data in a window._preloads JSON blob and renders
// articles inside .body.markup or .post-content. This extractor
// prefers the structured JSON for metadata and the rendered HTML for content.
type SubstackExtractor struct {
	*ExtractorBase
	postData substackPost
}

// substackPost represents the relevant fields from Substack's preloaded JSON.
type substackPost struct {
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	CanonicalURL    string `json:"canonical_url"`
	PostDate        string `json:"post_date"`
	BodyHTML        string `json:"body_html"`
	PublishingAttrs struct {
		AuthorName string `json:"author_name"`
	} `json:"publishedBylines"`
	// Nested author structure (Substack's JSON varies)
	Audience string `json:"audience"`
}

// substackPreloads wraps the window._preloads shape that holds the post.
type substackPreloads struct {
	Post substackPost `json:"post"`
}

// NewSubstackExtractor creates a new Substack extractor.
func NewSubstackExtractor(document *goquery.Document, url string, schemaOrgData any) *SubstackExtractor {
	ext := &SubstackExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
	ext.parsePreloads()
	return ext
}

// Name returns the extractor identifier.
func (e *SubstackExtractor) Name() string { return "SubstackExtractor" }

// CanExtract returns true if the page looks like a Substack post.
func (e *SubstackExtractor) CanExtract() bool {
	doc := e.GetDocument()
	// Substack pages include a meta generator tag or characteristic selectors
	gen, _ := doc.Find(`meta[name="generator"]`).Attr("content")
	if strings.Contains(strings.ToLower(gen), "substack") {
		return true
	}
	// Fallback: characteristic DOM selectors
	if doc.Find(".post-content, .body.markup, .available-content").Length() > 0 {
		return true
	}
	return false
}

// Extract returns the structured content.
func (e *SubstackExtractor) Extract() *ExtractorResult {
	doc := e.GetDocument()

	// Determine content HTML: prefer rendered DOM, fall back to JSON body_html
	contentHTML := ""
	contentSel := doc.Find(".available-content .body.markup").First()
	if contentSel.Length() == 0 {
		contentSel = doc.Find(".post-content").First()
	}
	if contentSel.Length() > 0 {
		contentHTML, _ = contentSel.Html()
	}
	if strings.TrimSpace(contentHTML) == "" && e.postData.BodyHTML != "" {
		contentHTML = e.postData.BodyHTML
	}

	// Build variables from best available sources
	title := e.bestTitle(doc)
	author := e.bestAuthor(doc)
	published := e.bestPublished(doc)
	description := e.bestDescription(doc)
	image := e.bestImage(doc)
	siteName := e.bestSiteName(doc)

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		Variables: map[string]string{
			"title":       title,
			"author":      author,
			"published":   published,
			"description": description,
			"image":       image,
			"site":        siteName,
		},
	}
}

// parsePreloads extracts the Substack preloaded JSON data.
func (e *SubstackExtractor) parsePreloads() {
	doc := e.GetDocument()
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()
		idx := strings.Index(text, "window._preloads")
		if idx < 0 {
			return
		}
		// Find the JSON object start
		eqIdx := strings.Index(text[idx:], "=")
		if eqIdx < 0 {
			return
		}
		jsonStr := strings.TrimSpace(text[idx+eqIdx+1:])
		// Trim trailing semicolons
		jsonStr = strings.TrimRight(jsonStr, "; \n\r\t")

		var preloads substackPreloads
		if err := json.Unmarshal([]byte(jsonStr), &preloads); err == nil {
			e.postData = preloads.Post
		}
	})
}

func (e *SubstackExtractor) bestTitle(doc *goquery.Document) string {
	if e.postData.Title != "" {
		return e.postData.Title
	}
	if t := doc.Find("h1.post-title, h1[data-testid='post-title']").First().Text(); t != "" {
		return strings.TrimSpace(t)
	}
	if t := e.metaContent(doc, "og:title"); t != "" {
		return t
	}
	return strings.TrimSpace(doc.Find("title").First().Text())
}

func (e *SubstackExtractor) bestAuthor(doc *goquery.Document) string {
	if a := doc.Find(".author-name, .post-header .byline a").First().Text(); a != "" {
		return strings.TrimSpace(a)
	}
	if a := e.metaContent(doc, "author"); a != "" {
		return a
	}
	return ""
}

func (e *SubstackExtractor) bestPublished(doc *goquery.Document) string {
	if e.postData.PostDate != "" {
		return e.postData.PostDate
	}
	if t, exists := doc.Find("time[datetime]").First().Attr("datetime"); exists {
		return t
	}
	if t := e.metaContent(doc, "article:published_time"); t != "" {
		return t
	}
	return ""
}

func (e *SubstackExtractor) bestDescription(doc *goquery.Document) string {
	if e.postData.Subtitle != "" {
		return e.postData.Subtitle
	}
	if d := e.metaContent(doc, "og:description"); d != "" {
		return d
	}
	return ""
}

func (e *SubstackExtractor) bestImage(doc *goquery.Document) string {
	if img := e.metaContent(doc, "og:image"); img != "" {
		return img
	}
	return ""
}

func (e *SubstackExtractor) bestSiteName(doc *goquery.Document) string {
	if s := e.metaContent(doc, "og:site_name"); s != "" {
		return s
	}
	return "Substack"
}

// metaContent retrieves a meta tag's content by property or name.
func (e *SubstackExtractor) metaContent(doc *goquery.Document, key string) string {
	// Try property first (og: tags)
	if c, exists := doc.Find(`meta[property="` + key + `"]`).First().Attr("content"); exists {
		return strings.TrimSpace(c)
	}
	// Try name
	if c, exists := doc.Find(`meta[name="` + key + `"]`).First().Attr("content"); exists {
		return strings.TrimSpace(c)
	}
	return ""
}
