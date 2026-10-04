package defuddle

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	trafilatura "github.com/markusmobius/go-trafilatura/v2"
	"golang.org/x/net/html"
)

const engineProse = "The research article describes a careful experiment and explains its findings with useful evidence. It gives readers enough background to understand the result and follow the argument. "

var errEngineUpstream = errors.New("upstream failed")

func engineParse(t *testing.T, source string, o *Options) *Result {
	t.Helper()
	d, err := NewDefuddle(source, o)
	if err != nil {
		t.Fatal(err)
	}
	r, err := d.Parse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestEngineRemovalCompatibility(t *testing.T) {
	t.Parallel()
	source := `<body><article><p>` + strings.Repeat(engineProse, 6) + `</p></article><nav><a href="/nav">navigation unrelated</a></nav></body>`
	def := engineParse(t, source, nil)
	partial := engineParse(t, source, &Options{RemoveExactSelectors: PtrBool(false), RemoveHiddenElements: PtrBool(false)})
	if def.Content != partial.Content {
		t.Fatal("partial removal switches changed extraction")
	}
	bypass := engineParse(t, source, &Options{RemoveExactSelectors: PtrBool(false), RemovePartialSelectors: PtrBool(false), RemoveHiddenElements: PtrBool(false), RemoveLowScoring: PtrBool(false), RemoveContentPatterns: PtrBool(false)})
	if !strings.Contains(bypass.Content, "navigation unrelated") {
		t.Fatal("all-false bypass discarded body")
	}
}

func TestEngineSelectorPrecedenceAndMiss(t *testing.T) {
	t.Parallel()
	source := `<body><article><p>` + strings.Repeat(engineProse, 5) + `</p></article><div class="pick"><code>first choice</code></div><div class="pick">second choice</div></body>`
	r := engineParse(t, source, &Options{ContentSelector: ".pick", Markdown: true})
	if !strings.Contains(r.Content, "first choice") || strings.Contains(r.Content, "second choice") || strings.Contains(r.Content, "research article") || r.ContentMarkdown == nil {
		t.Fatal(r.Content)
	}
	miss := engineParse(t, source, &Options{ContentSelector: ".missing"})
	if !strings.Contains(miss.Content, "research article") {
		t.Fatal(miss.Content)
	}
}

func TestEngineRichActualExtraction(t *testing.T) {
	t.Parallel()
	for _, normalize := range []bool{false, true} {
		t.Run(map[bool]string{false: "preservation", true: "normalization"}[normalize], func(t *testing.T) {
			t.Parallel()
			source := `<body><article><p>` + strings.Repeat(engineProse, 4) + `Inline <code>tab\tvalue</code> and formula <math><mi>x</mi><mo>+</mo><mn>1</mn></math>.</p><pre><code class="language-go">func main() {
    println("hello")
}
</code></pre><p>First reference<sup><a href="#fn1">1</a></sup> and second reference<sup><a href="#fn1">1</a></sup>.</p><ol class="footnotes"><li id="fn1">Rich note <code>note_code</code> <math><mi>y</mi></math></li></ol><p>` + engineProse + `</p></article><nav><pre><code>UNSELECTED_NAV_CODE</code></pre></nav></body>`
			r := engineParse(t, source, &Options{ProcessCode: normalize, ProcessMath: normalize, ProcessFootnotes: normalize, Markdown: true, Debug: true})
			if strings.Contains(r.Content, "DFR") {
				t.Fatal("marker leaked")
			}
			for _, want := range []string{"language-go", "    println", "<mi>x</mi>", "<mo>+</mo>", "<mn>1</mn>", "note_code", "<mi>y</mi>"} {
				if !strings.Contains(r.Content, want) {
					t.Fatalf("missing %q: %s", want, r.Content)
				}
			}
			if strings.Count(r.Content, "Rich note") != 1 {
				t.Fatal("definition duplicated or lost: " + r.Content)
			}
			if strings.Contains(r.Content, "UNSELECTED_NAV_CODE") {
				t.Fatal("unselected rich fragment restored")
			}
			recovered := false
			for _, step := range r.DebugInfo.ProcessingSteps {
				if step.Step == "extraction_recovery" {
					recovered = true
				}
			}
			if recovered {
				t.Fatal("expected actual extraction, got recovery")
			}
		})
	}
}

func TestEngineRecoveryBoundaries(t *testing.T) {
	t.Parallel()
	cases := map[string]extractionEngine{
		"error": func(*html.Node, trafilatura.Options) (*trafilatura.ExtractResult, error) {
			return nil, errEngineUpstream
		},
		"panic": func(*html.Node, trafilatura.Options) (*trafilatura.ExtractResult, error) { panic("upstream panic") },
		"nil":   func(*html.Node, trafilatura.Options) (*trafilatura.ExtractResult, error) { return nil, nil },
		"nil-node": func(*html.Node, trafilatura.Options) (*trafilatura.ExtractResult, error) {
			return &trafilatura.ExtractResult{}, nil
		},
		"empty": func(*html.Node, trafilatura.Options) (*trafilatura.ExtractResult, error) {
			return &trafilatura.ExtractResult{ContentNode: &html.Node{Type: html.ElementNode, Data: "div"}}, nil
		},
	}
	for name, engine := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d, err := NewDefuddle(`<body><p>Fallback 中文正文 <code>kept</code><math><mi>x</mi></math><img src="/image.jpg"></p><script>alert(1)</script></body>`, &Options{Debug: true, Markdown: true, URL: "https://example.org/page", RemoveImages: true})
			if err != nil {
				t.Fatal(err)
			}
			d.engine = engine
			r, err := d.Parse(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(r.Content, "Fallback") || strings.Contains(r.Content, "DFR") || strings.Contains(r.Content, "script") || strings.Contains(r.Content, "<img") || r.ContentMarkdown == nil {
				t.Fatal(r.Content)
			}
			recovered := false
			for _, step := range r.DebugInfo.ProcessingSteps {
				if step.Step == "extraction_recovery" {
					recovered = true
				}
			}
			if !recovered {
				t.Fatal("missing recovery diagnostic")
			}
		})
	}
}

func TestEngineMalformedMarkers(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"incomplete", "duplicate", "crossing", "unknown", "unknown-inside", "malformed-suffix", "reversed"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			doc, _ := goquery.NewDocumentFromReader(strings.NewReader(`<body><pre><code>one</code></pre><pre><code>two</code></pre></body>`))
			m, err := prepareRichContent(doc, doc.Find("body"))
			if err != nil {
				t.Fatal(err)
			}
			a, b := m.occurrences[0], m.occurrences[1]
			output := ""
			switch name {
			case "incomplete":
				output = a.begin
			case "duplicate":
				output = a.begin + a.end + a.begin + a.end
			case "crossing":
				output = a.begin + b.begin + a.end + b.end
			case "unknown":
				output = m.prefix + "999B"
			case "unknown-inside":
				output = a.begin + " " + m.prefix + "999B " + a.end
			case "malformed-suffix":
				output = strings.TrimSuffix(a.begin, ":") + "oops: " + a.end
			case "reversed":
				output = a.end + a.begin
			}
			if _, err := m.restore(&html.Node{Type: html.TextNode, Data: output}); err == nil {
				t.Fatal("accepted malformed marker")
			}
		})
	}
}

func TestEngineMarkersAdjacentToProse(t *testing.T) {
	t.Parallel()
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(`<body><code>kept code</code></body>`))
	m, err := prepareRichContent(doc, doc.Find("body"))
	if err != nil {
		t.Fatal(err)
	}
	o := m.occurrences[0]
	content, err := m.restore(&html.Node{Type: html.TextNode, Data: "before" + o.begin + "kept code" + o.end + "Formula"})
	if err != nil {
		t.Fatal(err)
	}
	if content.Find("code").Text() != "kept code" || !strings.Contains(content.Text(), "Formula") || strings.Contains(content.Text(), m.prefix) {
		t.Fatal(content.Text())
	}
}
