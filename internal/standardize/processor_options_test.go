package standardize

import (
	"strings"
	"testing"

	"github.com/dotcommander/defuddle/internal/elements"
	"github.com/dotcommander/defuddle/internal/urlutil"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

func TestContentCustomRoleOptions(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			doc := parseDoc(t, `<article><div role="paragraph">paragraph</div><div role="list"><div role="listitem">item</div></div><div role="button">button</div><div role="link" href="https://example.com">link</div><aside class="callout-tip"><p>tip</p></aside><lite-youtube videoid="abc"></lite-youtube></article><aside id="outside"><div role="paragraph">outside</div></aside>`)
			outside, _ := doc.Find("#outside").Html()
			ContentWithOptions(doc.Find("article"), meta(""), doc, Options{ProcessRoles: true, RoleOptions: &elements.RoleProcessingOptions{ConvertParagraphs: enabled, ConvertLists: enabled, ConvertButtons: enabled, ConvertLinks: enabled}}, true)
			for _, tag := range []string{"p", "ul", "button", "a"} {
				if enabled {
					assert.Positive(t, doc.Find("article "+tag).Length())
				}
			}
			if !enabled {
				assert.Equal(t, 5, doc.Find("article [role]").Length())
			}
			assert.Equal(t, 1, doc.Find("article blockquote").Length())
			assert.Equal(t, 1, doc.Find("article iframe").Length())
			after, _ := doc.Find("#outside").Html()
			assert.Equal(t, outside, after)
		})
	}
}

func TestContentDirectProcessorOptions(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "nil", true: "custom"}[custom], func(t *testing.T) {
			doc := parseDoc(t, `<article><pre data-lang="go">x</pre><h2>Heading<button>Copy</button></h2><img src="https://example.com/a.png" width="10" height="10"><span class="MathJax" data-latex="x">x</span><p>tail</p></article>`)
			opts := Options{ProcessCode: true, ProcessHeadings: true, ProcessImages: true, ProcessMath: true}
			if custom {
				opts.CodeOptions = &elements.CodeBlockProcessingOptions{FormatCode: false}
				opts.ImageOptions = &elements.ImageProcessingOptions{RemoveSmallImages: true, MinImageWidth: 100, MinImageHeight: 100}
				opts.HeadingOptions = &elements.HeadingProcessingOptions{RemoveNavigation: false}
				opts.MathOptions = &elements.MathProcessingOptions{ExtractLaTeX: false, ExtractMathML: false}
			}
			ContentWithOptions(doc.Find("article"), meta(""), doc, opts, true)
			if custom {
				assert.Zero(t, doc.Find("pre code").Length())
				assert.Zero(t, doc.Find("img").Length())
				assert.Equal(t, 1, doc.Find("h2 button").Length())
				assert.Empty(t, doc.Find("math").Text())
			} else {
				assert.Equal(t, 1, doc.Find("pre code").Length())
				assert.Zero(t, doc.Find("h2 button").Length())
			}
		})
	}
}

func TestContentCustomFootnoteDispatch(t *testing.T) {
	doc := parseDoc(t, `<article><p>text<a class="footnote-ref" href="#note">seven</a></p><div id="note">definition</div><p>tail</p></article>`)
	ContentWithOptions(doc.Find("article"), meta(""), doc, Options{ProcessFootnotes: true, FootnoteOptions: &elements.FootnoteProcessingOptions{}}, true)
	assert.Equal(t, "seven", doc.Find("article a").Text())
	assert.Equal(t, "note", doc.Find("article div").AttrOr("id", ""))
}

func TestStandardizeSpacesNBSPRuneWidths(t *testing.T) {
	doc := parseDoc(t, "<article><p>a\u00a0b</p><p>a\u00a0\u00a0b</p><pre>a\u00a0b</pre><code>a\u00a0\u00a0b</code></article>")
	standardizeSpaces(doc.Find("article"))
	assert.Equal(t, "a b", doc.Find("p").Eq(0).Text())
	assert.Equal(t, "a  b", doc.Find("p").Eq(1).Text())
	assert.Equal(t, "a\u00a0b", doc.Find("pre").Text())
	assert.Equal(t, "a\u00a0\u00a0b", doc.Find("code").Text())
	// Retain the documented exception for a single NBSP between word text siblings.
	parent := doc.Find("p").Eq(0).Get(0)
	parent.FirstChild.Data = "a"
	space := &html.Node{Type: html.TextNode, Data: "\u00a0"}
	parent.AppendChild(space)
	parent.AppendChild(&html.Node{Type: html.TextNode, Data: "b"})
	standardizeSpaces(doc.Find("p").Eq(0))
	assert.Equal(t, "\u00a0", space.Data)
}

func TestSVGDrawingAttributesAndSanitation(t *testing.T) {
	doc := parseDoc(t, `<article><svg viewBox="0 0 10 10" onload="bad()"><g transform="translate(1)"><path d="M0 0L1 1" onclick="bad()"></path><circle cx="1" cy="2" r="3"></circle></g><foreignObject><div style="color:red" data-junk="remove">HTML</div></foreignObject></svg></article>`)
	stripUnwantedAttributes(doc.Find("article"), false)
	assert.Equal(t, "M0 0L1 1", doc.Find("path").AttrOr("d", ""))
	assert.Equal(t, "translate(1)", doc.Find("g").AttrOr("transform", ""))
	assert.Equal(t, "3", doc.Find("circle").AttrOr("r", ""))
	_, exists := doc.Find("div").Attr("data-junk")
	assert.False(t, exists)
	urlutil.SanitizeUnsafe(doc.Find("article"))
	_, exists = doc.Find("svg").Attr("onload")
	assert.False(t, exists)
	_, exists = doc.Find("path").Attr("onclick")
	assert.False(t, exists)
	assert.Equal(t, "M0 0L1 1", doc.Find("path").AttrOr("d", ""))
}

func TestGeneratedStandardizeAttributesRoundTrip(t *testing.T) {
	raw := `q"&<>é`
	doc := parseDoc(t, `<article><aside></aside><div role="paragraph"><em>child</em></div><lite-youtube></lite-youtube></article>`)
	doc.Find("aside").SetAttr("class", "callout-"+raw)
	doc.Find("div").SetAttr("title", raw)
	doc.Find("lite-youtube").SetAttr("videoid", raw).SetAttr("videotitle", raw)
	standardizeElements(doc.Find("article"), doc, false)
	assert.Equal(t, raw, doc.Find("blockquote").AttrOr("data-callout", ""))
	assert.Equal(t, raw, doc.Find("p").AttrOr("title", ""))
	assert.Equal(t, 1, doc.Find("p em").Length())
	assert.Equal(t, raw, doc.Find("iframe").AttrOr("title", ""))
	assert.True(t, strings.HasSuffix(doc.Find("iframe").AttrOr("src", ""), raw))
	for _, sel := range []string{"blockquote", "p", "iframe"} {
		_, exists := doc.Find(sel).Attr("q")
		assert.False(t, exists)
	}
	var attrs strings.Builder
	writeAllowedAttributes(&attrs, doc.Find("p"))
	round := parseDoc(t, "<article><span"+attrs.String()+">child</span></article>")
	assert.Equal(t, raw, round.Find("span").AttrOr("title", ""))
	urlutil.SanitizeUnsafe(doc.Find("article"))
	assert.Equal(t, raw, doc.Find("p").AttrOr("title", ""))
}

func TestContentProcessorGatesSuppressCustomOptions(t *testing.T) {
	doc := parseDoc(t, `<article><pre>x</pre><h2>Heading<button>Copy</button></h2><span class="MathJax" data-latex="x">x</span><img src="https://example.com/a.png" width="10" height="10"><div role="paragraph">paragraph</div><p><a class="footnote-ref" href="#fn-note">seven</a></p><div id="fn-note">definition</div><p>tail</p></article>`)
	ContentWithOptions(doc.Find("article"), meta(""), doc, Options{
		CodeOptions: elements.DefaultCodeBlockProcessingOptions(), ImageOptions: elements.DefaultImageProcessingOptions(), HeadingOptions: elements.DefaultHeadingProcessingOptions(), MathOptions: elements.DefaultMathProcessingOptions(), RoleOptions: elements.DefaultRoleProcessingOptions(), FootnoteOptions: elements.DefaultFootnoteProcessingOptions(),
	}, true)
	assert.Zero(t, doc.Find("pre code").Length())
	assert.Equal(t, 1, doc.Find("h2 button").Length())
	assert.Zero(t, doc.Find("math").Length())
	assert.Equal(t, 1, doc.Find("img").Length())
	assert.Equal(t, 1, doc.Find(`[role="paragraph"]`).Length())
	assert.Equal(t, "seven", doc.Find("a.footnote-ref").Text())
}

func TestContentNilRoleOptionsPreserveDefaultPath(t *testing.T) {
	doc := parseDoc(t, `<article><div role="list"><div role="listitem"><span class="label">1.</span>item</div></div><div role="button">button</div><p>tail</p></article>`)
	ContentWithOptions(doc.Find("article"), meta(""), doc, Options{ProcessRoles: true}, true)
	assert.Equal(t, 1, doc.Find("article ul").Length())
	assert.Zero(t, doc.Find("article ol").Length())
	assert.Equal(t, 1, doc.Find(`article div[role="button"]`).Length())
}

func TestContentCustomRoleOptionsSurviveCleanup(t *testing.T) {
	t.Parallel()
	doc := parseDoc(t, `<article><div role="paragraph">paragraph</div><div role="list"><div role="listitem">item</div></div><div role="button">button</div><div role="link" href="https://example.com">link</div><p>tail</p></article><aside><div role="paragraph">outside</div></aside>`)
	outside, _ := doc.Find("aside").Html()
	ContentWithOptions(doc.Find("article"), meta(""), doc, Options{ProcessRoles: true, RoleOptions: &elements.RoleProcessingOptions{}}, false)
	assert.Equal(t, 5, doc.Find("article [role]").Length())
	assert.Equal(t, 1, doc.Find(`article div[role="paragraph"]`).Length())
	after, _ := doc.Find("aside").Html()
	assert.Equal(t, outside, after)
}
