package main

// Tests for the parse cobra subcommand's RunE handler (parseContent).
//
// parseContent → executeParseContent → renderOutput → writeOutput.
// writeOutput calls fmt.Print / fmt.Fprintf(os.Stderr) directly, so these
// tests swap os.Stdout and os.Stderr via os.Pipe().
// captureParseOutput serializes that process-wide mutation.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureHTML is a minimal valid HTML article used across parse tests.
const fixtureHTML = `<!doctype html><html><head><title>Fixture Title</title></head><body><article><p>Article body for parse cmd integration test, long enough to pass scoring.</p></article></body></html>`

var stdioCaptureMu sync.Mutex

// captureParseOutput swaps os.Stdout and os.Stderr for the duration of fn,
// returning everything written to each.
//
// WARNING: mutates os.Stdout and os.Stderr under stdioCaptureMu.
func captureParseOutput(t *testing.T, fn func() error) (stdout, stderr string, err error) {
	t.Helper()

	stdioCaptureMu.Lock()
	defer stdioCaptureMu.Unlock()

	origOut := os.Stdout
	origErr := os.Stderr

	rOut, wOut, pipeErr := os.Pipe()
	require.NoError(t, pipeErr, "os.Pipe stdout")
	rErr, wErr, pipeErr := os.Pipe()
	require.NoError(t, pipeErr, "os.Pipe stderr")

	os.Stdout = wOut
	os.Stderr = wErr
	defer func() {
		os.Stdout = origOut
		os.Stderr = origErr
	}()

	err = fn()

	_ = wOut.Close()
	_ = wErr.Close()

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)
	return string(outBytes), string(errBytes), err
}

// writeFixture writes fixtureHTML to a temp file and returns its path.
func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.html")
	require.NoError(t, os.WriteFile(path, []byte(fixtureHTML), 0o600))
	return path
}

// TestParseCmd_FileInput_DefaultOutput verifies that parsing a local HTML file
// with no flags prints the extracted article content to stdout.
//
// NOTE: captureParseOutput serializes os.Stdout/os.Stderr mutation.
func TestParseCmd_FileInput_DefaultOutput(t *testing.T) {
	t.Parallel()
	path := writeFixture(t)

	cmd := newParseCmd()

	stdout, _, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{path})
	})

	require.NoError(t, err)
	assert.Contains(t, stdout, "Article body for parse cmd integration test")
}

// TestParseCmd_FileInput_JSONOutput verifies that --json produces valid JSON
// with a non-empty title field matching the fixture.
//
// NOTE: captureParseOutput serializes os.Stdout/os.Stderr mutation.
func TestParseCmd_FileInput_JSONOutput(t *testing.T) {
	t.Parallel()
	path := writeFixture(t)

	cmd := newParseCmd()

	require.NoError(t, cmd.Flags().Set("json", "true"))

	stdout, _, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{path})
	})

	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &obj), "stdout is not valid JSON: %s", stdout)

	title, ok := obj["title"]
	require.True(t, ok, "JSON response missing 'title' key")
	assert.Equal(t, "Fixture Title", title)
}

// TestParseCmd_FileInput_MarkdownOutput verifies that --markdown produces
// markdown output containing the article text but not raw HTML tags.
//
// NOTE: captureParseOutput serializes os.Stdout/os.Stderr mutation.
func TestParseCmd_FileInput_MarkdownOutput(t *testing.T) {
	t.Parallel()
	path := writeFixture(t)

	cmd := newParseCmd()

	require.NoError(t, cmd.Flags().Set("markdown", "true"))

	stdout, _, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{path})
	})

	require.NoError(t, err)
	assert.NotEmpty(t, stdout)
	assert.Contains(t, stdout, "Article body for parse cmd integration test")
	assert.NotContains(t, stdout, "<article>", "markdown output should not contain raw <article> tag")
}

// TestParseCmd_FileInput_PropertyTitle verifies that --property title prints
// only the title to stdout with no stderr output.
//
// NOTE: captureParseOutput serializes os.Stdout/os.Stderr mutation.
func TestParseCmd_FileInput_PropertyTitle(t *testing.T) {
	t.Parallel()
	path := writeFixture(t)

	cmd := newParseCmd()

	require.NoError(t, cmd.Flags().Set("property", "title"))

	stdout, stderr, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{path})
	})

	require.NoError(t, err)
	assert.Equal(t, "Fixture Title", strings.TrimSpace(stdout))
	assert.Empty(t, stderr)
}

// TestParseCmd_FileInput_PropertyUnknown verifies that --property with an
// unknown name returns ErrPropertyNotFound.
//
// NOTE: captureParseOutput serializes os.Stdout/os.Stderr mutation.
func TestParseCmd_FileInput_PropertyUnknown(t *testing.T) {
	t.Parallel()
	path := writeFixture(t)

	cmd := newParseCmd()

	require.NoError(t, cmd.Flags().Set("property", "nonsense"))

	_, _, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{path})
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPropertyNotFound), "expected ErrPropertyNotFound, got: %v", err)
}

// TestParseCmd_FileInput_OutputFile verifies that --output writes the result to
// the specified file, produces no stdout, and writes a confirmation to stderr.
//
// NOTE: captureParseOutput serializes os.Stdout/os.Stderr mutation.
func TestParseCmd_FileInput_OutputFile(t *testing.T) {
	t.Parallel()
	path := writeFixture(t)
	outPath := filepath.Join(t.TempDir(), "out.html")

	cmd := newParseCmd()

	require.NoError(t, cmd.Flags().Set("output", outPath))

	stdout, stderr, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{path})
	})

	require.NoError(t, err)
	assert.Empty(t, stdout, "expected no stdout when --output is set")
	assert.Contains(t, stderr, outPath, "expected stderr to confirm output path")

	fileBytes, readErr := os.ReadFile(outPath)
	require.NoError(t, readErr, "output file should exist")
	assert.Contains(t, string(fileBytes), "Article body for parse cmd integration test")
}

// TestParseCmd_URLInput_HTTPTestServer starts a local httptest server serving
// the fixture HTML and verifies that parseContent fetches and extracts it.
//
// NOTE: captureParseOutput serializes os.Stdout/os.Stderr mutation.
func TestParseCmd_URLInput_HTTPTestServer(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, fixtureHTML)
	}))
	defer ts.Close()

	cmd := newParseCmd()

	stdout, _, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{ts.URL})
	})

	require.NoError(t, err)
	assert.Contains(t, stdout, "Article body for parse cmd integration test")
}

// TestParseCmd_NoSourceNoStdin_ReturnsUsageError verifies that invoking
// parseContent with no args and no piped stdin returns ErrParseUsage.
//
// If the test runner itself has a piped stdin (some CI setups), the handler
// will attempt to read it instead of returning the error, so we skip in that case.
//
// NOTE: captureParseOutput serializes os.Stdout/os.Stderr mutation.
func TestParseCmd_NoSourceNoStdin_ReturnsUsageError(t *testing.T) {
	t.Parallel()
	if isStdinPiped() {
		t.Skip("stdin is piped in this environment; skipping ErrParseUsage test")
	}

	cmd := newParseCmd()

	_, _, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{})
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrParseUsage), "expected ErrParseUsage, got: %v", err)
}
