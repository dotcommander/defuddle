package main

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
	"time"

	"github.com/alecthomas/kong"
	"github.com/dotcommander/defuddle/extractors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var stdioCaptureMu sync.Mutex

const fixtureHTML = `<!doctype html><html><head><title>Fixture Title</title></head><body><article><p>Article body for parse cmd integration test, long enough to pass scoring.</p></article></body></html>`

func captureOutput(t *testing.T, fn func() error) (stdout, stderr string, runErr error) {
	t.Helper()
	stdioCaptureMu.Lock()
	defer stdioCaptureMu.Unlock()

	originalOut, originalErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	require.NoError(t, err)
	rErr, wErr, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout, os.Stderr = wOut, wErr
	defer func() {
		os.Stdout, os.Stderr = originalOut, originalErr
	}()

	runErr = fn()
	require.NoError(t, wOut.Close())
	require.NoError(t, wErr.Close())
	outBytes, err := io.ReadAll(rOut)
	require.NoError(t, err)
	errBytes, err := io.ReadAll(rErr)
	require.NoError(t, err)
	os.Stdout, os.Stderr = originalOut, originalErr
	return string(outBytes), string(errBytes), runErr
}

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.html")
	require.NoError(t, os.WriteFile(path, []byte(fixtureHTML), 0o600))
	return path
}

func nonEmptyLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseCLI(t *testing.T, args ...string) (*CLI, *kong.Context) {
	t.Helper()
	cli := &CLI{}
	parser, err := kong.New(cli, kong.Name("defuddle"), kong.Vars{"version": "test"})
	require.NoError(t, err)
	ctx, err := parser.Parse(args)
	require.NoError(t, err)
	return cli, ctx
}

func TestKongParseCommandFlagsAndAlias(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "article.html")
	cli, ctx := parseCLI(t, "p", path, "-j", "-m", "--md", "-p", "title", "-o", "out", "-H", "X-Test: one", "--timeout", "2s", "--js")

	assert.Equal(t, "parse <source>", ctx.Command())
	assert.Equal(t, path, cli.Parse.Source)
	assert.True(t, cli.Parse.JSON)
	assert.True(t, cli.Parse.Markdown)
	assert.True(t, cli.Parse.MD)
	assert.Equal(t, "title", cli.Parse.Property)
	assert.Equal(t, "out", cli.Parse.Output)
	assert.Equal(t, []string{"X-Test: one"}, cli.Parse.Headers)
	assert.Equal(t, 2*time.Second, cli.Parse.Timeout)
	assert.True(t, cli.Parse.JS)
}

func TestKongBatchDefaultsAndFlags(t *testing.T) {
	t.Parallel()
	cli, _ := parseCLI(t, "batch", "-i", "urls.txt", "-c", "2", "-m", "--continue-on-error", "--timeout", "1500ms")
	assert.Equal(t, "urls.txt", cli.Batch.Input)
	assert.Equal(t, 2, cli.Batch.Concurrency)
	assert.True(t, cli.Batch.Markdown)
	assert.True(t, cli.Batch.ContinueOnError)
	assert.Equal(t, 1500*time.Millisecond, cli.Batch.Timeout)

	defaults, _ := parseCLI(t, "batch")
	assert.Equal(t, 5, defaults.Batch.Concurrency)
}

func TestParseOptionsFileJSON(t *testing.T) {
	path := writeFixture(t)
	opts := &ParseOptions{Source: path, JSON: true, Timeout: 30 * time.Second, RenderWait: "load", RenderTimeout: 30 * time.Second}

	stdout, stderr, err := captureOutput(t, opts.Run)
	require.NoError(t, err)
	assert.Empty(t, stderr)
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "Fixture Title", result["title"])
}

func TestParseOptionsFileOutputModes(t *testing.T) {
	path := writeFixture(t)
	tests := []struct {
		name string
		opts ParseOptions
		want string
	}{
		{name: "html", opts: ParseOptions{Source: path}, want: "Article body for parse cmd integration test"},
		{name: "markdown", opts: ParseOptions{Source: path, Markdown: true}, want: "Article body for parse cmd integration test"},
		{name: "property", opts: ParseOptions{Source: path, Property: "title"}, want: "Fixture Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := captureOutput(t, tt.opts.Run)
			require.NoError(t, err)
			assert.Empty(t, stderr)
			assert.Contains(t, stdout, tt.want)
			if tt.name == "markdown" {
				assert.NotContains(t, stdout, "<article>")
			}
		})
	}
}

func TestParseOptionsPropertyNotFound(t *testing.T) {
	opts := &ParseOptions{Source: writeFixture(t), Property: "nonsense"}
	_, _, err := captureOutput(t, opts.Run)
	assert.ErrorIs(t, err, ErrPropertyNotFound)
}

func TestParseOptionsOutputFile(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "out.html")
	opts := &ParseOptions{Source: writeFixture(t), Output: outPath}
	stdout, stderr, err := captureOutput(t, opts.Run)
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, outPath)
	contents, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "Article body for parse cmd integration test")
}

func TestParseOptionsLocalURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, fixtureHTML)
	}))
	t.Cleanup(server.Close)

	stdout, _, err := captureOutput(t, (&ParseOptions{Source: server.URL}).Run)
	require.NoError(t, err)
	assert.Contains(t, stdout, "Article body for parse cmd integration test")
}

func TestParseOptionsNoSource(t *testing.T) {
	t.Parallel()
	if isStdinPiped() {
		t.Skip("stdin is piped")
	}
	err := (&ParseOptions{}).Run()
	assert.True(t, errors.Is(err, ErrParseUsage))
}

func TestBatchOptionsNoURLsAndConcurrencyValidation(t *testing.T) {
	t.Parallel()
	empty := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(empty, []byte("# none\n"), 0o600))
	err := (&BatchOptions{Input: empty, Concurrency: 5}).Run()
	assert.True(t, errors.Is(err, ErrNoURLs))

	err = (&BatchOptions{Input: empty, Concurrency: 0}).Run()
	assert.True(t, errors.Is(err, ErrInvalidConcurrency))
}

func TestExtractorsOptionsMatchValidation(t *testing.T) {
	t.Parallel()
	err := (&ExtractorsOptions{Match: ":bad-url"}).Run()
	assert.True(t, errors.Is(err, ErrInvalidMatchURL))
}

func TestExtractorsOptionsListAndMatch(t *testing.T) {
	extractors.InitializeBuiltins()

	stdout, stderr, err := captureOutput(t, (&ExtractorsOptions{}).Run)
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "github.com")
	assert.Contains(t, stdout, "DOM-gated catchall (Discourse, Mastodon)")

	stdout, stderr, err = captureOutput(t, (&ExtractorsOptions{Match: "https://github.com/dotcommander/defuddle/issues/1"}).Run)
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.True(t, strings.HasPrefix(stdout, "MATCH:"))
	assert.Contains(t, stdout, "github.com")
}

func TestExtractorsOptionsNoMatch(t *testing.T) {
	stdout, stderr, err := captureOutput(t, (&ExtractorsOptions{Match: "https://no-extractor-for-this.example.invalid/path"}).Run)
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "no URL-specific extractor matches the given URL")
}

func TestBatchOptionsProducesJSONL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, fixtureHTML)
	}))
	t.Cleanup(server.Close)
	input := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(input, []byte(server.URL+"/a\n"+server.URL+"/b\n"), 0o600))

	stdout, stderr, err := captureOutput(t, (&BatchOptions{Input: input, Concurrency: 2}).Run)
	require.NoError(t, err)
	assert.Empty(t, stderr)
	lines := nonEmptyLines(stdout)
	require.Len(t, lines, 2)
	for _, line := range lines {
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &result))
		assert.Equal(t, "Fixture Title", result["title"])
	}
}

func TestBatchOptionsContinueOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, fixtureHTML)
	}))
	t.Cleanup(server.Close)
	badServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	badURL := badServer.URL + "/bad"
	badServer.Close()
	input := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(input, []byte(server.URL+"/good\n"+badURL+"\n"), 0o600))

	stdout, _, err := captureOutput(t, (&BatchOptions{Input: input, Concurrency: 2, ContinueOnError: true}).Run)
	require.NoError(t, err)
	lines := nonEmptyLines(stdout)
	require.Len(t, lines, 2)
	var foundError bool
	for _, line := range lines {
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &result))
		if result["error"] != nil {
			assert.Equal(t, badURL, result["url"])
			foundError = true
		}
	}
	assert.True(t, foundError)
}

func TestBatchOptionsStopsOnError(t *testing.T) {
	badServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	badURL := badServer.URL + "/bad"
	badServer.Close()
	input := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(input, []byte(badURL+"\n"), 0o600))

	stdout, _, err := captureOutput(t, (&BatchOptions{Input: input, Concurrency: 1}).Run)
	require.Error(t, err)
	assert.Contains(t, err.Error(), badURL)
	assert.Empty(t, stdout)
}

func TestParseOptionsRenderAuto(t *testing.T) {
	t.Run("static page does not render", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, fixtureHTML)
		}))
		t.Cleanup(server.Close)
		stdout, stderr, err := captureOutput(t, (&ParseOptions{Source: server.URL, RenderAuto: true}).Run)
		require.NoError(t, err)
		assert.Empty(t, stderr)
		assert.Contains(t, stdout, "Article body for parse cmd integration test")
	})

	t.Run("shell degrades without chrome", func(t *testing.T) {
		shellPage := `<!doctype html><html><head><title>App</title></head><body><div id="root"></div><noscript>You need to enable JavaScript to run this app.</noscript><script src="/main.js"></script></body></html>`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, shellPage)
		}))
		t.Cleanup(server.Close)
		opts := &ParseOptions{Source: server.URL, RenderAuto: true, ChromePath: filepath.Join(t.TempDir(), "no-such-chrome")}
		_, stderr, err := captureOutput(t, opts.Run)
		require.NoError(t, err)
		assert.Contains(t, stderr, "auto-render skipped")
	})
}

func TestKongRejectsUnknownCommandAndExtraParseArgument(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"unknown"}, {"parse", "one", "two"}} {
		cli := &CLI{}
		parser, err := kong.New(cli, kong.Name("defuddle"), kong.Vars{"version": "test"})
		require.NoError(t, err)
		_, err = parser.Parse(args)
		assert.Error(t, err)
	}
}
