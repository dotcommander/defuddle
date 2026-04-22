package extractors

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// registerConversation registers extractors for LLM chat transcript sites:
// ChatGPT, Claude, Grok, and Gemini.
func registerConversation(r *Registry) {
	r.Register(ExtractorMapping{
		Patterns: []any{
			regexp.MustCompile(`^https?://chatgpt\.com/(c|share)/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewChatGPTExtractor(doc, url, schemaOrgData)
		},
	})

	r.Register(ExtractorMapping{
		Patterns: []any{
			regexp.MustCompile(`^https?://claude\.ai/(chat|share)/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewClaudeExtractor(doc, url, schemaOrgData)
		},
	})

	r.Register(ExtractorMapping{
		Patterns: []any{
			"grok.com",
			"grok.x.ai",
			"x.ai",
			regexp.MustCompile(`^https?://grok\.com/(chat|share)(/.*)?$`),
			regexp.MustCompile(`^https?://grok\.x\.ai.*`),
			regexp.MustCompile(`^https?://x\.ai.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewGrokExtractor(doc, url, schemaOrgData)
		},
	})

	r.Register(ExtractorMapping{
		Patterns: []any{
			"gemini.google.com",
			regexp.MustCompile(`^https?://gemini\.google\.com/app/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewGeminiExtractor(doc, url, schemaOrgData)
		},
	})
}
