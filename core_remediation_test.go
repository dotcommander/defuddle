package defuddle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/extractors"
	"github.com/dotcommander/defuddle/internal/elements"
)

func remediationDoc(t *testing.T, source string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestCoreReactStreamingBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, middle, suffix string
		resolved             bool
	}{
		{"plain", `<p>old fallback</p>`, `<!--/$-->`, true},
		{"nested", `<p>old fallback</p><!--$?--><template></template><p>nested fallback</p><!--/$-->`, `<!--/$-->`, true},
		{"missing close", `<p>old fallback</p>`, "", false},
		{"unclosed nested", `<!--$?--><p>old fallback</p>`, `<!--/$-->`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := remediationDoc(t, `<html><body><!--$?--><template id='B:[]'></template>`+tc.middle+tc.suffix+`<p id="unrelated">unrelated</p><div hidden id='S:[]'><p>resolved article</p></div><script>$RC("B:[]", "S:[]");$RC("B:[]", "S:[]")</script></body></html>`)
			resolveReactStreaming(doc)
			if doc.Find("#unrelated").Length() != 1 {
				t.Fatal("unrelated sibling removed")
			}
			if tc.resolved {
				if strings.Contains(doc.Find("body").Text(), "fallback") || doc.Find("[hidden]").Length() != 0 || strings.Count(doc.Find("body").Text(), "resolved article") != 1 {
					t.Fatal(selectionHTML(doc.Find("body")))
				}
			} else if !strings.Contains(doc.Text(), "old fallback") || doc.Find("[hidden]").Length() != 1 {
				t.Fatal("malformed boundary changed")
			}
		})
	}
	t.Run("missing slot", func(t *testing.T) {
		t.Parallel()
		doc := remediationDoc(t, `<html><body><!--$?--><template id="B:1"></template><p>fallback</p><!--/$--><script>$RC("B:1","absent")</script></body></html>`)
		resolveReactStreaming(doc)
		if doc.Find("template").Length() != 1 || !strings.Contains(doc.Text(), "fallback") {
			t.Fatal("missing slot changed boundary")
		}
	})
}

func TestCoreInspectionLifecycle(t *testing.T) {
	t.Parallel()
	extractors.InitializeBuiltins()
	source := `<html><body><div><template shadowrootmode="open"><meta name="description" content="shadow metadata"><div data-testid="user-message"><div><template shadowrootmode="open"><p>nested shadow content</p></template></div><img src="kept.jpg" width="600" height="600"></div></template></div></body></html>`
	options := &Options{URL: "https://claude.ai/share/core-fixture", RemoveImages: true}
	parser, err := NewDefuddle(source, options)
	if err != nil {
		t.Fatal(err)
	}
	first, err := parser.Parse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.ExtractorType == nil || *first.ExtractorType != "claude" || !strings.Contains(first.Content, "nested shadow content") || first.Description != "Claude conversation with 1 messages" || strings.Contains(first.Content, "<img") {
		t.Fatalf("unexpected extraction: %+v", first)
	}
	// Conversation metadata intentionally overrides Description. MetaTags retain
	// the preprocessed shadow metadata independently of that extractor override.
	assertShadowMetadata := func(result *Result) {
		t.Helper()
		for _, tag := range result.MetaTags {
			if tag.Name != nil && *tag.Name == "description" && tag.Content != nil && *tag.Content == "shadow metadata" {
				return
			}
		}
		t.Fatal("preprocessed shadow metadata missing")
	}
	assertShadowMetadata(first)
	options.RemoveImages = false
	second, err := parser.Parse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertShadowMetadata(second)
	if !strings.Contains(second.Content, "kept.jpg") || !strings.Contains(second.Content, "nested shadow content") {
		t.Fatalf("prior parse mutated inspection source: %s", second.Content)
	}
}

func TestCoreJSONLDCommentStrings(t *testing.T) {
	t.Parallel()
	source := "// first\n{\n // inner\n\"@type\":\"Article\",\"url\":\"https://example.com/a//b\", /* block */\n\"description\":\"literal /* keep */ and \\\"quote\\\" // keep\" // trailing\n}\n"
	parser, err := NewDefuddle("", nil)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(parser.cleanJSONLDContent(source)), &got); err != nil {
		t.Fatal(err)
	}
	if got["url"] != "https://example.com/a//b" || got["description"] != `literal /* keep */ and "quote" // keep` {
		t.Fatal(got)
	}
}

func TestCoreNestedTableOwnership(t *testing.T) {
	t.Parallel()
	got, err := ExtractTables(`<table><tr><td>outer <table><thead><tr><th>inner header</th></tr></thead><tr><td>inner value</td></tr></table></td></tr><tr><th>outer header</th></tr><tr><td>outer value</td></tr></table>`)
	if err != nil {
		t.Fatal(err)
	}
	want := []Table{{Headers: []string{"outer header"}, Rows: [][]string{{"outer inner headerinner value"}, {"outer value"}}}, {Headers: []string{"inner header"}, Rows: [][]string{{"inner value"}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestCoreOptionHeadersAndProcessorBridge(t *testing.T) {
	t.Parallel()
	inherited := http.Header{"X-Test": []string{"inherited"}}
	overridden := http.Header{"X-Test": []string{"override"}}
	parser := &Defuddle{options: &Options{Headers: inherited}}
	merged := parser.mergeOptions(&Options{Headers: overridden})
	merged.Headers.Set("X-Test", "changed")
	if inherited.Get("X-Test") != "inherited" || overridden.Get("X-Test") != "override" {
		t.Fatal("caller headers mutated")
	}
	retained := parser.mergeOptions(&Options{})
	retained.Headers.Set("X-Test", "retained change")
	if inherited.Get("X-Test") != "inherited" {
		t.Fatal("inherited map aliased")
	}
	options := &Options{CodeOptions: &elements.CodeBlockProcessingOptions{}, ImageOptions: &elements.ImageProcessingOptions{}, HeadingOptions: &elements.HeadingProcessingOptions{}, MathOptions: &elements.MathProcessingOptions{}, FootnoteOptions: &elements.FootnoteProcessingOptions{}, RoleOptions: &elements.RoleProcessingOptions{}}
	got := standardizeOptions(options)
	if got.CodeOptions != options.CodeOptions || got.ImageOptions != options.ImageOptions || got.HeadingOptions != options.HeadingOptions || got.MathOptions != options.MathOptions || got.FootnoteOptions != options.FootnoteOptions || got.RoleOptions != options.RoleOptions {
		t.Fatal("processor option dropped")
	}
}

type coreRoundTripper func(*http.Request) (*http.Response, error)

func (f coreRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type coreCancelBody struct{ cancel context.CancelFunc }

func (b coreCancelBody) Read([]byte) (int, error) { b.cancel(); return 0, io.ErrUnexpectedEOF }
func (b coreCancelBody) Close() error             { return nil }

func TestCoreFetchCancellationIdentity(t *testing.T) {
	t.Parallel()
	for _, bodyRead := range []bool{false, true} {
		t.Run(map[bool]string{false: "transport", true: "body"}[bodyRead], func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			client := &http.Client{Transport: coreRoundTripper(func(r *http.Request) (*http.Response, error) {
				if !bodyRead {
					cancel()
					return nil, io.ErrUnexpectedEOF
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: coreCancelBody{cancel}, Request: r}, nil
			})}
			_, err := ParseFromURL(ctx, "https://example.com", &Options{Client: client})
			if !errors.Is(err, context.Canceled) || errors.Is(err, ErrTimeout) {
				t.Fatalf("wrong cancellation identity: %v", err)
			}
		})
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	t.Cleanup(cancel)
	_, err := ParseFromURL(ctx, "https://example.com", nil)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrTimeout) {
		t.Fatalf("wrong deadline identity: %v", err)
	}
	_, err = ParseFromString(ctx, "<article>content</article>", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("parse cancellation lost: %v", err)
	}
}

func TestCoreBatchRedirectBases(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/first" || r.URL.Path == "/second" {
			http.Redirect(w, r, "/final"+r.URL.Path+"/", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<article><p>`+strings.Repeat("article ", 210)+`</p><a href="asset.html">asset</a></article>`)
	}))
	t.Cleanup(server.Close)
	options := &Options{URL: "https://override.example/", Client: server.Client()}
	urls := []string{server.URL + "/first", server.URL + "/second"}
	results := ParseFromURLs(context.Background(), urls, options)
	for i, result := range results {
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		if result.URL != urls[i] || !strings.Contains(result.Result.Content, server.URL+"/final"+strings.TrimPrefix(urls[i], server.URL)+"/asset.html") {
			t.Fatalf("wrong base or identity: %+v", result)
		}
	}
	if options.URL != "https://override.example/" {
		t.Fatal("options mutated")
	}
}

// TestCorePublicProcessorOptions exercises the public parse boundary rather than
// only the standardize adapter. Both parses must leave the source DOM untouched.
func TestCorePublicProcessorOptions(t *testing.T) {
	t.Parallel()
	source := `<html><head><title>Processor settings</title></head><body><article><pre data-lang="go">x</pre><h2>Section<button>Copy</button></h2><img src="https://example.com/a.png" width="75" height="75"><span class="MathJax" data-latex="x">x</span><div role="paragraph">role text</div><p>text<a class="footnote-ref" href="#fn-note">seven</a></p><div id="fn-note">definition</div><p>` + strings.Repeat("article word ", 110) + `</p></article><aside><pre>outside code</pre><div role="paragraph">outside role</div><div id="outside-note">outside definition</div></aside></body></html>`
	base := Options{ContentSelector: "article", ProcessCode: true, ProcessImages: true, ProcessHeadings: true, ProcessMath: true, ProcessFootnotes: true, ProcessRoles: true,
		RemoveExactSelectors: PtrBool(false), RemovePartialSelectors: PtrBool(false), RemoveLowScoring: PtrBool(false), RemoveContentPatterns: PtrBool(false)}
	parse := func(options Options) *goquery.Document {
		t.Helper()
		parser, err := NewDefuddle(source, &options)
		if err != nil {
			t.Fatal(err)
		}
		before := selectionOuterHTML(parser.doc.Selection)
		result, err := parser.Parse(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if after := selectionOuterHTML(parser.doc.Selection); before != after {
			t.Fatal("public parse mutated inspection source or outside content")
		}
		if strings.Contains(result.Content, "outside") {
			t.Fatal("outside-scope content entered selected article")
		}
		return remediationDoc(t, result.Content)
	}
	defaults := parse(base)
	if defaults.Find("pre code").Length() != 1 || defaults.Find("h2 button").Length() != 0 || defaults.Find("img").Length() != 1 || defaults.Find("math").Text() != "x" || defaults.Find(`[role="paragraph"]`).Length() != 0 {
		t.Fatal("nil suboptions changed the existing code, heading, or role defaults")
	}
	custom := base
	custom.CodeOptions = &elements.CodeBlockProcessingOptions{FormatCode: false}
	custom.ImageOptions = &elements.ImageProcessingOptions{RemoveSmallImages: true, MinImageWidth: 100, MinImageHeight: 100}
	custom.HeadingOptions = &elements.HeadingProcessingOptions{RemoveNavigation: false}
	custom.MathOptions = &elements.MathProcessingOptions{ExtractLaTeX: false, ExtractMathML: false}
	custom.RoleOptions = &elements.RoleProcessingOptions{}
	custom.FootnoteOptions = &elements.FootnoteProcessingOptions{}
	doc := parse(custom)
	if doc.Find("pre code").Length() != 0 || doc.Find("img").Length() != 0 || doc.Find("h2 button").Length() != 1 || doc.Find("math").Text() != "" {
		t.Fatal("public code/image/heading/math suboptions were not effective")
	}
	if doc.Find(`[role="paragraph"]`).Length() != 1 {
		t.Fatal("disabled public role stage was undone by cleanup")
	}
	if !strings.Contains(doc.Text(), "seven") {
		t.Fatal("disabled public footnote numbering stage was ignored")
	}
}

func TestCorePublicProtectedScopes(t *testing.T) {
	t.Parallel()
	source := `<html><body><article><h2>Article</h2><div style="display:none"><math><mi>x</mi></math></div><div style="display:none">unrelated hidden clutter</div><pre><code class="advertisement">protected code</code></pre><ol class="footnotes"><li><span class="advertisement">protected footnote</span></li></ol><div class="advertisement">unrelated selector clutter</div><p>` + strings.Repeat("article word ", 110) + `</p></article></body></html>`
	result, err := ParseFromString(context.Background(), source, &Options{ContentSelector: "article"})
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{"protected code", "protected footnote", "<mi>x</mi>"} {
		if !strings.Contains(result.Content, kept) {
			t.Fatalf("protected subtree removed: %s", kept)
		}
	}
	for _, kept := range []string{"unrelated hidden clutter", "unrelated selector clutter"} {
		if !strings.Contains(result.Content, kept) {
			t.Fatalf("explicitly selected content removed: %s", kept)
		}
	}
	result, err = ParseFromString(context.Background(), source, &Options{ContentSelector: "article", RemoveHiddenElements: PtrBool(false), RemovePartialSelectors: PtrBool(false), RemoveLowScoring: PtrBool(false), RemoveContentPatterns: PtrBool(false)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "unrelated hidden clutter") || !strings.Contains(result.Content, "unrelated selector clutter") {
		t.Fatal("explicit removal overrides were ignored")
	}
}

// A bare article confirms Claude's fallback DOM signature but yields no messages.
// Its secondary normalization must not redispatch Claude on the generated article.
func TestCoreConversationNormalizationAvoidsRedispatch(t *testing.T) {
	t.Parallel()
	extractors.InitializeBuiltins()
	options := &Options{URL: "https://claude.ai/share/conversation/"}
	for _, tc := range []struct {
		name, source, description string
	}{
		{"empty fallback", `<article></article>`, "Claude conversation with 0 messages"},
		{"primary message", `<div data-testid="user-message"><p>hello <a href="asset.html">linked message</a></p></div>`, "Claude conversation with 1 messages"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseFromString(context.Background(), tc.source, options)
			if err != nil {
				t.Fatal(err)
			}
			if result.ExtractorType == nil || *result.ExtractorType != "claude" || result.Description != tc.description {
				t.Fatalf("top-level conversation dispatch changed: %+v", result)
			}
			if tc.name == "empty fallback" {
				if result.WordCount != 0 || strings.TrimSpace(remediationDoc(t, result.Content).Text()) != "" {
					t.Fatalf("empty fallback acquired content: %+v", result)
				}
			} else {
				content := remediationDoc(t, result.Content)
				if !strings.Contains(content.Text(), "linked message") || content.Find("a").AttrOr("href", "") != options.URL+"asset.html" {
					t.Fatalf("secondary normalization lost content or URL base: %s", result.Content)
				}
			}
		})
	}
}
