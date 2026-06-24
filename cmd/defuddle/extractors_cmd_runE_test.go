package main

// Tests for the extractors cobra subcommand's RunE handler.
//
// The handler writes to os.Stdout/os.Stderr directly (not cmd.OutOrStdout()),
// so these tests swap the global os.Stdout/os.Stderr via os.Pipe().
// captureOutput serializes that process-wide mutation.

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/dotcommander/defuddle/extractors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureOutput swaps os.Stdout and os.Stderr for the duration of fn,
// returning everything each received.
//
// WARNING: mutates os.Stdout and os.Stderr under stdioCaptureMu.
func captureOutput(t *testing.T, fn func() error) (stdout, stderr string, err error) {
	t.Helper()

	stdioCaptureMu.Lock()
	defer stdioCaptureMu.Unlock()

	origOut, origErr := os.Stdout, os.Stderr

	rOut, wOut, pipeErr := os.Pipe()
	require.NoError(t, pipeErr, "os.Pipe stdout")

	rErr, wErr, pipeErr := os.Pipe()
	require.NoError(t, pipeErr, "os.Pipe stderr")

	os.Stdout, os.Stderr = wOut, wErr
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	err = fn()

	_ = wOut.Close()
	_ = wErr.Close()

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)

	return string(outBytes), string(errBytes), err
}

// TestExtractorsCmd_RunE_ListMode verifies that running extractors with no
// --match flag prints >0 lines including a known extractor and the catchall label.
//
// NOTE: captureOutput serializes os.Stdout/os.Stderr mutation.
func TestExtractorsCmd_RunE_ListMode(t *testing.T) {
	t.Parallel()
	extractors.InitializeBuiltins()
	cmd := newExtractorsCmd()

	stdout, stderr, err := captureOutput(t, func() error {
		return cmd.RunE(cmd, nil)
	})

	require.NoError(t, err)
	assert.Empty(t, stderr)

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	assert.Greater(t, len(lines), 0, "expected at least one output line")
	assert.True(t, strings.Contains(stdout, "github.com"),
		"expected stdout to contain 'github.com', got: %s", stdout)
	assert.True(t, strings.Contains(stdout, "DOM-gated catchall (Discourse, Mastodon)"),
		"expected stdout to contain catchall label, got: %s", stdout)
}

// TestExtractorsCmd_RunE_MatchSuccess verifies that --match with a URL that has
// a site-specific extractor prints a MATCH: line to stdout and nothing to stderr.
//
// NOTE: captureOutput serializes os.Stdout/os.Stderr mutation.
func TestExtractorsCmd_RunE_MatchSuccess(t *testing.T) {
	t.Parallel()
	extractors.InitializeBuiltins()
	cmd := newExtractorsCmd()

	require.NoError(t, cmd.Flags().Set("match", "https://github.com/dotcommander/defuddle/issues/1"))

	stdout, stderr, err := captureOutput(t, func() error {
		return cmd.RunE(cmd, nil)
	})

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.True(t, strings.HasPrefix(stdout, "MATCH:"),
		"expected stdout to begin with 'MATCH:', got: %q", stdout)
	assert.True(t, strings.Contains(stdout, "github.com"),
		"expected stdout to contain 'github.com', got: %q", stdout)
}

// TestExtractorsCmd_RunE_MatchInvalidURL verifies that --match with a malformed
// URL returns ErrInvalidMatchURL without printing anything.
//
// NOTE: captureOutput serializes os.Stdout/os.Stderr mutation.
func TestExtractorsCmd_RunE_MatchInvalidURL(t *testing.T) {
	t.Parallel()
	extractors.InitializeBuiltins()
	cmd := newExtractorsCmd()

	require.NoError(t, cmd.Flags().Set("match", ":bad-url"))

	stdout, stderr, err := captureOutput(t, func() error {
		return cmd.RunE(cmd, nil)
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidMatchURL),
		"expected ErrInvalidMatchURL, got: %v", err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

// TestExtractorsCmd_RunE_MatchNoMatch verifies that --match with a URL that has
// no registered extractor returns nil, writes nothing to stdout, and writes the
// "no URL-specific extractor" message to stderr.
//
// NOTE: captureOutput serializes os.Stdout/os.Stderr mutation.
func TestExtractorsCmd_RunE_MatchNoMatch(t *testing.T) {
	t.Parallel()
	extractors.InitializeBuiltins()
	cmd := newExtractorsCmd()

	require.NoError(t, cmd.Flags().Set("match", "https://no-extractor-for-this.example.invalid/path"))

	stdout, stderr, err := captureOutput(t, func() error {
		return cmd.RunE(cmd, nil)
	})

	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "no URL-specific extractor matches the given URL")
}
