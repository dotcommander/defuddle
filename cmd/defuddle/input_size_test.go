package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotcommander/defuddle"
)

// TestReadCapped_SmallInputPasses verifies that input well under the limit is
// returned verbatim with no error.
func TestReadCapped_SmallInputPasses(t *testing.T) {
	t.Parallel()

	want := []byte("<html><body><p>hi</p></body></html>")
	got, err := readCapped(bytes.NewReader(want), "stdin")
	if err != nil {
		t.Fatalf("readCapped small input: unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("readCapped small input: got %q, want %q", got, want)
	}
}

// TestReadCapped_AtBoundaryPasses verifies that input of exactly maxInputSize
// bytes is accepted (boundary is inclusive).
func TestReadCapped_AtBoundaryPasses(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("a"), maxInputSize)
	got, err := readCapped(bytes.NewReader(payload), "stdin")
	if err != nil {
		t.Fatalf("readCapped at boundary: unexpected error: %v", err)
	}
	if len(got) != maxInputSize {
		t.Fatalf("readCapped at boundary: got %d bytes, want %d", len(got), maxInputSize)
	}
}

// TestReadCapped_OversizedStdinRejected verifies that stdin input one byte over
// the cap returns defuddle.ErrTooLarge.
func TestReadCapped_OversizedStdinRejected(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("a"), maxInputSize+1)
	_, err := readCapped(bytes.NewReader(payload), "stdin")
	if err == nil {
		t.Fatalf("readCapped oversized stdin: expected error, got nil")
	}
	if !errors.Is(err, defuddle.ErrTooLarge) {
		t.Fatalf("readCapped oversized stdin: want ErrTooLarge, got %v", err)
	}
	if !strings.Contains(err.Error(), "stdin") {
		t.Errorf("readCapped oversized stdin: error should mention source, got %q", err.Error())
	}
}

// TestReadFile_SmallInputPasses verifies that a small HTML file is read intact
// via the capped path.
func TestReadFile_SmallInputPasses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "small.html")
	want := "<html><body><p>hi</p></body></html>"
	if err := os.WriteFile(path, []byte(want), 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := readFile(path)
	if err != nil {
		t.Fatalf("readFile small input: unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("readFile small input: got %q, want %q", got, want)
	}
}

// TestReadFile_OversizedFileRejected verifies that an HTML file one byte over
// the cap returns an error wrapping defuddle.ErrTooLarge, with the file path
// surfaced in the message for diagnostics.
func TestReadFile_OversizedFileRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "big.html")
	payload := bytes.Repeat([]byte("a"), maxInputSize+1)
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := readFile(path)
	if err == nil {
		t.Fatalf("readFile oversized: expected error, got nil")
	}
	if !errors.Is(err, defuddle.ErrTooLarge) {
		t.Fatalf("readFile oversized: want ErrTooLarge, got %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("readFile oversized: error should mention path %q, got %q", path, err.Error())
	}
}
