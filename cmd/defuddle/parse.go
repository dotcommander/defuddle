// Package main: `defuddle parse` subcommand and its supporting helpers.
//
// parseCmd is the cobra wrapper; parseContent reads flags into a ParseOptions
// struct and hands off to executeParseContent, which drives the load → render
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
	"github.com/spf13/cobra"
)

func newParseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "parse [source]",
		Aliases: []string{"p"},
		Short:   "Parse and extract content from a URL, HTML file, or stdin",
		Long: `Parse content from a URL, local HTML file, or HTML piped via stdin
and extract structured information.

Examples:
  defuddle parse https://example.com/article
  defuddle parse article.html
  curl -s https://example.com/article | defuddle parse --markdown

You can output the content in different formats and extract specific properties.`,
		Args: cobra.MaximumNArgs(1),
		RunE: parseContent,
	}
	registerParseFlags(cmd)
	return cmd
}

type ParseOptions struct {
	Source           string
	JSON             bool
	TablesJSON       bool
	Markdown         bool
	Property         string
	Output           string
	UserAgent        string
	Headers          []string
	Timeout          time.Duration
	Debug            bool
	Proxy            string
	RemoveImages     bool
	ContentSelector  string
	NoClutterRemoval bool
	Render           bool
	RenderAuto       bool
	RenderWait       string
	RenderWaitFor    string
	RenderSettle     time.Duration
	RenderUA         string
	ChromePath       string
	RenderTimeout    time.Duration
}

// registerParseFlags attaches parse-mode flags.
func registerParseFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("json", "j", false, "Output as JSON with metadata and content")
	cmd.Flags().BoolP("markdown", "m", false, "Convert content to markdown format")
	cmd.Flags().Bool("md", false, "Alias for --markdown")
	cmd.Flags().StringP("property", "p", "", "Extract a specific property (e.g., title, description, domain)")
	cmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	cmd.Flags().Bool("tables-json", false, "Output detected tables as structured JSON ({caption, headers, rows})")
	cmd.Flags().String("user-agent", "", "Custom user agent string")
	cmd.Flags().StringArrayP("header", "H", []string{}, "Custom headers in format 'Key: Value'")
	cmd.Flags().Duration("timeout", 30*time.Second, "Request timeout")
	cmd.Flags().Bool("debug", false, "Enable debug mode")
	cmd.Flags().String("proxy", "", "Proxy URL (e.g., http://localhost:8080, socks5://localhost:1080)")
	cmd.Flags().Bool("remove-images", false, "Remove images from extracted content")
	cmd.Flags().String("content-selector", "", "CSS selector for content root (bypasses auto-detection)")
	cmd.Flags().Bool("no-clutter-removal", false, "Disable all clutter removal heuristics")
	cmd.Flags().Bool("render", false, "Render JavaScript via headless Chrome before extracting (requires Chrome/Chromium)")
	cmd.Flags().Bool("render-auto", false, "Auto-detect JS-heavy pages and render only when needed; unlike --render it degrades to plain-HTML parse if Chrome is unavailable")
	cmd.Flags().Bool("js", false, "Alias for --render")
	cmd.Flags().String("render-wait", "load", "Render wait strategy: 'load' or 'networkidle'")
	cmd.Flags().String("render-wait-for", "", "CSS selector to wait for (visible) before snapshot, e.g. 'table tbody tr' (requires --render)")
	cmd.Flags().Duration("render-settle", 0, "Extra settle delay after load before snapshot, e.g. 2s; for SPAs without a stable selector (requires --render)")
	cmd.Flags().String("render-user-agent", "", "User-agent for the render stage (default: Chrome default)")
	cmd.Flags().String("chrome-path", "", "Path to a Chrome/Chromium executable (default: auto-detect)")
	cmd.Flags().Duration("render-timeout", 30*time.Second, "Maximum time to spend rendering the page")
}

func parseContent(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// Resolve source: positional arg, or "-" sentinel when stdin is piped.
	// loadResult (below) already handles the "-" → os.Stdin branch.
	var source string
	switch {
	case len(args) == 1:
		source = args[0]
	case isStdinPiped():
		source = "-"
	default:
		return ErrParseUsage
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	tablesJSON, _ := cmd.Flags().GetBool("tables-json")
	markdown, _ := cmd.Flags().GetBool("markdown")
	mdAlias, _ := cmd.Flags().GetBool("md")
	property, _ := cmd.Flags().GetString("property")
	output, _ := cmd.Flags().GetString("output")
	userAgent, _ := cmd.Flags().GetString("user-agent")
	headers, _ := cmd.Flags().GetStringArray("header")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	debug, _ := cmd.Flags().GetBool("debug")
	proxy, _ := cmd.Flags().GetString("proxy")
	removeImages, _ := cmd.Flags().GetBool("remove-images")
	contentSelector, _ := cmd.Flags().GetString("content-selector")
	noClutterRemoval, _ := cmd.Flags().GetBool("no-clutter-removal")
	renderFlag, _ := cmd.Flags().GetBool("render")
	renderAuto, _ := cmd.Flags().GetBool("render-auto")
	jsAlias, _ := cmd.Flags().GetBool("js")
	renderWait, _ := cmd.Flags().GetString("render-wait")
	renderWaitFor, _ := cmd.Flags().GetString("render-wait-for")
	renderSettle, _ := cmd.Flags().GetDuration("render-settle")
	renderUA, _ := cmd.Flags().GetString("render-user-agent")
	chromePath, _ := cmd.Flags().GetString("chrome-path")
	renderTimeout, _ := cmd.Flags().GetDuration("render-timeout")

	// Handle markdown alias
	if mdAlias {
		markdown = true
	}
	if jsAlias {
		renderFlag = true
	}

	opts := &ParseOptions{
		Source:           source,
		JSON:             jsonOutput,
		TablesJSON:       tablesJSON,
		Markdown:         markdown,
		Property:         property,
		Output:           output,
		UserAgent:        userAgent,
		Headers:          headers,
		Timeout:          timeout,
		Debug:            debug,
		Proxy:            proxy,
		RemoveImages:     removeImages,
		ContentSelector:  contentSelector,
		NoClutterRemoval: noClutterRemoval,
		Render:           renderFlag,
		RenderAuto:       renderAuto,
		RenderWait:       renderWait,
		RenderWaitFor:    renderWaitFor,
		RenderSettle:     renderSettle,
		RenderUA:         renderUA,
		ChromePath:       chromePath,
		RenderTimeout:    renderTimeout,
	}

	if debug {
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
	client, err := buildHTTPClient(opts.UserAgent, opts.Headers, opts.Proxy, opts.Timeout)
	if err != nil {
		return err
	}

	defuddleOpts := buildDefuddleOptions(opts)
	defuddleOpts.Client = client

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
