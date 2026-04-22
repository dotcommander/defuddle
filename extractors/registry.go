package extractors

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

// initializeBuiltins registers all built-in extractors in category order.
// Registration order matters — first match wins, so catchall goes last.
func (r *Registry) initializeBuiltins() {
	registerSocial(r)
	registerNews(r)
	registerTech(r)
	registerConversation(r)
	registerCatchall(r) // MUST be last — contains DOM-gated Mastodon/Discourse
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
