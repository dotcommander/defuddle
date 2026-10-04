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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dotcommander/defuddle"
)

type ParseOptions struct {
	parent context.Context

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
	RenderWait       string        `name:"render-wait" help:"Render wait strategy: load or networkidle (default load; networkidle for --render-auto when unset)."`
	RenderWaitFor    string        `name:"render-wait-for" help:"CSS selector to wait for before snapshot."`
	RenderSettle     time.Duration `name:"render-settle" help:"Extra settle delay after load."`
	RenderUA         string        `name:"render-user-agent" help:"User-agent for the render stage."`
	ChromePath       string        `name:"chrome-path" help:"Path to a Chrome/Chromium executable."`
	RenderTimeout    time.Duration `name:"render-timeout" default:"30s" help:"Maximum rendering time."`
}

func (opts *ParseOptions) Run() error {
	return opts.run(commandContext(opts.parent))
}

func (opts *ParseOptions) run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
	if err := validateRenderWait(opts.RenderWait); err != nil {
		return err
	}
	if opts.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	return executeParseContent(ctx, opts)
}

func commandContext(parent context.Context) context.Context {
	if parent != nil {
		return parent
	}
	return context.Background()
}

// validateRenderWait rejects --render-wait values outside the accepted set.
// Empty is valid: it selects the path default (load for --render, networkidle
// for --render-auto escalation) so an explicit value is always distinguishable
// from an unset one.
func validateRenderWait(v string) error {
	switch v {
	case "", "load", "networkidle":
		return nil
	}
	return fmt.Errorf("%w: %q (valid: load, networkidle)", ErrInvalidRenderWait, v)
}

// buildContext returns a context (with optional timeout) and its cancel func.
// Callers must always defer cancel().
func buildContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}

func executeParseContent(ctx context.Context, opts *ParseOptions) error {
	// Build HTTP client from fetch flags (validates headers, applies UA/proxy/timeout).
	// Returns nil client when no flags are set, so fetchHTML uses equivalent defaults.
	fetch, err := buildHTTPClient(opts.UserAgent, opts.Headers, opts.Proxy, opts.Timeout)
	if err != nil {
		return err
	}

	defuddleOpts := buildDefuddleOptions(opts)
	if fetch != nil {
		defuddleOpts.Client = fetch.client
		defuddleOpts.Headers = fetch.headers
	}

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
		URL:              sourceBaseURL(opts.Source),
		Markdown:         opts.Markdown,
		SeparateMarkdown: opts.Markdown,
		RemoveImages:     opts.RemoveImages,
		ContentSelector:  opts.ContentSelector,
	}
	if opts.NoClutterRemoval {
		o.RemoveExactSelectors = new(bool)   //nolint:staticcheck // All five deprecated controls preserve --no-clutter-removal compatibility.
		o.RemovePartialSelectors = new(bool) //nolint:staticcheck // All five deprecated controls preserve --no-clutter-removal compatibility.
		o.RemoveHiddenElements = new(bool)   //nolint:staticcheck // All five deprecated controls preserve --no-clutter-removal compatibility.
		o.RemoveLowScoring = new(bool)       //nolint:staticcheck // All five deprecated controls preserve --no-clutter-removal compatibility.
		o.RemoveContentPatterns = new(bool)  //nolint:staticcheck // All five deprecated controls preserve --no-clutter-removal compatibility.
	}
	return o
}

// sourceBaseURL returns the base URL used to resolve relative URLs in
// extracted content. HTTP(S) sources are their own base; a local file input
// resolves against a file:// URL derived from its absolute path, matching the
// upstream TypeScript CLI (JSDOM.fromFile exposes a file:// document URL and
// the TS CLI passes no explicit URL for local files). Stdin ("-") has no
// meaningful base: an empty URL leaves relative references untouched instead
// of fabricating malformed ones.
func sourceBaseURL(source string) string {
	switch {
	case source == "", source == "-":
		return ""
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		return source
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		return ""
	}
	return (&url.URL{Scheme: "file", Path: abs}).String()
}

// loadResult fetches and parses content from stdin, a URL, or a local file.
func loadResult(ctx context.Context, opts *ParseOptions, defuddleOpts *defuddle.Options) (*defuddle.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Rendering drives Chrome to the source URL; file and stdin inputs have no
	// URL to navigate, so warn instead of silently dropping the render flags.
	if (opts.Render || opts.RenderAuto) && !isHTTPSource(opts.Source) {
		fmt.Fprintf(os.Stderr, "defuddle: ignoring %s: JavaScript rendering requires an http(s) URL source\n", renderFlagLabel(opts))
	}
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
	case isHTTPSource(opts.Source):
		if opts.Render {
			return renderAndParse(ctx, opts, defuddleOpts)
		}
		if opts.RenderAuto {
			return autoRenderAndParse(ctx, opts, defuddleOpts)
		}
		fetchCtx, cancel := buildContext(ctx, opts.Timeout)
		document, err := fetchHTML(fetchCtx, opts.Source, defuddleOpts.Client, defuddleOpts.Headers)
		cancel()
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		networkOptions := *defuddleOpts
		networkOptions.URL = document.URL
		return defuddle.ParseFromString(ctx, document.HTML, &networkOptions)
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

func isHTTPSource(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}

func renderFlagLabel(opts *ParseOptions) string {
	switch {
	case opts.Render && opts.RenderAuto:
		return "--render/--render-auto"
	case opts.RenderAuto:
		return "--render-auto"
	default:
		return "--render"
	}
}

// renderOutput formats result according to opts, returning the string to write.
func renderOutput(result *defuddle.Result, opts *ParseOptions) (string, error) {
	if opts.Property != "" {
		value, found := getProperty(result, opts.Property)
		if !found {
			return "", fmt.Errorf("%w: %q (valid: %s)", ErrPropertyNotFound, opts.Property, strings.Join(knownProperties, ", "))
		}
		// Trailing newline matches the upstream CLI's console.log shape and
		// keeps terminal output readable; --output writes it as part of content.
		return value + "\n", nil
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
