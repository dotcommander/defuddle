package main

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestScanURLs_SkipsBlanksAndComments verifies that blank lines and lines
// starting with '#' are skipped, while surrounding whitespace is trimmed.
func TestScanURLs_SkipsBlanksAndComments(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"https://example.com/a",
		"",
		"   ",
		"# this is a comment",
		"  https://example.com/b  ",
		"\t",
		"#another",
		"https://example.com/c",
	}, "\n")

	got, err := scanURLs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("scanURLs: unexpected error: %v", err)
	}

	want := []string{
		"https://example.com/a",
		"https://example.com/b",
		"https://example.com/c",
	}
	if len(got) != len(want) {
		t.Fatalf("scanURLs: got %d urls, want %d (got=%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("scanURLs[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestScanURLs_OversizedLineRejected verifies that a single line larger than
// maxURLLineSize causes bufio.ErrTooLong rather than silent truncation.
func TestScanURLs_OversizedLineRejected(t *testing.T) {
	t.Parallel()

	// One line of (maxURLLineSize + 1) bytes, no newline before it.
	oversized := bytes.Repeat([]byte("a"), maxURLLineSize+1)
	r := bytes.NewReader(oversized)

	_, err := scanURLs(r)
	if err == nil {
		t.Fatalf("scanURLs oversized: expected error, got nil")
	}
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("scanURLs oversized: want bufio.ErrTooLong, got %v", err)
	}
}

// TestScanURLs_EmptyInput verifies that an empty reader yields no URLs and no
// error — caller (runBatch) is responsible for surfacing ErrNoURLs.
func TestScanURLs_EmptyInput(t *testing.T) {
	t.Parallel()

	got, err := scanURLs(strings.NewReader(""))
	if err != nil {
		t.Fatalf("scanURLs empty: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("scanURLs empty: got %d urls, want 0", len(got))
	}
}

// TestScanURLs_CommentOnlyInput verifies that input consisting entirely of
// blanks and comments yields zero URLs.
func TestScanURLs_CommentOnlyInput(t *testing.T) {
	t.Parallel()

	input := "# header\n\n# another\n   \n#last\n"
	got, err := scanURLs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("scanURLs comments: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("scanURLs comments: got %d urls, want 0 (got=%v)", len(got), got)
	}
}

// TestBatchCmd_TimeoutFlagRegistered verifies that the batch subcommand
// exposes a --timeout flag with the expected default and that durations
// parse cleanly.
func TestBatchCmd_TimeoutFlagRegistered(t *testing.T) {
	t.Parallel()

	flag := batchCmd.Flags().Lookup("timeout")
	if flag == nil {
		t.Fatalf("batchCmd: --timeout flag not registered")
	}
	if flag.DefValue != "0s" {
		t.Errorf("batchCmd --timeout default: got %q, want %q", flag.DefValue, "0s")
	}

	// Round-trip a parse to confirm the value is wired as a duration.
	if err := batchCmd.Flags().Set("timeout", "1500ms"); err != nil {
		t.Fatalf("set --timeout: %v", err)
	}
	defer func() {
		// Reset to default so other tests see a clean flag set.
		_ = batchCmd.Flags().Set("timeout", "0s")
	}()

	got, err := batchCmd.Flags().GetDuration("timeout")
	if err != nil {
		t.Fatalf("get --timeout: %v", err)
	}
	if got.String() != "1.5s" {
		t.Errorf("batchCmd --timeout parsed: got %s, want 1.5s", got)
	}
}

// TestBatchContext_NoTimeout verifies that a zero duration returns a plain
// cancellable context with no deadline.
func TestBatchContext_NoTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := batchContext(0)
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Fatalf("batchContext(0): expected no deadline, got one")
	}
}

// TestBatchContext_WithTimeout verifies that a positive duration installs a
// deadline within the requested window.
func TestBatchContext_WithTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := batchContext(50 * 1000 * 1000) // 50ms in ns
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("batchContext(50ms): expected deadline, got none")
	}
	if deadline.IsZero() {
		t.Fatalf("batchContext(50ms): deadline is zero")
	}
}
