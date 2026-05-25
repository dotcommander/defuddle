package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/dotcommander/defuddle"
)

func TestGetPropertyAcceptsDocumentedCamelCaseNames(t *testing.T) {
	t.Parallel()

	markdown := "markdown content"
	extractorType := "github"
	result := &defuddle.Result{
		Metadata: defuddle.Metadata{
			Title:     "Example Title",
			WordCount: 42,
			ParseTime: 17,
		},
		ContentMarkdown: &markdown,
		ExtractorType:   &extractorType,
	}

	tests := []struct {
		name string
		want string
	}{
		{name: "title", want: "Example Title"},
		{name: "wordCount", want: "42"},
		{name: "parseTime", want: "17"},
		{name: "contentMarkdown", want: markdown},
		{name: "extractorType", want: extractorType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := getProperty(result, tt.name)
			if !ok {
				t.Fatalf("getProperty(%q): expected property to be found", tt.name)
			}
			if got != tt.want {
				t.Fatalf("getProperty(%q): got %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestKnownPropertiesUsesDocumentedDisplayNames(t *testing.T) {
	t.Parallel()

	for _, want := range []string{"content", "wordCount", "parseTime", "metaTags", "schemaOrgData", "extractorType", "contentMarkdown"} {
		if !slices.Contains(knownProperties, want) {
			t.Fatalf("knownProperties missing %q: %s", want, strings.Join(knownProperties, ", "))
		}
	}
	for _, unexpected := range []string{"wordcount", "parsetime", "metatags", "schemaorgdata", "extractortype", "contentmarkdown"} {
		if slices.Contains(knownProperties, unexpected) {
			t.Fatalf("knownProperties contains internal key %q: %s", unexpected, strings.Join(knownProperties, ", "))
		}
	}
}
