package extractors

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// leetcodeSuffixRe strips the " - LeetCode" site suffix from og:title.
var leetcodeSuffixRe = regexp.MustCompile(`\s*[-–—]\s*LeetCode\s*$`)

// LeetCodeExtractor handles LeetCode problem page content extraction.
//
// Detection relies on the data-track-load="description_content" element,
// which is present on problem pages only. Content is extracted directly
// from that container; the outer SPA shell is discarded.
//
// TypeScript original:
//
//	export class LeetCodeExtractor extends BaseExtractor {
//	  canExtract(): boolean {
//	    return this.document.querySelector('[data-track-load="description_content"]') !== null;
//	  }
//	  extract(): ExtractorResult {
//	    return { content: '', contentHtml: '',
//	      contentSelector: '[data-track-load="description_content"]', ... }
//	  }
//	}
type LeetCodeExtractor struct {
	*ExtractorBase
}

// NewLeetCodeExtractor creates a new LeetCode extractor.
func NewLeetCodeExtractor(document *goquery.Document, url string, schemaOrgData any) *LeetCodeExtractor {
	return &LeetCodeExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
}

// Name returns the extractor identifier.
func (e *LeetCodeExtractor) Name() string { return "LeetCodeExtractor" }

// CanExtract returns true when the problem description container is present.
func (e *LeetCodeExtractor) CanExtract() bool {
	return e.GetDocument().Find(`[data-track-load="description_content"]`).Length() > 0
}

// Extract returns the structured content for a LeetCode problem page.
// The upstream TS extractor returns a contentSelector; here we extract the
// container HTML directly to stay consistent with how the Go pipeline works.
func (e *LeetCodeExtractor) Extract() *ExtractorResult {
	doc := e.GetDocument()

	ogTitle, _ := doc.Find(`meta[property="og:title"]`).First().Attr("content")
	title := strings.TrimSpace(leetcodeSuffixRe.ReplaceAllString(ogTitle, ""))
	if title == "" {
		title = ogTitle
	}

	content := ""
	if el := doc.Find(`[data-track-load="description_content"]`).First(); el.Length() > 0 {
		content, _ = el.Html()
		content = strings.TrimSpace(content)
	}

	return &ExtractorResult{
		Content:     content,
		ContentHTML: content,
		Variables: map[string]string{
			"title": title,
			"site":  "LeetCode",
		},
	}
}
