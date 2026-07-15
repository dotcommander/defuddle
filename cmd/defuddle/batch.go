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
)

// maxURLLineSize bounds one input line for batch mode. URLs in practice are
// well under 8 KiB; anything larger is treated as malformed input and
// surfaced as a bufio.ErrTooLong error rather than silently truncated.
const maxURLLineSize = 64 * 1024

type BatchOptions struct {
	Input           string        `short:"i" help:"Read URLs from file instead of stdin."`
	Concurrency     int           `short:"c" default:"5" help:"Maximum concurrent requests."`
	Markdown        bool          `short:"m" help:"Include markdown in output."`
	ContinueOnError bool          `name:"continue-on-error" help:"Continue processing on individual URL errors."`
	Timeout         time.Duration `help:"Overall batch timeout; 0 disables."`
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

func (opts *BatchOptions) Run() error {
	if err := validateConcurrency(opts.Concurrency); err != nil {
		return err
	}

	var reader io.Reader = os.Stdin
	if opts.Input != "" {
		f, err := os.Open(opts.Input) // #nosec G304 - user-provided input file
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

	parseOpts := &defuddle.Options{
		Markdown:         opts.Markdown,
		SeparateMarkdown: opts.Markdown,
		MaxConcurrency:   opts.Concurrency,
	}

	ctx, cancel := batchContext(opts.Timeout)
	defer cancel()

	results := defuddle.ParseFromURLs(ctx, urls, parseOpts)

	return encodeBatchResults(results, opts.ContinueOnError)
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
