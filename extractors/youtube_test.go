package extractors

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// youtubeVideoObjectSchema returns a minimal Schema.org VideoObject map.
func youtubeVideoObjectSchema(title, author, description, uploadDate string) map[string]any {
	return map[string]any{
		"@type":       "VideoObject",
		"name":        title,
		"author":      author,
		"description": description,
		"uploadDate":  uploadDate,
		"thumbnailUrl": []any{
			fmt.Sprintf("https://img.youtube.com/vi/dQw4w9WgXcQ/%s.jpg", "maxresdefault"),
		},
	}
}

// ---------------------------------------------------------------------------
// CanExtract
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_CanExtract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"youtube.com watch URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"youtube.com short path", "https://youtube.com/shorts/abc123", true},
		{"youtu.be short link", "https://youtu.be/dQw4w9WgXcQ", true},
		{"unrelated URL still true", "https://example.com/video", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, "<html><body></body></html>")
			ext := NewYouTubeExtractor(doc, tc.url, nil)
			assert.Equal(t, tc.want, ext.CanExtract())
		})
	}
}

func TestYouTubeExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, "", nil)
	assert.Equal(t, "YouTubeExtractor", ext.Name())
}

// ---------------------------------------------------------------------------
// getVideoID
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_GetVideoID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
		want string
	}{
		{"youtube.com v param", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"youtube.com with extra params", "https://youtube.com/watch?v=abc123&t=30s", "abc123"},
		{"youtu.be short link", "https://youtu.be/xyzVID", "xyzVID"},
		{"youtu.be with query string", "https://youtu.be/xyzVID?t=10", "xyzVID"},
		{"unknown host", "https://example.com/watch?v=ignored", ""},
		{"invalid url", "://bad url", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, "<html><body></body></html>")
			ext := NewYouTubeExtractor(doc, tc.url, nil)
			assert.Equal(t, tc.want, ext.getVideoID())
		})
	}
}

// ---------------------------------------------------------------------------
// GetMetadata — title, description, channel via schema.org
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_Extract_FromSchemaOrg(t *testing.T) {
	t.Parallel()

	schema := youtubeVideoObjectSchema(
		"Never Gonna Give You Up",
		"Rick Astley",
		"Official music video",
		"1987-11-16",
	)
	html := `<html><head><title>Never Gonna Give You Up - YouTube</title></head><body></body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Never Gonna Give You Up", result.Variables["title"])
	assert.Equal(t, "Rick Astley", result.Variables["author"])
	assert.Equal(t, "YouTube", result.Variables["site"])
	assert.Equal(t, "1987-11-16", result.Variables["published"])
	assert.Equal(t, "Official music video", result.Variables["description"])
	assert.Contains(t, result.Variables["image"], "maxresdefault")
	assert.Contains(t, result.ContentHTML, "dQw4w9WgXcQ")
	assert.Contains(t, result.ContentHTML, "<iframe")
}

func TestYouTubeExtractor_Extract_SchemaOrgArray(t *testing.T) {
	t.Parallel()

	schema := []any{
		map[string]any{"@type": "BreadcrumbList"},
		youtubeVideoObjectSchema("Test Video", "Test Channel", "A description", "2024-01-01"),
	}
	doc := newTestDoc(t, "<html><head><title>Test Video - YouTube</title></head><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=testID1", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Test Video", result.Variables["title"])
	assert.Equal(t, "Test Channel", result.Variables["author"])
}

func TestYouTubeExtractor_Extract_PrefersDescriptiveLDJSONVideoObject(t *testing.T) {
	t.Parallel()

	html := `<html><head>
<title>Fallback Title - YouTube</title>
<script type="application/ld+json">[
  {
    "@type": "VideoObject",
    "@id": "https://www.youtube.com/watch?v=vid123",
    "name": "Pinned comment title",
    "comment": {"text": "first comment"}
  },
  {
    "@type": "VideoObject",
    "@id": "https://www.youtube.com/watch?v=vid123",
    "name": "Actual Video Title",
    "author": "Actual Channel",
    "description": "Actual video description",
    "uploadDate": "2026-01-02",
    "thumbnailUrl": ["https://img.youtube.com/vi/vid123/maxresdefault.jpg"]
  }
]</script>
</head><body></body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=vid123", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Actual Video Title", result.Variables["title"])
	assert.Equal(t, "Actual Channel", result.Variables["author"])
	assert.Equal(t, "Actual video description", result.Variables["description"])
	assert.Equal(t, "2026-01-02", result.Variables["published"])
}

func TestYouTubeExtractor_Extract_RejectsStaleOpenGraphFallback(t *testing.T) {
	t.Parallel()

	html := `<html><head>
<title>Current Title - YouTube</title>
<meta property="og:url" content="https://www.youtube.com/watch?v=old123">
<meta property="og:title" content="Stale OG Title">
<meta property="og:description" content="Stale OG Description">
</head><body></body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=current123", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Current Title", result.Variables["title"])
	assert.NotEqual(t, "Stale OG Description", result.Variables["description"])
}

func TestYouTubeExtractor_Extract_TitleFallbackFromDocument(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>My Fallback Title - YouTube</title></head><body></body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=fallback1", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "My Fallback Title", result.Variables["title"])
}

func TestYouTubeExtractor_Extract_DescriptionFallbackFromDOM(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Video - YouTube</title></head>
<body><div id="description">Description from the DOM element</div></body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=domDesc1", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.Variables["description"], "Description from the DOM element")
}

func TestYouTubeExtractor_Extract_PlayerResponseMetadataFallback(t *testing.T) {
	t.Parallel()

	script := `var ytInitialPlayerResponse = {"videoDetails":{"title":"Inline title","shortDescription":"Inline description"},"microformat":{"playerMicroformatRenderer":{"publishDate":"2026-08-20"}}};`
	doc := newTestDoc(t, fmt.Sprintf("<html><head><script>%s</script></head><body></body></html>", script))
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=inline123", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Inline title", result.Variables["title"])
	assert.Equal(t, "Inline description", result.Variables["description"])
	assert.Equal(t, "2026-08-20", result.Variables["published"])
}

func TestYouTubeExtractor_Extract_PlayerResponseDescriptionFallbacks(t *testing.T) {
	t.Parallel()

	emptyDOMScript := `var ytInitialPlayerResponse = {"videoDetails":{"shortDescription":"Inline despite empty DOM"}};`
	emptyDOM := newTestDoc(t, fmt.Sprintf(`<html><head><script>%s</script></head><body><div id="description"> </div></body></html>`, emptyDOMScript))
	assert.Equal(t, "Inline despite empty DOM", NewYouTubeExtractor(emptyDOM, "https://www.youtube.com/watch?v=emptydom", nil).getDescription(nil))

	microformatScript := `var ytInitialPlayerResponse = {"microformat":{"playerMicroformatRenderer":{"description":{"simpleText":"Microformat description"}}}};`
	microformat := newTestDoc(t, fmt.Sprintf(`<html><head><script>%s</script></head><body></body></html>`, microformatScript))
	assert.Equal(t, "Microformat description", NewYouTubeExtractor(microformat, "https://www.youtube.com/watch?v=microformat", nil).getDescription(nil))
}

func TestYouTubeExtractor_PlayerResponseFallbackPreservesSchemaAndDOM(t *testing.T) {
	t.Parallel()

	schema := youtubeVideoObjectSchema("Schema title", "Channel", "Schema description", "2020-01-02")
	script := `var ytInitialPlayerResponse = {"videoDetails":{"title":"Inline title","shortDescription":"Inline description"},"microformat":{"playerMicroformatRenderer":{"publishDate":"2026-08-20"}}};`
	html := fmt.Sprintf(`<html><head><title>DOM title - YouTube</title><script>%s</script></head><body><div id="description">DOM description</div></body></html>`, script)
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=preserve123", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Schema title", result.Variables["title"])
	assert.Equal(t, "Schema description", result.Variables["description"])
	assert.Equal(t, "2020-01-02", result.Variables["published"])
}

func TestYouTubeExtractor_PlayerResponseJSONHandlesQuotedBraces(t *testing.T) {
	t.Parallel()

	script := `var ytInitialPlayerResponse = {"videoDetails":{"title":"A { brace } and a \"quote\"","shortDescription":"Keeps a } brace"}};`
	doc := newTestDoc(t, fmt.Sprintf("<html><head><script>%s</script></head><body></body></html>", script))
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=quoted123", nil)

	assert.Equal(t, `A { brace } and a "quote"`, ext.getTitle(nil))
	assert.Equal(t, "Keeps a } brace", ext.getDescription(nil))
}

func TestYouTubeExtractor_ContentHTML_TranscriptLinks(t *testing.T) {
	t.Parallel()

	script := `var ytInitialPlayerResponse = {"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"baseUrl":"https://www.youtube.com/api/timedtext?v=caption123&lang=en","name":{"simpleText":"English & French"},"languageCode":"en"},{"baseUrl":"https://www.youtube.com/api/timedtext?v=caption123&lang=es","name":{"simpleText":"Spanish"},"kind":"asr"},{"baseUrl":"javascript:alert(1)","name":{"simpleText":"Unsafe scheme"}},{"baseUrl":"https://www.youtube.com/watch?v=caption123","name":{"simpleText":"Unsafe path"}}]}}};`
	doc := newTestDoc(t, fmt.Sprintf("<html><head><script>%s</script></head><body></body></html>", script))
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=caption123", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, `href="https://www.youtube.com/api/timedtext?v=caption123&amp;lang=en"`)
	assert.Contains(t, result.ContentHTML, "Transcript (English &amp; French)")
	assert.Contains(t, result.ContentHTML, "Transcript (Spanish, auto-generated)")
	assert.NotContains(t, result.ContentHTML, "Unsafe scheme")
	assert.NotContains(t, result.ContentHTML, "Unsafe path")
}

// ---------------------------------------------------------------------------
// Channel name resolution — DOM selectors
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_ChannelName_FromYtdOwnerRenderer(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Video - YouTube</title></head>
<body>
  <ytd-video-owner-renderer>
    <div id="channel-name"><a href="/@RickAstleyYT">Rick Astley</a></div>
  </ytd-video-owner-renderer>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=channelDOM1", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Rick Astley", result.Variables["author"])
}

func TestYouTubeExtractor_ChannelName_FromOwnerNameFallback(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Video - YouTube</title></head>
<body>
  <div id="owner-name"><a href="/@SomeChannel">Some Channel</a></div>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=channelDOM2", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Some Channel", result.Variables["author"])
}

// ---------------------------------------------------------------------------
// Channel name from microdata — itemprop="author"
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_ChannelName_FromMicrodataMetaTag(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Video - YouTube</title></head>
<body>
  <span itemprop="author">
    <meta itemprop="name" content="Microdata Author">
  </span>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=microdata1", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Microdata Author", result.Variables["author"])
}

func TestYouTubeExtractor_ChannelName_FromMicrodataLinkTag(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Video - YouTube</title></head>
<body>
  <span itemprop="author">
    <link itemprop="name" content="Link Itemprop Author">
  </span>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=microdata2", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Link Itemprop Author", result.Variables["author"])
}

func TestYouTubeExtractor_ChannelName_FromMicrodataTextContent(t *testing.T) {
	t.Parallel()

	html := `<html><head><title>Video - YouTube</title></head>
<body>
  <span itemprop="author">
    <span itemprop="name">Text Itemprop Author</span>
  </span>
</body></html>`
	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=microdata3", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Text Itemprop Author", result.Variables["author"])
}

// ---------------------------------------------------------------------------
// Channel name from ytInitialPlayerResponse
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_ChannelName_FromPlayerResponse_VideoDetailsAuthor(t *testing.T) {
	t.Parallel()

	script := `var ytInitialPlayerResponse = {"videoDetails":{"author":"Player Response Author","videoId":"abc123"}};`
	html := fmt.Sprintf(`<html><head><title>Video - YouTube</title></head>
<body><script>%s</script></body></html>`, script)

	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=abc123", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Player Response Author", result.Variables["author"])
}

func TestYouTubeExtractor_ChannelName_FromPlayerResponse_OwnerChannelName(t *testing.T) {
	t.Parallel()

	script := `var ytInitialPlayerResponse = {"videoDetails":{"ownerChannelName":"Owner Channel Name","videoId":"xyz789"}};`
	html := fmt.Sprintf(`<html><head><title>Video - YouTube</title></head>
<body><script>%s</script></body></html>`, script)

	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=xyz789", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Owner Channel Name", result.Variables["author"])
}

func TestYouTubeExtractor_ChannelName_FromPlayerResponse_Microformat(t *testing.T) {
	t.Parallel()

	script := `var ytInitialPlayerResponse = {"microformat":{"playerMicroformatRenderer":{"ownerChannelName":"Microformat Channel"}}};`
	html := fmt.Sprintf(`<html><head><title>Video - YouTube</title></head>
<body><script>%s</script></body></html>`, script)

	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=micro1", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Microformat Channel", result.Variables["author"])
}

// ---------------------------------------------------------------------------
// Author fallback priority: DOM > playerResponse > schema.org
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_ChannelName_DOMWinsOverPlayerResponse(t *testing.T) {
	t.Parallel()

	script := `var ytInitialPlayerResponse = {"videoDetails":{"author":"Player Response Author"}};`
	html := fmt.Sprintf(`<html><head><title>Video - YouTube</title></head>
<body>
  <script>%s</script>
  <ytd-video-owner-renderer>
    <div id="channel-name"><a href="/@DOMChannel">DOM Channel</a></div>
  </ytd-video-owner-renderer>
</body></html>`, script)

	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=priority1", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "DOM Channel", result.Variables["author"])
}

func TestYouTubeExtractor_ChannelName_PlayerResponseWinsOverSchemaOrg(t *testing.T) {
	t.Parallel()

	schema := youtubeVideoObjectSchema("Title", "Schema Author", "desc", "2024-01-01")
	script := `var ytInitialPlayerResponse = {"videoDetails":{"author":"Player Author"}};`
	html := fmt.Sprintf(`<html><head><title>Video - YouTube</title></head>
<body><script>%s</script></body></html>`, script)

	doc := newTestDoc(t, html)
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=priority2", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Player Author", result.Variables["author"])
}

func TestYouTubeExtractor_ChannelName_SchemaOrgFallbackWhenNoDOMOrScript(t *testing.T) {
	t.Parallel()

	schema := youtubeVideoObjectSchema("Title", "Schema Only Author", "desc", "2024-01-01")
	doc := newTestDoc(t, "<html><head><title>Video - YouTube</title></head><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=fallback2", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Schema Only Author", result.Variables["author"])
}

// ---------------------------------------------------------------------------
// Thumbnail
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_Thumbnail_GeneratedFromVideoID(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "https://img.youtube.com/vi/dQw4w9WgXcQ/maxresdefault.jpg", result.Variables["image"])
}

func TestYouTubeExtractor_Thumbnail_FromSchemaOrgString(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"@type":        "VideoObject",
		"name":         "Test",
		"thumbnailUrl": "https://example.com/thumb.jpg",
	}
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=thumb1", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "https://example.com/thumb.jpg", result.Variables["image"])
}

// ---------------------------------------------------------------------------
// Content HTML — iframe and description formatting
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_ContentHTML_ContainsEmbed(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=embedTest", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, `src="https://www.youtube.com/embed/embedTest"`)
	assert.Equal(t, result.Content, result.ContentHTML)
}

func TestYouTubeExtractor_ContentHTML_NoIframeWhenNoVideoID(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://example.com/no-video", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.NotContains(t, result.ContentHTML, "<iframe")
}

func TestYouTubeExtractor_ContentHTML_DescriptionNewlinesFormatted(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"@type":       "VideoObject",
		"description": "Line one\nLine two\nLine three",
	}
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=newlines1", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "Line one<br>Line two<br>Line three")
}

// ---------------------------------------------------------------------------
// Description truncation
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_DescriptionTruncation(t *testing.T) {
	t.Parallel()

	longDesc := strings.Repeat("word ", 60) // >200 chars
	schema := map[string]any{
		"@type":       "VideoObject",
		"description": longDesc,
	}
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=trunc1", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.LessOrEqual(t, len(result.Variables["description"]), 200)
}

func TestYouTubeExtractor_DescriptionShort_NotTruncated(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"@type":       "VideoObject",
		"description": "Short description.",
	}
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=short1", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "Short description.", result.Variables["description"])
}

// ---------------------------------------------------------------------------
// ExtractedContent map
// ---------------------------------------------------------------------------

func TestYouTubeExtractor_ExtractedContent_Fields(t *testing.T) {
	t.Parallel()

	schema := youtubeVideoObjectSchema("Title", "Channel", "desc", "2024-01-01")
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewYouTubeExtractor(doc, "https://www.youtube.com/watch?v=ecTest1", schema)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Equal(t, "ecTest1", result.ExtractedContent["videoId"])
	assert.Equal(t, "Channel", result.ExtractedContent["author"])
}
