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

	"github.com/dotcommander/defuddle"
	"github.com/dotcommander/defuddle/extractors"
	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
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

// knownProperties lists all valid property names for --property extraction.
var knownProperties = []string{
	"content", "title", "description", "domain", "favicon", "image",
	"author", "site", "published", "wordCount", "parseTime",
	"metaTags", "schemaOrgData", "extractorType", "contentMarkdown",
}

var rootCmd = &cobra.Command{
	Use:     "defuddle",
	Short:   "Extract and structure content from web pages",
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	Long: `defuddle is a CLI tool for extracting and structuring content from web pages.
It can parse HTML, extract metadata, and convert content to various formats.`,
}

var parseCmd = &cobra.Command{
	Use:     "parse <source>",
	Aliases: []string{"p"},
	Short:   "Parse and extract content from a URL or HTML file",
	Long: `Parse content from a URL or local HTML file and extract structured information.
You can output the content in different formats and extract specific properties.`,
	Args: cobra.ExactArgs(1),
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

	enc := jsontext.NewEncoder(os.Stdout)
	for _, r := range results {
		if r.Err != nil {
			if !continueOnError {
				return fmt.Errorf("error parsing %s: %w", r.URL, r.Err)
			}
			errObj := map[string]string{"url": r.URL, "error": r.Err.Error()}
			if err := json.MarshalEncode(enc, errObj); err != nil {
				return fmt.Errorf("encoding error result: %w", err)
			}
			fmt.Println()
			continue
		}
		if err := json.MarshalEncode(enc, r.Result); err != nil {
			return fmt.Errorf("encoding result for %s: %w", r.URL, err)
		}
		fmt.Println()
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
	source := args[0]

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
	headerMap := make(map[string]string)
	for _, header := range opts.Headers {
		key, value, err := parseHeader(header)
		if err != nil {
			return err
		}
		headerMap[key] = value
	}

	// Create defuddle options
	defuddleOpts := &defuddle.Options{
		Debug:            opts.Debug,
		URL:              opts.Source,
		Markdown:         opts.Markdown,
		SeparateMarkdown: opts.Markdown,
		RemoveImages:     opts.RemoveImages,
		ContentSelector:  opts.ContentSelector,
	}
	if opts.NoClutterRemoval {
		defuddleOpts.RemoveExactSelectors = new(bool)
		defuddleOpts.RemovePartialSelectors = new(bool)
		defuddleOpts.RemoveHiddenElements = new(bool)
		defuddleOpts.RemoveLowScoring = new(bool)
		defuddleOpts.RemoveContentPatterns = new(bool)
	}

	ctx, cancel := buildContext(opts.Timeout)
	defer cancel()

	var result *defuddle.Result
	var err error

	// Parse content based on source type
	switch {
	case opts.Source == "-":
		// Parse from stdin
		stdinBytes, readErr := io.ReadAll(os.Stdin)
		if readErr != nil {
			return fmt.Errorf("reading stdin: %w", readErr)
		}
		defuddleInstance, createErr := defuddle.NewDefuddle(string(stdinBytes), defuddleOpts)
		if createErr != nil {
			return fmt.Errorf("error creating defuddle instance: %w", createErr)
		}
		result, err = defuddleInstance.Parse(ctx)
	case strings.HasPrefix(opts.Source, "http://") || strings.HasPrefix(opts.Source, "https://"):
		// Parse from URL
		result, err = defuddle.ParseFromURL(ctx, opts.Source, defuddleOpts)
	default:
		// Parse from file
		htmlContent, fileErr := readFile(opts.Source)
		if fileErr != nil {
			return fileErr
		}
		defuddleInstance, createErr := defuddle.NewDefuddle(htmlContent, defuddleOpts)
		if createErr != nil {
			return fmt.Errorf("error creating defuddle instance: %w", createErr)
		}
		result, err = defuddleInstance.Parse(ctx)
	}

	if err != nil {
		return fmt.Errorf("error loading content: %w", err)
	}

	// Handle property extraction
	if opts.Property != "" {
		value, found := getProperty(result, opts.Property)
		if !found {
			return fmt.Errorf("%w: %q (valid: %s)", ErrPropertyNotFound, opts.Property, strings.Join(knownProperties, ", "))
		}
		return writeOutput(opts.Output, value)
	}

	// Handle different output formats
	var content string
	switch {
	case opts.JSON:
		jsonData, err := json.Marshal(result, jsontext.Multiline(true))
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		content = string(jsonData)
	case opts.Markdown:
		if result.ContentMarkdown != nil {
			content = *result.ContentMarkdown
		} else {
			content = result.Content
		}
	default:
		content = result.Content
	}

	return writeOutput(opts.Output, content)
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
	prop := strings.ToLower(property)

	switch prop {
	case "content":
		return result.Content, true
	case "title":
		return result.Title, true
	case "description":
		return result.Description, true
	case "domain":
		return result.Domain, true
	case "favicon":
		return result.Favicon, true
	case "image":
		return result.Image, true
	case "author":
		return result.Author, true
	case "site":
		return result.Site, true
	case "published":
		return result.Published, true
	case "wordcount":
		return strconv.Itoa(result.WordCount), true
	case "parsetime":
		return strconv.FormatInt(result.ParseTime, 10), true
	case "metatags":
		if result.MetaTags != nil {
			jsonBytes, err := json.Marshal(result.MetaTags)
			if err != nil {
				return "", true
			}
			return string(jsonBytes), true
		}
		return "", true
	case "schemaorgdata":
		if result.SchemaOrgData != nil {
			jsonBytes, err := json.Marshal(result.SchemaOrgData)
			if err != nil {
				return "", true
			}
			return string(jsonBytes), true
		}
		return "null", true
	case "extractortype":
		if result.ExtractorType != nil {
			return *result.ExtractorType, true
		}
		return "", true
	case "contentmarkdown":
		if result.ContentMarkdown != nil {
			return *result.ContentMarkdown, true
		}
		return "", true
	default:
		return "", false
	}
}
