package defuddle

import (
	"context"
	"strings"
	"testing"

	trafilatura "github.com/markusmobius/go-trafilatura/v2"
	"golang.org/x/net/html"
)

func TestEngineSupportedMathFormats(t *testing.T) {
	t.Parallel()
	formats := map[string]string{
		"katex":      `<span class="katex"><span class="katex-mathml"><math><mi>x</mi></math></span><span class="katex-html">x</span></span>`,
		"mwe":        `<span class="mwe-math-element"><math><mi>y</mi></math></span>`,
		"normalized": `<span data-latex="z^2">z squared</span>`,
		"script":     `<script type="math/tex">a^2 + b^2</script>`,
		"mathml":     `<math display="block"><mfrac><mi>a</mi><mi>b</mi></mfrac></math>`,
	}
	for name, fragment := range formats {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, fallback := range []bool{false, true} {
				d, err := NewDefuddle(`<body><article><p>`+strings.Repeat(engineProse, 4)+fragment+`</p><p>`+engineProse+`</p></article></body>`, &Options{Debug: true})
				if err != nil {
					t.Fatal(err)
				}
				if fallback {
					d.engine = func(n *html.Node, o trafilatura.Options) (*trafilatura.ExtractResult, error) {
						o.Config.MinExtractedSize = 1000000
						return trafilatura.ExtractDocument(n, o)
					}
				}
				r, err := d.Parse(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				expected := map[string]string{"katex": "katex-mathml", "mwe": "mwe-math-element", "normalized": "data-latex", "mathml": "mfrac", "script": "a^2 + b^2"}[name]
				if !strings.Contains(r.Content, expected) || strings.Contains(r.Content, "DFR") {
					t.Fatal(r.Content)
				}
				for _, step := range r.DebugInfo.ProcessingSteps {
					if step.Step == "extraction_recovery" {
						t.Fatal("expected upstream extraction: " + step.Details)
					}
				}
			}
		})
	}
}
