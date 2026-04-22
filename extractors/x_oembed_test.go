package extractors

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildOembedBody marshals an xOembedResponse into a JSON body document.
// Using struct marshaling, never raw JSON string literals (CLAUDE.md gate).
func buildOembedBody(t *testing.T, resp xOembedResponse) *goquery.Document {
	t.Helper()
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	bodyHTML := fmt.Sprintf("<html><body>%s</body></html>", string(b))
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyHTML))
	require.NoError(t, err)
	return doc
}

func parseXOembedDoc(t *testing.T, rawHTML string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	require.NoError(t, err)
	return doc
}

func TestXOEmbedExtractor_CanExtract_ValidOembed(t *testing.T) {
	t.Parallel()

	resp := xOembedResponse{
		HTML:         `<blockquote><p>Hello world</p></blockquote>`,
		AuthorName:   "testuser",
		AuthorURL:    "https://twitter.com/testuser",
		ProviderName: "Twitter",
	}
	doc := buildOembedBody(t, resp)
	ext := NewXOEmbedExtractor(doc, "https://publish.twitter.com/oembed?url=https://x.com/testuser/status/123", nil)
	assert.True(t, ext.CanExtract())
}

func TestXOEmbedExtractor_CanExtract_NotOembed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		html string
	}{
		{
			name: "plain HTML page",
			html: `<html><body><p>Not JSON</p></body></html>`,
		},
		{
			name: "JSON without html field",
			html: fmt.Sprintf(`<html><body>%s</body></html>`, func() string {
				b, _ := json.Marshal(map[string]string{"author_name": "alice"})
				return string(b)
			}()),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseXOembedDoc(t, tc.html)
			ext := NewXOEmbedExtractor(doc, "https://publish.twitter.com/oembed", nil)
			assert.False(t, ext.CanExtract())
		})
	}
}

func TestXOEmbedExtractor_Extract_Metadata(t *testing.T) {
	t.Parallel()

	resp := xOembedResponse{
		HTML:         `<blockquote><p>This is a tweet about Go.</p><a href="https://x.com/gopher/status/1">Jan 1</a></blockquote>`,
		AuthorName:   "The Go Gopher",
		AuthorURL:    "https://twitter.com/gopher",
		ProviderName: "Twitter",
	}
	doc := buildOembedBody(t, resp)
	ext := NewXOEmbedExtractor(doc, "https://publish.twitter.com/oembed?url=https://x.com/gopher/status/1", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "@gopher on X", result.Variables["title"])
	assert.Equal(t, "@gopher", result.Variables["author"])
	assert.Equal(t, "X (Twitter)", result.Variables["site"])
	assert.Contains(t, result.Variables["description"], "tweet about Go")
}

func TestXOEmbedExtractor_Extract_AuthorFallback(t *testing.T) {
	t.Parallel()

	// No author_url — falls back to author_name.
	resp := xOembedResponse{
		HTML:       `<blockquote><p>Hello</p></blockquote>`,
		AuthorName: "Jane Doe",
	}
	doc := buildOembedBody(t, resp)
	ext := NewXOEmbedExtractor(doc, "https://publish.x.com/oembed?url=https://x.com/janedoe/status/2", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Equal(t, "@Jane Doe", result.Variables["author"])
}

func TestXOEmbedExtractor_Extract_ContentHTML(t *testing.T) {
	t.Parallel()

	resp := xOembedResponse{
		HTML:       `<blockquote class="twitter-tweet"><p lang="en">Great news about <a href="#">#golang</a>!</p><a href="https://x.com/user/status/1">Date</a></blockquote>`,
		AuthorName: "gopher",
		AuthorURL:  "https://twitter.com/gopher",
	}
	doc := buildOembedBody(t, resp)
	ext := NewXOEmbedExtractor(doc, "https://publish.twitter.com/oembed", nil)
	require.True(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.ContentHTML, "Great news")
	assert.Contains(t, result.ContentHTML, "extractor-twitter")
}

func TestXOEmbedExtractor_Extract_EmptyWhenNoData(t *testing.T) {
	t.Parallel()

	doc := parseXOembedDoc(t, `<html><body><p>plain page</p></body></html>`)
	ext := NewXOEmbedExtractor(doc, "https://publish.twitter.com/oembed", nil)
	require.False(t, ext.CanExtract())

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Empty(t, result.Content)
	assert.Empty(t, result.ContentHTML)
}

func TestXOEmbedExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := parseXOembedDoc(t, `<html><body></body></html>`)
	ext := NewXOEmbedExtractor(doc, "", nil)
	assert.Equal(t, "XOEmbedExtractor", ext.Name())
}
