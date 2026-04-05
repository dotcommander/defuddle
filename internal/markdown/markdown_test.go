package markdown

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
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

// --- Phase 6: srcset / getBestImageSrc ---

func makeImgNode(attrs ...string) *html.Node {
	n := &html.Node{
		Type: html.ElementNode,
		Data: "img",
	}
	for i := 0; i+1 < len(attrs); i += 2 {
		n.Attr = append(n.Attr, html.Attribute{Key: attrs[i], Val: attrs[i+1]})
	}
	return n
}

func TestGetBestImageSrc_SimpleSrcset(t *testing.T) {
	t.Parallel()
	n := makeImgNode(
		"src", "small.jpg",
		"srcset", "small.jpg 300w, medium.jpg 600w, large.jpg 1200w",
	)
	assert.Equal(t, "large.jpg", getBestImageSrc(n))
}

func TestGetBestImageSrc_SubstackCDNCommas(t *testing.T) {
	t.Parallel()
	// Substack CDN URLs contain commas: w_424,c_limit,f_webp
	n := makeImgNode(
		"src", "fallback.jpg",
		"srcset", "https://cdn.example.com/w_424,c_limit,f_webp/img.jpg 424w, https://cdn.example.com/w_848,c_limit,f_webp/img.jpg 848w",
	)
	got := getBestImageSrc(n)
	assert.Equal(t, "https://cdn.example.com/w_848,c_limit,f_webp/img.jpg", got)
}

func TestGetBestImageSrc_DensityDescriptorIgnored(t *testing.T) {
	t.Parallel()
	// Density descriptors (2x) should be skipped, fall back to src
	n := makeImgNode(
		"src", "default.jpg",
		"srcset", "retina.jpg 2x",
	)
	assert.Equal(t, "default.jpg", getBestImageSrc(n))
}

func TestGetBestImageSrc_NoSrcset(t *testing.T) {
	t.Parallel()
	n := makeImgNode("src", "only.jpg")
	assert.Equal(t, "only.jpg", getBestImageSrc(n))
}

func TestGetBestImageSrc_EmptySrcset(t *testing.T) {
	t.Parallel()
	n := makeImgNode("src", "fallback.jpg", "srcset", "")
	assert.Equal(t, "fallback.jpg", getBestImageSrc(n))
}

func TestGetBestImageSrc_SingleWidthEntry(t *testing.T) {
	t.Parallel()
	n := makeImgNode("src", "small.jpg", "srcset", "big.jpg 800w")
	assert.Equal(t, "big.jpg", getBestImageSrc(n))
}

// --- Phase 6: Embed format ---

func TestConvertHTML_YouTubeEmbed(t *testing.T) {
	t.Parallel()
	html := `<iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ"></iframe>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "![](https://www.youtube.com/watch?v=dQw4w9WgXcQ)")
	assert.NotContains(t, md, "![[ ")
}

func TestConvertHTML_YouTubeNoCookieEmbed(t *testing.T) {
	t.Parallel()
	html := `<iframe src="https://www.youtube-nocookie.com/embed/abc123"></iframe>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "![](https://www.youtube.com/watch?v=abc123)")
}

func TestConvertHTML_TweetDirectEmbed(t *testing.T) {
	t.Parallel()
	html := `<iframe src="https://twitter.com/elonmusk/status/1234567890"></iframe>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "![](https://x.com/elonmusk/status/1234567890)")
}

func TestConvertHTML_TweetPlatformEmbed(t *testing.T) {
	t.Parallel()
	html := `<iframe src="https://platform.twitter.com/embed/Tweet.html?id=9876543210"></iframe>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "![](https://x.com/i/status/9876543210)")
}

func TestConvertHTML_XDotComTweetEmbed(t *testing.T) {
	t.Parallel()
	html := `<iframe src="https://x.com/user/status/111222333"></iframe>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "![](https://x.com/user/status/111222333)")
}

// --- Phase 6: wbr stripping ---

func TestConvertHTML_WbrStripped(t *testing.T) {
	t.Parallel()
	html := `<p>super<wbr>cali<wbr/>fragilistic</p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "supercalifragilistic")
}

// --- Phase 6: bang-before-image post-processing ---

func TestPostProcess_BangBeforeImage(t *testing.T) {
	t.Parallel()
	input := "Wow!![alt](img.jpg)"
	result := postProcess(input)
	assert.Contains(t, result, "Wow! ![alt](img.jpg)")
}

func TestPostProcess_BangBeforeLinkedImage(t *testing.T) {
	t.Parallel()
	input := "Check this![![alt](img.jpg)](link)"
	result := postProcess(input)
	assert.Contains(t, result, "Check this! [![alt](img.jpg)](link)")
}

func TestPostProcess_NormalImageUnchanged(t *testing.T) {
	t.Parallel()
	input := "Here is ![alt](img.jpg) an image"
	result := postProcess(input)
	assert.Contains(t, result, "![alt](img.jpg)")
}

// --- Phase 6: Image with srcset and title ---

func TestConvertHTML_ImageWithSrcset(t *testing.T) {
	t.Parallel()
	html := `<p><img src="small.jpg" srcset="medium.jpg 600w, large.jpg 1200w" alt="Photo"></p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "![Photo](large.jpg)")
}

func TestConvertHTML_ImageWithTitle(t *testing.T) {
	t.Parallel()
	html := `<p><img src="photo.jpg" alt="A photo" title="My photo"></p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, `![A photo](photo.jpg "My photo")`)
}

func TestConvertHTML_FigureWithSrcset(t *testing.T) {
	t.Parallel()
	html := `<figure><img src="small.jpg" srcset="big.jpg 1200w" alt="Fig"><figcaption>Cap</figcaption></figure>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "![Fig](big.jpg)")
	assert.Contains(t, md, "Cap")
}

// --- Phase 6: HTML element preservation ---

func TestConvertHTML_VideoPreserved(t *testing.T) {
	t.Parallel()
	html := `<p>Before</p><video src="movie.mp4" controls></video><p>After</p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "<video")
}

func TestConvertHTML_AudioPreserved(t *testing.T) {
	t.Parallel()
	html := `<p>Text</p><audio src="song.mp3" controls></audio>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "<audio")
}

func TestConvertHTML_SubPreserved(t *testing.T) {
	t.Parallel()
	html := `<p>H<sub>2</sub>O</p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "<sub>")
}

func TestConvertHTML_ScriptRemoved(t *testing.T) {
	t.Parallel()
	html := `<p>Content</p><script>alert("xss")</script>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.NotContains(t, md, "alert")
	assert.Contains(t, md, "Content")
}

func TestConvertHTML_StyleRemoved(t *testing.T) {
	t.Parallel()
	html := `<style>.foo{color:red}</style><p>Content</p>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.NotContains(t, md, ".foo")
	assert.Contains(t, md, "Content")
}
