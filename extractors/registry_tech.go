package extractors

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// registerTech registers extractors for tech, code, and forum platforms:
// YouTube, Reddit, Hacker News, GitHub, Wikipedia, C2 Wiki, and LeetCode.
func registerTech(r *Registry) {
	r.Register(ExtractorMapping{
		Patterns: []any{
			"youtube.com",
			"youtu.be",
			regexp.MustCompile(`youtube\.com/watch\?v=.*`),
			regexp.MustCompile(`youtu\.be/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewYouTubeExtractor(doc, url, schemaOrgData)
		},
	})

	r.Register(ExtractorMapping{
		Patterns: []any{
			"reddit.com",
			"old.reddit.com",
			"new.reddit.com",
			regexp.MustCompile(`reddit\.com/r/.*/comments/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewRedditExtractor(doc, url, schemaOrgData)
		},
	})

	r.Register(ExtractorMapping{
		Patterns: []any{
			regexp.MustCompile(`news\.ycombinator\.com/item\?id=.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewHackerNewsExtractor(doc, url, schemaOrgData)
		},
	})

	r.Register(ExtractorMapping{
		Patterns: []any{
			"github.com",
			regexp.MustCompile(`^https?://github\.com/.*/(issues|pull)/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewGitHubExtractor(doc, url, schemaOrgData)
		},
	})

	// Wikipedia — all language subdomains (en, de, zh, simple, etc.).
	r.Register(ExtractorMapping{
		Patterns: []any{
			regexp.MustCompile(`(?i)[a-z-]+\.wikipedia\.org`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewWikipediaExtractor(doc, url, schemaOrgData)
		},
	})

	// C2 Wiki — c2.com/cgi/wiki (Ward Cunningham's original wiki).
	r.Register(ExtractorMapping{
		Patterns: []any{
			regexp.MustCompile(`(?i)c2\.com/(cgi/wiki|wiki/)`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			e := NewC2WikiExtractor(doc, url, schemaOrgData)
			if e.CanExtract() {
				return e
			}
			return nil
		},
	})

	// LeetCode — problem pages identified by data-track-load="description_content".
	r.Register(ExtractorMapping{
		Patterns: []any{"leetcode.com"},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			e := NewLeetCodeExtractor(doc, url, schemaOrgData)
			if e.CanExtract() {
				return e
			}
			return nil
		},
	})
}
