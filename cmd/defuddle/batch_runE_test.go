package main

// Tests for the batch cobra subcommand's RunE handler (runBatch).
//
// runBatch writes to os.Stdout directly (not cmd.OutOrStdout()), so these
// tests swap the global os.Stdout via os.Pipe().
// captureBatchOutput swaps os.Stdout; tests cannot t.Parallel().

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureBatchOutput swaps os.Stdout for the duration of fn, returning
// everything written to it.
//
// WARNING: mutates os.Stdout — callers must NOT call t.Parallel().
func captureBatchOutput(t *testing.T, fn func() error) (stdout string, err error) {
	t.Helper()

	origOut := os.Stdout

	rOut, wOut, pipeErr := os.Pipe()
	require.NoError(t, pipeErr, "os.Pipe stdout")

	os.Stdout = wOut
	defer func() { os.Stdout = origOut }()

	err = fn()

	wOut.Close()

	outBytes, _ := io.ReadAll(rOut)
	return string(outBytes), err
}

// resetBatchFlags restores all batch flags to their defaults so sequential
// tests start from a clean state.
func resetBatchFlags(t *testing.T) {
	t.Helper()
	require.NoError(t, batchCmd.Flags().Set("input", ""))
	require.NoError(t, batchCmd.Flags().Set("concurrency", "5"))
	require.NoError(t, batchCmd.Flags().Set("markdown", "false"))
	require.NoError(t, batchCmd.Flags().Set("continue-on-error", "false"))
	require.NoError(t, batchCmd.Flags().Set("timeout", "0s"))
}

// minimalHTML is a valid HTML article used by httptest servers in batch tests.
const minimalHTML = `<html><head><title>Page T</title></head><body><article><p>hello world content for extraction test</p></article></body></html>`

// TestRunBatch_ProducesJSONLForEachURL starts an httptest server returning
// minimal valid HTML and verifies that two URLs produce exactly two
// newline-terminated JSON objects, each with a non-empty title field.
//
// NOTE: no t.Parallel() — captureBatchOutput mutates os.Stdout.
func TestRunBatch_ProducesJSONLForEachURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, minimalHTML)
	}))
	defer ts.Close()

	urlA := ts.URL + "/a"
	urlB := ts.URL + "/b"

	input := strings.Join([]string{urlA, urlB}, "\n")
	f := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(f, []byte(input), 0o600))

	resetBatchFlags(t)
	t.Cleanup(func() { resetBatchFlags(t) })

	require.NoError(t, batchCmd.Flags().Set("input", f))
	require.NoError(t, batchCmd.Flags().Set("concurrency", "2"))

	stdout, err := captureBatchOutput(t, func() error {
		return runBatch(batchCmd, nil)
	})
	require.NoError(t, err)

	// Split on newline, drop empty trailing entry.
	lines := []string{}
	for _, l := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	assert.Len(t, lines, 2, "expected exactly 2 JSONL lines; got:\n%s", stdout)

	for i, line := range lines {
		var obj map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &obj), "line %d is not valid JSON: %s", i, line)
		title, ok := obj["title"]
		assert.True(t, ok, "line %d: missing 'title' key", i)
		assert.NotEmpty(t, title, "line %d: 'title' is empty", i)
	}
}

// TestRunBatch_NoURLsReturnsErrNoURLs verifies that an input file consisting
// only of blanks and comments returns ErrNoURLs with no stdout output.
//
// NOTE: no t.Parallel() — captureBatchOutput mutates os.Stdout.
func TestRunBatch_NoURLsReturnsErrNoURLs(t *testing.T) {
	input := "# comment\n\n   \n# another\n"
	f := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(f, []byte(input), 0o600))

	resetBatchFlags(t)
	t.Cleanup(func() { resetBatchFlags(t) })

	require.NoError(t, batchCmd.Flags().Set("input", f))

	stdout, err := captureBatchOutput(t, func() error {
		return runBatch(batchCmd, nil)
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoURLs), "expected ErrNoURLs, got: %v", err)
	assert.Empty(t, stdout)
}

// badURL returns a URL that produces a network-level error (connection
// refused) by starting a server, capturing its address, closing it, then
// returning a URL pointing at that now-closed address.
func badURL(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	addr := ts.URL
	ts.Close() // close immediately — any request will get connection refused
	return addr + "/bad"
}

// TestRunBatch_ContinueOnErrorMixedResults runs three URLs — two good, one
// with a network-level error (connection refused) — with --continue-on-error.
// All three produce a JSON line; the bad one has both "url" and "error" keys.
//
// NOTE: no t.Parallel() — captureBatchOutput mutates os.Stdout.
func TestRunBatch_ContinueOnErrorMixedResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, minimalHTML)
	}))
	defer ts.Close()

	good := ts.URL + "/good"
	bad := badURL(t)

	input := strings.Join([]string{good, bad, good}, "\n")
	f := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(f, []byte(input), 0o600))

	resetBatchFlags(t)
	t.Cleanup(func() { resetBatchFlags(t) })

	require.NoError(t, batchCmd.Flags().Set("input", f))
	require.NoError(t, batchCmd.Flags().Set("continue-on-error", "true"))

	stdout, err := captureBatchOutput(t, func() error {
		return runBatch(batchCmd, nil)
	})
	require.NoError(t, err)

	lines := []string{}
	for _, l := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	assert.Len(t, lines, 3, "expected 3 JSONL lines; got:\n%s", stdout)

	// Find the error line (has both "url" and "error" keys).
	foundErr := false
	for i, line := range lines {
		var obj map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &obj), "line %d is not valid JSON: %s", i, line)
		_, hasErr := obj["error"]
		_, hasURL := obj["url"]
		if hasErr {
			assert.True(t, hasURL, "line %d: error result missing 'url' key", i)
			assert.Equal(t, bad, obj["url"], "line %d: error result URL mismatch", i)
			foundErr = true
		}
	}
	assert.True(t, foundErr, "expected at least one error-shaped JSON line in:\n%s", stdout)
}

// TestRunBatch_AbortsOnFirstErrorWithoutContinueFlag puts a network-error URL
// first (connection refused), without --continue-on-error. Expects a non-nil
// error whose message contains the bad URL, and fewer JSON lines than total inputs.
//
// NOTE: no t.Parallel() — captureBatchOutput mutates os.Stdout.
func TestRunBatch_AbortsOnFirstErrorWithoutContinueFlag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, minimalHTML)
	}))
	defer ts.Close()

	bad := badURL(t)
	good := ts.URL + "/good"

	// bad URL first so the first error result aborts iteration.
	input := strings.Join([]string{bad, good, good}, "\n")
	f := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(f, []byte(input), 0o600))

	resetBatchFlags(t)
	t.Cleanup(func() { resetBatchFlags(t) })

	require.NoError(t, batchCmd.Flags().Set("input", f))
	// continue-on-error deliberately left false (default).

	stdout, err := captureBatchOutput(t, func() error {
		return runBatch(batchCmd, nil)
	})

	require.Error(t, err, "expected non-nil error when bad URL encountered without --continue-on-error")
	assert.Contains(t, err.Error(), bad, "error message should contain the bad URL")

	// Count complete JSON lines — should be fewer than 3 (all inputs).
	lines := []string{}
	for _, l := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	assert.Less(t, len(lines), 3, "expected fewer than 3 JSON lines on abort; got:\n%s", stdout)
}

// TestRunBatch_RejectsZeroConcurrency verifies that --concurrency 0 returns
// ErrInvalidConcurrency before any HTTP requests are made.
//
// NOTE: no t.Parallel() — captureBatchOutput mutates os.Stdout.
func TestRunBatch_RejectsZeroConcurrency(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, minimalHTML)
	}))
	defer ts.Close()

	input := ts.URL + "/page\n"
	f := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(f, []byte(input), 0o600))

	resetBatchFlags(t)
	t.Cleanup(func() { resetBatchFlags(t) })

	require.NoError(t, batchCmd.Flags().Set("input", f))
	require.NoError(t, batchCmd.Flags().Set("concurrency", "0"))

	stdout, err := captureBatchOutput(t, func() error {
		return runBatch(batchCmd, nil)
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidConcurrency), "expected ErrInvalidConcurrency, got: %v", err)
	assert.Empty(t, stdout)
}
