package defuddle

import (
	"context"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/metadata"
)

func TestSemanticArticleRecovery(t *testing.T) {
	t.Parallel()
	paragraph := strings.Repeat("A useful article explains the subject in detail. ", 8)
	cases := []struct {
		name, html  string
		wantArticle bool
		wantOutside bool
		wantHeading bool
	}{
		{"noisy", `<html><head><title>Page title</title></head><body><header>Site header</header><nav>Navigation links</nav><article><h1>Article heading</h1><p>` + paragraph + `</p></article><aside>Sidebar noise</aside><footer>Footer noise</footer></body></html>`, true, false, false},
		{"short shell", `<html><body><div id="app"><article><h1>Static fallback</h1><p>This server response contains enough content for Defuddle to extract a useful fallback document even though the page advertises a JavaScript application shell.</p></article></div><script src="/app.js"></script></body></html>`, true, false, true},
		{"multiple articles", `<html><body><article><h1>First heading</h1><p>` + paragraph + `</p></article><article><h1>Second heading</h1><p>` + paragraph + `</p></article></body></html>`, false, false, false},
		{"nested sidebar", `<html><body><article><h1>Article heading</h1><p>` + paragraph + `</p><aside>Inside noise</aside></article></body></html>`, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := ParseFromString(t.Context(), tc.html, &Options{URL: "https://example.com/page", Markdown: true, Debug: true})
			if err != nil {
				t.Fatal(err)
			}
			if result.ContentMarkdown == nil {
				t.Fatal("expected Markdown")
			}
			chosen := false
			for _, step := range result.DebugInfo.ProcessingSteps {
				chosen = chosen || step.Step == "semantic_article_recovery"
			}
			if chosen != tc.wantArticle {
				t.Fatalf("article recovery = %t, want %t; content = %q", chosen, tc.wantArticle, *result.ContentMarkdown)
			}
			if tc.wantArticle && strings.Contains(*result.ContentMarkdown, "Sidebar noise") != tc.wantOutside {
				t.Fatalf("clutter retained: %q", *result.ContentMarkdown)
			}
			if tc.wantHeading && !strings.Contains(*result.ContentMarkdown, "## Static fallback") {
				t.Fatalf("heading missing: %q", *result.ContentMarkdown)
			}
		})
	}
}

func TestArticleRecoveryRejectsUnaccountedText(t *testing.T) {
	t.Parallel()
	for _, extra := range []string{`<div class="advertisement">Promotion</div>`, `<span hidden>Hidden text</span>`} {
		t.Run(extra, func(t *testing.T) {
			t.Parallel()
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<article><h1>Heading</h1><p>Article body.</p>` + extra + `</article>`))
			if err != nil {
				t.Fatal(err)
			}
			extracted, err := goquery.NewDocumentFromReader(strings.NewReader(`<p>Article body.</p>`))
			if err != nil {
				t.Fatal(err)
			}
			_, recovered := recoverSemanticArticle(doc.Find("article"), extracted.Find("body"), &metadata.Metadata{}, "", doc, &Options{})
			if recovered {
				t.Fatal("recovery reintroduced text absent from extracted paragraphs")
			}
		})
	}
}

func TestArticleRecoveryLeavesExplicitSelectorUntouched(t *testing.T) {
	t.Parallel()
	result, err := ParseFromString(context.Background(), `<html><body><article><h1>Heading</h1><p>Body text.</p></article></body></html>`, &Options{ContentSelector: "article", Debug: true, Markdown: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.DebugInfo.ProcessingSteps {
		if step.Step == "semantic_article_recovery" {
			t.Fatal("explicit selector passed through recovery")
		}
	}
}
