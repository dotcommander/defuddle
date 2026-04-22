package extractors

// TypeScript original code - ExtractorRegistry functionality
// Reference: extractor-registry.ts

import (
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

// ExtractorConstructor represents a function that creates an extractor
// TypeScript original code:
//
//	type ExtractorConstructor = new (document: Document, url: string, schemaOrgData?: any) => BaseExtractor;
type ExtractorConstructor func(document *goquery.Document, url string, schemaOrgData any) BaseExtractor

// ExtractorMapping represents the mapping configuration for an extractor
// TypeScript original code:
//
//	interface ExtractorMapping {
//	  patterns: (string | RegExp)[];
//	  extractor: ExtractorConstructor;
//	}
type ExtractorMapping struct {
	Patterns  []any // Can be string or *regexp.Regexp
	Extractor ExtractorConstructor
}

// Registry manages site-specific extractors with a clean, extensible API
// TypeScript original code:
//
//	export class ExtractorRegistry {
//	  private static mappings: ExtractorMapping[] = [];
//	  private static domainCache: Map<string, ExtractorConstructor | null> = new Map();
//	}
type Registry struct {
	mappings    []ExtractorMapping
	domainCache sync.Map // Cache for domain -> constructor mappings
}

// NewRegistry creates a new extractor registry
// TypeScript original code:
//
//	constructor() { this.initialize(); }
func NewRegistry() *Registry {
	registry := &Registry{
		mappings: make([]ExtractorMapping, 0),
	}
	return registry
}

// Register adds a new extractor mapping to the registry
// TypeScript original code:
//
//	static register(mapping: ExtractorMapping) {
//	  this.mappings.push(mapping);
//	}
func (r *Registry) Register(mapping ExtractorMapping) *Registry {
	r.mappings = append(r.mappings, mapping)
	return r // Enable method chaining
}

// FindExtractor finds the appropriate extractor for the given URL
// TypeScript original code:
//
//	static findExtractor(document: Document, url: string, schemaOrgData?: any): BaseExtractor | null {
//	  try {
//	    const domain = new URL(url).hostname;
//
//	    // Check cache first
//	    if (this.domainCache.has(domain)) {
//	      const cachedExtractor = this.domainCache.get(domain);
//	      return cachedExtractor ? new cachedExtractor(document, url, schemaOrgData) : null;
//	    }
//
//	    // Find matching extractor
//	    for (const { patterns, extractor } of this.mappings) {
//	      const matches = patterns.some(pattern => {
//	        if (pattern instanceof RegExp) {
//	          return pattern.test(url);
//	        }
//	        return domain.includes(pattern);
//	      });
//
//	      if (matches) {
//	        // Cache the result
//	        this.domainCache.set(domain, extractor);
//	        return new extractor(document, url, schemaOrgData);
//	      }
//	    }
//
//	    // Cache the negative result
//	    this.domainCache.set(domain, null);
//	    return null;
//	  } catch (error) {
//	    console.error('Error in findExtractor:', error);
//	    return null;
//	  }
//	}
func (r *Registry) FindExtractor(document *goquery.Document, urlStr string, schemaOrgData any) BaseExtractor {
	if urlStr == "" {
		return nil
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}

	domain := parsedURL.Hostname()

	// Check cache first
	if cached, ok := r.domainCache.Load(domain); ok {
		if constructor, ok := cached.(ExtractorConstructor); ok && constructor != nil {
			return constructor(document, urlStr, schemaOrgData)
		}
		return nil
	}

	// Find matching extractor
	for _, mapping := range r.mappings {
		if r.matchesPatterns(urlStr, domain, mapping.Patterns) {
			// Cache the result
			r.domainCache.Store(domain, mapping.Extractor)
			return mapping.Extractor(document, urlStr, schemaOrgData)
		}
	}

	// Cache the negative result
	r.domainCache.Store(domain, nil)
	return nil
}

// matchesPatterns checks if the URL matches any of the patterns
// TypeScript original code: pattern matching logic in findExtractor
func (r *Registry) matchesPatterns(urlStr, domain string, patterns []any) bool {
	for _, pattern := range patterns {
		switch p := pattern.(type) {
		case string:
			// Simple domain matching - check if domain ends with the pattern
			// This handles cases like "reddit.com" matching "www.reddit.com"
			if domain == p || strings.HasSuffix(domain, "."+p) {
				return true
			}
			// Also check if the pattern is contained in the domain for backwards compatibility
			if strings.Contains(domain, p) {
				return true
			}
		case *regexp.Regexp:
			// Regex pattern matching
			if p.MatchString(urlStr) {
				return true
			}
		}
	}
	return false
}

// ClearCache clears the domain cache
// TypeScript original code:
//
//	static clearCache() {
//	  this.domainCache.clear();
//	}
func (r *Registry) ClearCache() *Registry {
	r.domainCache.Clear()
	return r // Enable method chaining
}

// MatchesURL checks if a URL matches any pattern in the given mapping.
func (r *Registry) MatchesURL(urlStr string, mapping ExtractorMapping) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return r.matchesPatterns(urlStr, parsedURL.Hostname(), mapping.Patterns)
}

// GetMappings returns a copy of current mappings (read-only access)
// This is a Go-specific method for introspection
func (r *Registry) GetMappings() []ExtractorMapping {
	mappings := make([]ExtractorMapping, len(r.mappings))
	copy(mappings, r.mappings)
	return mappings
}

// Global registry instance and convenience functions
// TypeScript original code initializes extractors automatically

var (
	// DefaultRegistry is the global registry instance that can be extended by users
	DefaultRegistry = NewRegistry()

	// InitializeBuiltins initializes all built-in extractors
	// TypeScript original code:
	//
	//	ExtractorRegistry.initialize();
	InitializeBuiltins = sync.OnceFunc(func() {
		DefaultRegistry.initializeBuiltins()
	})
)

// initializeBuiltins registers all built-in extractors
// TypeScript original code: initialize() method with all extractor registrations
func (r *Registry) initializeBuiltins() {
	// Register XArticle extractor BEFORE Twitter so article pages take priority.
	// TypeScript original code:
	//   // X Article extractor must be registered BEFORE Twitter to take priority
	//   this.register({ patterns: ['x.com', 'twitter.com'], extractor: XArticleExtractor });
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

	// The combined XArticle+Twitter entry above handles both x.com and twitter.com.
	// A separate registration is not needed; twitter.com/x.com domains are already
	// covered with canExtract()-based fallback to TwitterExtractor.

	// Register YouTube extractor
	// TypeScript original code:
	//   this.register({
	//     patterns: ['youtube.com', 'youtu.be', /youtube\.com\/watch\?v=.*/, /youtu\.be\/.*/],
	//     extractor: YoutubeExtractor
	//   });
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

	// Register Reddit extractor
	// TypeScript original code:
	//   this.register({
	//     patterns: ['reddit.com', 'old.reddit.com', 'new.reddit.com', /^https:\/\/[^\/]+\.reddit\.com/],
	//     extractor: RedditExtractor
	//   });
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

	// Register HackerNews extractor
	// TypeScript original code:
	//   this.register({
	//     patterns: [/news\.ycombinator\.com\/item\?id=.*/],
	//     extractor: HackerNewsExtractor
	//   });
	r.Register(ExtractorMapping{
		Patterns: []any{
			regexp.MustCompile(`news\.ycombinator\.com/item\?id=.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewHackerNewsExtractor(doc, url, schemaOrgData)
		},
	})

	// Register ChatGPT extractor
	// TypeScript original code:
	//   this.register({
	//     patterns: [/^https?:\/\/chatgpt\.com\/(c|share)\/.*/],
	//     extractor: ChatGPTExtractor
	//   });
	r.Register(ExtractorMapping{
		Patterns: []any{
			regexp.MustCompile(`^https?://chatgpt\.com/(c|share)/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewChatGPTExtractor(doc, url, schemaOrgData)
		},
	})

	// Register Claude extractor
	// TypeScript original code:
	//   this.register({
	//     patterns: [/^https?:\/\/claude\.ai\/(chat|share)\/.*/],
	//     extractor: ClaudeExtractor
	//   });
	r.Register(ExtractorMapping{
		Patterns: []any{
			regexp.MustCompile(`^https?://claude\.ai/(chat|share)/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewClaudeExtractor(doc, url, schemaOrgData)
		},
	})

	// Register Grok extractor
	// TypeScript original code:
	//   this.register({
	//     patterns: [/^https?:\/\/grok\.com\/(chat|share)(\/.*)?$/],
	//     extractor: GrokExtractor
	//   });
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

	// Register Gemini extractor
	// TypeScript original code:
	//   this.register({
	//     patterns: [/^https?:\/\/gemini\.google\.com\/app\/.*/],
	//     extractor: GeminiExtractor
	//   });
	r.Register(ExtractorMapping{
		Patterns: []any{
			"gemini.google.com",
			regexp.MustCompile(`^https?://gemini\.google\.com/app/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewGeminiExtractor(doc, url, schemaOrgData)
		},
	})

	// Register GitHub extractor
	// TypeScript original code:
	//   this.register({
	//     patterns: ['github.com'],
	//     extractor: GitHubExtractor
	//   });
	r.Register(ExtractorMapping{
		Patterns: []any{
			"github.com",
			regexp.MustCompile(`^https?://github\.com/.*/(issues|pull)/.*`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewGitHubExtractor(doc, url, schemaOrgData)
		},
	})

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
	// Wikipedia — all language subdomains (en, de, zh, simple, etc.).
	r.Register(ExtractorMapping{
		Patterns: []any{
			regexp.MustCompile(`(?i)[a-z-]+\.wikipedia\.org`),
		},
		Extractor: func(doc *goquery.Document, url string, schemaOrgData any) BaseExtractor {
			return NewWikipediaExtractor(doc, url, schemaOrgData)
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

	// DOM-gated catch-all — registered LAST. Tries Discourse first (meta generator
	// signal), then Mastodon (DOM structure signal). Both CanExtract() gate strictly,
	// so non-matching pages return nil. Discourse must precede Mastodon here because
	// some Discourse instances also serve ActivityPub endpoints that contain Mastodon
	// structural selectors.
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

// Convenience functions for working with the default registry

// Register adds a mapping to the default registry
// TypeScript original code: ExtractorRegistry.register (static method)
func Register(mapping ExtractorMapping) {
	InitializeBuiltins() // Ensure built-ins are initialized
	DefaultRegistry.Register(mapping)
}

// FindExtractor finds an extractor using the default registry
// TypeScript original code: ExtractorRegistry.findExtractor (static method)
func FindExtractor(document *goquery.Document, url string, schemaOrgData any) BaseExtractor {
	InitializeBuiltins() // Ensure built-ins are initialized
	return DefaultRegistry.FindExtractor(document, url, schemaOrgData)
}

// ClearCache clears the cache of the default registry
func ClearCache() {
	DefaultRegistry.ClearCache()
}
