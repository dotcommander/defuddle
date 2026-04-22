package extractors

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// c2WikiURLRe matches c2.com CGI wiki URLs (both old ?PageName and path-based).
var c2WikiURLRe = regexp.MustCompile(`(?i)c2\.com/(cgi/wiki|wiki/)`)

// c2WikiPageRe extracts the CamelCase page name from the URL query string.
var c2WikiPageRe = regexp.MustCompile(`[?&]([A-Za-z]\w*)`)

// c2WikiCamelCaseRe splits CamelCase titles into words (e.g. "WelcomeVisitors" → "Welcome Visitors").
var c2WikiCamelCaseRe = regexp.MustCompile(`([a-z])([A-Z])`)

// C2WikiExtractor handles C2 (Ward Cunningham's original wiki) page extraction.
//
// Upstream note: the TypeScript extractor uses canExtractAsync() + a JSON API
// fetch from https://c2.com/wiki/remodel/pages/<Title>. The Go framework is
// synchronous, so this extractor performs DOM extraction from the rendered HTML
// that c2.com's CGI serves at the wiki URL.
//
// TypeScript original:
//
//	export class C2WikiExtractor extends BaseExtractor {
//	  canExtract(): boolean { return false; }
//	  canExtractAsync(): boolean { return this.getPageTitle() !== null; }
//	  async extractAsync(): Promise<ExtractorResult> { ... JSON API ... }
//	}
type C2WikiExtractor struct {
	*ExtractorBase
	pageTitle string // CamelCase title extracted from URL; empty if not a wiki URL
}

// NewC2WikiExtractor creates a new C2 Wiki extractor.
// The page title is resolved from the URL at construction time to keep
// CanExtract / Extract consistent.
func NewC2WikiExtractor(document *goquery.Document, url string, schemaOrgData any) *C2WikiExtractor {
	e := &C2WikiExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
	e.pageTitle = e.resolvePageTitle(url)
	return e
}

// Name returns the extractor identifier.
func (e *C2WikiExtractor) Name() string { return "C2WikiExtractor" }

// CanExtract returns true when the URL matches a C2 wiki page pattern.
func (e *C2WikiExtractor) CanExtract() bool {
	return e.pageTitle != ""
}

// Extract returns the structured content for a C2 wiki page.
// Content is extracted from the rendered HTML that c2.com's CGI serves.
func (e *C2WikiExtractor) Extract() *ExtractorResult {
	doc := e.GetDocument()

	title := c2WikiCamelCaseRe.ReplaceAllString(e.pageTitle, "$1 $2")

	// c2.com wiki CGI pages render their body content inside a plain <body>
	// that contains <p> tags, <hr> separators, and list elements.
	// Extract all body paragraphs and headers, stripping the navigation links.
	contentHTML := e.extractBody(doc)

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		Variables: map[string]string{
			"title": title,
			"site":  "C2 Wiki",
		},
	}
}

// resolvePageTitle parses the CamelCase page name from a C2 wiki URL.
// Returns the page name, or empty string if the URL doesn't match.
func (e *C2WikiExtractor) resolvePageTitle(rawURL string) string {
	if !c2WikiURLRe.MatchString(rawURL) {
		return ""
	}
	if m := c2WikiPageRe.FindStringSubmatch(rawURL); m != nil {
		return m[1]
	}
	// Default entry page.
	return "WelcomeVisitors"
}

// extractBody pulls readable content from the rendered c2.com CGI HTML.
// Navigation links (single-word lines that are just wiki links) and bare
// horizontal rules are excluded; substantive paragraphs are kept.
func (e *C2WikiExtractor) extractBody(doc *goquery.Document) string {
	var parts []string

	doc.Find("body").Children().Each(func(_ int, sel *goquery.Selection) {
		tag := goquery.NodeName(sel)
		switch tag {
		case "form", "script", "style":
			return
		case "hr":
			parts = append(parts, "<hr>")
		default:
			h, err := goquery.OuterHtml(sel)
			if err == nil && strings.TrimSpace(h) != "" {
				parts = append(parts, h)
			}
		}
	})

	return strings.Join(parts, "\n")
}
