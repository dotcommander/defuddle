package elements

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func processorDoc(t *testing.T, source string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(source))
	require.NoError(t, err)
	return doc
}

func TestScopedRolesIndividualFlags(t *testing.T) {
	cases := []struct {
		role, tag string
		options   RoleProcessingOptions
	}{
		{"paragraph", "p", RoleProcessingOptions{ConvertParagraphs: true}},
		{"list", "ul", RoleProcessingOptions{ConvertLists: true}},
		{"button", "button", RoleProcessingOptions{ConvertButtons: true}},
		{"link", "a", RoleProcessingOptions{ConvertLinks: true}},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			for _, enabled := range []bool{false, true} {
				doc := processorDoc(t, fmt.Sprintf(`<article><div role="%s" title="original"><em>child</em></div></article><aside><div role="%s">outside</div></aside>`, tc.role, tc.role))
				raw := `q"&<>é`
				doc.Find("article div").SetAttr("title", raw)
				options := tc.options
				if !enabled {
					options = RoleProcessingOptions{}
				}
				ProcessRolesInScope(doc.Find("article"), &options)
				tag := "div"
				if enabled {
					tag = tc.tag
				}
				assert.Equal(t, 1, doc.Find("article "+tag).Length())
				assert.Equal(t, raw, doc.Find("article "+tag).AttrOr("title", ""))
				assert.Equal(t, 1, doc.Find("article "+tag+" em").Length())
				assert.Equal(t, tc.role, doc.Find("aside div").AttrOr("role", ""))
				serialized, err := doc.Html()
				require.NoError(t, err)
				round := processorDoc(t, serialized)
				assert.Equal(t, raw, round.Find("article "+tag).AttrOr("title", ""))
			}
		})
	}
}

func TestScopedFootnotesStages(t *testing.T) {
	for _, disabled := range []string{"none", "detect", "link", "number", "accessibility", "section"} {
		t.Run(disabled, func(t *testing.T) {
			doc := processorDoc(t, `<article><p>text<a class="footnote-ref" href="#inside-note">seven</a><a class="footnote-ref" href="#outside-note">outside ref</a></p><div id="inside-note">A &amp; B &lt;literal&gt;</div></article><aside><div id="outside-note">outside definition</div><a class="footnote-ref" href="#inside-note">outside reference</a></aside>`)
			before, _ := doc.Find("aside").Html()
			opts := DefaultFootnoteProcessingOptions()
			opts.FootnotePrefix = `custom"&é`
			opts.SectionTitle = `Notes & <literal>`
			switch disabled {
			case "detect":
				opts.DetectFootnotes = false
			case "link":
				opts.LinkFootnotes = false
			case "number":
				opts.NumberFootnotes = false
			case "accessibility":
				opts.ImproveAccessibility = false
			case "section":
				opts.GenerateSection = false
			}
			notes := ProcessFootnotesInScope(doc, doc.Find("article"), opts)
			after, _ := doc.Find("aside").Html()
			assert.Equal(t, before, after)
			if disabled == "detect" {
				assert.Empty(t, notes)
				assert.Zero(t, doc.Find("article .footnotes").Length())
				return
			}
			require.Len(t, notes, 2)
			assert.Equal(t, "A & B <literal>", notes[0].Content)
			assert.Empty(t, notes[1].Content)
			assert.Zero(t, notes[1].Definition.Length())
			assert.Equal(t, disabled != "link", notes[0].Linked)
			assert.False(t, notes[1].Linked)
			ref := notes[0].Reference
			if disabled == "number" {
				assert.Equal(t, "seven", ref.Text())
			} else {
				assert.Equal(t, "1", ref.Text())
			}
			if disabled == "accessibility" {
				_, ok := ref.Attr("role")
				assert.False(t, ok)
			} else {
				assert.Equal(t, "doc-noteref", ref.AttrOr("role", ""))
			}
			if disabled == "section" {
				assert.Zero(t, doc.Find("article .footnotes").Length())
			} else {
				assert.Equal(t, opts.SectionTitle, doc.Find("article .footnotes h2").Text())
				assert.Equal(t, 1, doc.Find("article .footnotes li").Length())
				number := 1
				if disabled == "number" {
					number = 0
				}
				assert.Equal(t, fmt.Sprintf("%s:%d", opts.FootnotePrefix, number), doc.Find("article .footnotes li").AttrOr("id", ""))
				assert.Contains(t, doc.Find("article .footnotes li").Text(), "A & B <literal>")
			}
		})
	}
}

func TestScopedFootnoteSectionLocations(t *testing.T) {
	for _, location := range []string{"end", "after-content", "custom"} {
		t.Run(location, func(t *testing.T) {
			for _, root := range []string{"article", "div"} {
				doc := processorDoc(t, fmt.Sprintf(`<%s id="scope"><main><p><a class="footnote-ref" href="#note">1</a></p><div id="note">definition</div></main><p>tail</p></%s><aside>outside</aside>`, root, root))
				scope := doc.Find("#scope")
				opts := DefaultFootnoteProcessingOptions()
				opts.SectionLocation = location
				ProcessFootnotesInScope(doc, scope, opts)
				assert.Equal(t, 1, scope.Find(".footnotes").Length())
				assert.Zero(t, doc.Find("body > .footnotes").Length())
				if location == "after-content" && root == "div" {
					assert.True(t, scope.Find("main").Next().HasClass("footnotes"))
				} else {
					assert.True(t, scope.Children().Last().HasClass("footnotes"))
				}
			}
		})
	}
}

func TestCodeAndMathGeneratedAttributeRoundTrip(t *testing.T) {
	raw := `q"&<>é`
	doc := processorDoc(t, `<article><pre>original</pre></article>`)
	var code CodeBlockProcessor
	code.formatCodeBlock(doc.Find("pre"), raw, "<body>&", nil)
	assert.Equal(t, raw, doc.Find("code").AttrOr("data-lang", ""))
	assert.Equal(t, "language-"+raw, doc.Find("code").AttrOr("class", ""))
	assert.Equal(t, "<body>&", doc.Find("code").Text())
	var math MathProcessor
	mathDoc := processorDoc(t, math.createCleanMathElement(nil, raw, false))
	assert.Equal(t, raw, mathDoc.Find("math").AttrOr("data-latex", ""))
	assert.Equal(t, raw, mathDoc.Find("math").Text())
	mathMLDoc := processorDoc(t, math.createCleanMathElement(&MathData{MathML: `<math><mi>x</mi></math>`}, raw, true))
	assert.Equal(t, raw, mathMLDoc.Find("math").AttrOr("data-latex", ""))
	assert.Equal(t, 1, mathMLDoc.Find("math mi").Length())
}
