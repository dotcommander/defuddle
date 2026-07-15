// Package main: `defuddle parse` subcommand and its supporting helpers.
//
// ParseOptions is Kong's parse command and hands off to executeParseContent, which drives the load → render
// → write pipeline. loadResult routes stdin / URL / file inputs through the
// defuddle library; renderOutput formats the Result for JSON, markdown, raw
// content, or a single --property accessor.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/dotcommander/defuddle"
)

type ParseOptions struct {
	Source           string        `arg:"" optional:"" name:"source" help:"URL or HTML file; reads stdin when omitted."`
	JSON             bool          `short:"j" help:"Output as JSON with metadata and content."`
	TablesJSON       bool          `name:"tables-json" help:"Output detected tables as structured JSON."`
	Markdown         bool          `short:"m" help:"Convert content to markdown format."`
	MD               bool          `name:"md" help:"Alias for --markdown."`
	Property         string        `short:"p" help:"Extract a specific property."`
	Output           string        `short:"o" help:"Output file path (default: stdout)."`
	UserAgent        string        `name:"user-agent" help:"Custom user agent string."`
	Headers          []string      `short:"H" name:"header" help:"Custom header in format 'Key: Value'."`
	Timeout          time.Duration `default:"30s" help:"Request timeout."`
	Debug            bool          `help:"Enable debug mode."`
	Proxy            string        `help:"Proxy URL."`
	RemoveImages     bool          `name:"remove-images" help:"Remove images from extracted content."`
	ContentSelector  string        `name:"content-selector" help:"CSS selector for content root."`
	NoClutterRemoval bool          `name:"no-clutter-removal" help:"Disable all clutter removal heuristics."`
	Render           bool          `help:"Render JavaScript via headless Chrome before extracting."`
	RenderAuto       bool          `name:"render-auto" help:"Render only pages detected as JavaScript-heavy."`
	JS               bool          `name:"js" help:"Alias for --render."`
	RenderWait       string        `name:"render-wait" default:"load" enum:"load,networkidle" help:"Render wait strategy."`
	RenderWaitFor    string        `name:"render-wait-for" help:"CSS selector to wait for before snapshot."`
	RenderSettle     time.Duration `name:"render-settle" help:"Extra settle delay after load."`
	RenderUA         string        `name:"render-user-agent" help:"User-agent for the render stage."`
	ChromePath       string        `name:"chrome-path" help:"Path to a Chrome/Chromium executable."`
	RenderTimeout    time.Duration `name:"render-timeout" default:"30s" help:"Maximum rendering time."`
}

func (opts *ParseOptions) Run() error {
	// Resolve source: positional arg, or "-" sentinel when stdin is piped.
	// loadResult (below) already handles the "-" → os.Stdin branch.
	var source string
	switch {
	case opts.Source != "":
		source = opts.Source
	case isStdinPiped():
		source = "-"
	default:
		return ErrParseUsage
	}

	opts.Source = source
	opts.Markdown = opts.Markdown || opts.MD
	opts.Render = opts.Render || opts.JS
	if opts.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	return executeParseContent(opts)
}

// buildContext returns a context (with optional timeout) and its cancel func.
// Callers must always defer cancel().
func buildContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

func executeParseContent(opts *ParseOptions) error {
	// Build HTTP client from fetch flags (validates headers, applies UA/proxy/timeout).
	// Returns nil client when no flags are set, so defuddle uses its hardened default.
	fetch, err := buildHTTPClient(opts.UserAgent, opts.Headers, opts.Proxy, opts.Timeout)
	if err != nil {
		return err
	}

	defuddleOpts := buildDefuddleOptions(opts)
	if fetch != nil {
		defuddleOpts.Client = fetch.client
		defuddleOpts.Headers = fetch.headers
	}

	ctx, cancel := buildContext(opts.Timeout)
	defer cancel()

	result, err := loadResult(ctx, opts, defuddleOpts)
	if err != nil {
		return fmt.Errorf("error loading content: %w", err)
	}

	content, err := renderOutput(result, opts)
	if err != nil {
		return err
	}

	return writeOutput(opts.Output, content)
}

// buildDefuddleOptions converts ParseOptions into a defuddle.Options.
func buildDefuddleOptions(opts *ParseOptions) *defuddle.Options {
	o := &defuddle.Options{
		Debug:            opts.Debug,
		URL:              opts.Source,
		Markdown:         opts.Markdown,
		SeparateMarkdown: opts.Markdown,
		RemoveImages:     opts.RemoveImages,
		ContentSelector:  opts.ContentSelector,
	}
	if opts.NoClutterRemoval {
		o.RemoveExactSelectors = new(bool)
		o.RemovePartialSelectors = new(bool)
		o.RemoveHiddenElements = new(bool)
		o.RemoveLowScoring = new(bool)
		o.RemoveContentPatterns = new(bool)
	}
	return o
}

// loadResult fetches and parses content from stdin, a URL, or a local file.
func loadResult(ctx context.Context, opts *ParseOptions, defuddleOpts *defuddle.Options) (*defuddle.Result, error) {
	switch {
	case opts.Source == "-":
		stdinBytes, err := readCapped(os.Stdin, "stdin")
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		d, err := defuddle.NewDefuddle(string(stdinBytes), defuddleOpts)
		if err != nil {
			return nil, fmt.Errorf("error creating defuddle instance: %w", err)
		}
		return d.Parse(ctx)
	case strings.HasPrefix(opts.Source, "http://") || strings.HasPrefix(opts.Source, "https://"):
		if opts.Render {
			return renderAndParse(ctx, opts, defuddleOpts)
		}
		if opts.RenderAuto {
			return autoRenderAndParse(ctx, opts, defuddleOpts)
		}
		return defuddle.ParseFromURL(ctx, opts.Source, defuddleOpts)
	default:
		htmlContent, err := readFile(opts.Source)
		if err != nil {
			return nil, err
		}
		d, err := defuddle.NewDefuddle(htmlContent, defuddleOpts)
		if err != nil {
			return nil, fmt.Errorf("error creating defuddle instance: %w", err)
		}
		return d.Parse(ctx)
	}
}

// renderOutput formats result according to opts, returning the string to write.
func renderOutput(result *defuddle.Result, opts *ParseOptions) (string, error) {
	if opts.Property != "" {
		value, found := getProperty(result, opts.Property)
		if !found {
			return "", fmt.Errorf("%w: %q (valid: %s)", ErrPropertyNotFound, opts.Property, strings.Join(knownProperties, ", "))
		}
		return value, nil
	}

	switch {
	case opts.TablesJSON:
		tables, err := defuddle.ExtractTables(result.Content)
		if err != nil {
			return "", fmt.Errorf("error extracting tables: %w", err)
		}
		jsonData, err := json.MarshalIndent(tables, "", "  ")
		if err != nil {
			return "", fmt.Errorf("error marshaling tables JSON: %w", err)
		}
		return string(jsonData), nil
	case opts.JSON:
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("error marshaling JSON: %w", err)
		}
		return string(jsonData), nil
	case opts.Markdown:
		if result.ContentMarkdown != nil {
			return *result.ContentMarkdown, nil
		}
		return result.Content, nil
	default:
		return result.Content, nil
	}
}

func parseHeader(header string) (string, string, error) {
	parts := strings.SplitN(header, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%w: %s", ErrInvalidHeaderFormat, header)
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", fmt.Errorf("%w: empty header name", ErrInvalidHeaderFormat)
	}
	return key, strings.TrimSpace(parts[1]), nil
}

// isStdinPiped reports whether os.Stdin is connected to a pipe or file,
// rather than a terminal. Used to decide whether bare `defuddle parse`
// should consume piped HTML or print a usage error.
func isStdinPiped() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}
