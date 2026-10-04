// Package main provides the defuddle CLI application.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/alecthomas/kong"
	"github.com/dotcommander/defuddle/extractors"
)

// Build-injected via go build -ldflags "-X main.version=...".
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
	ErrInvalidProxyScheme  = errors.New("invalid proxy URL scheme")
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
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCodeFor(err))
	}
}

func run(parent context.Context) error {
	extractors.InitializeBuiltins()
	cli := &CLI{}
	cli.Parse.parent = parent
	cli.Batch.parent = parent
	parser, err := newParser(cli, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("constructing command parser: %w", err)
	}
	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCLIUsage, err)
	}
	return ctx.Run()
}
