package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dotcommander/defuddle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateRenderWait verifies the accepted --render-wait values: empty is
// valid (path default), load/networkidle are valid, anything else is a
// validation error.
func TestValidateRenderWait(t *testing.T) {
	t.Parallel()

	for _, v := range []string{"", "load", "networkidle"} {
		require.NoError(t, validateRenderWait(v), "validateRenderWait(%q)", v)
	}

	err := validateRenderWait("bogus")
	require.ErrorIs(t, err, ErrInvalidRenderWait)
	assert.Contains(t, err.Error(), `bogus`)
	assert.Equal(t, exitValidation, exitCodeFor(err))
}

// TestEffectiveAutoRenderOpts verifies the auto-path default: an unset
// --render-wait escalates to networkidle, explicit values are preserved.
func TestEffectiveAutoRenderOpts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"", "networkidle"},
		{"load", "load"},
		{"networkidle", "networkidle"},
	}
	for _, tt := range cases {
		opts := &ParseOptions{Source: "https://example.com/a", RenderWait: tt.in, ChromePath: "/x"}
		got := effectiveAutoRenderOpts(opts)
		assert.Equal(t, tt.want, got.RenderWait, "RenderWait in=%q", tt.in)
		// Unrelated fields must carry over; the source struct stays untouched.
		assert.Equal(t, "https://example.com/a", got.Source)
		assert.Equal(t, "/x", got.ChromePath)
		assert.Equal(t, tt.in, opts.RenderWait, "effectiveAutoRenderOpts must not mutate its input")
	}
}

// TestRenderFlagsWarnOnNonURLSource verifies that render flags on file/stdin
// sources warn on stderr naming the ignored flag(s), and stay silent when no
// render flag is set.
func TestRenderFlagsWarnOnNonURLSource(t *testing.T) {
	// Not parallel: captureOutput swaps process-global stdio (see the
	// captureOutput-using tests in cli_test.go for the same convention).

	cases := []struct {
		name       string
		render     bool
		renderAuto bool
		wantFlag   string
	}{
		{"render on stdin", true, false, "--render"},
		{"render-auto on stdin", false, true, "--render-auto"},
		{"both on stdin", true, true, "--render/--render-auto"},
		{"render on file", true, false, "--render"},
	}
	for _, tt := range cases {
		// Not parallel: captureOutput swaps process-global stdio, serialized by
		// stdioCaptureMu against other capture users but not against tests that
		// read os.Stdout/os.Stderr directly (e.g. kong.New).
		t.Run(tt.name, func(t *testing.T) {
			source := "-"
			if tt.name == "render on file" {
				source = writeFixture(t)
			}
			opts := &ParseOptions{
				Source:     source,
				Render:     tt.render,
				RenderAuto: tt.renderAuto,
			}
			_, stderr, err := captureOutput(t, func() error {
				_, err := loadResult(context.Background(), opts, &defuddle.Options{})
				return err
			})
			require.NoError(t, err)
			assert.Contains(t, stderr, "ignoring "+tt.wantFlag)
			assert.Contains(t, stderr, "requires an http(s) URL source")
		})
	}

	t.Run("no render flags stay silent", func(t *testing.T) {
		opts := &ParseOptions{Source: writeFixture(t)}
		_, stderr, err := captureOutput(t, func() error {
			_, err := loadResult(context.Background(), opts, &defuddle.Options{})
			return err
		})
		require.NoError(t, err)
		assert.Empty(t, stderr)
	})
}

// TestWriteOutputErrorClassification verifies that a failing --output write
// (e.g. into a missing directory) classifies as validation (exit 2), not
// not_found.
func TestWriteOutputErrorClassification(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing-dir", "out.html")
	err := writeOutput(missing, "x")
	require.ErrorIs(t, err, ErrOutputWrite)
	assert.Equal(t, exitValidation, exitCodeFor(err))
}

// TestPropertyOutputEndsWithNewline verifies the --property output shape:
// value plus a single trailing newline, matching the upstream CLI.
func TestPropertyOutputEndsWithNewline(t *testing.T) {
	t.Parallel()

	out, err := renderOutput(&defuddle.Result{Metadata: defuddle.Metadata{Title: "T"}}, &ParseOptions{Property: "title"})
	require.NoError(t, err)
	assert.Equal(t, "T\n", out)
}
