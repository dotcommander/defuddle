package markdown

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// C3-01: renderCallout must require exact class "markdown-alert", not match
// "markdown-alert-title", and strip alert title safely on token boundaries
// without panicking on Unicode case-folding length changes.
func TestRenderCallout_ExactClassAndSafeTitleStrip(t *testing.T) {
	t.Parallel()

	t.Run("markdown-alert-title div is not treated as callout container", func(t *testing.T) {
		t.Parallel()
		md, err := ConvertHTML(`<div class="markdown-alert-title">Note</div><p>Body</p>`)
		require.NoError(t, err)
		assert.NotContains(t, md, "[!TITLE]")
		assert.NotContains(t, md, "[!NOTE]")
		assert.Contains(t, md, "Note")
		assert.Contains(t, md, "Body")
	})

	t.Run("does not strip prefix of longer word like Notebook", func(t *testing.T) {
		t.Parallel()
		md, err := ConvertHTML(`<div class="markdown-alert markdown-alert-note"><p>Notebook is useful</p></div>`)
		require.NoError(t, err)
		assert.Equal(t, "> [!NOTE]\n> Notebook is useful", md)
	})

	t.Run("strips matching title token case-insensitively", func(t *testing.T) {
		t.Parallel()
		md, err := ConvertHTML(`<div class="markdown-alert markdown-alert-warning"><p class="markdown-alert-title">Warning</p><p>Watch out</p></div>`)
		require.NoError(t, err)
		assert.Equal(t, "> [!WARNING]\n> Watch out", md)
	})

	t.Run("unicode case folding does not panic or corrupt UTF-8", func(t *testing.T) {
		t.Parallel()
		// 'ß' uppercases to "SS" (different length/bytes) and 'ﬁ' uppercases to "FI".
		md, err := ConvertHTML(`<div class="markdown-alert markdown-alert-ss"><p>ß</p></div>`)
		require.NoError(t, err)
		assert.True(t, utf8.ValidString(md))
		assert.Contains(t, md, "[!SS]")

		md2, err := ConvertHTML(`<div class="markdown-alert markdown-alert-fi"><p>ﬁle</p></div>`)
		require.NoError(t, err)
		assert.True(t, utf8.ValidString(md2))
		assert.Contains(t, md2, "ﬁle")
	})
}

// C3-02: Task-list checkboxes (including HTML5 boolean checked), nested task
// lists, and <ol start="N"> must render properly without dropped checkboxes or
// doubled numbering.
func TestConvertHTML_TaskListsAndOrderedListStart(t *testing.T) {
	t.Parallel()

	t.Run("HTML5 boolean checked and unchecked task items", func(t *testing.T) {
		t.Parallel()
		md, err := ConvertHTML(`<ul><li class="task-list-item"><input type="checkbox" checked> Done</li><li class="task-list-item"><input type="checkbox"> Todo</li></ul>`)
		require.NoError(t, err)
		assert.Equal(t, "- [x] Done\n- [ ] Todo", md)
	})

	t.Run("nested task list preserves indentation and checkboxes", func(t *testing.T) {
		t.Parallel()
		html := `<ul>
			<li class="task-list-item"><input type="checkbox" checked> Parent
				<ul>
					<li class="task-list-item"><input type="checkbox"> Child</li>
				</ul>
			</li>
		</ul>`
		md, err := ConvertHTML(html)
		require.NoError(t, err)
		assert.Contains(t, md, "- [x] Parent")
		assert.Contains(t, md, "  - [ ] Child")
	})

	t.Run("ordered list with start attribute renders starting number once", func(t *testing.T) {
		t.Parallel()
		md, err := ConvertHTML(`<ol start="5"><li>Fifth</li><li>Sixth</li></ol>`)
		require.NoError(t, err)
		assert.Equal(t, "5. Fifth\n6. Sixth", md)
	})

	t.Run("ordered list with start attribute and task checkboxes", func(t *testing.T) {
		t.Parallel()
		md, err := ConvertHTML(`<ol start="3"><li class="task-list-item"><input type="checkbox" checked> Third</li><li class="task-list-item"><input type="checkbox"> Fourth</li></ol>`)
		require.NoError(t, err)
		assert.Equal(t, "3. [x] Third\n4. [ ] Fourth", md)
	})
}

// C3-03: Multi-byte UTF-8 data-callout values must not be sliced by byte index 1.
func TestRenderCalloutBlockquote_MultiByteUTF8(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		calloutType string
		wantHeader  string
	}{
		{"german umlaut", "über", "> [!über] Über"},
		{"cjk", "注意", "> [!注意] 注意"},
		{"ascii", "tip", "> [!tip] Tip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			md, err := ConvertHTML(`<blockquote data-callout="` + tc.calloutType + `">Details</blockquote>`)
			require.NoError(t, err)
			assert.True(t, utf8.ValidString(md))
			assert.Contains(t, md, tc.wantHeader)
			assert.Contains(t, md, "> Details")
		})
	}
}

// C3-04: bestSrcsetURL must handle descriptor-less candidates and comma-adjacent
// candidates without corrupting URLs, while preserving CDN URLs with commas.
func TestBestSrcsetURL_EdgeCases(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		srcset string
		want   string
	}{
		{
			name:   "descriptor-less candidate before width candidate",
			srcset: "default.jpg, large.jpg 1200w",
			want:   "large.jpg",
		},
		{
			name:   "comma-adjacent candidate without space after comma",
			srcset: "small.jpg 300w,large.jpg 1200w",
			want:   "large.jpg",
		},
		{
			name:   "comma-adjacent with CDN URLs containing commas",
			srcset: "https://cdn.example.com/w_424,c_limit/img.jpg 424w,https://cdn.example.com/w_848,c_limit/img.jpg 848w",
			want:   "https://cdn.example.com/w_848,c_limit/img.jpg",
		},
		{
			name:   "trailing descriptor-less candidate",
			srcset: "medium.jpg 600w, fallback.jpg",
			want:   "medium.jpg",
		},
		{
			name:   "density descriptor followed by comma-adjacent width candidate",
			srcset: "retina.jpg 2x,wide.jpg 1600w",
			want:   "wide.jpg",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, bestSrcsetURL(tc.srcset))
		})
	}
}

// C3-05: renderImage and renderFigure must escape src/title/alt, and renderFigure
// must not emit ![]() when src is empty.
func TestRenderImageAndFigure_EscapingAndEmptySrc(t *testing.T) {
	t.Parallel()

	t.Run("standalone image escapes src spaces, parens, quotes in title, brackets in alt", func(t *testing.T) {
		t.Parallel()
		md, err := ConvertHTML(`<p><img src="https://example.com/my path/img.jpg" alt="A [bracketed] alt" title="Say &quot;hi&quot;"></p>`)
		require.NoError(t, err)
		assert.Equal(t, `![A \[bracketed\] alt](<https://example.com/my path/img.jpg> "Say \"hi\"")`, md)

		md2, err := ConvertHTML(`<p><img src="https://example.com/img_(1).jpg" alt="Line 1&#10;Line 2"></p>`)
		require.NoError(t, err)
		assert.Equal(t, `![Line 1 Line 2](https://example.com/img_\(1\).jpg)`, md2)
	})

	t.Run("figure escapes src, title, and alt", func(t *testing.T) {
		t.Parallel()
		md, err := ConvertHTML(`<figure><img src="https://example.com/a b/pic.jpg" alt="Fig [1]" title="My &quot;Fig&quot;"><figcaption>Caption</figcaption></figure>`)
		require.NoError(t, err)
		assert.Contains(t, md, `![Fig \[1\]](<https://example.com/a b/pic.jpg> "My \"Fig\"")`)
		assert.Contains(t, md, "Caption")
	})

	t.Run("figure with empty img src does not emit empty image syntax", func(t *testing.T) {
		t.Parallel()
		md, err := ConvertHTML(`<figure><img alt="Missing"><figcaption>Only caption</figcaption></figure>`)
		require.NoError(t, err)
		assert.NotContains(t, md, "![](")
		assert.NotContains(t, md, "![Missing]()")
		assert.Equal(t, "Only caption", md)
	})
}

// C3-07: Tables with colspan="1" or rowspan="1" are not complex and should
// render as Markdown tables rather than raw HTML.
func TestHasComplexTableStructure_SpanOneIsNotComplex(t *testing.T) {
	t.Parallel()

	t.Run("colspan=1 and rowspan=1 render as markdown table", func(t *testing.T) {
		t.Parallel()
		html := `<table><thead><tr><th colspan="1">A</th><th rowspan="1">B</th></tr></thead><tbody><tr><td colspan="1">1</td><td rowspan="1">2</td></tr></tbody></table>`
		md, err := ConvertHTML(html)
		require.NoError(t, err)
		assert.NotContains(t, md, "<table")
		assert.Contains(t, md, "| A | B |")
		assert.Contains(t, md, "| 1 | 2 |")
	})

	t.Run("colspan=2 renders as raw HTML table", func(t *testing.T) {
		t.Parallel()
		html := `<table><tr><th colspan="2">Merged</th></tr><tr><td>1</td><td>2</td></tr></table>`
		md, err := ConvertHTML(html)
		require.NoError(t, err)
		assert.Contains(t, md, "<table")
		assert.Contains(t, md, `colspan="2"`)
	})
}

// C3-08: renderCodeBlock must choose a fence strictly longer than the longest
// consecutive backtick run in the code.
func TestRenderCodeBlock_DynamicFenceLength(t *testing.T) {
	t.Parallel()

	html := "<pre><code class=\"language-md\">````go\nfmt.Println()\n````</code></pre>"
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Contains(t, md, "`````md\n````go\nfmt.Println()\n````\n`````")
}

// C3-09: Footnote definitions must normalize slash-free cite_note- IDs, match
// renderFootnoteRef case, indent multi-paragraph footnotes, and skip missing IDs.
func TestFootnotes_CiteNoteCaseMultiParagraphAndEmptyID(t *testing.T) {
	t.Parallel()

	t.Run("slash-free cite_note- ID is normalized", func(t *testing.T) {
		t.Parallel()
		html := `<div id="footnotes"><ol><li id="cite_note-42">Citation text</li></ol></div>`
		md, err := ConvertHTML(html)
		require.NoError(t, err)
		assert.Equal(t, "[^42]: Citation text", md)
	})

	t.Run("case matches between footnote ref and definition", func(t *testing.T) {
		t.Parallel()
		html := `<p>Body<sup id="fnref:NoteA"><a href="#fn:NoteA">1</a></sup></p><div id="footnotes"><ol><li id="fn:NoteA">Note content</li></ol></div>`
		md, err := ConvertHTML(html)
		require.NoError(t, err)
		assert.Contains(t, md, "Body[^notea]")
		assert.Contains(t, md, "[^notea]: Note content")
	})

	t.Run("multi-paragraph footnote indents continuation paragraphs", func(t *testing.T) {
		t.Parallel()
		html := `<div id="footnotes"><ol><li id="fn:1"><p>First paragraph.</p><p>Second paragraph.</p></li></ol></div>`
		md, err := ConvertHTML(html)
		require.NoError(t, err)
		assert.Equal(t, "[^1]: First paragraph.\n\n    Second paragraph.", md)
	})

	t.Run("footnote li without id does not emit empty [^]:", func(t *testing.T) {
		t.Parallel()
		html := `<div id="footnotes"><ol><li>Orphan item</li><li id="fn:1">Valid item</li></ol></div>`
		md, err := ConvertHTML(html)
		require.NoError(t, err)
		assert.NotContains(t, md, "[^]:")
		assert.Equal(t, "[^1]: Valid item", md)
	})
}

// C3-10: renderComplexLink must preserve the heading level (e.g. ## for h2).
func TestRenderComplexLink_PreservesHeadingLevel(t *testing.T) {
	t.Parallel()

	html := `<a href="https://example.com/post"><h3>Card Heading</h3><p>Card summary</p></a>`
	md, err := ConvertHTML(html)
	require.NoError(t, err)
	assert.Equal(t, "### Card Heading\n\nCard summary\n\n[View original](https://example.com/post)", md)
}

// C3-11: removeEmptyLinks must not enter fenced-code mode on a line-leading
// inline code span using 3+ backticks (CommonMark §4.5).
func TestRemoveEmptyLinks_LineLeadingTripleBacktickInlineCode(t *testing.T) {
	t.Parallel()

	input := "```code``` [](gone)\n[](also-gone)"
	want := "```code``` \n"
	assert.Equal(t, want, removeEmptyLinks(input))
}
