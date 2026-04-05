// Package main provides the defuddle CLI application.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dotcommander/defuddle"
	"github.com/dotcommander/defuddle/extractors"
	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/spf13/cobra"
)

const version = "0.2.2"

// Define static errors to avoid dynamic error creation
var (
	ErrInvalidHeaderFormat = fmt.Errorf("invalid header format (expected 'Key: Value')")
	ErrDirectoryTraversal  = fmt.Errorf("invalid file path: directory traversal detected")
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
	Version: version,
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
	Source    string
	JSON      bool
	Markdown  bool
	Property  string
	Output    string
	UserAgent string
	Headers   []string
	Timeout   time.Duration
	Debug     bool
	Proxy     string
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

	rootCmd.AddCommand(parseCmd)
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

	// Handle markdown alias
	if mdAlias {
		markdown = true
	}

	opts := &ParseOptions{
		Source:    source,
		JSON:      jsonOutput,
		Markdown:  markdown,
		Property:  property,
		Output:    output,
		UserAgent: userAgent,
		Headers:   headers,
		Timeout:   timeout,
		Debug:     debug,
		Proxy:     proxy,
	}

	if debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	return executeParseContent(opts)
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
	}

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
		ctx := context.Background()
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
		}
		result, err = defuddleInstance.Parse(ctx)
	case strings.HasPrefix(opts.Source, "http://") || strings.HasPrefix(opts.Source, "https://"):
		// Parse from URL
		ctx := context.Background()
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
		}
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

		ctx := context.Background()
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
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
			// If ContentMarkdown is not available, try to convert HTML content to markdown
			// Create a new defuddle instance specifically for markdown conversion
			markdownOpts := &defuddle.Options{
				Debug:            false,
				URL:              opts.Source,
				Markdown:         true,
				SeparateMarkdown: true,
			}

			// Create temporary HTML document for conversion
			htmlContent := fmt.Sprintf("<html><body>%s</body></html>", result.Content)
			defuddleInstance, err := defuddle.NewDefuddle(htmlContent, markdownOpts)
			if err == nil {
				ctx := context.Background()
				if opts.Timeout > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
					defer cancel()
				}

				markdownResult, markdownErr := defuddleInstance.Parse(ctx)
				if markdownErr == nil && markdownResult.ContentMarkdown != nil {
					content = *markdownResult.ContentMarkdown
				} else {
					// Fallback to original content if markdown conversion fails
					content = result.Content
				}
			} else {
				// Fallback to original content if defuddle creation fails
				content = result.Content
			}
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
	// Add basic path validation to prevent directory traversal
	if strings.Contains(filename, "..") {
		return ErrDirectoryTraversal
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
