package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetSchemaProperty_TableDriven covers all documented edge cases for the
// getSchemaProperty function.
func TestGetSchemaProperty_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     any
		property string
		want     string
	}{
		// ── nil / zero-value inputs ─────────────────────────────────────────
		{
			name:     "nil data returns empty string",
			data:     nil,
			property: "title",
			want:     "",
		},
		{
			name:     "missing top-level property returns empty string",
			data:     map[string]any{"headline": "Hello"},
			property: "title",
			want:     "",
		},

		// ── simple string property ──────────────────────────────────────────
		{
			name:     "simple string property",
			data:     map[string]any{"headline": "My Article"},
			property: "headline",
			want:     "My Article",
		},
		{
			name:     "empty string property returns empty string",
			data:     map[string]any{"headline": ""},
			property: "headline",
			want:     "",
		},

		// ── dotted path – nested object ──────────────────────────────────────
		{
			name: "dotted path author.name",
			data: map[string]any{
				"author": map[string]any{
					"@type": "Person",
					"name":  "Jane Doe",
				},
			},
			property: "author.name",
			want:     "Jane Doe",
		},
		{
			name: "dotted path missing intermediate key",
			data: map[string]any{
				"author": map[string]any{"@type": "Person"},
			},
			property: "author.name",
			want:     "",
		},
		{
			name: "deeply nested mainEntityOfPage.url",
			data: map[string]any{
				"mainEntityOfPage": map[string]any{
					"@type": "WebPage",
					"url":   "https://example.com/article",
				},
			},
			property: "mainEntityOfPage.url",
			want:     "https://example.com/article",
		},

		// ── array of strings → joined with ", " ─────────────────────────────
		{
			name: "array of strings joined with comma",
			data: map[string]any{
				"keywords": []any{"go", "testing", "schema"},
			},
			property: "keywords",
			want:     "go, testing, schema",
		},
		{
			name: "array with empty strings filtered out",
			data: map[string]any{
				"keywords": []any{"go", "", "testing"},
			},
			property: "keywords",
			want:     "go, testing",
		},
		{
			name: "single-element string array",
			data: map[string]any{
				"keywords": []any{"golang"},
			},
			property: "keywords",
			want:     "golang",
		},

		// ── array index notation – author.[0].name ───────────────────────────
		{
			name: "array index [0] access",
			data: map[string]any{
				"author": []any{
					map[string]any{"name": "First Author"},
					map[string]any{"name": "Second Author"},
				},
			},
			property: "author.[0].name",
			want:     "First Author",
		},
		{
			name: "array index [1] access",
			data: map[string]any{
				"author": []any{
					map[string]any{"name": "First Author"},
					map[string]any{"name": "Second Author"},
				},
			},
			property: "author.[1].name",
			want:     "Second Author",
		},
		{
			name: "array index out of bounds returns empty string",
			data: map[string]any{
				"author": []any{
					map[string]any{"name": "Only Author"},
				},
			},
			property: "author.[5].name",
			want:     "",
		},

		// ── array flatten via dotted path without index ──────────────────────
		// The Go implementation flattens all array items by descending into each
		// element when props remain. The `[]` token is NOT a supported flatten
		// operator and returns empty; use "author.name" to collect all names.
		{
			name: "array flatten via author.name collects all names",
			data: map[string]any{
				"author": []any{
					map[string]any{"name": "Alice"},
					map[string]any{"name": "Bob"},
					map[string]any{"name": "Carol"},
				},
			},
			property: "author.name",
			want:     "Alice, Bob, Carol",
		},
		{
			name: "array flatten via author.name single element",
			data: map[string]any{
				"author": []any{
					map[string]any{"name": "Solo Writer"},
				},
			},
			property: "author.name",
			want:     "Solo Writer",
		},
		// ── [] token is unrecognised – returns empty string ──────────────────
		{
			name: "bare [] token in path returns empty (not a supported operator)",
			data: map[string]any{
				"author": []any{
					map[string]any{"name": "Alice"},
					map[string]any{"name": "Bob"},
				},
			},
			property: "author.[].name",
			want:     "",
		},

		// ── object resolves to object → returns .name fallback ──────────────
		{
			name: "path resolves to object with .name returns name",
			data: map[string]any{
				"publisher": map[string]any{
					"@type": "Organization",
					"name":  "Acme Corp",
				},
			},
			property: "publisher",
			want:     "Acme Corp",
		},
		{
			name: "path resolves to object without .name returns empty",
			data: map[string]any{
				"publisher": map[string]any{
					"@type": "Organization",
					"url":   "https://acme.example",
				},
			},
			property: "publisher",
			want:     "",
		},

		// ── non-exact match fallback – property buried in nested object ──────
		{
			name: "non-exact match finds deeply buried headline",
			data: map[string]any{
				"@graph": []any{
					map[string]any{
						"@type":    "Article",
						"headline": "Buried Title",
					},
				},
			},
			property: "headline",
			want:     "Buried Title",
		},
		{
			name: "non-exact match traverses nested map",
			data: map[string]any{
				"outer": map[string]any{
					"inner": map[string]any{
						"datePublished": "2024-01-15",
					},
				},
			},
			property: "datePublished",
			want:     "2024-01-15",
		},

		// ── []any in non-exact match path (recently fixed) ──────────────────
		{
			name: "non-exact match traverses []any graph for nested property",
			data: map[string]any{
				"@graph": []any{
					map[string]any{
						"@type":       "WebSite",
						"url":         "https://site.example",
						"description": "Site description",
					},
					map[string]any{
						"@type":    "Article",
						"headline": "Graph Article Title",
					},
				},
			},
			property: "headline",
			want:     "Graph Article Title",
		},
		{
			name: "non-exact match across []any returns first matching value",
			data: map[string]any{
				"@graph": []any{
					map[string]any{
						"@type": "Person",
						"name":  "Nested Author",
					},
				},
			},
			property: "name",
			want:     "Nested Author",
		},

		// ── mixed types in array ─────────────────────────────────────────────
		// When array contains both strings and non-string scalars the function
		// only joins when ALL items are strings or float64; otherwise it recurses.
		{
			name: "array of float64 values joined",
			data: map[string]any{
				"ratings": []any{float64(4), float64(5), float64(3)},
			},
			property: "ratings",
			want:     "4, 5, 3",
		},

		// ── image.url and image patterns ────────────────────────────────────
		{
			name: "image.url dotted path",
			data: map[string]any{
				"image": map[string]any{
					"@type": "ImageObject",
					"url":   "https://example.com/img.jpg",
				},
			},
			property: "image.url",
			want:     "https://example.com/img.jpg",
		},
		{
			name: "image as plain string",
			data: map[string]any{
				"image": "https://example.com/cover.jpg",
			},
			property: "image",
			want:     "https://example.com/cover.jpg",
		},

		// ── language ────────────────────────────────────────────────────────
		{
			name: "inLanguage simple string",
			data: map[string]any{
				"@type":      "Article",
				"inLanguage": "en",
			},
			property: "inLanguage",
			want:     "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getSchemaProperty(tt.data, tt.property)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExtract_SchemaOrgTitle verifies that Extract uses schema.org headline for title.
func TestExtract_SchemaOrgTitle(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head><title>HTML Title</title></head><body></body></html>`)
	schema := map[string]any{
		"@type":    "Article",
		"headline": "Schema Headline",
	}
	m := Extract(doc, schema, nil, "")
	assert.Equal(t, "Schema Headline", m.Title)
}

// TestExtract_SchemaOrgAuthorSingle verifies single author object extraction.
func TestExtract_SchemaOrgAuthorSingle(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	schema := map[string]any{
		"@type": "Article",
		"author": map[string]any{
			"@type": "Person",
			"name":  "Jane Doe",
		},
	}
	m := Extract(doc, schema, nil, "")
	assert.Equal(t, "Jane Doe", m.Author)
}

// TestExtract_SchemaOrgAuthorArray verifies multiple authors from an array.
func TestExtract_SchemaOrgAuthorArray(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	schema := map[string]any{
		"@type": "Article",
		"author": []any{
			map[string]any{"@type": "Person", "name": "Alice"},
			map[string]any{"@type": "Person", "name": "Bob"},
		},
	}
	m := Extract(doc, schema, nil, "")
	assert.Equal(t, "Alice, Bob", m.Author)
}

// TestExtract_SchemaOrgDatePublished verifies date extraction from schema.org.
func TestExtract_SchemaOrgDatePublished(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	schema := map[string]any{
		"@type":         "Article",
		"datePublished": "2024-06-15T09:00:00Z",
	}
	m := Extract(doc, schema, nil, "")
	assert.Contains(t, m.Published, "2024")
}

// TestExtract_SchemaOrgDescription verifies description from schema.org.
func TestExtract_SchemaOrgDescription(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	schema := map[string]any{
		"@type":       "Article",
		"description": "A schema.org description",
	}
	m := Extract(doc, schema, nil, "")
	assert.Equal(t, "A schema.org description", m.Description)
}

// TestExtract_SchemaOrgImageObject verifies image URL extraction from ImageObject.
func TestExtract_SchemaOrgImageObject(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	schema := map[string]any{
		"@type": "Article",
		"image": map[string]any{
			"@type": "ImageObject",
			"url":   "https://example.com/article.jpg",
		},
	}
	m := Extract(doc, schema, nil, "")
	assert.Equal(t, "https://example.com/article.jpg", m.Image)
}

// TestExtract_SchemaOrgImageString verifies plain string image extraction.
func TestExtract_SchemaOrgImageString(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	schema := map[string]any{
		"@type": "Article",
		"image": "https://example.com/direct.jpg",
	}
	m := Extract(doc, schema, nil, "")
	assert.Equal(t, "https://example.com/direct.jpg", m.Image)
}

// TestExtract_SchemaOrgPublisherName verifies site name from publisher.
func TestExtract_SchemaOrgPublisherName(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	schema := map[string]any{
		"@type": "Article",
		"publisher": map[string]any{
			"@type": "Organization",
			"name":  "Example News",
		},
	}
	m := Extract(doc, schema, nil, "")
	assert.Equal(t, "Example News", m.Site)
}

// TestExtract_SchemaOrgInLanguage verifies language extraction from schema.org.
func TestExtract_SchemaOrgInLanguage(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	schema := map[string]any{
		"@type":      "Article",
		"inLanguage": "fr",
	}
	m := Extract(doc, schema, nil, "")
	assert.Equal(t, "fr", m.Language)
}

// TestExtract_SchemaOrgGraphArray verifies extraction from @graph array structure.
func TestExtract_SchemaOrgGraphArray(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	// @graph is a common JSON-LD pattern where multiple typed nodes sit in an array.
	schema := map[string]any{
		"@context": "https://schema.org",
		"@graph": []any{
			map[string]any{
				"@type": "WebSite",
				"name":  "My Site",
				"url":   "https://mysite.example",
			},
			map[string]any{
				"@type":         "Article",
				"headline":      "Graph Article",
				"datePublished": "2024-03-01",
				"author": map[string]any{
					"@type": "Person",
					"name":  "Graph Author",
				},
			},
		},
	}
	m := Extract(doc, schema, nil, "")
	// The non-exact match fallback should find headline buried in the @graph array.
	assert.Equal(t, "Graph Article", m.Title)
}

// TestExtract_SchemaOrgNilData verifies Extract is safe with nil schema data.
func TestExtract_SchemaOrgNilData(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head><title>Fallback</title></head><body></body></html>`)
	m := Extract(doc, nil, nil, "")
	assert.NotNil(t, m)
	assert.Equal(t, "Fallback", m.Title)
}
