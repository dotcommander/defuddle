package extractors

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled patterns for Wikipedia extraction.
var (
	// wikipediaHostRe matches any language subdomain of wikipedia.org.
	wikipediaHostRe = regexp.MustCompile(`(?i)[a-z-]+\.wikipedia\.org`)
	// wikipediaSuffixRe strips the " - Wikipedia" site suffix from og:title.
	wikipediaSuffixRe = regexp.MustCompile(`\s*[-–—]\s*Wikipedia\s*$`)
)

// WikipediaExtractor handles Wikipedia article content extraction.
//
// TypeScript original:
//
//	export class WikipediaExtractor extends BaseExtractor {
//	  canExtract(): boolean {
//	    return this.document.querySelector('#mw-content-text') !== null;
//	  }
//	  extract(): ExtractorResult {
//	    const ogTitle = this.document.querySelector('meta[property="og:title"]')?.getAttribute('content') || '';
//	    const title = ogTitle.replace(/\s*[-–—]\s*Wikipedia\s*$/, '') || ogTitle;
//	    return { content: '', contentHtml: '', contentSelector: '#mw-content-text',
//	      variables: { title, author: 'Wikipedia', site: 'Wikipedia' } };
//	  }
//	}
type WikipediaExtractor struct {
	*ExtractorBase
}

// NewWikipediaExtractor creates a new Wikipedia extractor.
func NewWikipediaExtractor(document *goquery.Document, url string, schemaOrgData any) *WikipediaExtractor {
	return &WikipediaExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
}

// Name returns the extractor identifier.
func (e *WikipediaExtractor) Name() string { return "WikipediaExtractor" }

// CanExtract returns true when the MediaWiki content container is present.
// This is the canonical signal used by the upstream TS extractor.
func (e *WikipediaExtractor) CanExtract() bool {
	return e.GetDocument().Find("#mw-content-text").Length() > 0
}

// Extract returns the structured content for a Wikipedia article.
// Content extraction delegates to the standard pipeline via ContentSelector;
// this extractor only resolves the metadata variables.
func (e *WikipediaExtractor) Extract() *ExtractorResult {
	doc := e.GetDocument()

	rawTitle, _ := doc.Find(`meta[property="og:title"]`).First().Attr("content")
	rawTitle = strings.TrimSpace(rawTitle)

	title := wikipediaSuffixRe.ReplaceAllString(rawTitle, "")
	if title == "" {
		title = rawTitle
	}

	content := e.extractContent(doc)

	return &ExtractorResult{
		Content:     content,
		ContentHTML: content,
		Variables: map[string]string{
			"title":  title,
			"author": "Wikipedia",
			"site":   "Wikipedia",
		},
	}
}

// extractContent pulls the article body from #mw-content-text .mw-parser-output,
// stripping MediaWiki chrome elements that add noise (edit links, navboxes,
// reference brackets, hatnote nav, infobox table borders).
func (e *WikipediaExtractor) extractContent(doc *goquery.Document) string {
	container := doc.Find("#mw-content-text .mw-parser-output").First()
	if container.Length() == 0 {
		container = doc.Find("#mw-content-text").First()
	}
	if container.Length() == 0 {
		return ""
	}

	// Strip edit-section links (e.g. "[edit]" anchors beside headings).
	container.Find(".mw-editsection").Remove()

	// Strip navboxes, portals, and sister-project boxes.
	container.Find(".navbox, .navbox-inner, .portal, .noprint, .mbox-small").Remove()

	// Strip "Further reading" / "See also" / "External links" nav sections.
	container.Find(".navigation-not-searchable").Remove()

	// Strip reference superscripts like [1], [2].
	container.Find("sup.reference").Remove()

	html, _ := container.Html()
	return strings.TrimSpace(html)
}
