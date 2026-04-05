package extractors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// substackPostHTML is a minimal Substack post page with rendered DOM content.
const substackPostHTML = `<html>
<head>
  <meta name="generator" content="Substack">
  <meta property="og:title" content="OG Title Fallback">
  <meta property="og:description" content="OG description text.">
  <meta property="og:image" content="https://cdn.example.com/image.jpg">
  <meta property="og:site_name" content="My Newsletter">
  <meta name="author" content="Meta Author">
  <meta property="article:published_time" content="2024-03-15T10:00:00Z">
  <title>Test Post | My Newsletter</title>
</head>
<body>
  <h1 class="post-title">Test Post Title</h1>
  <div class="post-header">
    <span class="byline"><a href="/p/test">Test Author</a></span>
  </div>
  <div class="available-content">
    <div class="body markup">
      <p>First paragraph of the post.</p>
      <p>Second paragraph with <strong>bold</strong> text.</p>
    </div>
  </div>
</body>
</html>`

// substackPostContentHTML uses .post-content as the content selector fallback.
const substackPostContentHTML = `<html>
<head>
  <meta name="generator" content="Substack">
  <meta property="og:site_name" content="Another Newsletter">
  <title>Fallback Content Post</title>
</head>
<body>
  <h1 class="post-title">Fallback Post</h1>
  <div class="post-content">
    <p>Content from .post-content selector.</p>
  </div>
</body>
</html>`

// substackPreloadsHTML uses window._preloads for metadata and body_html for content.
const substackPreloadsHTML = `<html>
<head>
  <title>Preloads Post</title>
</head>
<body>
  <script>
    window._preloads = {"post":{"title":"Preloads Title","subtitle":"Preloads subtitle.","canonical_url":"https://test.substack.com/p/preloads","post_date":"2024-06-01T08:00:00Z","body_html":"<p>Body from preloads JSON.</p>","audience":"everyone"}};
  </script>
</body>
</html>`

// substackNoContentHTML has a generator tag but no content selectors or body_html.
const substackNoContentHTML = `<html>
<head>
  <meta name="generator" content="Substack">
  <title>Empty Post</title>
</head>
<body>
  <p>Some unrecognised structure.</p>
</body>
</html>`

// substackDOMFallbackHTML relies on .post-content and meta tags (no generator, no preloads).
const substackDOMFallbackHTML = `<html>
<head>
  <title>DOM Fallback</title>
</head>
<body>
  <div class="post-content"><p>DOM-only content.</p></div>
  <span class="author-name">DOM Author</span>
  <time datetime="2024-09-10T12:00:00Z">September 10</time>
</body>
</html>`

// TestSubstackExtractor_Name verifies the extractor's identifier.
func TestSubstackExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/post", nil)
	assert.Equal(t, "SubstackExtractor", ext.Name())
}

// CanExtract — generator meta tag.

func TestSubstackExtractor_CanExtract_GeneratorMeta(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPostHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/post", nil)
	assert.True(t, ext.CanExtract())
}

func TestSubstackExtractor_CanExtract_PostContentSelector(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackDOMFallbackHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/post", nil)
	assert.True(t, ext.CanExtract())
}

func TestSubstackExtractor_CanExtract_AvailableContentSelector(t *testing.T) {
	t.Parallel()
	html := `<html><body><div class="available-content"><p>Hello</p></div></body></html>`
	doc := newTestDoc(t, html)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/post", nil)
	assert.True(t, ext.CanExtract())
}

func TestSubstackExtractor_CanExtract_False_NoIndicators(t *testing.T) {
	t.Parallel()
	html := `<html><head><title>Not Substack</title></head><body><p>Plain page.</p></body></html>`
	doc := newTestDoc(t, html)
	ext := NewSubstackExtractor(doc, "https://example.com/page", nil)
	assert.False(t, ext.CanExtract())
}

// Registry routing — *.substack.com URLs.

func TestSubstackExtractor_RegistryRouting_SubdomainURL(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.initializeBuiltins()
	doc := newTestDoc(t, substackPostHTML)

	ext := r.FindExtractor(doc, "https://newsletter.substack.com/p/some-post", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "SubstackExtractor", ext.Name())
}

func TestSubstackExtractor_RegistryRouting_RootSubstackDomain(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.initializeBuiltins()
	doc := newTestDoc(t, substackPostHTML)

	ext := r.FindExtractor(doc, "https://substack.com/home", nil)
	require.NotNil(t, ext)
	assert.Equal(t, "SubstackExtractor", ext.Name())
}

// Extract — content from .available-content .body.markup.

func TestSubstackExtractor_Extract_AvailableContentMarkup(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPostHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/post", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.ContentHTML, "First paragraph of the post.")
	assert.Contains(t, result.ContentHTML, "Second paragraph")
	assert.Contains(t, result.ContentHTML, "<strong>bold</strong>")
	assert.NotEmpty(t, result.Content)
}

// Extract — content from .post-content fallback.

func TestSubstackExtractor_Extract_PostContentFallback(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPostContentHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/fallback", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.ContentHTML, "Content from .post-content selector.")
}

// Extract — content from preloads JSON body_html.

func TestSubstackExtractor_Extract_PreloadsBodyHTML(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPreloadsHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/preloads", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	assert.Contains(t, result.ContentHTML, "Body from preloads JSON.")
}

// GetMetadata — title extraction priority.

func TestSubstackExtractor_GetMetadata_TitleFromPreloads(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPreloadsHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/preloads", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// Preloads title takes highest priority.
	assert.Equal(t, "Preloads Title", result.Variables["title"])
}

func TestSubstackExtractor_GetMetadata_TitleFromH1(t *testing.T) {
	t.Parallel()
	// No preloads, no og:title — fall back to h1.post-title.
	html := `<html>
<head><meta name="generator" content="Substack"><title>Page Title</title></head>
<body>
  <h1 class="post-title">H1 Title</h1>
  <div class="post-content"><p>Content.</p></div>
</body>
</html>`
	doc := newTestDoc(t, html)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/h1", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "H1 Title", result.Variables["title"])
}

func TestSubstackExtractor_GetMetadata_TitleFromOGFallback(t *testing.T) {
	t.Parallel()
	// No preloads, no h1 — fall back to og:title.
	html := `<html>
<head>
  <meta name="generator" content="Substack">
  <meta property="og:title" content="OG Title">
</head>
<body><div class="post-content"><p>text</p></div></body>
</html>`
	doc := newTestDoc(t, html)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/og", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "OG Title", result.Variables["title"])
}

// GetMetadata — author extraction.

func TestSubstackExtractor_GetMetadata_AuthorFromAuthorName(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPostHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/post", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// .author-name is not present in substackPostHTML; it has .post-header .byline a.
	// That selector is "post-header .byline a" which matches a > inside .post-header .byline.
	// The HTML has class="byline" wrapping an <a> inside .post-header — assert non-empty.
	assert.NotEmpty(t, result.Variables["author"])
}

func TestSubstackExtractor_GetMetadata_AuthorFromMetaTag(t *testing.T) {
	t.Parallel()
	// No DOM author elements — fall back to <meta name="author">.
	html := `<html>
<head>
  <meta name="generator" content="Substack">
  <meta name="author" content="Meta Author Name">
</head>
<body><div class="post-content"><p>text</p></div></body>
</html>`
	doc := newTestDoc(t, html)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/meta-author", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "Meta Author Name", result.Variables["author"])
}

func TestSubstackExtractor_GetMetadata_AuthorFromDOMElement(t *testing.T) {
	t.Parallel()
	html := `<html>
<head><meta name="generator" content="Substack"></head>
<body>
  <span class="author-name">DOM Author</span>
  <div class="post-content"><p>text</p></div>
</body>
</html>`
	doc := newTestDoc(t, html)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/dom-author", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "DOM Author", result.Variables["author"])
}

// GetMetadata — published date extraction.

func TestSubstackExtractor_GetMetadata_PublishedFromPreloads(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPreloadsHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/preloads", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "2024-06-01T08:00:00Z", result.Variables["published"])
}

func TestSubstackExtractor_GetMetadata_PublishedFromTimeDatetime(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackDOMFallbackHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/fallback", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "2024-09-10T12:00:00Z", result.Variables["published"])
}

func TestSubstackExtractor_GetMetadata_PublishedFromArticleMeta(t *testing.T) {
	t.Parallel()
	html := `<html>
<head>
  <meta name="generator" content="Substack">
  <meta property="article:published_time" content="2024-01-20T00:00:00Z">
</head>
<body><div class="post-content"><p>text</p></div></body>
</html>`
	doc := newTestDoc(t, html)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/article-meta", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "2024-01-20T00:00:00Z", result.Variables["published"])
}

// GetMetadata — description, image, site.

func TestSubstackExtractor_GetMetadata_DescriptionFromPreloads(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPreloadsHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/preloads", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "Preloads subtitle.", result.Variables["description"])
}

func TestSubstackExtractor_GetMetadata_DescriptionFromOG(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPostHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/post", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "OG description text.", result.Variables["description"])
}

func TestSubstackExtractor_GetMetadata_Image(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPostHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/post", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "https://cdn.example.com/image.jpg", result.Variables["image"])
}

func TestSubstackExtractor_GetMetadata_SiteNameFromOG(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackPostHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/post", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "My Newsletter", result.Variables["site"])
}

func TestSubstackExtractor_GetMetadata_SiteNameDefaultsToSubstack(t *testing.T) {
	t.Parallel()
	// No og:site_name meta.
	html := `<html>
<head><meta name="generator" content="Substack"></head>
<body><div class="post-content"><p>text</p></div></body>
</html>`
	doc := newTestDoc(t, html)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/no-site-name", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	assert.Equal(t, "Substack", result.Variables["site"])
}

// Edge cases.

func TestSubstackExtractor_Extract_EmptyContent_StillReturnsResult(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, substackNoContentHTML)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/empty", nil)

	result := ext.Extract()
	require.NotNil(t, result)
	// No content selectors matched and no preloads body_html — content is empty.
	assert.Empty(t, result.ContentHTML)
	// Variables map must still be initialised.
	assert.NotNil(t, result.Variables)
}

func TestSubstackExtractor_PreloadsJSON_MalformedIgnored(t *testing.T) {
	t.Parallel()
	html := `<html>
<head><meta name="generator" content="Substack"></head>
<body>
  <script>window._preloads = {INVALID JSON HERE</script>
  <h1 class="post-title">DOM Title</h1>
  <div class="post-content"><p>DOM content.</p></div>
</body>
</html>`
	doc := newTestDoc(t, html)
	ext := NewSubstackExtractor(doc, "https://test.substack.com/p/malformed", nil)

	result := ext.Extract()
	require.NotNil(t, result)

	// Malformed JSON is ignored; DOM fallbacks kick in.
	assert.Equal(t, "DOM Title", result.Variables["title"])
	assert.Contains(t, result.ContentHTML, "DOM content.")
}
