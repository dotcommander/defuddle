// Package main: bounded I/O helpers for parse mode.
//
// All local input paths (stdin, --output file, HTML files) share a single
// size ceiling (maxInputSize) and a directory-traversal guard so a pathological
// input cannot OOM the process or escape its working set.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotcommander/defuddle"
)

// maxInputSize bounds local input reads (stdin and HTML files) for parse mode.
// Matches the library's URL fetch cap (defuddle.maxResponseSize = 5 MiB) so all
// input paths share one ceiling. URL parsing is unaffected — the library enforces
// its own cap internally.
const maxInputSize = 5 * 1024 * 1024

// readCapped reads from r up to maxInputSize bytes. If r yields more, it returns
// defuddle.ErrTooLarge so callers can branch with errors.Is. The source argument
// is included in the wrapped error for diagnostics ("stdin" or a file path).
func readCapped(r io.Reader, source string) ([]byte, error) {
	// Read one extra byte to detect overflow without an extra Read call.
	buf, err := io.ReadAll(io.LimitReader(r, maxInputSize+1))
	if err != nil {
		return nil, err
	}
	if len(buf) > maxInputSize {
		return nil, fmt.Errorf("read %s: input exceeds %d bytes: %w", source, maxInputSize, defuddle.ErrTooLarge)
	}
	return buf, nil
}

func readFile(filename string) (string, error) {
	if err := validateFilePath(filename); err != nil {
		return "", err
	}
	f, err := os.Open(filename) // #nosec G304 - path validated above
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}
	defer func() { _ = f.Close() }()
	content, err := readCapped(f, filename)
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
