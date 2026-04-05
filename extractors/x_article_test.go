package extractors

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// xArticleBaseHTML is a minimal X article page with a Draft.js-style body,
// a structured author block, and a title element.
const xArticleBaseHTML = `<html>
<head>
  <title>Article title on X</title>
  <meta property="og:title" content="Gary Blankenship on X: This is the article title">
  <span itemprop="author">
    <meta itemprop="name" content="Gary Blankenship">
    <meta itemprop="additionalName" content="gblankenship">
  </span>
</head>
<body>
  <div data-testid="twitterArticleRichTextView">
    <div data-testid="twitter-article-title">How I Built a Go Parser</div>
    <div class="longform-unstyled" data-offset-key="abc-0-0">
      First paragraph of the article.
    </div>
    <div class="longform-unstyled" data-offset-key="abc-1-0">
      Second paragraph with <strong>bold text</strong>.
    </div>
  </div>
</body>
</html>`

const xArticleNoContainerHTML = `<html>
<head><title>X</title></head>
<body><p>Just a tweet, not an article.</p></body>
</html>`

func TestXArticleExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewXArticleExtractor(doc, "https://x.com/user/article/123456789", nil)
	assert.Equal(t, "XArticleExtractor", ext.Name())
}

func TestXArticleExtractor_CanExtract_WithContainer(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, xArticleBaseHTML)
	ext := NewXArticleExtractor(doc, "https://x.com/gblankenship/article/123456789", nil)
	assert.True(t, ext.CanExtract())
}

func TestXArticleExtractor_CanExtract_NoContainer(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, xArticleNoContainerHTML)
	ext := NewXArticleExtractor(doc, "https://x.com/user/status/987654321", nil)
	assert.False(t, ext.CanExtract())
}

func TestXArticleExtractor_CanExtract_URLVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		url         string
		html        string
		wantExtract bool
	}{
		{
			name:        "article URL with container",
			url:         "https://x.com/gblankenship/article/123456789",
			html:        xArticleBaseHTML,
			wantExtract: true,
		},
		{
			name:        "status URL without container",
			url:         "https://x.com/user/status/111",
			html:        xArticleNoContainerHTML,
			wantExtract: false,
		},
		{
			name: "article URL but container present",
			url:  "https://x.com/user/article/999",
			html: `<html><body>
			  <div data-testid="twitterArticleRichTextView"><p>Content</p></div>
			</body></html>`,
			wantExtract: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, tt.html)
			ext := NewXArticleExtractor(doc, tt.url, nil)
			assert.Equal(t, tt.wantExtract, ext.CanExtract())
		})
	}
}

func TestXArticleExtractor_Extract_ReturnsContent(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, xArticleBaseHTML)
	ext := NewXArticleExtractor(doc, "https://x.com/gblankenship/article/123456789", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, `class="x-article"`)
	assert.Contains(t, result.ContentHTML, "First paragraph of the article.")
	assert.Contains(t, result.ContentHTML, "Second paragraph")
}

func TestXArticleExtractor_Extract_ConvertsDraftParagraphs(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, xArticleBaseHTML)
	ext := NewXArticleExtractor(doc, "https://x.com/gblankenship/article/123456789", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	// Draft.js divs should become <p> elements.
	assert.Contains(t, result.ContentHTML, "<p>")
	// data-offset-key attributes should be stripped.
	assert.NotContains(t, result.ContentHTML, "data-offset-key")
}

func TestXArticleExtractor_Extract_NoContainer(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, xArticleNoContainerHTML)
	ext := NewXArticleExtractor(doc, "https://x.com/user/status/111", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Empty(t, result.ContentHTML)
}

func TestXArticleExtractor_Extract_ArticleIDInExtractedContent(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, xArticleBaseHTML)
	ext := NewXArticleExtractor(doc, "https://x.com/gblankenship/article/123456789", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "123456789", result.ExtractedContent["articleId"])
}

func TestXArticleExtractor_Extract_SiteVariable(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, xArticleBaseHTML)
	ext := NewXArticleExtractor(doc, "https://x.com/gblankenship/article/123456789", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "X (Twitter)", result.Variables["site"])
}

func TestXArticleExtractor_GetMetadata_TitleFromTestID(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, xArticleBaseHTML)
	ext := NewXArticleExtractor(doc, "https://x.com/gblankenship/article/123456789", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "How I Built a Go Parser", result.Variables["title"])
}

func TestXArticleExtractor_GetMetadata_TitleFallbackWhenNoTitleElement(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div data-testid="twitterArticleRichTextView">
	    <div class="longform-unstyled">Some content</div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewXArticleExtractor(doc, "https://x.com/user/article/111", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Untitled X Article", result.Variables["title"])
}

func TestXArticleExtractor_GetMetadata_AuthorFromItemprop(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, xArticleBaseHTML)
	ext := NewXArticleExtractor(doc, "https://x.com/gblankenship/article/123456789", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	// Author should combine name and handle.
	assert.Equal(t, "Gary Blankenship (@gblankenship)", result.Variables["author"])
}

func TestXArticleExtractor_GetMetadata_AuthorFromURLWhenNoItemprop(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div data-testid="twitterArticleRichTextView">
	    <p>Content</p>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewXArticleExtractor(doc, "https://x.com/testuser/article/999", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "@testuser", result.Variables["author"])
}

func TestXArticleExtractor_GetMetadata_AuthorFromOgTitleWhenNoURL(t *testing.T) {
	t.Parallel()

	html := `<html>
	<head>
	  <meta property="og:title" content="Jane Doe on X: Some article">
	</head>
	<body>
	  <div data-testid="twitterArticleRichTextView"><p>Content</p></div>
	</body>
	</html>`

	doc := newTestDoc(t, html)
	// URL doesn't match the author pattern.
	ext := NewXArticleExtractor(doc, "https://x.com/status/12345", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Jane Doe", result.Variables["author"])
}

func TestXArticleExtractor_GetMetadata_AuthorUnknownFallback(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div data-testid="twitterArticleRichTextView"><p>Content</p></div>
	</body></html>`

	doc := newTestDoc(t, html)
	// URL doesn't contain a username segment matching the pattern.
	ext := NewXArticleExtractor(doc, "https://x.com/home", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Unknown", result.Variables["author"])
}

func TestXArticleExtractor_Extract_BoldSpansConverted(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div data-testid="twitterArticleRichTextView">
	    <span style="font-weight: bold">Important text</span>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewXArticleExtractor(doc, "https://x.com/user/article/111", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "<strong>Important text</strong>")
}

func TestXArticleExtractor_Extract_HeadersPreserved(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div data-testid="twitterArticleRichTextView">
	    <h2><span>Section heading</span></h2>
	    <div class="longform-unstyled">Paragraph text</div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewXArticleExtractor(doc, "https://x.com/user/article/222", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "<h2>Section heading</h2>")
}

func TestXArticleExtractor_Extract_EmbeddedTweetConverted(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div data-testid="twitterArticleRichTextView">
	    <div data-testid="simpleTweet">
	      <div data-testid="User-Name">
	        <a>Jane</a><a>@jane</a>
	      </div>
	      <div data-testid="tweetText">Hello from the tweet!</div>
	    </div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewXArticleExtractor(doc, "https://x.com/user/article/333", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "<blockquote")
	assert.Contains(t, result.ContentHTML, "Hello from the tweet!")
	assert.Contains(t, result.ContentHTML, "<cite>")
}

func TestXArticleExtractor_Extract_CodeBlockConverted(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div data-testid="twitterArticleRichTextView">
	    <div data-testid="markdown-code-block">
	      <pre><code class="language-go">fmt.Println("hello")</code></pre>
	    </div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewXArticleExtractor(doc, "https://x.com/user/article/444", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "<pre><code")
	assert.Contains(t, result.ContentHTML, `data-lang="go"`)
	assert.Contains(t, result.ContentHTML, "fmt.Println")
}

func TestXArticleExtractor_GetArticleID_FromURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"standard article", "https://x.com/user/article/123456789", "123456789"},
		{"article with trailing slash", "https://x.com/user/article/999/", "999"},
		{"no article segment", "https://x.com/user/status/111", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, "<html><body></body></html>")
			ext := NewXArticleExtractor(doc, tt.url, nil)
			assert.Equal(t, tt.want, ext.getArticleID())
		})
	}
}

func TestXArticleExtractor_UpgradeImageQuality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "replaces existing name param",
			src:  "https://pbs.twimg.com/media/abc.jpg?format=jpg&name=small",
			want: "https://pbs.twimg.com/media/abc.jpg?format=jpg&name=large",
		},
		{
			name: "appends to existing query string",
			src:  "https://pbs.twimg.com/media/abc.jpg?format=jpg",
			want: "https://pbs.twimg.com/media/abc.jpg?format=jpg&name=large",
		},
		{
			name: "no query string",
			src:  "https://pbs.twimg.com/media/abc.jpg",
			want: "https://pbs.twimg.com/media/abc.jpg?name=large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := upgradeXImageQuality(tt.src)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestXArticleExtractor_CreateDescription_TruncatesLong(t *testing.T) {
	t.Parallel()

	// Build article text longer than 140 chars.
	longContent := strings.Repeat("word ", 50) // 250 chars
	html := `<html><body>
	  <div data-testid="twitterArticleRichTextView">` + longContent + `</div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewXArticleExtractor(doc, "https://x.com/user/article/111", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.LessOrEqual(t, len(result.Variables["description"]), 143) // 140 + "..."
	assert.Contains(t, result.Variables["description"], "...")
}

func TestXArticleExtractor_CreateDescription_ShortContent(t *testing.T) {
	t.Parallel()

	html := `<html><body>
	  <div data-testid="twitterArticleRichTextView">
	    <div class="longform-unstyled">Short.</div>
	  </div>
	</body></html>`

	doc := newTestDoc(t, html)
	ext := NewXArticleExtractor(doc, "https://x.com/user/article/222", nil)

	result := ext.Extract()

	require.NotNil(t, result)
	assert.NotContains(t, result.Variables["description"], "...")
}
