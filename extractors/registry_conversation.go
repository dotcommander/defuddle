package extractors

import "regexp"

// registerConversation registers extractors for LLM chat transcript sites:
// ChatGPT, Claude, Grok, and Gemini.
func registerConversation(r *Registry) {
	register(r, NewChatGPTExtractor,
		regexp.MustCompile(`^https?://chatgpt\.com/(c|share)/.*`),
	)
	register(r, NewClaudeExtractor,
		regexp.MustCompile(`^https?://claude\.ai/(chat|share)/.*`),
	)
	register(r, NewGrokExtractor,
		"grok.com",
		"grok.x.ai",
		"x.ai",
		regexp.MustCompile(`^https?://grok\.com/(chat|share)(/.*)?$`),
		regexp.MustCompile(`^https?://grok\.x\.ai.*`),
		regexp.MustCompile(`^https?://x\.ai.*`),
	)
	register(r, NewGeminiExtractor,
		"gemini.google.com",
		regexp.MustCompile(`^https?://gemini\.google\.com/app/.*`),
	)
}
