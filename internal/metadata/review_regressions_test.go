package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthorDublinCoreFormsAndCitationPrecedence(t *testing.T) {
	doc := parseDoc(t, `<html><body><span class="author">DOM Fallback</span></body></html>`)
	dc := []MetaTag{
		{Name: ptr("dc.creator"), Content: ptr("Smith, Jane")},
		{Name: ptr("dc.creator"), Content: ptr("Jones, Alex")},
		{Property: ptr("dc.creator"), Content: ptr("Smith, Jane")},
		{Property: ptr("dc.creator"), Content: ptr("Taylor, Morgan")},
	}
	assert.Equal(t, "Jane Smith, Alex Jones, Morgan Taylor", getAuthor(doc, nil, dc))
	assert.Equal(t, "Jane Smith, Morgan Taylor", getAuthor(doc, nil, dc[2:]))
	assert.Equal(t, "Jane Smith, Alex Jones", getAuthor(doc, nil, dc[:2]))
	citation := append([]MetaTag{{Name: ptr("citation_author"), Content: ptr("Researcher, Primary")}}, dc...)
	assert.Equal(t, "Primary Researcher", getAuthor(doc, nil, citation))
}

func TestAuthorSchemaObjectsAndArrays(t *testing.T) {
	doc := parseDoc(t, `<html><body><span class="author">DOM Fallback</span></body></html>`)
	for _, tt := range []struct {
		name   string
		author any
		want   string
	}{
		{"object", map[string]any{"name": "Jane Smith"}, "Jane Smith"},
		{"object array", []any{map[string]any{"name": "Jane Smith"}, map[string]any{"name": "Alex Jones"}, map[string]any{"name": "Jane Smith"}}, "Jane Smith, Alex Jones"},
		{"scalar name", map[string]any{"name": []any{"Jane Smith", "Alex Jones"}}, "Jane Smith, Alex Jones"},
		{"unsupported scalar preserves fallback", "Jane Smith", "DOM Fallback"},
	} {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.want, getAuthor(doc, map[string]any{"author": tt.author}, nil)) })
	}
}

func TestAuthorDOMHeuristicsAreBounded(t *testing.T) {
	for _, tt := range []struct{ name, body, want string }{
		{"navigation excluded", `<a href="/authorization">Authorize</a><a href="/authors">Authors index</a>`, ""},
		{"author path", `<a href="/author/jane">Jane Smith</a>`, "Jane Smith"},
		{"three matches", `<a href="/author/a">Alice</a><a href="/author/b">Bob</a><a href="/author/c">Carol</a>`, "Alice, Bob, Carol"},
		{"ambiguous links skipped", `<a href="/author/a">Alice</a><a href="/author/b">Bob</a><a href="/author/c">Carol</a><a href="/author/d">Dan</a><span itemprop="author">Actual Author</span>`, "Actual Author"},
		{"ambiguous classes skipped", `<span class="author">Alice</span><span class="author">Bob</span><span class="author">Carol</span><span class="author">Dan</span>`, ""},
		{"ambiguous grouped links skipped", `<div class="authors"><a>Alice</a><a>Bob</a><a>Carol</a><a>Dan</a></div>`, ""},
		{"explicit authors unrestricted", `<span itemprop="author">Alice</span><span itemprop="author">Bob</span><span itemprop="author">Carol</span><span itemprop="author">Dan</span>`, "Alice, Bob, Carol, Dan"},
		{"deduplicated selectors", `<a class="author" itemprop="author" href="/author/jane">Jane Smith</a>`, "Jane Smith"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := parseDoc(t, `<html><body>`+tt.body+`</body></html>`)
			assert.Equal(t, tt.want, getAuthor(doc, nil, nil))
		})
	}
	doc := parseDoc(t, `<html><body><a href="/author/dom">DOM Author</a></body></html>`)
	assert.Equal(t, "Schema Author", getAuthor(doc, map[string]any{"author": map[string]any{"name": "Schema Author"}}, nil))
	assert.Equal(t, "Meta Author", getAuthor(doc, map[string]any{"author": map[string]any{"name": "Schema Author"}}, []MetaTag{{Name: ptr("author"), Content: ptr("Meta Author")}}))
}

func TestSchemaRecursiveFallbackIsLexicalAndPreservesArrays(t *testing.T) {
	schema := map[string]any{
		"z": map[string]any{"author": map[string]any{"name": "Last"}},
		"a": []any{map[string]any{"author": map[string]any{"name": "Second"}}, map[string]any{"author": map[string]any{"name": "First"}}},
	}
	for range 100 {
		assert.Equal(t, "Second, First, Last", getSchemaProperty(schema, "author.name"))
	}
	schema["author"] = map[string]any{"name": "Direct"}
	assert.Equal(t, "Direct", getSchemaProperty(schema, "author.name"))
}
