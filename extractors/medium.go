package extractors

import (
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// mediumUITextSet is the set of standalone UI strings Medium injects into the
// article DOM. These are stripped before description extraction.
var mediumUITextSet = map[string]struct{}{
	"Member-only story": {},
	"Listen":            {},
	"Share":             {},
	"Top highlight":     {},
	"·":                 {},
	"Press enter or click to view image in full size": {},
}

// mediumDateRe matches a short date string like "Jan 1, 2024" or "January 1, 2024"
// in isolation (used to strip standalone date nodes).
var mediumDateRe = regexp.MustCompile(`^\w{3,9}\s+\d{1,2},\s+\d{4}`)

// mediumRelativeTimeRe matches "· 3 days ago" style nodes.
var mediumRelativeTimeRe = regexp.MustCompile(`^·\s*\d+\s*\w+\s*ago$`)

// mediumReadTimeRe matches "· 5 min read" or "5 min read" style nodes.
var mediumReadTimeRe = regexp.MustCompile(`^·?\s*\d+\s*min\s*read$`)

// MediumExtractor handles Medium article content extraction.
// Detection requires an <article> element AND either the meteredContent class
// or the og:site_name / al:android:app_name meta tag equal to "Medium".
//
// TypeScript original:
//
//	export class MediumExtractor extends BaseExtractor {
//	  private article: Element | null;
//	  constructor(...) {
//	    this.article = document.querySelector('article.meteredContent') || document.querySelector('article');
//	  }
//	  canExtract(): boolean {
//	    if (!this.article) return false;
//	    if (this.article.classList?.contains('meteredContent')) return true;
//	    const siteName = document.querySelector('meta[property="og:site_name"]')?.getAttribute('content') || '';
//	    const appName = document.querySelector('meta[property="al:android:app_name"]')?.getAttribute('content') || '';
//	    return siteName === 'Medium' || appName === 'Medium';
//	  }
//	}
type MediumExtractor struct {
	*ExtractorBase
	article *goquery.Selection
}

// NewMediumExtractor creates a new Medium extractor and resolves the article element.
func NewMediumExtractor(document *goquery.Document, url string, schemaOrgData any) *MediumExtractor {
	e := &MediumExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
	// Prefer metered-content article; fall back to any article.
	if a := document.Find("article.meteredContent").First(); a.Length() > 0 {
		e.article = a
	} else if a := document.Find("article").First(); a.Length() > 0 {
		e.article = a
	}
	return e
}

// Name returns the extractor identifier.
func (e *MediumExtractor) Name() string { return "MediumExtractor" }

// CanExtract returns true when an article element exists and the page looks
// like a Medium publication (metered class, og:site_name, or android app name).
func (e *MediumExtractor) CanExtract() bool {
	if e.article == nil || e.article.Length() == 0 {
		return false
	}
	if e.article.HasClass("meteredContent") {
		return true
	}
	doc := e.GetDocument()
	siteName, _ := doc.Find(`meta[property="og:site_name"]`).First().Attr("content")
	if strings.TrimSpace(siteName) == "Medium" {
		return true
	}
	appName, _ := doc.Find(`meta[property="al:android:app_name"]`).First().Attr("content")
	return strings.TrimSpace(appName) == "Medium"
}

// Extract returns structured content for a Medium article.
// Cleaning happens before description extraction so UI text is not captured.
func (e *MediumExtractor) Extract() *ExtractorResult {
	title := e.getTitle()
	subtitle := e.getSubtitle()
	author := e.getAuthor()
	publication := e.getPublication()

	// Clean before building description — removes UI noise from the live DOM clone.
	e.cleanArticle()
	description := subtitle
	if description == "" {
		description = e.getDescription()
	}

	content := e.buildContent()

	return &ExtractorResult{
		Content:     content,
		ContentHTML: content,
		ExtractedContent: map[string]any{
			"publication": publication,
		},
		Variables: map[string]string{
			"title":       title,
			"author":      author,
			"site":        mediumSiteLabel(publication),
			"description": description,
		},
	}
}

// mediumSiteLabel returns the publication name when present, otherwise "Medium".
func mediumSiteLabel(publication string) string {
	if publication != "" {
		return publication
	}
	return "Medium"
}

// cleanArticle removes Medium UI chrome from the article element in-place.
// Mirrors the upstream TS cleanArticle() method.
func (e *MediumExtractor) cleanArticle() {
	if e.article == nil {
		return
	}

	// Unwrap role="button" containers around images (pipeline strips [role="button"]).
	e.article.Find(`figure [role="button"]`).Each(func(_ int, btn *goquery.Selection) {
		inner, _ := btn.Html()
		btn.ReplaceWithHtml(inner)
	})

	// Demote role="tooltip" so pipeline doesn't strip it.
	e.article.Find(`[role="tooltip"]`).Each(func(_ int, el *goquery.Selection) {
		el.RemoveAttr("role")
	})

	// Remove subscription promo banners (links to medium.com/plans).
	e.article.Find(`a[href*="medium.com/plans"]`).Each(func(_ int, link *goquery.Selection) {
		wrapper := link.Closest("div")
		if wrapper.Length() > 0 && wrapper.Get(0) != e.article.Get(0) {
			wrapper.Remove()
		} else {
			link.Remove()
		}
	})

	// Remove related article previews.
	e.article.Find(`[data-testid="post-preview"]`).Remove()

	// Remove engagement buttons.
	e.article.Find(`[data-testid*="Clap"], [data-testid*="Bookmark"], [data-testid*="Share"], [data-testid*="Response"]`).Remove()

	// Remove author photo, name, and read time UI elements.
	e.article.Find(`[data-testid="authorPhoto"], [data-testid="authorName"], [data-testid="storyReadTime"]`).Remove()

	// Remove standalone UI noise: dates, read-times, and sentinel text.
	e.article.Find("p, span, div").Each(func(_ int, el *goquery.Selection) {
		text := strings.TrimSpace(el.Text())
		if text == "" {
			return
		}
		if _, ok := mediumUITextSet[text]; ok {
			el.Remove()
			return
		}
		if mediumDateRe.MatchString(text) && len(text) < 30 {
			el.Remove()
			return
		}
		if mediumRelativeTimeRe.MatchString(text) {
			el.Remove()
			return
		}
		if mediumReadTimeRe.MatchString(text) {
			el.Remove()
		}
	})
}

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
		allNonWord := true
		for _, r := range text {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				allNonWord = false
				break
			}
		}
		if allNonWord {
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
