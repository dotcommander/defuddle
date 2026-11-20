package extractors

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// registerCatchall registers DOM-signature catch-all extractors. These are
// evaluated after all explicit and user-registered mappings and rely on strict
// DOM/meta signatures to gate.
// Discourse is tried before Mastodon because some Discourse instances serve
// ActivityPub endpoints that contain Mastodon structural selectors.
// Custom-domain Medium and Substack publications are also matched here.
func registerCatchall(r *Registry) {
	r.registerCatchallMapping(ExtractorMapping{
		Patterns: []any{regexp.MustCompile(`.*`)},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			if d := NewDiscourseExtractor(doc, url, schemaOrgData); d.CanExtract() {
				return d
			}
			if m := NewMastodonExtractor(doc, url, schemaOrgData); m.CanExtract() {
				return m
			}
			if med := NewMediumExtractor(doc, url, schemaOrgData); med.CanExtract() {
				return med
			}
			if sub := NewSubstackExtractor(doc, url, schemaOrgData); sub.hasSubstackSignature() {
				return sub
			}
			return nil
		},
	})
}
