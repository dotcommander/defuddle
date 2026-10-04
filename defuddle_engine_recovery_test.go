package defuddle

import (
	"context"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	trafilatura "github.com/markusmobius/go-trafilatura/v2"
	"golang.org/x/net/html"
)

func TestEngineNativeFallbackRich(t *testing.T) {
	t.Parallel()
	d, err := NewDefuddle(`<body><article><p>`+strings.Repeat(engineProse, 3)+`</p><pre><code class="language-python">if ready:
    run()
</code></pre><p>Formula <math><mi>x</mi></math> follows the explanation.</p></article></body>`, &Options{Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	// Force the upstream's native fallback decision with a fresh configuration.
	d.engine = func(n *html.Node, o trafilatura.Options) (*trafilatura.ExtractResult, error) {
		o.Config.MinExtractedSize = 1000000
		return trafilatura.ExtractDocument(n, o)
	}
	r, err := d.Parse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Content, "    run()") || !strings.Contains(r.Content, "language-python") || !strings.Contains(r.Content, "<math") || strings.Contains(r.Content, "DFR") {
		t.Fatal(r.Content)
	}
	for _, step := range r.DebugInfo.ProcessingSteps {
		if step.Step == "extraction_recovery" {
			t.Fatal("native fallback unexpectedly needed facade recovery: " + step.Details)
		}
	}
}

func TestEngineMalformedCandidateRecovers(t *testing.T) {
	t.Parallel()
	d, err := NewDefuddle(`<body><p>Marker-free recovery body</p><pre><code>selected code</code></pre></body>`, &Options{Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	d.engine = func(n *html.Node, _ trafilatura.Options) (*trafilatura.ExtractResult, error) {
		doc := goquery.NewDocumentFromNode(n)
		text := doc.Find("body").Text()
		start := strings.Index(text, "DFR")
		if start < 0 {
			t.Fatal("missing preservation projection")
		}
		// Truncate in the middle of an occurrence, as malformed extraction could.
		return &trafilatura.ExtractResult{ContentNode: &html.Node{Type: html.TextNode, Data: text[:start+40]}}, nil
	}
	r, err := d.Parse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Content, "Marker-free recovery body") || !strings.Contains(r.Content, "selected code") || strings.Contains(r.Content, "DFR") {
		t.Fatal(r.Content)
	}
	found := false
	for _, step := range r.DebugInfo.ProcessingSteps {
		if step.Step == "extraction_recovery" && strings.Contains(step.Details, "rich-content") {
			found = true
		}
	}
	if !found {
		t.Fatal("missing malformed rich candidate recovery")
	}
}

func TestEngineDebugOutputParity(t *testing.T) {
	t.Parallel()
	source := `<article><div><p>` + strings.Repeat(engineProse, 3) + `<code>value</code></p></div></article>`
	plain := engineParse(t, source, nil)
	debug := engineParse(t, source, &Options{Debug: true})
	if plain.Content != debug.Content {
		t.Fatal("debug changed extracted output")
	}
}

func TestEngineAmbiguousFootnoteIDs(t *testing.T) {
	t.Parallel()
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(`<body><p>reference <a href="#fn1">1</a></p><ol class="footnotes"><li id="fn1">first definition</li><li id="fn1">ambiguous definition</li></ol></body>`))
	m, err := prepareRichContent(doc, doc.Find("body"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.definitions) != 0 {
		t.Fatal("invented ambiguous definition relationship")
	}
}

func TestEngineEnabledFootnotesKeepAmbiguity(t *testing.T) {
	t.Parallel()
	source := `<body><article><p>` + strings.Repeat(engineProse, 4) + `<sup class="reference"><a href="#cite_note-x">1</a></sup></p><ol class="references"><li id="cite_note-x">first definition</li><li id="cite_note-x">ambiguous definition</li></ol></article></body>`
	d, err := NewDefuddle(source, &Options{ProcessFootnotes: true})
	if err != nil {
		t.Fatal(err)
	}
	d.engine = func(n *html.Node, o trafilatura.Options) (*trafilatura.ExtractResult, error) {
		prepared := goquery.NewDocumentFromNode(n)
		if prepared.Find(`[id="cite_note-x"]`).Length() != 2 || prepared.Find(`a[href$="#cite_note-x"]`).Length() != 1 {
			t.Fatal("normalization erased original ambiguity")
		}
		return trafilatura.ExtractDocument(n, o)
	}
	r, err := d.Parse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Content, `id="fn:1"`) || strings.Contains(r.Content, "DFR") {
		t.Fatal("invented footnote: " + r.Content)
	}
}

func TestEngineEnabledFootnotesKeepRichDefinition(t *testing.T) {
	t.Parallel()
	source := `<body><article><p>` + strings.Repeat(engineProse, 4) + `<sup class="reference"><a href="#cite_note-x">1</a></sup></p><ol class="references"><li id="cite_note-x"><p>Rich definition prose</p><pre><code class="language-go">    rich_definition_code()</code></pre><math display="block"><mi>z</mi></math></li></ol></article></body>`
	r := engineParse(t, source, &Options{ProcessFootnotes: true, Debug: true})
	for _, want := range []string{"Rich definition prose", "    rich_definition_code()", "language-go", "<math"} {
		if !strings.Contains(r.Content, want) {
			t.Fatalf("missing %q: %s", want, r.Content)
		}
	}
	if strings.Count(r.Content, "Rich definition prose") != 1 || strings.Contains(r.Content, "DFR") {
		t.Fatal(r.Content)
	}
	for _, step := range r.DebugInfo.ProcessingSteps {
		if step.Step == "extraction_recovery" {
			t.Fatal(step.Details)
		}
	}
}
