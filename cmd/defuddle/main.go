// Package main provides the defuddle CLI application.
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/dotcommander/defuddle/extractors"
	"github.com/spf13/cobra"
)

// Build-injected via ldflags (goreleaser, go build -ldflags "-X main.version=...")
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func resolvedVersion() string {
	if version != "dev" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return version
}

// Define static errors to avoid dynamic error creation
var (
	ErrInvalidHeaderFormat = fmt.Errorf("invalid header format (expected 'Key: Value')")
	ErrDirectoryTraversal  = fmt.Errorf("invalid file path: directory traversal detected")
	ErrNoURLs              = errors.New("no URLs provided")
	ErrPropertyNotFound    = fmt.Errorf("property not found in response")
	ErrHTTPClientBuild     = errors.New("failed to construct HTTP client")
	ErrParseUsage          = errors.New("usage: defuddle parse <url|file> (or pipe HTML via stdin)")
	ErrInvalidMatchURL     = errors.New("invalid match URL")
	ErrInvalidConcurrency  = errors.New("concurrency must be at least 1")
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "defuddle",
		Short:   "Extract and structure content from web pages",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", resolvedVersion(), commit, date),
		Long: `defuddle is a CLI tool for extracting and structuring content from web pages.
It can parse HTML, extract metadata, and convert content to various formats.`,
	}
	cmd.AddCommand(newParseCmd(), newExtractorsCmd(), newBatchCmd())
	return cmd
}

func main() {
	extractors.InitializeBuiltins()
	rootCmd := newRootCmd()
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
