package main

// Regression tests for the parse command's base-URL selection (QA finding F-1):
// local file inputs must resolve relative URLs against a file:// URL derived
// from the input path, and stdin parsing must leave relative URLs untouched.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const urlBaseFixtureHTML = `<!doctype html><html><head><title>URL Base Fixture</title></head><body><article><p>Article body carrying relative references for base URL resolution testing, long enough to pass content scoring heuristics.</p><a href="/root/link">root</a> <a href="sibling/page.html">sibling</a> <img src="img/pic.png" alt="pic"></article></body></html>`

func writeURLBaseFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "page.html")
	require.NoError(t, os.WriteFile(path, []byte(urlBaseFixtureHTML), 0o600))
	return path
}

func TestSourceBaseURL(t *testing.T) {
	t.Parallel()

	rel, err := filepath.Abs(filepath.Join("some", "dir", "page.html"))
	require.NoError(t, err)

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "stdin has no base", in: "-", want: ""},
		{name: "empty has no base", in: "", want: ""},
		{name: "https passes through", in: "https://example.com/a/b.html", want: "https://example.com/a/b.html"},
		{name: "http passes through", in: "http://example.com/x", want: "http://example.com/x"},
		{name: "absolute file path", in: rel, want: "file://" + rel},
		{name: "relative file path", in: filepath.Join("some", "dir", "page.html"), want: "file://" + rel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, sourceBaseURL(tc.in))
		})
	}
}

func TestParseFileResolvesRelativeURLsAgainstFileBase(t *testing.T) {
	path := writeURLBaseFixture(t)
	dir := filepath.Dir(path)
	opts := &ParseOptions{Source: path, Timeout: 30 * time.Second, RenderWait: "load", RenderTimeout: 30 * time.Second}

	stdout, stderr, err := captureOutput(t, opts.Run)
	require.NoError(t, err)
	assert.Empty(t, stderr)

	assert.Contains(t, stdout, `href="file:///root/link"`)
	assert.Contains(t, stdout, `href="file://`+dir+`/sibling/page.html"`)
	assert.Contains(t, stdout, `src="file://`+dir+`/img/pic.png"`)
	assert.NotContains(t, stdout, `"img/pic.png"`)
}

func TestParseFileJSONMetadataUnaffectedByFileBase(t *testing.T) {
	path := writeURLBaseFixture(t)
	opts := &ParseOptions{Source: path, JSON: true, Timeout: 30 * time.Second, RenderWait: "load", RenderTimeout: 30 * time.Second}

	stdout, _, err := captureOutput(t, opts.Run)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "URL Base Fixture", result["title"])
}

func TestParseStdinLeavesRelativeURLsUntouched(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		r, w, err := os.Pipe()
		require.NoError(t, err)
		_, err = io.WriteString(w, urlBaseFixtureHTML)
		require.NoError(t, err)
		require.NoError(t, w.Close())
		original := os.Stdin
		os.Stdin = r
		defer func() { os.Stdin = original }()

		opts := &ParseOptions{Source: "-", Timeout: 30 * time.Second, RenderWait: "load", RenderTimeout: 30 * time.Second}
		return opts.Run()
	})
	require.NoError(t, err)

	assert.Contains(t, stdout, `href="/root/link"`)
	assert.Contains(t, stdout, `href="sibling/page.html"`)
	assert.Contains(t, stdout, `src="img/pic.png"`)
	assert.NotContains(t, stdout, "file://")
	assert.True(t, strings.Contains(stdout, "<article"), "sanity: content was extracted")
}
