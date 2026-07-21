// Package main provides the defuddle CLI application.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/alecthomas/kong"
	"github.com/dotcommander/defuddle/extractors"
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
	ErrParseUsage          = errors.New("usage: defuddle parse <url|file> (or pipe HTML via stdin)")
	ErrInvalidMatchURL     = errors.New("invalid match URL")
	ErrInvalidConcurrency  = errors.New("concurrency must be at least 1")
	ErrCLIUsage            = errors.New("invalid command line")
)

type CLI struct {
	Parse      ParseOptions      `cmd:"" aliases:"p" help:"Parse and extract content from a URL, HTML file, or stdin."`
	Extractors ExtractorsOptions `cmd:"" help:"List registered site-specific extractors."`
	Batch      BatchOptions      `cmd:"" help:"Parse multiple URLs, output JSONL."`
	Version    kong.VersionFlag  `name:"version" help:"Print version information and quit."`
}

func newParser(cli *CLI, stdout, stderr io.Writer) (*kong.Kong, error) {
	return kong.New(cli,
		kong.Name("defuddle"),
		kong.Description("Extract and structure content from web pages."),
		kong.Vars{"version": fmt.Sprintf("%s (commit: %s, built: %s)", resolvedVersion(), commit, date)},
		kong.Writers(stdout, stderr),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:   true,
			Tree:      true,
			Summary:   true,
			FlagsLast: true,
		}),
	)
}

func main() {
	extractors.InitializeBuiltins()
	cli := &CLI{}
	parser, err := newParser(cli, os.Stdout, os.Stderr)
	if err == nil {
		var ctx *kong.Context
		ctx, err = parser.Parse(os.Args[1:])
		if err != nil {
			err = fmt.Errorf("%w: %w", ErrCLIUsage, err)
		} else {
			err = ctx.Run()
		}
	} else {
		err = fmt.Errorf("constructing command parser: %w", err)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCodeFor(err))
	}
}
