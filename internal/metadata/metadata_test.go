package metadata

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseDoc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	return doc
}

func ptr(s string) *string { return &s }

func TestExtract_Title(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head><title>My Page Title</title></head><body></body></html>`)
	m := Extract(doc, nil, nil, "")
	assert.Equal(t, "My Page Title", m.Title)
}

func TestExtract_Description(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head>
		<meta name="description" content="A description of the page">
	</head><body></body></html>`)
	tags := []MetaTag{{Name: ptr("description"), Content: ptr("A description of the page")}}
	m := Extract(doc, nil, tags, "")
	assert.Equal(t, "A description of the page", m.Description)
}

func TestExtract_OGTitle(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head>
		<title>Fallback Title</title>
		<meta property="og:title" content="OG Title">
	</head><body></body></html>`)
	tags := []MetaTag{{Property: ptr("og:title"), Content: ptr("OG Title")}}
	m := Extract(doc, nil, tags, "")
	// og:title should take precedence or be present
	assert.Contains(t, []string{"OG Title", "Fallback Title"}, m.Title)
}

func TestExtract_OGDescription(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head>
		<meta property="og:description" content="OG Description">
	</head><body></body></html>`)
	tags := []MetaTag{{Property: ptr("og:description"), Content: ptr("OG Description")}}
	m := Extract(doc, nil, tags, "")
	assert.Equal(t, "OG Description", m.Description)
}

func TestExtract_OGImage(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head>
		<meta property="og:image" content="https://example.com/image.jpg">
	</head><body></body></html>`)
	tags := []MetaTag{{Property: ptr("og:image"), Content: ptr("https://example.com/image.jpg")}}
	m := Extract(doc, nil, tags, "")
	assert.Equal(t, "https://example.com/image.jpg", m.Image)
}

func TestExtract_TwitterCard(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head>
		<meta name="twitter:title" content="Twitter Title">
		<meta name="twitter:description" content="Twitter Desc">
	</head><body></body></html>`)
	tags := []MetaTag{
		{Name: ptr("twitter:title"), Content: ptr("Twitter Title")},
		{Name: ptr("twitter:description"), Content: ptr("Twitter Desc")},
	}
	m := Extract(doc, nil, tags, "")
	// Twitter metadata should be used as fallback
	assert.NotEmpty(t, m.Title)
}

func TestExtract_Author(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head>
		<meta name="author" content="Jane Doe">
	</head><body></body></html>`)
	tags := []MetaTag{{Name: ptr("author"), Content: ptr("Jane Doe")}}
	m := Extract(doc, nil, tags, "")
	assert.Equal(t, "Jane Doe", m.Author)
}

func TestExtract_Published(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head>
		<meta property="article:published_time" content="2024-03-15T10:00:00Z">
	</head><body></body></html>`)
	tags := []MetaTag{{Property: ptr("article:published_time"), Content: ptr("2024-03-15T10:00:00Z")}}
	m := Extract(doc, nil, tags, "")
	assert.Contains(t, m.Published, "2024")
}

func TestExtract_Domain(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head>
		<meta property="og:url" content="https://www.example.com/page">
	</head><body></body></html>`)
	tags := []MetaTag{{Property: ptr("og:url"), Content: ptr("https://www.example.com/page")}}
	m := Extract(doc, nil, tags, "https://www.example.com/page")
	assert.Contains(t, m.Domain, "example.com")
}

func TestExtract_MissingMetadata(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head></head><body></body></html>`)
	m := Extract(doc, nil, nil, "")

	require.NotNil(t, m)
	// All fields should be zero-value, not panic
	assert.Empty(t, m.Description)
	assert.Empty(t, m.Author)
	assert.Empty(t, m.Image)
	assert.Empty(t, m.Published)
	assert.Equal(t, 0, m.WordCount)
}

func TestExtract_BaseURL(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head><title>Test</title></head><body></body></html>`)
	m := Extract(doc, nil, nil, "https://example.org/article")
	assert.Contains(t, m.Domain, "example.org")
}

func TestExtract_SchemaOrgData(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<html><head><title>Schema Test</title></head><body></body></html>`)

	schemaData := map[string]any{
		"@type":       "Article",
		"headline":    "Schema Headline",
		"description": "Schema description",
		"author": map[string]any{
			"@type": "Person",
			"name":  "Schema Author",
		},
	}

	m := Extract(doc, schemaData, nil, "")
	// Schema.org data should influence extraction
	require.NotNil(t, m)
	assert.Equal(t, schemaData, m.SchemaOrgData)
}
