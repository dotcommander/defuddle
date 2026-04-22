package extractors

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildNYTScript constructs a window.__preloadedData inline script from a
// Go value, avoiding raw JSON string literals in tests.
func buildNYTScript(t *testing.T, article map[string]any) string {
	t.Helper()
	payload := map[string]any{
		"initialData": map[string]any{
			"data": map[string]any{
				"article": article,
			},
		},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return `<script>window.__preloadedData = ` + string(b) + `;</script>`
}

// buildNYTDoc constructs a minimal NYT page document from an article map.
func buildNYTDoc(t *testing.T, article map[string]any) *goquery.Document {
	t.Helper()
	script := buildNYTScript(t, article)
	rawHTML := `<html><head>` + script + `</head><body></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

// nytMinimalArticle returns a minimal valid article map with body content.
func nytMinimalArticle() map[string]any {
	return map[string]any{
		"headline": map[string]any{
			"default": "Go 1.26 Released",
		},
		"summary":        "The Go team announces version 1.26.",
		"firstPublished": "2026-04-22T00:00:00.000Z",
		"bylines": []any{
			map[string]any{
				"creators": []any{
					map[string]any{"displayName": "Jane Doe"},
					map[string]any{"displayName": "John Smith"},
				},
			},
		},
		"body": map[string]any{
			"content": []any{
				map[string]any{
					"__typename": "ParagraphBlock",
					"content": []any{
						map[string]any{"__typename": "TextInline", "text": "Go 1.26 is now available.", "formats": []any{}},
					},
				},
			},
		},
	}
}

func parseNYTDoc(t *testing.T, rawHTML string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

func TestNytimesExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	t.Run("valid preloaded data", func(t *testing.T) {
		t.Parallel()
		doc := buildNYTDoc(t, nytMinimalArticle())
		ext := NewNytimesExtractor(doc, "https://www.nytimes.com/2026/04/22/article", nil)
		assert.True(t, ext.CanExtract())
	})

	t.Run("no preloaded data script", func(t *testing.T) {
		t.Parallel()
		doc := parseNYTDoc(t, `<html><head></head><body><p>No data.</p></body></html>`)
		ext := NewNytimesExtractor(doc, "https://www.nytimes.com/", nil)
		assert.False(t, ext.CanExtract())
	})

	t.Run("preloaded data with no body blocks", func(t *testing.T) {
		t.Parallel()
		article := map[string]any{
			"headline": map[string]any{"default": "Empty"},
			"body":     map[string]any{"content": []any{}},
		}
		doc := buildNYTDoc(t, article)
		ext := NewNytimesExtractor(doc, "https://www.nytimes.com/", nil)
		assert.False(t, ext.CanExtract())
	})
}

func TestNytimesExtractor_Extract_Metadata(t *testing.T) {
	t.Parallel()

	doc := buildNYTDoc(t, nytMinimalArticle())
	ext := NewNytimesExtractor(doc, "https://www.nytimes.com/2026/04/22/article", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "Go 1.26 Released", result.Variables["title"])
	assert.Equal(t, "Jane Doe, John Smith", result.Variables["author"])
	assert.Equal(t, "2026-04-22T00:00:00.000Z", result.Variables["published"])
	assert.Equal(t, "The Go team announces version 1.26.", result.Variables["description"])
}

func TestNytimesExtractor_Extract_ParagraphBlock(t *testing.T) {
	t.Parallel()

	doc := buildNYTDoc(t, nytMinimalArticle())
	ext := NewNytimesExtractor(doc, "https://www.nytimes.com/2026/04/22/article", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	assert.Contains(t, result.ContentHTML, "<p>Go 1.26 is now available.</p>")
}

func TestNytimesExtractor_Extract_HeadingBlocks(t *testing.T) {
	t.Parallel()

	article := map[string]any{
		"headline": map[string]any{"default": "Test"},
		"body": map[string]any{
			"content": []any{
				map[string]any{
					"__typename": "Heading2Block",
					"content":    []any{map[string]any{"__typename": "TextInline", "text": "Section One", "formats": []any{}}},
				},
				map[string]any{
					"__typename": "Heading3Block",
					"content":    []any{map[string]any{"__typename": "TextInline", "text": "Sub-section", "formats": []any{}}},
				},
			},
		},
	}

	doc := buildNYTDoc(t, article)
	ext := NewNytimesExtractor(doc, "https://www.nytimes.com/test", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	assert.Contains(t, result.ContentHTML, "<h2>Section One</h2>")
	assert.Contains(t, result.ContentHTML, "<h3>Sub-section</h3>")
}

func TestNytimesExtractor_Extract_InlineFormats(t *testing.T) {
	t.Parallel()

	article := map[string]any{
		"headline": map[string]any{"default": "Formats"},
		"body": map[string]any{
			"content": []any{
				map[string]any{
					"__typename": "ParagraphBlock",
					"content": []any{
						map[string]any{
							"__typename": "TextInline",
							"text":       "bold word",
							"formats":    []any{map[string]any{"__typename": "BoldFormat"}},
						},
						map[string]any{
							"__typename": "TextInline",
							"text":       " and a ",
							"formats":    []any{},
						},
						map[string]any{
							"__typename": "TextInline",
							"text":       "link",
							"formats":    []any{map[string]any{"__typename": "LinkFormat", "url": "https://go.dev"}},
						},
					},
				},
			},
		},
	}

	doc := buildNYTDoc(t, article)
	ext := NewNytimesExtractor(doc, "https://www.nytimes.com/test", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	assert.Contains(t, result.ContentHTML, "<strong>bold word</strong>")
	assert.Contains(t, result.ContentHTML, `<a href="https://go.dev">link</a>`)
}

func TestNytimesExtractor_Extract_ImageBlock(t *testing.T) {
	t.Parallel()

	article := map[string]any{
		"headline": map[string]any{"default": "Images"},
		"body": map[string]any{
			"content": []any{
				map[string]any{
					"__typename": "ImageBlock",
					"media": map[string]any{
						"altText": "A gopher",
						"caption": map[string]any{"text": "The Go mascot"},
						"credit":  "Go Team",
						"crops": []any{
							map[string]any{
								"renditions": []any{
									map[string]any{
										"name": "superJumbo",
										"url":  "https://static.nyt.com/gopher-superJumbo.jpg",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	doc := buildNYTDoc(t, article)
	ext := NewNytimesExtractor(doc, "https://www.nytimes.com/test", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	assert.Contains(t, result.ContentHTML, "gopher-superJumbo.jpg")
	assert.Contains(t, result.ContentHTML, "figcaption")
	assert.Contains(t, result.ContentHTML, "Go Team")
}

func TestNytimesExtractor_Extract_SprinkledBodyPreferred(t *testing.T) {
	t.Parallel()

	para := func(text string) map[string]any {
		return map[string]any{
			"__typename": "ParagraphBlock",
			"content":    []any{map[string]any{"__typename": "TextInline", "text": text, "formats": []any{}}},
		}
	}
	article := map[string]any{
		"headline":      map[string]any{"default": "Sprinkled"},
		"sprinkledBody": map[string]any{"content": []any{para("from sprinkled body")}},
		"body":          map[string]any{"content": []any{para("from regular body")}},
	}

	doc := buildNYTDoc(t, article)
	ext := NewNytimesExtractor(doc, "https://www.nytimes.com/test", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	assert.Contains(t, result.ContentHTML, "from sprinkled body")
	assert.NotContains(t, result.ContentHTML, "from regular body")
}

func TestNytimesExtractor_Extract_UndefinedInPayload(t *testing.T) {
	t.Parallel()

	// Simulate a payload containing JS `undefined` values.
	rawScript := `<script>window.__preloadedData = {"initialData":{"data":{"article":{"headline":{"default":"Undef test"},"summary":undefined,"body":{"content":[{"__typename":"ParagraphBlock","content":[{"__typename":"TextInline","text":"content ok","formats":[]}]}]}}}}}</script>`
	rawHTML := `<html><head>` + rawScript + `</head><body></body></html>`

	doc := parseNYTDoc(t, rawHTML)
	ext := NewNytimesExtractor(doc, "https://www.nytimes.com/test", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	assert.Contains(t, result.ContentHTML, "content ok")
}

func TestNytimesExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := parseNYTDoc(t, `<html><body></body></html>`)
	ext := NewNytimesExtractor(doc, "", nil)
	assert.Equal(t, "NytimesExtractor", ext.Name())
}
