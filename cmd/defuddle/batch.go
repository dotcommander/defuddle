// Package main: batch subcommand.
//
// runBatch streams URLs line-by-line from stdin or --input. Streaming (vs
// io.ReadAll) bounds memory for large URL lists; a bounded scanner buffer
// caps any single line so a pathological input cannot OOM the process.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dotcommander/defuddle"
	"github.com/spf13/cobra"
)

// maxURLLineSize bounds one input line for batch mode. URLs in practice are
// well under 8 KiB; anything larger is treated as malformed input and
// surfaced as a bufio.ErrTooLong error rather than silently truncated.
const maxURLLineSize = 64 * 1024

func newBatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Parse multiple URLs, output JSONL",
		Long:  `Reads one URL per line from stdin (default) or --input file. Outputs one JSON object per line to stdout.`,
		RunE:  runBatch,
	}
	registerBatchFlags(cmd)
	return cmd
}

// registerBatchFlags attaches batch-mode flags.
func registerBatchFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("input", "i", "", "Read URLs from file instead of stdin")
	cmd.Flags().IntP("concurrency", "c", 5, "Maximum concurrent requests")
	cmd.Flags().BoolP("markdown", "m", false, "Include markdown in output")
	cmd.Flags().Bool("continue-on-error", false, "Continue processing on individual URL errors")
	cmd.Flags().Duration("timeout", 0, "Overall batch timeout (e.g. 30s, 2m); 0 disables")
}

// scanURLs reads one URL per line from r, skipping blank lines and lines that
// begin with '#'. Lines longer than maxURLLineSize cause bufio.ErrTooLong; the
// caller decides whether to surface or skip.
func scanURLs(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	// Grow up to maxURLLineSize before returning ErrTooLong.
	scanner.Buffer(make([]byte, 0, 64*1024), maxURLLineSize)

	var urls []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning input: %w", err)
	}
	return urls, nil
}

func runBatch(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	inputFile, _ := cmd.Flags().GetString("input")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	markdown, _ := cmd.Flags().GetBool("markdown")
	continueOnError, _ := cmd.Flags().GetBool("continue-on-error")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	if err := validateConcurrency(concurrency); err != nil {
		return err
	}

	var reader io.Reader = os.Stdin
	if inputFile != "" {
		f, err := os.Open(inputFile) // #nosec G304 - user-provided input file
		if err != nil {
			return fmt.Errorf("opening input file: %w", err)
		}
		defer func() { _ = f.Close() }()
		reader = f
	}

	urls, err := scanURLs(reader)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	if len(urls) == 0 {
		return ErrNoURLs
	}

	opts := &defuddle.Options{
		Markdown:         markdown,
		SeparateMarkdown: markdown,
		MaxConcurrency:   concurrency,
	}

	ctx, cancel := batchContext(timeout)
	defer cancel()

	results := defuddle.ParseFromURLs(ctx, urls, opts)

	return encodeBatchResults(results, continueOnError)
}

// encodeBatchResults writes each parse result to stdout as JSON. On a parse
// error it returns immediately unless continueOnError is set, in which case it
// emits an error object and continues.
func encodeBatchResults(results []defuddle.URLResult, continueOnError bool) error {
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

// batchContext returns a cancellable context with an optional deadline.
// Callers must always defer cancel().
func batchContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(context.Background(), timeout)
	}
	return context.WithCancel(context.Background())
}

func validateConcurrency(concurrency int) error {
	if concurrency < 1 {
		return fmt.Errorf("%w: got %d", ErrInvalidConcurrency, concurrency)
	}
	return nil
}
