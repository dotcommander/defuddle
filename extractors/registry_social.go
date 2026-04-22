package extractors

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// registerSocial registers extractors for social and microblogging platforms:
// Twitter/X, Bluesky, Threads, LinkedIn, and X oEmbed API endpoints.
func registerSocial(r *Registry) {
	// XArticle BEFORE Twitter — article pages take priority; fallback to TwitterExtractor via CanExtract().
	r.Register(ExtractorMapping{
		Patterns: []any{
			"x.com",
			"twitter.com",
			regexp.MustCompile(`x\.com/.*/article/.*`),
			regexp.MustCompile(`twitter\.com/.*/article/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			xa := NewXArticleExtractor(doc, url, schemaOrgData)
			if xa.CanExtract() {
				return xa
			}
			return NewTwitterExtractor(doc, url, schemaOrgData)
		},
	})

	// Bluesky — registered before the Mastodon catch-all so bsky.app URLs are
	// handled by the specific extractor without triggering the DOM scan.
	r.Register(ExtractorMapping{
		Patterns: []any{"bsky.app"},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewBlueskyExtractor(doc, url, schemaOrgData)
		},
	})

	// Threads — registered before the Mastodon catch-all; CanExtract() gates on
	// Threads-specific DOM signals so non-Threads pages fall through cleanly.
	r.Register(ExtractorMapping{
		Patterns: []any{"threads.com", "threads.net"},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewThreadsExtractor(doc, url, schemaOrgData)
		},
	})

	// LinkedIn — publicly-accessible feed posts; login-walled pages return nil.
	r.Register(ExtractorMapping{
		Patterns: []any{"linkedin.com"},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			e := NewLinkedInExtractor(doc, url, schemaOrgData)
			if e.CanExtract() {
				return e
			}
			return nil
		},
	})

	// XOEmbed — oEmbed API endpoint hosts for X (Twitter).
	// URL patterns are disjoint from the twitter.com/x.com entry above which
	// handles the main site HTML; these match only the publish.* API hosts.
	r.Register(ExtractorMapping{
		Patterns: []any{
			"publish.twitter.com",
			"publish.x.com",
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			e := NewXOEmbedExtractor(doc, url, schemaOrgData)
			if e.CanExtract() {
				return e
			}
			return nil
		},
	})
}
