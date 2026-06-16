package extractors

import "regexp"

// registerTech registers extractors for tech, code, and forum platforms:
// YouTube, Reddit, Hacker News, GitHub, Wikipedia, C2 Wiki, and LeetCode.
func registerTech(r *Registry) {
	register(r, NewYouTubeExtractor,
		"youtube.com",
		"youtu.be",
		regexp.MustCompile(`youtube\.com/watch\?v=.*`),
		regexp.MustCompile(`youtu\.be/.*`),
	)
	register(r, NewRedditExtractor,
		"reddit.com",
		"old.reddit.com",
		"new.reddit.com",
		regexp.MustCompile(`reddit\.com/r/.*/comments/.*`),
	)
	register(r, NewHackerNewsExtractor,
		regexp.MustCompile(`news\.ycombinator\.com/item\?id=.*`),
	)
	register(r, NewGitHubExtractor,
		"github.com",
		regexp.MustCompile(`^https?://github\.com/.*/(issues|pull)/.*`),
	)
	// Wikipedia — all language subdomains (en, de, zh, simple, etc.).
	register(r, NewWikipediaExtractor,
		regexp.MustCompile(`(?i)[a-z-]+\.wikipedia\.org`),
	)
	// C2 Wiki — c2.com/cgi/wiki (Ward Cunningham's original wiki).
	registerIfExtractable(r, NewC2WikiExtractor,
		regexp.MustCompile(`(?i)c2\.com/(cgi/wiki|wiki/)`),
	)
	// LeetCode — problem pages identified by data-track-load="description_content".
	registerIfExtractable(r, NewLeetCodeExtractor,
		"leetcode.com",
	)
}
