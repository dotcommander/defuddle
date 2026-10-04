package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrontmatter(t *testing.T) {
	t.Parallel()
	meta, _ := json.Marshal(map[string]string{"url": "https://example.com/article"})
	for _, prefix := range []string{"", "\ufeff", "<!DOCTYPE html>\n"} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()
			input := prefix + "<!-- " + string(meta) + " -->\n<html><body>hi</body></html>"
			got, u, err := stripFrontmatter(input)
			if err != nil || u != "https://example.com/article" || strings.Contains(got, "<!--") {
				t.Fatalf("got %q %q %v", got, u, err)
			}
		})
	}
	if _, _, err := stripFrontmatter("<!-- {invalid} --><html>"); err == nil {
		t.Fatal("malformed metadata accepted")
	}
}

func TestUnterminatedFrontmatter(t *testing.T) {
	t.Parallel()
	meta, _ := json.Marshal(map[string]string{"url": "https://example.com/article"})
	for _, prefix := range []string{"", "<!DOCTYPE html>\n"} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()
			input := prefix + "<!-- " + string(meta) + "\n<html><body>article</body></html>"
			f := fixture{ID: "unterminated", HTMLPath: filepath.Join(t.TempDir(), "page.html"), URL: "https://example.com/explicit"}
			if err := os.WriteFile(f.HTMLPath, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := prepareInput(f); err == nil {
				t.Fatal("explicit URL bypassed unterminated frontmatter")
			}
		})
	}
	for _, input := range []string{"<!-- ordinary comment --><html>", "<!-- ordinary unterminated comment"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			got, u, err := stripFrontmatter(input)
			if err != nil || got != input || u != "" {
				t.Fatalf("ordinary comment changed: %q %q %v", got, u, err)
			}
		})
	}
}

func TestManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	data, _ := json.Marshal([]fixture{{ID: "one", HTMLPath: "one.html", ReferenceText: "gold.txt"}})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := loadManifest(path)
	if err != nil || len(got) != 1 || got[0].HTMLPath != filepath.Join(dir, "one.html") || got[0].ReferenceText != filepath.Join(dir, "gold.txt") {
		t.Fatalf("got %v %v", got, err)
	}
	data, _ = json.Marshal([]fixture{{ID: "one", HTMLPath: "one.html"}, {ID: "one", HTMLPath: "two.html"}})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(path); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestInputErrorsAndURLPrecedence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := fixture{ID: "test", HTMLPath: filepath.Join(dir, "missing.html")}
	if _, _, err := prepareInput(f); err == nil {
		t.Fatal("missing input accepted")
	}
	f.HTMLPath = filepath.Join(dir, "page.html")
	if err := os.WriteFile(f.HTMLPath, []byte(`<html><head><link rel="canonical" href="https://example.com/canonical"></head></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	_, u, err := prepareInput(f)
	if err != nil || u.String() != "https://example.com/canonical" {
		t.Fatalf("canonical %v %v", u, err)
	}
	f.URL = "https://example.com/override"
	_, u, err = prepareInput(f)
	if err != nil || u.String() != f.URL {
		t.Fatalf("override %v %v", u, err)
	}
	f.URL = "file:///local"
	if _, _, err := prepareInput(f); err == nil {
		t.Fatal("non-HTTP URL accepted")
	}
	if err := os.WriteFile(f.HTMLPath, make([]byte, maxInputBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readBounded(f.HTMLPath); err == nil {
		t.Fatal("oversized file accepted")
	}
}

func TestTextAndAgreement(t *testing.T) {
	t.Parallel()
	got, s, err := normalizeHTML(`<h1>Hello <em>world</em></h1><p>A &amp; B</p><script>hidden</script><pre>code</pre>`)
	if err != nil || got != "Hello world A & B code" || s.Headings != 1 || s.CodeBlocks != 1 {
		t.Fatalf("got %q %+v %v", got, s, err)
	}
	if got := jaccard("A a b", "b c"); got != 1.0/3 {
		t.Fatalf("jaccard %v", got)
	}
	if jaccard("", "") != 1 {
		t.Fatal("empty set agreement")
	}
}

func TestAdapterContinuesAfterError(t *testing.T) {
	t.Parallel()
	request, _ := json.Marshal(fixture{ID: "missing", HTMLPath: filepath.Join(t.TempDir(), "missing.html")})
	var out bytes.Buffer
	if err := runAdapter(t.Context(), "defuddle", strings.NewReader(string(request)+"\n"+string(request)+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(&out)
	for range 2 {
		var result adapterResult
		if err := dec.Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.ID != "missing" || result.Text != "" || result.Error == "" {
			t.Fatalf("got %+v", result)
		}
	}
}

func TestAdapterRejectsTruncatedJSON(t *testing.T) {
	t.Parallel()
	request, _ := json.Marshal(map[string]string{"id": "truncated"})
	truncated := string(request[:len(request)-1])
	for _, newline := range []string{"", "\n"} {
		t.Run(newline, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			if err := runAdapter(t.Context(), "defuddle", strings.NewReader(truncated+newline), &out); err == nil {
				t.Fatal("truncated protocol request accepted")
			}
			if out.Len() != 0 {
				t.Fatalf("truncated request emitted response: %s", out.String())
			}
		})
	}
}

func TestAdapterContinuesAfterCompleteMalformedJSON(t *testing.T) {
	t.Parallel()
	request, _ := json.Marshal(fixture{ID: "missing", HTMLPath: filepath.Join(t.TempDir(), "missing.html")})
	var out bytes.Buffer
	if err := runAdapter(t.Context(), "defuddle", strings.NewReader("{invalid}\n"+string(request)+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(&out)
	for i := range 2 {
		var result adapterResult
		if err := dec.Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.Text != "" || result.Error == "" || (i == 1 && result.ID != "missing") {
			t.Fatalf("got %+v", result)
		}
	}
}

func TestSavedFixtureEngines(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("saved-fixture integration")
	}
	f := fixture{ID: "blog", HTMLPath: "../../.reference/defuddle/tests/fixtures/general--stephango.com-buy-wisely.html"}
	if _, err := os.Stat("../../.reference/defuddle/tests/fixtures"); os.IsNotExist(err) {
		t.Skip("optional reference checkout absent")
	}
	input, u, err := prepareInput(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, engine := range []string{"defuddle", "trafilatura", "readability"} {
		t.Run(engine, func(t *testing.T) {
			t.Parallel()
			r, err := extract(context.Background(), engine, input, u)
			if err != nil {
				t.Fatal(err)
			}
			text, _, err := normalizeHTML(r.HTML)
			if err != nil || !strings.Contains(strings.ToLower(text), "buy") {
				t.Fatalf("missing article text: %q %v", text, err)
			}
		})
	}
}
