package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A non-shell page under --render-auto must NOT escalate: it is fetched once and
// parsed as plain HTML (no Chrome involved), producing normal output.
func TestParseCmd_RenderAuto_StaticPageNoRender(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, fixtureHTML)
	}))
	defer ts.Close()

	cmd := newParseCmd()
	require.NoError(t, cmd.Flags().Set("render-auto", "true"))

	stdout, _, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{ts.URL})
	})

	require.NoError(t, err)
	assert.Contains(t, stdout, "Article body for parse cmd integration test")
}

// A shell page under --render-auto with no usable Chrome must degrade
// gracefully (no error, stderr note) rather than hard-fail like explicit --render.
// A bogus --chrome-path forces the render failure deterministically regardless of
// whether a real Chrome exists in the test environment.
func TestParseCmd_RenderAuto_ShellDegradesWithoutChrome(t *testing.T) {
	t.Parallel()
	shellPage := `<!doctype html><html><head><title>App</title></head><body><div id="root"></div><noscript>You need to enable JavaScript to run this app.</noscript><script src="/main.js"></script></body></html>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, shellPage)
	}))
	defer ts.Close()

	cmd := newParseCmd()
	require.NoError(t, cmd.Flags().Set("render-auto", "true"))
	require.NoError(t, cmd.Flags().Set("chrome-path", filepath.Join(t.TempDir(), "no-such-chrome")))

	_, stderr, err := captureParseOutput(t, func() error {
		return parseContent(cmd, []string{ts.URL})
	})

	require.NoError(t, err, "auto mode must not hard-fail when Chrome is missing")
	assert.Contains(t, stderr, "auto-render skipped")
}
