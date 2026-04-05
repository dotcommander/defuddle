package markdown

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertHTML_BasicParagraph(t *testing.T) {
	t.Parallel()
	md, err := ConvertHTML("<p>Hello world</p>")
	require.NoError(t, err)
	assert.Equal(t, "Hello world", strings.TrimSpace(md))
}

func TestConvertHTML_CodeBlockWithLanguage(t *testing.T) {
	t.Parallel()
	html := `<pre><code class="language-go">fmt.Println("hello")</code></pre>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "```go")
	assert.Contains(t, md, `fmt.Println("hello")`)
	assert.Contains(t, md, "```")
}

func TestConvertHTML_CodeBlockDataLang(t *testing.T) {
	t.Parallel()
	html := `<pre><code data-lang="python">print("hi")</code></pre>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "```python")
}

func TestConvertHTML_HighlightMark(t *testing.T) {
	t.Parallel()
	md, err := ConvertHTML("<p>This is <mark>important</mark> text</p>")
	require.NoError(t, err)
	assert.Contains(t, md, "==important==")
}

func TestConvertHTML_StripLeadingTitle(t *testing.T) {
	t.Parallel()
	html := `<h1>My Title</h1><p>Content here</p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.False(t, strings.HasPrefix(md, "# My Title"), "leading title should be stripped")
	assert.Contains(t, md, "Content here")
}

func TestConvertHTML_CollapseTripleNewlines(t *testing.T) {
	t.Parallel()
	html := `<p>First</p><br><br><br><br><p>Second</p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.NotContains(t, md, "\n\n\n")
}

func TestConvertHTML_FootnoteRef(t *testing.T) {
	t.Parallel()
	html := `<p>Text<sup id="fnref:1"><a href="#fn:1">1</a></sup></p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "[^1]")
}

func TestConvertHTML_FootnoteBacklinkRemoved(t *testing.T) {
	t.Parallel()
	html := `<p>Footnote text <a href="#fnref:1">↩︎</a></p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.NotContains(t, md, "↩︎")
}

func TestConvertHTML_Figure(t *testing.T) {
	t.Parallel()
	html := `<figure><img src="photo.jpg" alt="A photo"><figcaption>Caption text</figcaption></figure>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "![A photo](photo.jpg)")
	assert.Contains(t, md, "Caption text")
}

func TestConvertHTML_ButtonRemoved(t *testing.T) {
	t.Parallel()
	html := `<p>Content</p><button>Click me</button>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.NotContains(t, md, "Click me")
}

func TestConvertHTML_CalloutBlockquote(t *testing.T) {
	t.Parallel()
	html := `<blockquote data-callout="warning">Be careful here</blockquote>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "[!warning]")
	assert.Contains(t, md, "Be careful here")
}
