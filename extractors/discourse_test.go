package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDiscourseDoc(t *testing.T, body string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	require.NoError(t, err)
	return doc
}

// discourseTopicPage is a minimal but structurally complete Discourse topic page.
const discourseTopicPage = `<html><head>
<meta name="generator" content="Discourse 3.2.0" />
<meta property="og:site_name" content="Ruby Forum" />
<meta property="og:title" content="How to use blocks?" />
<meta property="article:published_time" content="2024-03-15T10:00:00Z" />
</head><body>
<h1 data-topic-id="12345">How to use blocks?</h1>
<div class="topic-post topic-owner">
  <div class="names"><a data-user-card="alice">alice</a></div>
  <div class="cooked"><p>Here is my question about Ruby blocks.</p></div>
  <a class="post-date" href="/t/how-to-use-blocks/12345/1">link</a>
  <div class="relative-date" data-time="1710489600000"></div>
</div>
<div class="topic-post">
  <div class="names"><a data-user-card="bob">bob</a></div>
  <div class="cooked"><p>Use yield inside the method.</p></div>
  <a class="post-date" href="/t/how-to-use-blocks/12345/2">link</a>
  <div class="relative-date" data-time="1710493200000"></div>
</div>
</body></html>`

// discourseNoGenerator has no generator meta — not Discourse.
const discourseNoGenerator = `<html><head></head><body>
<div class="topic-post"><div class="cooked"><p>Some content</p></div></div>
</body></html>`

// discourseGeneratorNoPost has the generator but no .topic-post.
const discourseGeneratorNoPost = `<html><head>
<meta name="generator" content="Discourse 3.2.0" />
</head><body><p>Index page</p></body></html>`

func TestDiscourseExtractor_CanExtract_True(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseTopicPage)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/test/1", nil)
	assert.True(t, e.CanExtract())
}

func TestDiscourseExtractor_CanExtract_NoGenerator(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseNoGenerator)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/test/1", nil)
	assert.False(t, e.CanExtract())
}

func TestDiscourseExtractor_CanExtract_NoTopicPost(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseGeneratorNoPost)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/", nil)
	assert.False(t, e.CanExtract())
}

func TestDiscourseExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseTopicPage)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/test/1", nil)
	assert.Equal(t, "DiscourseExtractor", e.Name())
}

func TestDiscourseExtractor_Extract_Title(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseTopicPage)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/how-to-use-blocks/12345", nil)
	result := e.Extract()
	assert.Equal(t, "How to use blocks?", result.Variables["title"])
}

func TestDiscourseExtractor_Extract_SiteAndAuthor(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseTopicPage)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/how-to-use-blocks/12345", nil)
	result := e.Extract()
	assert.Equal(t, "Ruby Forum", result.Variables["site"])
	assert.Equal(t, "alice", result.Variables["author"])
}

func TestDiscourseExtractor_Extract_Published(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseTopicPage)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/how-to-use-blocks/12345", nil)
	result := e.Extract()
	assert.Equal(t, "2024-03-15", result.Variables["published"])
}

func TestDiscourseExtractor_Extract_TopicID(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseTopicPage)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/how-to-use-blocks/12345", nil)
	result := e.Extract()
	assert.Equal(t, "12345", result.ExtractedContent["topicId"])
}

func TestDiscourseExtractor_Extract_ContentHasOPAndReply(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseTopicPage)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/how-to-use-blocks/12345", nil)
	result := e.Extract()
	assert.Contains(t, result.ContentHTML, "Here is my question about Ruby blocks")
	assert.Contains(t, result.ContentHTML, "Use yield inside the method")
	assert.Contains(t, result.ContentHTML, "extractor-discourse")
}

func TestDiscourseExtractor_Extract_FallbackSiteLabel(t *testing.T) {
	t.Parallel()
	const page = `<html><head>
<meta name="generator" content="Discourse 3.2.0" />
</head><body>
<div class="topic-post topic-owner">
  <div class="names"><a data-user-card="u">u</a></div>
  <div class="cooked"><p>text</p></div>
</div>
</body></html>`
	doc := newDiscourseDoc(t, page)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/test/1", nil)
	result := e.Extract()
	assert.Equal(t, "Discourse", result.Variables["site"])
}

func TestDiscourseExtractor_Extract_Description(t *testing.T) {
	t.Parallel()
	doc := newDiscourseDoc(t, discourseTopicPage)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/how-to-use-blocks/12345", nil)
	result := e.Extract()
	assert.NotEmpty(t, result.Variables["description"])
	assert.LessOrEqual(t, len([]rune(result.Variables["description"])), 140)
}

func TestDiscourseExtractor_Extract_NoRepliesWhenOnlyOP(t *testing.T) {
	t.Parallel()
	const page = `<html><head>
<meta name="generator" content="Discourse 3.2.0" />
</head><body>
<div class="topic-post topic-owner">
  <div class="names"><a data-user-card="alice">alice</a></div>
  <div class="cooked"><p>Only post.</p></div>
</div>
</body></html>`
	doc := newDiscourseDoc(t, page)
	e := NewDiscourseExtractor(doc, "https://forum.example.com/t/test/1", nil)
	result := e.Extract()
	assert.NotContains(t, result.ContentHTML, `class="comments"`)
}
