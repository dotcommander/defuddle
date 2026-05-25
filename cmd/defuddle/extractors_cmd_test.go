package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/dotcommander/defuddle/extractors"
)

func TestFindMatchingExtractorLabel(t *testing.T) {
	t.Parallel()

	extractors.InitializeBuiltins()
	mappings := extractors.DefaultRegistry.GetMappings()

	tests := []struct {
		name      string
		matchURL  string
		wantMatch bool
		wantPart  string
		wantErr   bool
	}{
		{
			name:      "github issue",
			matchURL:  "https://github.com/dotcommander/defuddle/issues/1",
			wantMatch: true,
			wantPart:  "github.com",
		},
		{
			name:      "generic url does not match catchall",
			matchURL:  "https://example.com/plain",
			wantMatch: false,
		},
		{
			name:      "deceptive github host does not fall through to catchall",
			matchURL:  "https://notgithub.com/dotcommander/defuddle/issues/1",
			wantMatch: false,
		},
		{
			name:     "invalid url",
			matchURL: ":bad-url",
			wantErr:  true,
		},
		{
			name:     "missing host",
			matchURL: "example.com/path",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok, err := findMatchingExtractorLabel(tt.matchURL, mappings)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidMatchURL) {
					t.Fatalf("findMatchingExtractorLabel(%q): want ErrInvalidMatchURL, got %v", tt.matchURL, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("findMatchingExtractorLabel(%q): unexpected error: %v", tt.matchURL, err)
			}
			if ok != tt.wantMatch {
				t.Fatalf("findMatchingExtractorLabel(%q): match=%v, want %v (label=%q)", tt.matchURL, ok, tt.wantMatch, got)
			}
			if tt.wantPart != "" && !strings.Contains(got, tt.wantPart) {
				t.Fatalf("findMatchingExtractorLabel(%q): label %q missing %q", tt.matchURL, got, tt.wantPart)
			}
		})
	}
}

func TestMappingLabelNamesDOMGatedCatchall(t *testing.T) {
	t.Parallel()

	extractors.InitializeBuiltins()
	for _, m := range extractors.DefaultRegistry.GetMappings() {
		if isDOMGatedCatchall(m) {
			got := mappingLabel(m)
			if got != "DOM-gated catchall (Discourse, Mastodon)" {
				t.Fatalf("mappingLabel(catchall): got %q", got)
			}
			return
		}
	}
	t.Fatalf("built-in mappings did not include DOM-gated catchall")
}
