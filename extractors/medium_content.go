package extractors

import (
	"html"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// buildContent renders the cleaned article element to HTML.
func (e *MediumExtractor) buildContent() string {
	if e.article == nil {
		return ""
	}
	content, _ := e.article.Html()
	return strings.TrimSpace(content)
}

// getTitle resolves the article title from the storyTitle test-id, then h1.
func (e *MediumExtractor) getTitle() string {
	doc := e.GetDocument()
	if el := doc.Find(`[data-testid="storyTitle"]`).First(); el.Length() > 0 {
		return strings.TrimSpace(el.Text())
	}
	if e.article != nil {
		if h1 := e.article.Find("h1").First(); h1.Length() > 0 {
			return strings.TrimSpace(h1.Text())
		}
	}
	return ""
}

// getSubtitle returns the subtitle paragraph text if present.
func (e *MediumExtractor) getSubtitle() string {
	doc := e.GetDocument()
	text, _ := doc.Find(".pw-subtitle-paragraph").First().Attr("textContent")
	if text == "" {
		text = strings.TrimSpace(doc.Find(".pw-subtitle-paragraph").First().Text())
	}
	return strings.TrimSpace(text)
}

// getAuthor resolves the author name from the authorName test-id.
func (e *MediumExtractor) getAuthor() string {
	doc := e.GetDocument()
	return strings.TrimSpace(doc.Find(`[data-testid="authorName"]`).First().Text())
}

// getPublication resolves the publication name from og:site_name or schema.org.
// Returns "" when the page is a personal Medium blog (site_name == "Medium").
func (e *MediumExtractor) getPublication() string {
	doc := e.GetDocument()
	if siteName, _ := doc.Find(`meta[property="og:site_name"]`).First().Attr("content"); siteName != "" && siteName != "Medium" {
		return siteName
	}
	// Walk schema.org data for publisher.name.
	schemas := normalizeSchemaSlice(e.GetSchemaOrgData())
	for _, s := range schemas {
		if m, ok := s.(map[string]any); ok {
			if pub, ok := m["publisher"].(map[string]any); ok {
				if name, ok := pub["name"].(string); ok && name != "" {
					return name
				}
			}
		}
	}
	return ""
}

// normalizeSchemaSlice ensures schema.org data is always a []any for uniform iteration.
func normalizeSchemaSlice(data any) []any {
	switch v := data.(type) {
	case []any:
		return v
	case nil:
		return nil
	default:
		return []any{v}
	}
}

// getDescription extracts a plain-text description (≤140 chars) from the first
// meaningful paragraph after UI noise has been cleaned.
// hasWordChar reports whether s contains at least one ASCII letter.
func hasWordChar(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

func (e *MediumExtractor) getDescription() string {
	if e.article == nil {
		return ""
	}
	var desc string
	e.article.Find("p").EachWithBreak(func(_ int, p *goquery.Selection) bool {
		text := strings.TrimSpace(p.Text())
		if len(text) < 3 {
			return true
		}
		// Skip purely numeric/punctuation paragraphs.
		if !hasWordChar(text) {
			return true
		}
		runes := []rune(text)
		if len(runes) > 140 {
			text = string(runes[:140])
		}
		text = whitespaceRe.ReplaceAllString(text, " ")
		desc = html.EscapeString(text)
		return false
	})
	return desc
}
