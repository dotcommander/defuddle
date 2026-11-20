package elements

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestC4_02_StandardizeFootnotesNoDuplicateSupOrBacklink(t *testing.T) {
	input := `<html><body><article>
		<p>First<sup id="fnref:1"><a href="#fn:1">1</a></sup> and second<span id="fnref2" class="footnote-ref"><a href="#fn2">2</a></span>.</p>
		<p>Grouped<sup><a href="#fn:1">1</a><a href="#fn:2">2</a></sup>.</p>
		<div class="footnotes"><ol>
			<li id="fn:1">First note</li>
			<li id="fn2">Second note</li>
		</ol></div>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	article := doc.Find("article")
	StandardizeFootnotesInScope(doc, article)

	// Paragraph 0 should have exactly 2 <sup> refs (one for fn:1, one for fn:2), not 4.
	p0Sups := article.Find("p").Eq(0).Find("sup")
	if p0Sups.Length() != 2 {
		htmlStr, _ := article.Find("p").Eq(0).Html()
		t.Fatalf("expected 2 <sup> refs in first paragraph, got %d; html=%s", p0Sups.Length(), htmlStr)
	}

	// Paragraph 1 grouped bare <sup> should expand to 2 <sup> refs.
	p1Sups := article.Find("p").Eq(1).Find("sup")
	if p1Sups.Length() != 2 {
		htmlStr, _ := article.Find("p").Eq(1).Html()
		t.Fatalf("expected 2 <sup> refs in grouped paragraph, got %d; html=%s", p1Sups.Length(), htmlStr)
	}

	// Each footnote should have exactly 2 backlinks (1 from p0 + 1 from p1), not duplicated.
	if got := article.Find(`#footnotes li[id="fn:1"] a.footnote-backref`).Length(); got != 2 {
		t.Fatalf("expected 2 backlinks for fn:1, got %d", got)
	}
	if got := article.Find(`#footnotes li[id="fn:2"] a.footnote-backref`).Length(); got != 2 {
		t.Fatalf("expected 2 backlinks for fn:2, got %d", got)
	}
}

func TestC4_03_ProcessFootnotesNumbersBeforeLinking(t *testing.T) {
	input := `<html><body><article>
		<p>Claim<a href="#fn-a">a</a> and second<a href="#fn-b">b</a>.</p>
		<div class="footnotes">
			<p id="fn-a">Note A text</p>
			<p id="fn-b">Note B text</p>
		</div>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	opts := DefaultFootnoteProcessingOptions()
	opts.GenerateSection = false
	fns := ProcessFootnotesInScope(doc, doc.Find("article"), opts)
	if len(fns) != 2 {
		t.Fatalf("expected 2 footnotes, got %d", len(fns))
	}

	htmlStr, _ := doc.Find("article").Html()
	if strings.Contains(htmlStr, "fn:0") || strings.Contains(htmlStr, "fnref:0") {
		t.Fatalf("unexpected zero-indexed footnote ID in output: %s", htmlStr)
	}
	if !strings.Contains(htmlStr, `id="fnref:1"`) || !strings.Contains(htmlStr, `href="#fn:1"`) {
		t.Fatalf("expected fnref:1 / #fn:1 in output: %s", htmlStr)
	}
	if !strings.Contains(htmlStr, `id="fnref:2"`) || !strings.Contains(htmlStr, `href="#fn:2"`) {
		t.Fatalf("expected fnref:2 / #fn:2 in output: %s", htmlStr)
	}
}

func TestC4_06_SpanImagesPreserveProseAndRemoveConsumedSiblingCaption(t *testing.T) {
	// 1. Span with surrounding prose and span with multiple images must be preserved.
	input := `<html><body><article>
		<p><span>Keep this prose <img src="https://example.com/a.jpg" alt="a"> and trailing words</span></p>
		<p><span><img src="https://example.com/1.jpg" alt="one"><img src="https://example.com/2.jpg" alt="two"></span></p>
		<div class="media-wrap">
			<span><img src="https://example.com/hero.jpg" alt="hero"></span>
			<div class="image-caption">A meaningful sibling caption for the hero image</div>
		</div>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	article := doc.Find("article")
	ProcessImagesInScope(article, DefaultImageProcessingOptions())

	p0Text := article.Find("p").Eq(0).Text()
	if !strings.Contains(p0Text, "Keep this prose") || !strings.Contains(p0Text, "trailing words") {
		t.Fatalf("surrounding prose inside span was deleted: %q", p0Text)
	}
	if got := article.Find("p").Eq(1).Find("img").Length(); got != 2 {
		t.Fatalf("expected both images in multi-image span to be preserved, got %d", got)
	}

	// Sibling caption should be moved into <figcaption> and removed from outside <figure>.
	wrap := article.Find(".media-wrap")
	if wrap.Find("figure figcaption").Length() != 1 {
		htmlStr, _ := wrap.Html()
		t.Fatalf("expected <figure><figcaption> in media-wrap, got: %s", htmlStr)
	}
	if wrap.Find(".image-caption").Length() != 0 {
		htmlStr, _ := wrap.Html()
		t.Fatalf("expected consumed sibling .image-caption to be removed, got: %s", htmlStr)
	}
}

func TestC4_07_MathCleanupScriptsUsesParentBeforeDetach(t *testing.T) {
	input := `<html><body><article>
		<div class="eq">
			<span class="MathJax_Preview">x^2</span>
			<span class="MathJax" data-latex="x^2">x^2</span>
			<script type="math/tex">x^2</script>
		</div>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	article := doc.Find("article")
	ProcessMathInScope(article, DefaultMathProcessingOptions())

	if article.Find(".MathJax_Preview").Length() != 0 {
		t.Fatalf("expected .MathJax_Preview sibling to be cleaned up")
	}
	if article.Find(`script[type^="math/"]`).Length() != 0 {
		t.Fatalf("expected math/tex script sibling to be cleaned up")
	}
	if article.Find("math").Length() != 1 {
		t.Fatalf("expected 1 clean <math> element, got %d", article.Find("math").Length())
	}
}

func TestC4_08_ProcessMathInScopeSelectorsAndStandaloneBlockMath(t *testing.T) {
	input := `<html><body><article>
		<mjx-container><math><mi>a</mi></math></mjx-container>
		<span class="mwe-math-element"><math><mi>b</mi></math></span>
		<img class="latex" src="https://s0.wp.com/latex.php?latex=E%3Dmc%5E2&bg=ffffff"/>
		<math display="block"><mi>c</mi></math>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	article := doc.Find("article")
	ProcessMathInScope(article, DefaultMathProcessingOptions())

	if article.Find("mjx-container, .mwe-math-element, img.latex").Length() != 0 {
		htmlStr, _ := article.Html()
		t.Fatalf("expected mjx-container, mwe-math-element, and img.latex to be converted in scope; html=%s", htmlStr)
	}
	if got := article.Find(`math[display="block"]`).Length(); got != 1 {
		htmlStr, _ := article.Html()
		t.Fatalf("expected standalone <math display=\"block\"> to retain block display, got %d; html=%s", got, htmlStr)
	}
}

func TestC4_09_EscapePictureAndUniImageAttributesAndCaption(t *testing.T) {
	input := `<html><body><article>
		<picture><source srcset='https://example.com/a.jpg?x=1&amp;y="2" 1x'></picture>
		<uni-image-full-width>
			<img src='https://example.com/u.jpg?a=1&amp;b=2' alt='He said "hi" &amp; <bye>'>
			<figcaption>Caption with &lt;script&gt;alert(1)&lt;/script&gt; &amp; quotes</figcaption>
		</uni-image-full-width>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	article := doc.Find("article")
	ProcessImagesInScope(article, DefaultImageProcessingOptions())

	if article.Find("script").Length() != 0 {
		htmlStr, _ := article.Html()
		t.Fatalf("unescaped caption injected a <script> tag: %s", htmlStr)
	}
	uniImg := article.Find("figure img").First()
	if got := uniImg.AttrOr("alt", ""); got != `He said "hi" & <bye>` {
		t.Fatalf("expected escaped alt round-trip, got %q", got)
	}
	picImg := article.Find("picture img").First()
	if got := picImg.AttrOr("src", ""); !strings.Contains(got, `"2"`) {
		t.Fatalf("expected escaped picture src round-trip, got %q", got)
	}
}

func TestC4_10_TransformLazyImagesDoesNotOverwriteValidSrcOrSrcset(t *testing.T) {
	input := `<html><body><article>
		<img loading="lazy"
			src="https://example.com/real.jpg"
			srcset="https://example.com/real-1x.jpg 1x, https://example.com/real-2x.jpg 2x"
			data-thumb="https://example.com/thumb.jpg"
			data-alt-srcset="https://example.com/alt-1x.jpg 1x, https://example.com/alt-2x.jpg 2x" />
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	article := doc.Find("article")
	ProcessImagesInScope(article, DefaultImageProcessingOptions())

	img := article.Find("img").First()
	if got := img.AttrOr("src", ""); got != "https://example.com/real.jpg" {
		t.Fatalf("valid src was overwritten: got %q", got)
	}
	if got := img.AttrOr("srcset", ""); got != "https://example.com/real-1x.jpg 1x, https://example.com/real-2x.jpg 2x" {
		t.Fatalf("valid srcset was overwritten: got %q", got)
	}
}

func TestC4_12_WordPressSyntaxHighlighterSelfMatchAndNbspRuneCount(t *testing.T) {
	// 2 non-breaking spaces (\u00a0) are 4 bytes in UTF-8 but 2 runes -> should become 2 spaces.
	input := `<html><body><div class="syntaxhighlighter python">
		<table><tbody><tr><td class="code"><div class="container">
			<div class="line"><code class="plain">def f():</code></div>
			<div class="line"><code class="spaces">` + "\u00a0\u00a0" + `</code><code class="plain">return 1</code></div>
		</div></td></tr></tbody></table>
	</div></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	p := NewCodeBlockProcessor(doc)
	got := p.extractStructuredContent(doc.Find(".syntaxhighlighter").First())
	want := "def f():\n  return 1"
	if got != want {
		t.Fatalf("extractWordPressContent mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestC4_13_ColonDelimitedFootnoteIDs(t *testing.T) {
	input := `<html><body><article>
		<p>Text<a id="fnref:1" href="#fn:1">1</a> and <span id="fnref:2" class="footnote-ref">2</span></p>
		<div class="footnotes"><ol>
			<li id="fn:1">Colon def 1</li>
			<li id="fn:2">Colon def 2</li>
		</ol></div>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	p := NewFootnoteProcessor(doc)

	if def := p.findFootnoteDefinition("1"); def == nil || def.Length() == 0 {
		t.Fatalf("expected findFootnoteDefinition(\"1\") to match id=\"fn:1\"")
	}
	if id := p.extractInlineFootnoteID(doc.Find(`a[id="fnref:1"]`)); id != "1" {
		t.Fatalf("expected extractInlineFootnoteID on a[id=\"fnref:1\"] = %q, got %q", "1", id)
	}
	if id := p.extractInlineFootnoteID(doc.Find(`span[id="fnref:2"]`)); id != "2" {
		t.Fatalf("expected extractInlineFootnoteID on span[id=\"fnref:2\"] = %q, got %q", "2", id)
	}
}
