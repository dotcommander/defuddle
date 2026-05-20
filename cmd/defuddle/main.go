// Package main provides the defuddle CLI application.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/dotcommander/defuddle/extractors"
	"github.com/spf13/cobra"
)

// Build-injected via ldflags (goreleaser, go build -ldflags "-X main.version=...")
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Define static errors to avoid dynamic error creation
var (
	ErrInvalidHeaderFormat = fmt.Errorf("invalid header format (expected 'Key: Value')")
	ErrDirectoryTraversal  = fmt.Errorf("invalid file path: directory traversal detected")
	ErrNoURLs              = errors.New("no URLs provided")
	ErrPropertyNotFound    = fmt.Errorf("property not found in response")
)

var rootCmd = &cobra.Command{
	Use:     "defuddle",
	Short:   "Extract and structure content from web pages",
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	Long: `defuddle is a CLI tool for extracting and structuring content from web pages.
It can parse HTML, extract metadata, and convert content to various formats.`,
}

func init() {
	// Initialize built-in extractors
	extractors.InitializeBuiltins()

	registerParseCmd()
	registerExtractorsCmd()
	registerBatchCmd()

	rootCmd.AddCommand(parseCmd)
	rootCmd.AddCommand(extractorsCmd)
	rootCmd.AddCommand(batchCmd)
}

func main() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
