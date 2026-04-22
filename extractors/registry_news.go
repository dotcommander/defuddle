package extractors

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// registerNews registers extractors for news and article publishing platforms:
// NYTimes, Medium (including custom domains), LWN, and Substack.
func registerNews(r *Registry) {
	// Substack — matches *.substack.com and custom domains with Substack generator meta
	r.Register(ExtractorMapping{
		Patterns: []any{
			"substack.com",
			regexp.MustCompile(`\.substack\.com`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewSubstackExtractor(doc, url, schemaOrgData)
		},
	})

	// Medium — medium.com, *.medium.com, and custom-domain publications that
	// identify themselves via the og:site_name or al:android:app_name meta tags.
	r.Register(ExtractorMapping{
		Patterns: []any{
			"medium.com",
			regexp.MustCompile(`\.medium\.com`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			m := NewMediumExtractor(doc, url, schemaOrgData)
			if m.CanExtract() {
				return m
			}
			return nil
		},
	})

	// NYTimes — www.nytimes.com and nytimes.com.
	r.Register(ExtractorMapping{
		Patterns: []any{
			"nytimes.com",
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewNytimesExtractor(doc, url, schemaOrgData)
		},
	})

	// LWN — lwn.net and *.lwn.net (technical news site).
	r.Register(ExtractorMapping{
		Patterns: []any{
			"lwn.net",
			regexp.MustCompile(`(?i)\.lwn\.net`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewLWNExtractor(doc, url, schemaOrgData)
		},
	})
}
