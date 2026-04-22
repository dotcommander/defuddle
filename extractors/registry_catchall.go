package extractors

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// registerCatchall registers DOM-signature catch-all extractors. This MUST be
// called last — both patterns match any URL and rely on CanExtract() to gate.
// Discourse is tried before Mastodon because some Discourse instances serve
// ActivityPub endpoints that contain Mastodon structural selectors.
func registerCatchall(r *Registry) {
	r.Register(ExtractorMapping{
		Patterns: []any{regexp.MustCompile(`.*`)},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			if d := NewDiscourseExtractor(doc, url, schemaOrgData); d.CanExtract() {
				return d
			}
			if m := NewMastodonExtractor(doc, url, schemaOrgData); m.CanExtract() {
				return m
			}
			return nil
		},
	})
}
