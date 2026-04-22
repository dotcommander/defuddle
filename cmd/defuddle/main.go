// Package main provides the defuddle CLI application.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"github.com/dotcommander/defuddle"
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

// propertyExtractors maps lowercase property names to their Result accessor.
// The keys also serve as the canonical list of valid --property values.
var propertyExtractors = map[string]func(*defuddle.Result) string{
	"content":     func(r *defuddle.Result) string { return r.Content },
	"title":       func(r *defuddle.Result) string { return r.Title },
	"description": func(r *defuddle.Result) string { return r.Description },
	"domain":      func(r *defuddle.Result) string { return r.Domain },
	"favicon":     func(r *defuddle.Result) string { return r.Favicon },
	"image":       func(r *defuddle.Result) string { return r.Image },
	"author":      func(r *defuddle.Result) string { return r.Author },
	"site":        func(r *defuddle.Result) string { return r.Site },
	"published":   func(r *defuddle.Result) string { return r.Published },
	"wordcount":   func(r *defuddle.Result) string { return strconv.Itoa(r.WordCount) },
	"parsetime":   func(r *defuddle.Result) string { return strconv.FormatInt(r.ParseTime, 10) },
	"metatags": func(r *defuddle.Result) string {
		if r.MetaTags == nil {
			return ""
		}
		b, err := json.Marshal(r.MetaTags)
		if err != nil {
			return ""
		}
		return string(b)
	},
	"schemaorgdata": func(r *defuddle.Result) string {
		if r.SchemaOrgData == nil {
			return "null"
		}
		b, err := json.Marshal(r.SchemaOrgData)
		if err != nil {
			return ""
		}
		return string(b)
	},
	"extractortype": func(r *defuddle.Result) string {
		if r.ExtractorType != nil {
			return *r.ExtractorType
		}
		return ""
	},
	"contentmarkdown": func(r *defuddle.Result) string {
		if r.ContentMarkdown != nil {
			return *r.ContentMarkdown
		}
		return ""
	},
}

// knownProperties is the sorted display list for error messages.
var knownProperties = func() []string {
	keys := make([]string, 0, len(propertyExtractors))
	for k := range propertyExtractors {
		keys = append(keys, k)
	}
	return keys
}()

var rootCmd = &cobra.Command{
	Use:     "defuddle",
	Short:   "Extract and structure content from web pages",
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	Long: `defuddle is a CLI tool for extracting and structuring content from web pages.
It can parse HTML, extract metadata, and convert content to various formats.`,
}

var parseCmd = &cobra.Command{
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

type ParseOptions struct {
	Source           string
	JSON             bool
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
}

func init() {
	// Initialize built-in extractors
	extractors.InitializeBuiltins()

	parseCmd.Flags().BoolP("json", "j", false, "Output as JSON with metadata and content")
	parseCmd.Flags().BoolP("markdown", "m", false, "Convert content to markdown format")
	parseCmd.Flags().Bool("md", false, "Alias for --markdown")
	parseCmd.Flags().StringP("property", "p", "", "Extract a specific property (e.g., title, description, domain)")
	parseCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	parseCmd.Flags().String("user-agent", "", "Custom user agent string")
	parseCmd.Flags().StringArrayP("header", "H", []string{}, "Custom headers in format 'Key: Value'")
	parseCmd.Flags().Duration("timeout", 30*time.Second, "Request timeout")
	parseCmd.Flags().Bool("debug", false, "Enable debug mode")
	parseCmd.Flags().String("proxy", "", "Proxy URL (e.g., http://localhost:8080, socks5://localhost:1080)")
	parseCmd.Flags().Bool("remove-images", false, "Remove images from extracted content")
	parseCmd.Flags().String("content-selector", "", "CSS selector for content root (bypasses auto-detection)")
	parseCmd.Flags().Bool("no-clutter-removal", false, "Disable all clutter removal heuristics")

	extractorsCmd.Flags().String("match", "", "Show which extractor matches the given URL")

	batchCmd.Flags().StringP("input", "i", "", "Read URLs from file instead of stdin")
	batchCmd.Flags().IntP("concurrency", "c", 5, "Maximum concurrent requests")
	batchCmd.Flags().BoolP("markdown", "m", false, "Include markdown in output")
	batchCmd.Flags().Bool("continue-on-error", false, "Continue processing on individual URL errors")

	rootCmd.AddCommand(parseCmd)
	rootCmd.AddCommand(extractorsCmd)
	rootCmd.AddCommand(batchCmd)
}

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

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Parse multiple URLs, output JSONL",
	Long:  `Reads one URL per line from stdin (default) or --input file. Outputs one JSON object per line to stdout.`,
	RunE:  runBatch,
}

func runBatch(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	inputFile, _ := cmd.Flags().GetString("input")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	markdown, _ := cmd.Flags().GetBool("markdown")
	continueOnError, _ := cmd.Flags().GetBool("continue-on-error")

	var reader io.Reader = os.Stdin
	if inputFile != "" {
		f, err := os.Open(inputFile) // #nosec G304 - user-provided input file
		if err != nil {
			return fmt.Errorf("opening input file: %w", err)
		}
		defer func() { _ = f.Close() }()
		reader = f
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	var urls []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}

	if len(urls) == 0 {
		return ErrNoURLs
	}

	opts := &defuddle.Options{
		Markdown:         markdown,
		SeparateMarkdown: markdown,
		MaxConcurrency:   concurrency,
	}

	ctx := context.Background()
	results := defuddle.ParseFromURLs(ctx, urls, opts)

	enc := json.NewEncoder(os.Stdout)
	for _, r := range results {
		if r.Err != nil {
			if !continueOnError {
				return fmt.Errorf("error parsing %s: %w", r.URL, r.Err)
			}
			errObj := map[string]string{"url": r.URL, "error": r.Err.Error()}
			if err := enc.Encode(errObj); err != nil {
				return fmt.Errorf("encoding error result: %w", err)
			}
			continue
		}
		if err := enc.Encode(r.Result); err != nil {
			return fmt.Errorf("encoding result for %s: %w", r.URL, err)
		}
	}
	return nil
}

func main() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
		return fmt.Errorf("usage: defuddle parse <url|file> (or pipe HTML via stdin)")
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
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

	// Handle markdown alias
	if mdAlias {
		markdown = true
	}

	opts := &ParseOptions{
		Source:           source,
		JSON:             jsonOutput,
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
	// Parse headers
	for _, header := range opts.Headers {
		if _, _, err := parseHeader(header); err != nil {
			return err
		}
	}

	defuddleOpts := buildDefuddleOptions(opts)

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
		stdinBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		d, err := defuddle.NewDefuddle(string(stdinBytes), defuddleOpts)
		if err != nil {
			return nil, fmt.Errorf("error creating defuddle instance: %w", err)
		}
		return d.Parse(ctx)
	case strings.HasPrefix(opts.Source, "http://") || strings.HasPrefix(opts.Source, "https://"):
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
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func readFile(filename string) (string, error) {
	if err := validateFilePath(filename); err != nil {
		return "", err
	}
	content, err := os.ReadFile(filename) // #nosec G304 - path validated above
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}
	return string(content), nil
}

func validateFilePath(filename string) error {
	// Reject directory traversal by cleaning the path and checking for ".." components.
	// strings.Contains(filename, "..") is bypassable (e.g. "a..b" matches but is safe;
	// "%2e%2e" or unicode variants could slip through after URL decode).
	// filepath.Clean resolves all ".." sequences first, so the check is exact.
	cleaned := filepath.Clean(filename)
	for part := range strings.SplitSeq(cleaned, string(filepath.Separator)) {
		if part == ".." {
			return ErrDirectoryTraversal
		}
	}
	return nil
}

func writeOutput(filename, content string) error {
	if filename == "" {
		fmt.Print(content)
		return nil
	}

	err := os.WriteFile(filename, []byte(content), 0600) // More secure file permissions
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Output written to %s\n", filename)
	return nil
}

func getProperty(result *defuddle.Result, property string) (string, bool) {
	// Convert to lowercase for case-insensitive matching (matching TypeScript behavior)
	fn, ok := propertyExtractors[strings.ToLower(property)]
	if !ok {
		return "", false
	}
	return fn(result), true
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
