package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// C2 Wiki DOM fixtures.
const c2WikiPageHTML = `<html>
<body>
  <p>This wiki page discusses <a href="/cgi/wiki?WikiWikiWeb">WikiWikiWeb</a>.</p>
  <p>It contains collaborative notes and patterns.</p>
  <hr>
  <p>Edit this page if you have something to add.</p>
</body>
</html>`

const c2WikiMinimalHTML = `<html><body><p>Welcome.</p></body></html>`

func parseC2WikiDoc(t *testing.T, rawHTML string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

func TestC2WikiExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		wantCan bool
	}{
		{"cgi wiki URL with page", "https://c2.com/cgi/wiki?WelcomeVisitors", true},
		{"cgi wiki URL no param (default page)", "https://c2.com/cgi/wiki", true},
		{"wiki/ path URL", "https://c2.com/wiki/WelcomeVisitors", true},
		{"non-wiki c2 URL", "https://c2.com/about", false},
		{"unrelated domain", "https://example.com/cgi/wiki?Foo", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseC2WikiDoc(t, c2WikiMinimalHTML)
			ext := NewC2WikiExtractor(doc, tc.url, nil)
			assert.Equal(t, tc.wantCan, ext.CanExtract())
		})
	}
}

func TestC2WikiExtractor_Extract_Metadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		url       string
		wantTitle string
	}{
		{
			name:      "CamelCase page name split into words",
			url:       "https://c2.com/cgi/wiki?WelcomeVisitors",
			wantTitle: "Welcome Visitors",
		},
		{
			name:      "single-word page name unchanged",
			url:       "https://c2.com/cgi/wiki?Refactoring",
			wantTitle: "Refactoring",
		},
		{
			name:      "default page when no param",
			url:       "https://c2.com/cgi/wiki",
			wantTitle: "Welcome Visitors",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseC2WikiDoc(t, c2WikiPageHTML)
			ext := NewC2WikiExtractor(doc, tc.url, nil)
			require.True(t, ext.CanExtract())

			result := ext.Extract()
			require.NotNil(t, result)

			assert.Equal(t, tc.wantTitle, result.Variables["title"])
			assert.Equal(t, "C2 Wiki", result.Variables["site"])
		})
	}
}

func TestC2WikiExtractor_Extract_Content(t *testing.T) {
	t.Parallel()

	doc := parseC2WikiDoc(t, c2WikiPageHTML)
	ext := NewC2WikiExtractor(doc, "https://c2.com/cgi/wiki?WikiWikiWeb", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.ContentHTML, "collaborative notes")
	assert.Contains(t, result.ContentHTML, "Edit this page")
}

func TestC2WikiExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := parseC2WikiDoc(t, c2WikiMinimalHTML)
	ext := NewC2WikiExtractor(doc, "https://c2.com/cgi/wiki?Foo", nil)
	assert.Equal(t, "C2WikiExtractor", ext.Name())
}
