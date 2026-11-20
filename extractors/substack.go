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
	postData    substackPost
	hasPreloads bool
}

// substackByline represents an entry in Substack's publishedBylines array.
type substackByline struct {
	Name       string `json:"name"`
	AuthorName string `json:"author_name"`
}

// substackPost represents the relevant fields from Substack's preloaded JSON.
type substackPost struct {
	Title            string          `json:"title"`
	Subtitle         string          `json:"subtitle"`
	CanonicalURL     string          `json:"canonical_url"`
	PostDate         string          `json:"post_date"`
	BodyHTML         string          `json:"body_html"`
	PublishedBylines json.RawMessage `json:"publishedBylines"`
	Audience         string          `json:"audience"`
}

// authorName extracts the primary author name from PublishedBylines, supporting
// both Substack's standard array format and a single-object shape.
func (p substackPost) authorName() string {
	if len(p.PublishedBylines) == 0 {
		return ""
	}
	var list []substackByline
	if err := json.Unmarshal(p.PublishedBylines, &list); err == nil {
		for _, b := range list {
			if b.Name != "" {
				return b.Name
			}
			if b.AuthorName != "" {
				return b.AuthorName
			}
		}
		return ""
	}
	var single substackByline
	if err := json.Unmarshal(p.PublishedBylines, &single); err == nil {
		if single.Name != "" {
			return single.Name
		}
		return single.AuthorName
	}
	return ""
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

// hasSubstackSignature returns true only when strong Substack-specific signals
// (generator meta tag or parsed window._preloads) are present. Used by the
// catchall registry so generic blogs with .post-content are not misclassified.
func (e *SubstackExtractor) hasSubstackSignature() bool {
	if e.hasPreloads {
		return true
	}
	doc := e.GetDocument()
	gen, _ := doc.Find(`meta[name="generator"]`).Attr("content")
	return strings.Contains(strings.ToLower(gen), "substack")
}

// CanExtract returns true if the page looks like a Substack post.
func (e *SubstackExtractor) CanExtract() bool {
	if e.hasSubstackSignature() {
		return true
	}
	doc := e.GetDocument()
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
	contentSel := firstMatchingSelection(doc, []string{
		".available-content .body.markup",
		".body.markup",
		".post-content",
		".available-content",
	}).First()
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
// Supports both direct object literals (`window._preloads = {...};`) and
// Substack's string-wrapped form (`window._preloads = JSON.parse("...");`).
func (e *SubstackExtractor) parsePreloads() {
	doc := e.GetDocument()
	doc.Find("script").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := s.Text()
		idx := strings.Index(text, "window._preloads")
		if idx < 0 {
			return true
		}
		eqIdx := strings.Index(text[idx:], "=")
		if eqIdx < 0 {
			return true
		}
		rhs := strings.TrimSpace(text[idx+eqIdx+1:])
		jsonPayload, ok := extractSubstackPreloadsJSON(rhs)
		if !ok {
			return true
		}

		var preloads substackPreloads
		if err := json.Unmarshal([]byte(jsonPayload), &preloads); err == nil {
			e.postData = preloads.Post
			e.hasPreloads = true
			return false
		}
		return true
	})
}

// extractSubstackPreloadsJSON extracts the JSON object text from the right-hand
// side of a `window._preloads = ...` assignment, handling both `JSON.parse("...")`
// and raw `{...}` object literals followed by arbitrary trailing JS.
func extractSubstackPreloadsJSON(rhs string) (string, bool) {
	if strings.HasPrefix(rhs, "JSON.parse(") {
		inner := strings.TrimSpace(strings.TrimPrefix(rhs, "JSON.parse("))
		if len(inner) == 0 || inner[0] != '"' {
			return "", false
		}
		lit, ok := extractJSONStringLiteral(inner)
		if !ok {
			return "", false
		}
		var decoded string
		if err := json.Unmarshal([]byte(lit), &decoded); err != nil {
			return "", false
		}
		return decoded, true
	}
	if strings.HasPrefix(rhs, "{") {
		return extractBalancedJSONObject(rhs)
	}
	return "", false
}

// extractJSONStringLiteral returns the leading double-quoted JSON string literal
// from s, honoring backslash escapes.
func extractJSONStringLiteral(s string) (string, bool) {
	if len(s) == 0 || s[0] != '"' {
		return "", false
	}
	escaped := false
	for i := 1; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		switch s[i] {
		case '\\':
			escaped = true
		case '"':
			return s[:i+1], true
		}
	}
	return "", false
}

// extractBalancedJSONObject returns the leading balanced `{...}` JSON object
// from s, ignoring braces inside double-quoted strings.
func extractBalancedJSONObject(s string) (string, bool) {
	if len(s) == 0 || s[0] != '{' {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i+1], true
			}
		}
	}
	return "", false
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
	if a := e.postData.authorName(); a != "" {
		return a
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
