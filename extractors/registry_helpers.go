package extractors

import "github.com/PuerkitoBio/goquery"

// register adds an extractor that applies whenever one of its patterns matches.
// ctor is any New*Extractor constructor; its concrete pointer return type is
// adapted to the BaseExtractor the registry stores.
func register[T BaseExtractor](r *Registry, ctor func(*goquery.Document, string, any) T, patterns ...any) {
	r.Register(ExtractorMapping{
		Patterns: patterns,
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return ctor(doc, url, schemaOrgData)
		},
	})
}

// registerIfExtractable is register for self-gating extractors: the mapping
// applies only when the constructed extractor's CanExtract reports true, else the
// registry falls through to the next mapping. Only the CanExtract guard differs
// from register.
func registerIfExtractable[T BaseExtractor](r *Registry, ctor func(*goquery.Document, string, any) T, patterns ...any) {
	r.Register(ExtractorMapping{
		Patterns: patterns,
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			if e := ctor(doc, url, schemaOrgData); e.CanExtract() {
				return e
			}
			return nil
		},
	})
}
