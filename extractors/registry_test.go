package extractors

import (
	"regexp"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDoc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	return doc
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	require.NotNil(t, r)
	assert.Empty(t, r.GetMappings())
}

func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	noop := func(_ *goquery.Document, _ string, _ any) BaseExtractor { return nil }
	mapping := ExtractorMapping{
		Patterns:  []any{"example.com"},
		Extractor: noop,
	}

	result := r.Register(mapping)

	// Returns self for chaining.
	assert.Same(t, r, result)
	assert.Len(t, r.GetMappings(), 1)
}

func TestRegistry_Register_MultipleChained(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	noop := func(_ *goquery.Document, _ string, _ any) BaseExtractor { return nil }

	r.Register(ExtractorMapping{Patterns: []any{"a.com"}, Extractor: noop}).
		Register(ExtractorMapping{Patterns: []any{"b.com"}, Extractor: noop})

	assert.Len(t, r.GetMappings(), 2)
}

func TestRegistry_FindExtractor_ByDomainString(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	called := false
	r.Register(ExtractorMapping{
		Patterns: []any{"example.com"},
		Extractor: func(_ *goquery.Document, _ string, _ any) BaseExtractor {
			called = true
			return nil
		},
	})

	doc := newTestDoc(t, "<html><body></body></html>")
	r.FindExtractor(doc, "https://example.com/page", nil)

	assert.True(t, called, "extractor constructor should be called for matching domain")
}

func TestRegistry_FindExtractor_BySubdomain(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	called := false
	r.Register(ExtractorMapping{
		Patterns: []any{"example.com"},
		Extractor: func(_ *goquery.Document, _ string, _ any) BaseExtractor {
			called = true
			return nil
		},
	})

	doc := newTestDoc(t, "<html><body></body></html>")
	r.FindExtractor(doc, "https://www.example.com/page", nil)

	assert.True(t, called, "extractor constructor should match subdomains")
}

func TestRegistry_FindExtractor_ByRegex(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	called := false
	re := regexp.MustCompile(`reddit\.com/r/.*/comments/.*`)
	r.Register(ExtractorMapping{
		Patterns: []any{re},
		Extractor: func(_ *goquery.Document, _ string, _ any) BaseExtractor {
			called = true
			return nil
		},
	})

	doc := newTestDoc(t, "<html><body></body></html>")
	r.FindExtractor(doc, "https://reddit.com/r/golang/comments/abc123/title", nil)

	assert.True(t, called, "extractor constructor should be called for matching regex")
}

func TestRegistry_FindExtractor_NilForUnknown(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	noop := func(_ *goquery.Document, _ string, _ any) BaseExtractor { return nil }
	r.Register(ExtractorMapping{
		Patterns:  []any{"example.com"},
		Extractor: noop,
	})

	doc := newTestDoc(t, "<html><body></body></html>")
	extractor := r.FindExtractor(doc, "https://unknown-site.org/page", nil)

	assert.Nil(t, extractor)
}

func TestRegistry_FindExtractor_EmptyURLReturnsNil(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	doc := newTestDoc(t, "<html><body></body></html>")
	extractor := r.FindExtractor(doc, "", nil)

	assert.Nil(t, extractor)
}

func TestRegistry_DomainCache(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	callCount := 0
	r.Register(ExtractorMapping{
		Patterns: []any{"cached.com"},
		Extractor: func(_ *goquery.Document, _ string, _ any) BaseExtractor {
			callCount++
			return nil
		},
	})

	doc := newTestDoc(t, "<html><body></body></html>")

	// First lookup — traverses mappings and populates cache.
	r.FindExtractor(doc, "https://cached.com/first", nil)
	assert.Equal(t, 1, callCount)

	// Second lookup — cache hit, constructor called again to produce new instance.
	r.FindExtractor(doc, "https://cached.com/second", nil)
	assert.Equal(t, 2, callCount, "constructor is called on each lookup even when cache holds the constructor")
}

func TestRegistry_ClearCache(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	noop := func(_ *goquery.Document, _ string, _ any) BaseExtractor { return nil }
	r.Register(ExtractorMapping{
		Patterns:  []any{"clearcache.com"},
		Extractor: noop,
	})

	doc := newTestDoc(t, "<html><body></body></html>")
	r.FindExtractor(doc, "https://clearcache.com/page", nil) // populate cache

	result := r.ClearCache()

	// Returns self for chaining.
	assert.Same(t, r, result)

	// After clearing, lookup should still work (re-populates cache).
	_ = r.FindExtractor(doc, "https://clearcache.com/page", nil)
}

func TestRegistry_ClearCache_NegativeEntry(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	doc := newTestDoc(t, "<html><body></body></html>")

	// Populate a negative cache entry for an unknown domain.
	r.FindExtractor(doc, "https://nobody.com/page", nil)

	// Clear should remove the negative entry without panicking.
	r.ClearCache()
}

func TestInitializeBuiltins_RegistersNineExtractors(t *testing.T) {
	t.Parallel()

	// Use a fresh registry to avoid coupling to the sync.Once global state.
	r := NewRegistry()
	r.initializeBuiltins()

	mappings := r.GetMappings()
	assert.Len(t, mappings, 10, "should register exactly 10 built-in extractors")
}

func TestInitializeBuiltins_EachExtractorRoutes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
	}{
		{"Twitter", "https://twitter.com/user/status/123"},
		{"YouTube", "https://youtube.com/watch?v=abc123"},
		{"Reddit", "https://reddit.com/r/golang/comments/abc123/title"},
		{"HackerNews", "https://news.ycombinator.com/item?id=12345"},
		{"ChatGPT", "https://chatgpt.com/c/abc-123"},
		{"Claude", "https://claude.ai/chat/abc-123"},
		{"Grok", "https://grok.x.ai/chat"},
		{"Gemini", "https://gemini.google.com/app/abc123"},
		{"GitHub", "https://github.com/owner/repo/issues/1"},
		{"Substack", "https://newsletter.substack.com/p/some-post"},
	}

	r := NewRegistry()
	r.initializeBuiltins()

	doc := newTestDoc(t, "<html><body></body></html>")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			extractor := r.FindExtractor(doc, tc.url, nil)
			assert.NotNil(t, extractor, "expected extractor for %s URL %s", tc.name, tc.url)
		})
	}
}
