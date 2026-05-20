// Package main: `defuddle extractors` subcommand — lists registered
// site-specific extractor mappings or matches a single URL when --match is set.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/dotcommander/defuddle/extractors"
	"github.com/spf13/cobra"
)

var extractorsCmd = &cobra.Command{
	Use:   "extractors",
	Short: "List registered site-specific extractors",
	RunE: func(cmd *cobra.Command, _ []string) error {
		matchURL, _ := cmd.Flags().GetString("match")
		mappings := extractors.DefaultRegistry.GetMappings()

		for _, m := range mappings {
			if matchURL != "" {
				if !extractors.DefaultRegistry.MatchesURL(matchURL, m) {
					continue
				}
				fmt.Println("MATCH:", mappingLabel(m))
				return nil
			}
			fmt.Println(mappingLabel(m))
		}

		if matchURL != "" {
			fmt.Fprintln(os.Stderr, "no extractor matches the given URL")
		}
		return nil
	},
}

// registerExtractorsCmd attaches extractors-mode flags. Called from init() in main.go.
func registerExtractorsCmd() {
	extractorsCmd.Flags().String("match", "", "Show which extractor matches the given URL")
}

// mappingLabel returns a human-readable string listing the patterns for an extractor mapping.
func mappingLabel(m extractors.ExtractorMapping) string {
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
