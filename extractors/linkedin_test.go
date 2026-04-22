package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLinkedInDoc(t *testing.T, body string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	require.NoError(t, err)
	return doc
}

// linkedinPostPage is a minimal LinkedIn feed post with text, author, and a comment.
const linkedinPostPage = `<html><body>
<div role="article" class="feed-shared-update-v2" data-urn="urn:li:activity:12345">
  <div class="update-components-actor__title">
    <span class="visually-hidden">Jane Doe</span>
    Jane Doe
  </div>
  <div class="update-components-text update-components-update-v2__commentary">
    <p>Excited to share this update with everyone!</p>
  </div>
  <article class="comments-comment-entity">
    <span class="comments-comment-meta__description-title">Bob Smith</span>
    <div class="comments-comment-entity__content">
      <div class="update-components-text"><p>Great post!</p></div>
    </div>
    <time class="comments-comment-meta__data">2d</time>
  </article>
</div>
</body></html>`

// linkedinLoginPage has no feed-update article — simulates login wall.
const linkedinLoginPage = `<html><body>
<div class="login-form"><p>Sign in to LinkedIn</p></div>
</body></html>`

// linkedinQuotedPage has a quoted/reposted post inside the update.
const linkedinQuotedPage = `<html><body>
<div role="article" class="feed-shared-update-v2" data-urn="urn:li:activity:99">
  <div class="update-components-actor__title">Alice</div>
  <div class="update-components-text update-components-update-v2__commentary">
    <p>Check out this repost!</p>
  </div>
  <div class="feed-shared-update-v2__update-content-wrapper">
    <div class="update-components-actor__title">Original Author</div>
    <div class="update-components-actor__sub-description">
      <span aria-hidden="true">2w • 🌐</span>
    </div>
    <div class="update-components-text update-components-update-v2__commentary">
      <p>Original post content.</p>
    </div>
    <a class="update-components-mini-update-v2__link-to-details-page" href="/posts/original-123">link</a>
  </div>
</div>
</body></html>`

func TestLinkedInExtractor_CanExtract_True(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinPostPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:12345", nil)
	assert.True(t, e.CanExtract())
}

func TestLinkedInExtractor_CanExtract_LoginWall(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinLoginPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/", nil)
	assert.False(t, e.CanExtract())
}

func TestLinkedInExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinPostPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:12345", nil)
	assert.Equal(t, "LinkedInExtractor", e.Name())
}

func TestLinkedInExtractor_Extract_TitleContainsAuthor(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinPostPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:12345", nil)
	result := e.Extract()
	assert.Contains(t, result.Variables["title"], "LinkedIn")
}

func TestLinkedInExtractor_Extract_Site(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinPostPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:12345", nil)
	result := e.Extract()
	assert.Equal(t, "LinkedIn", result.Variables["site"])
}

func TestLinkedInExtractor_Extract_PostURN(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinPostPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:12345", nil)
	result := e.Extract()
	assert.Equal(t, "urn:li:activity:12345", result.ExtractedContent["postUrn"])
}

func TestLinkedInExtractor_Extract_ContentHasPostText(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinPostPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:12345", nil)
	result := e.Extract()
	assert.Contains(t, result.ContentHTML, "Excited to share this update")
	assert.Contains(t, result.ContentHTML, "extractor-linkedin")
}

func TestLinkedInExtractor_Extract_CommentsIncluded(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinPostPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:12345", nil)
	result := e.Extract()
	assert.Contains(t, result.ContentHTML, "Bob Smith")
	assert.Contains(t, result.ContentHTML, "Great post!")
}

func TestLinkedInExtractor_Extract_Description(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinPostPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:12345", nil)
	result := e.Extract()
	assert.NotEmpty(t, result.Variables["description"])
	assert.LessOrEqual(t, len([]rune(result.Variables["description"])), 140)
}

func TestLinkedInExtractor_Extract_QuotedPost(t *testing.T) {
	t.Parallel()
	doc := newLinkedInDoc(t, linkedinQuotedPage)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:99", nil)
	result := e.Extract()
	assert.Contains(t, result.ContentHTML, "Check out this repost")
	assert.Contains(t, result.ContentHTML, "quoted-post")
	assert.Contains(t, result.ContentHTML, "Original post content")
}

func TestLinkedInExtractor_Extract_NoAuthorFallback(t *testing.T) {
	t.Parallel()
	const page = `<html><body>
<div role="article" class="feed-shared-update-v2" data-urn="urn:li:activity:0">
</div>
</body></html>`
	doc := newLinkedInDoc(t, page)
	e := NewLinkedInExtractor(doc, "https://www.linkedin.com/feed/update/urn:li:activity:0", nil)
	result := e.Extract()
	assert.Equal(t, "LinkedIn Post", result.Variables["title"])
}

func TestLinkedInContains(t *testing.T) {
	t.Parallel()
	const page = `<html><body>
<div id="parent"><span id="child">text</span></div>
<div id="unrelated">other</div>
</body></html>`
	doc := newLinkedInDoc(t, page)
	parent := doc.Find("#parent")
	child := doc.Find("#child")
	unrelated := doc.Find("#unrelated")

	assert.True(t, linkedinContains(parent, child))
	assert.False(t, linkedinContains(child, parent))
	assert.False(t, linkedinContains(unrelated, child))
}
