// Package main: `defuddle extractors` subcommand — lists registered
// site-specific extractor mappings or matches a single URL when --match is set.
package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/dotcommander/defuddle/extractors"
)

type ExtractorsOptions struct {
	Match string `help:"Show which extractor matches the given URL."`
}

func (opts *ExtractorsOptions) Run() error {
	extractors.InitializeBuiltins()
	matchURL := opts.Match
	mappings := extractors.DefaultRegistry.GetMappings()

	if matchURL != "" {
		label, ok, err := findMatchingExtractorLabel(matchURL, mappings)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("MATCH:", label)
		} else {
			fmt.Fprintln(os.Stderr, "no URL-specific extractor matches the given URL")
		}
		return nil
	}

	for _, m := range mappings {
		fmt.Println(mappingLabel(m))
	}
	return nil
}

// mappingLabel returns a human-readable string listing the patterns for an extractor mapping.
func mappingLabel(m extractors.ExtractorMapping) string {
	if isDOMGatedCatchall(m) {
		return "DOM-gated catchall (Discourse, Mastodon)"
	}

	patterns := make([]string, 0, len(m.Patterns))
	for _, p := range m.Patterns {
		switch v := p.(type) {
		case string:
			patterns = append(patterns, v)
		case *regexp.Regexp:
			patterns = append(patterns, v.String())
		}
	}
	return strings.Join(patterns, ", ")
}

func findMatchingExtractorLabel(matchURL string, mappings []extractors.ExtractorMapping) (string, bool, error) {
	parsed, err := url.Parse(matchURL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return "", false, fmt.Errorf("%w: %q", ErrInvalidMatchURL, matchURL)
	}

	for _, m := range mappings {
		if isDOMGatedCatchall(m) {
			continue
		}
		if extractors.DefaultRegistry.MatchesURL(matchURL, m) {
			return mappingLabel(m), true, nil
		}
	}
	return "", false, nil
}

func isDOMGatedCatchall(m extractors.ExtractorMapping) bool {
	if len(m.Patterns) != 1 {
		return false
	}
	re, ok := m.Patterns[0].(*regexp.Regexp)
	return ok && re.String() == ".*"
}
