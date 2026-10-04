package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func panicFixtures(t *testing.T) []fixture {
	t.Helper()
	dir := t.TempDir()
	fixtures := make([]fixture, 0, 3)
	for _, id := range []string{"before", "panic", "after"} {
		f := fixture{ID: id, HTMLPath: filepath.Join(dir, id+".html"), URL: "https://example.com/" + id}
		if err := os.WriteFile(f.HTMLPath, []byte("<html><body><p>"+id+"</p></body></html>"), 0600); err != nil {
			t.Fatal(err)
		}
		fixtures = append(fixtures, f)
	}
	return fixtures
}

func injectedExtractor(_ context.Context, engine, _ string, pageURL *url.URL) (extraction, error) {
	if pageURL.Path == "/panic" && engine == "defuddle" {
		panic("injected inconsistent node")
	}
	return extraction{HTML: "<p>success " + pageURL.Path + "</p>", Metadata: metadata{Title: engine, Authors: []string{}}}, nil
}

func TestAdapterContainsEnginePanic(t *testing.T) {
	t.Parallel()
	fixtures := panicFixtures(t)
	var requests, output bytes.Buffer
	for _, f := range fixtures {
		if err := json.NewEncoder(&requests).Encode(f); err != nil {
			t.Fatal(err)
		}
	}
	if err := runAdapterWithExtractor(t.Context(), "defuddle", &requests, &output, injectedExtractor); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(&output)
	for _, f := range fixtures {
		var result adapterResult
		if err := dec.Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.ID != f.ID {
			t.Fatalf("identity lost: %+v", result)
		}
		if f.ID == "panic" {
			if result.Text != "" || result.Metadata.Title != "" || !strings.Contains(result.Error, `fixture "panic" engine "defuddle"`) || !strings.Contains(result.Error, "injected inconsistent node") {
				t.Fatalf("panic was not recorded as an empty failure: %+v", result)
			}
		} else if result.Error != "" || !strings.Contains(result.Text, f.ID) {
			t.Fatalf("healthy request failed: %+v", result)
		}
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		t.Fatalf("extra response: %v", err)
	}
}

func TestReportContainsEnginePanic(t *testing.T) {
	t.Parallel()
	fixtures := panicFixtures(t)
	var output bytes.Buffer
	err := writeReportWithExtractor(t.Context(), fixtures, &output, injectedExtractor)
	if err == nil || !strings.Contains(err.Error(), "injected inconsistent node") {
		t.Fatalf("aggregate failure missing: %v", err)
	}
	dec := json.NewDecoder(&output)
	for _, f := range fixtures {
		var report fixtureReport
		if err := dec.Decode(&report); err != nil {
			t.Fatal(err)
		}
		if report.ID != f.ID || len(report.Results) != 3 {
			t.Fatalf("result denominator changed: %+v", report)
		}
		for _, result := range report.Results {
			if f.ID == "panic" && result.Engine == "defuddle" {
				if result.Error == "" || result.Text != "" || result.HTML != "" || result.TextBytes != 0 || result.HTMLBytes != 0 {
					t.Fatalf("partial failed result retained: %+v", result)
				}
			} else if result.Error != "" || !strings.Contains(result.Text, f.ID) {
				t.Fatalf("peer engine or later fixture failed: %+v", result)
			}
		}
		wantAgreement := 3
		if f.ID == "panic" {
			wantAgreement = 1
		}
		if len(report.Agreement) != wantAgreement {
			t.Fatalf("failed engine included in agreement: %+v", report.Agreement)
		}
	}
}
