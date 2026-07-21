package extractors

import (
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

	e.removeUITextNoise()
}

// removeUITextNoise removes p/span/div elements whose text is a known UI label,
// a date, a relative time, or a read-time estimate.
func (e *MediumExtractor) removeUITextNoise() {
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
