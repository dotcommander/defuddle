package main

// Tests for the parse cobra subcommand's RunE handler (parseContent).
//
// parseContent → executeParseContent → renderOutput → writeOutput.
// writeOutput calls fmt.Print / fmt.Fprintf(os.Stderr) directly, so these
// tests swap os.Stdout and os.Stderr via os.Pipe().
// captureParseOutput mutates os.Stdout/os.Stderr — callers must NOT t.Parallel().

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
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureHTML is a minimal valid HTML article used across parse tests.
const fixtureHTML = `<!doctype html><html><head><title>Fixture Title</title></head><body><article><p>Article body for parse cmd integration test, long enough to pass scoring.</p></article></body></html>`

// captureParseOutput swaps os.Stdout and os.Stderr for the duration of fn,
// returning everything written to each.
//
// WARNING: mutates os.Stdout and os.Stderr — callers must NOT call t.Parallel().
func captureParseOutput(t *testing.T, fn func() error) (stdout, stderr string, err error) {
	t.Helper()

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

	wOut.Close()
	wErr.Close()

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)
	return string(outBytes), string(errBytes), err
}

// resetParseFlags restores all parse flags to their defaults so sequential
// tests start from a clean state.
//
// The "header" flag is a StringArray; pflag.SliceValue.Replace is used to
// empty it cleanly — Flags().Set("header", "") would add an empty string
// entry that fails header parsing.
func resetParseFlags(t *testing.T) {
	t.Helper()
	require.NoError(t, parseCmd.Flags().Set("json", "false"))
	require.NoError(t, parseCmd.Flags().Set("markdown", "false"))
	require.NoError(t, parseCmd.Flags().Set("md", "false"))
	require.NoError(t, parseCmd.Flags().Set("property", ""))
	require.NoError(t, parseCmd.Flags().Set("output", ""))
	require.NoError(t, parseCmd.Flags().Set("user-agent", ""))
	// StringArray flags cannot be reset via Set("", ...) — use SliceValue.Replace.
	hf := parseCmd.Flags().Lookup("header")
	require.NotNil(t, hf)
	require.NoError(t, hf.Value.(pflag.SliceValue).Replace([]string{}))
	require.NoError(t, parseCmd.Flags().Set("timeout", "30s"))
	require.NoError(t, parseCmd.Flags().Set("debug", "false"))
	require.NoError(t, parseCmd.Flags().Set("proxy", ""))
	require.NoError(t, parseCmd.Flags().Set("remove-images", "false"))
	require.NoError(t, parseCmd.Flags().Set("content-selector", ""))
	require.NoError(t, parseCmd.Flags().Set("no-clutter-removal", "false"))
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
// NOTE: no t.Parallel() — captureParseOutput mutates os.Stdout/os.Stderr.
func TestParseCmd_FileInput_DefaultOutput(t *testing.T) {
	path := writeFixture(t)

	resetParseFlags(t)
	defer resetParseFlags(t)

	stdout, _, err := captureParseOutput(t, func() error {
		return parseContent(parseCmd, []string{path})
	})

	require.NoError(t, err)
	assert.Contains(t, stdout, "Article body for parse cmd integration test")
}

// TestParseCmd_FileInput_JSONOutput verifies that --json produces valid JSON
// with a non-empty title field matching the fixture.
//
// NOTE: no t.Parallel() — captureParseOutput mutates os.Stdout/os.Stderr.
func TestParseCmd_FileInput_JSONOutput(t *testing.T) {
	path := writeFixture(t)

	resetParseFlags(t)
	defer resetParseFlags(t)

	require.NoError(t, parseCmd.Flags().Set("json", "true"))

	stdout, _, err := captureParseOutput(t, func() error {
		return parseContent(parseCmd, []string{path})
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
// NOTE: no t.Parallel() — captureParseOutput mutates os.Stdout/os.Stderr.
func TestParseCmd_FileInput_MarkdownOutput(t *testing.T) {
	path := writeFixture(t)

	resetParseFlags(t)
	defer resetParseFlags(t)

	require.NoError(t, parseCmd.Flags().Set("markdown", "true"))

	stdout, _, err := captureParseOutput(t, func() error {
		return parseContent(parseCmd, []string{path})
	})

	require.NoError(t, err)
	assert.NotEmpty(t, stdout)
	assert.Contains(t, stdout, "Article body for parse cmd integration test")
	assert.NotContains(t, stdout, "<article>", "markdown output should not contain raw <article> tag")
}

// TestParseCmd_FileInput_PropertyTitle verifies that --property title prints
// only the title to stdout with no stderr output.
//
// NOTE: no t.Parallel() — captureParseOutput mutates os.Stdout/os.Stderr.
func TestParseCmd_FileInput_PropertyTitle(t *testing.T) {
	path := writeFixture(t)

	resetParseFlags(t)
	defer resetParseFlags(t)

	require.NoError(t, parseCmd.Flags().Set("property", "title"))

	stdout, stderr, err := captureParseOutput(t, func() error {
		return parseContent(parseCmd, []string{path})
	})

	require.NoError(t, err)
	assert.Equal(t, "Fixture Title", strings.TrimSpace(stdout))
	assert.Empty(t, stderr)
}

// TestParseCmd_FileInput_PropertyUnknown verifies that --property with an
// unknown name returns ErrPropertyNotFound.
//
// NOTE: no t.Parallel() — captureParseOutput mutates os.Stdout/os.Stderr.
func TestParseCmd_FileInput_PropertyUnknown(t *testing.T) {
	path := writeFixture(t)

	resetParseFlags(t)
	defer resetParseFlags(t)

	require.NoError(t, parseCmd.Flags().Set("property", "nonsense"))

	_, _, err := captureParseOutput(t, func() error {
		return parseContent(parseCmd, []string{path})
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPropertyNotFound), "expected ErrPropertyNotFound, got: %v", err)
}

// TestParseCmd_FileInput_OutputFile verifies that --output writes the result to
// the specified file, produces no stdout, and writes a confirmation to stderr.
//
// NOTE: no t.Parallel() — captureParseOutput mutates os.Stdout/os.Stderr.
func TestParseCmd_FileInput_OutputFile(t *testing.T) {
	path := writeFixture(t)
	outPath := filepath.Join(t.TempDir(), "out.html")

	resetParseFlags(t)
	defer resetParseFlags(t)

	require.NoError(t, parseCmd.Flags().Set("output", outPath))

	stdout, stderr, err := captureParseOutput(t, func() error {
		return parseContent(parseCmd, []string{path})
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
// NOTE: no t.Parallel() — captureParseOutput mutates os.Stdout/os.Stderr.
func TestParseCmd_URLInput_HTTPTestServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, fixtureHTML)
	}))
	defer ts.Close()

	resetParseFlags(t)
	defer resetParseFlags(t)

	stdout, _, err := captureParseOutput(t, func() error {
		return parseContent(parseCmd, []string{ts.URL})
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
// NOTE: no t.Parallel() — captureParseOutput mutates os.Stdout/os.Stderr.
func TestParseCmd_NoSourceNoStdin_ReturnsUsageError(t *testing.T) {
	if isStdinPiped() {
		t.Skip("stdin is piped in this environment; skipping ErrParseUsage test")
	}

	resetParseFlags(t)
	defer resetParseFlags(t)

	_, _, err := captureParseOutput(t, func() error {
		return parseContent(parseCmd, []string{})
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrParseUsage), "expected ErrParseUsage, got: %v", err)
}
