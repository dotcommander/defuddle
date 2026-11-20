package standardize

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
	"github.com/dotcommander/defuddle/internal/metadata"
)

func TestC4_01_RemoveHeadingAnchorsPreservesSelfLinkedHeadingText(t *testing.T) {
	input := `<html><body><article>
		<h2><a href="#intro">Introduction</a></h2>
		<p>Intro body.</p>
		<h3>Section One <a href="#sec1" class="permalink">#</a></h3>
		<p>Section one body.</p>
		<h4>Section Two <a href="#sec2" title="Permalink"></a></h4>
		<p>Section two body.</p>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	article := doc.Find("article")
	Content(article, &metadata.Metadata{Title: "Doc"}, doc, false)

	if got := strings.TrimSpace(article.Find("h2").First().Text()); got != "Introduction" {
		t.Fatalf("expected self-linked h2 text %q to be preserved, got %q", "Introduction", got)
	}
	if article.Find("h2 a").Length() != 0 {
		t.Fatalf("expected self-link anchor inside h2 to be unwrapped")
	}
	if got := strings.TrimSpace(article.Find("h3").First().Text()); got != "Section One" {
		t.Fatalf("expected symbol permalink removed from h3, got %q", got)
	}
	if got := strings.TrimSpace(article.Find("h4").First().Text()); got != "Section Two" {
		t.Fatalf("expected empty permalink removed from h4, got %q", got)
	}
}

func TestC4_04_TransformListElementsNoBodyLeakAndNestedHandling(t *testing.T) {
	input := `<html><body><article>
		<div role="list">
			<div role="listitem">
				<div class="label">1)</div>
				<div class="content">
					<div role="paragraph">Outer item</div>
					<div role="list">
						<div role="listitem"><div class="content">Inner item</div></div>
						<div role="listitem">Plain nested item</div>
					</div>
				</div>
			</div>
			<div role="listitem">Second outer without content wrapper</div>
		</div>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	article := doc.Find("article")
	standardizeElements(article, doc, false)

	// No scratchpad <ul>/<ol>/<li> leaked directly under <body>.
	if leaked := doc.Find("body > ul, body > ol, body > li").Length(); leaked != 0 {
		t.Fatalf("expected 0 leaked scratchpad list nodes under body, got %d", leaked)
	}

	// Outer list should be <ol> with exactly 2 direct <li> children.
	outerList := article.ChildrenFiltered("ol")
	if outerList.Length() != 1 {
		htmlStr, _ := article.Html()
		t.Fatalf("expected 1 outer <ol>, got %d; html=%s", outerList.Length(), htmlStr)
	}
	outerItems := outerList.ChildrenFiltered("li")
	if outerItems.Length() != 2 {
		htmlStr, _ := outerList.Html()
		t.Fatalf("expected 2 direct outer <li> items (not pulled-up nested items), got %d; html=%s", outerItems.Length(), htmlStr)
	}

	// Second outer item without .content must not be dropped.
	if got := strings.TrimSpace(outerItems.Eq(1).Text()); got != "Second outer without content wrapper" {
		t.Fatalf("expected fallback listitem text, got %q", got)
	}

	// Nested list inside first item should be <ul> with 2 <li> items.
	nestedItems := outerItems.Eq(0).Find("ul > li")
	if nestedItems.Length() != 2 {
		htmlStr, _ := outerItems.Eq(0).Html()
		t.Fatalf("expected 2 nested <li> items, got %d; html=%s", nestedItems.Length(), htmlStr)
	}
	if got := strings.TrimSpace(nestedItems.Eq(0).Text()); got != "Inner item" {
		t.Fatalf("expected first nested item %q, got %q", "Inner item", got)
	}
	if got := strings.TrimSpace(nestedItems.Eq(1).Text()); got != "Plain nested item" {
		t.Fatalf("expected second nested item %q, got %q", "Plain nested item", got)
	}

	// Standalone div[role="listitem"] must transform into <li>, not <div class="content">.
	standaloneDoc, err := goquery.NewDocumentFromReader(strings.NewReader(`<html><body><div role="listitem"><div class="content">Solo</div></div></body></html>`))
	if err != nil {
		t.Fatalf("parse standalone html: %v", err)
	}
	res := transformListItemElement(standaloneDoc.Find(`div[role="listitem"]`), standaloneDoc)
	if res == nil || goquery.NodeName(res) != "li" {
		t.Fatalf("expected transformListItemElement to return <li>, got %v", res)
	}
	if strings.TrimSpace(res.Text()) != "Solo" {
		t.Fatalf("expected solo li text %q, got %q", "Solo", strings.TrimSpace(res.Text()))
	}
}

func TestC4_05_FlattenElementPreservesDirectTextWithSingleBlockChild(t *testing.T) {
	// Wrap in <section> so div is not a direct child of root (Case 2) and reaches Case 5.
	input := `<html><body><article><section><div>Intro text<p>Body paragraph</p></div></section></article></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	article := doc.Find("article")
	div := article.Find("div").First()
	changed := flattenElement(div, article, constants.GetBlockElements())
	if changed {
		t.Fatalf("expected flattenElement to skip mixed direct-text + single-block div")
	}
	text := article.Text()
	if !strings.Contains(text, "Intro text") || !strings.Contains(text, "Body paragraph") {
		t.Fatalf("expected both intro text and body paragraph preserved, got %q", text)
	}
}

func TestC4_11_EnsureInlineSpacingNoSpuriousSpaces(t *testing.T) {
	input := `<html><body><article>
		<p><span><strong>Hello</strong><em>,</em> world</span></p>
		<p><span><a href="#fn:1">foo</a><sup>1</sup></span></p>
		<table><tbody><tr><td><span>A</span><span>B</span></td></tr></tbody></table>
		<ul><li><span>Item</span><em>!</em></li></ul>
		<math><mi>x</mi><mo>+</mo><mi>y</mi></math>
	</article></body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	article := doc.Find("article")
	removeEmptyLines(article, doc, false)

	p0, _ := article.Find("p").Eq(0).Html()
	if strings.Contains(p0, "</strong> <em>,</em>") {
		t.Fatalf("spurious space inserted before punctuation element: %s", p0)
	}

	p1, _ := article.Find("p").Eq(1).Html()
	if strings.Contains(p1, "</a> <sup>") {
		t.Fatalf("spurious space inserted before <sup>: %s", p1)
	}

	tdHTML, _ := article.Find("td").First().Html()
	if strings.Contains(tdHTML, "</span> <span>") {
		t.Fatalf("spurious space inserted inside <td>: %s", tdHTML)
	}

	liHTML, _ := article.Find("li").First().Html()
	if strings.Contains(liHTML, "</span> <em>") {
		t.Fatalf("spurious space inserted inside <li>: %s", liHTML)
	}

	mathHTML, _ := article.Find("math").First().Html()
	if strings.Contains(mathHTML, "</mi> <mo>") || strings.Contains(mathHTML, "</mo> <mi>") {
		t.Fatalf("spurious space inserted inside <math>: %s", mathHTML)
	}
}
