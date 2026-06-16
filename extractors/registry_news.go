package extractors

import "regexp"

// registerNews registers extractors for news and article publishing platforms:
// NYTimes, Medium (including custom domains), LWN, and Substack.
func registerNews(r *Registry) {
	// Substack — matches *.substack.com and custom domains with Substack generator meta
	register(r, NewSubstackExtractor,
		"substack.com",
		regexp.MustCompile(`\.substack\.com`),
	)
	// Medium — medium.com, *.medium.com, and custom-domain publications that
	// identify themselves via the og:site_name or al:android:app_name meta tags.
	registerIfExtractable(r, NewMediumExtractor,
		"medium.com",
		regexp.MustCompile(`\.medium\.com`),
	)
	// NYTimes — www.nytimes.com and nytimes.com.
	register(r, NewNytimesExtractor,
		"nytimes.com",
	)
	// LWN — lwn.net and *.lwn.net (technical news site).
	register(r, NewLWNExtractor,
		"lwn.net",
		regexp.MustCompile(`(?i)\.lwn\.net`),
	)
}
