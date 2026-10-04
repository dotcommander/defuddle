package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dotcommander/defuddle"
)

const remediationShell = `<!doctype html><html><head><title>App</title></head><body><div id="root"></div><noscript>You need to enable JavaScript to run this app.</noscript><script src="/main.js"></script></body></html>`

func TestCLIStaticRoutesUseFinalResponseBase(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/article/final", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Base fixture</title></head><body><article><p>` + strings.Repeat("Readable article prose with enough useful detail. ", 30) + `<a href="child">linked article</a><img src="photo.jpg" alt="Photo"></p></article></body></html>`))
	}))
	defer server.Close()
	for _, auto := range []bool{false, true} {
		opts := &ParseOptions{Source: server.URL + "/start", RenderAuto: auto, ContentSelector: "article", Timeout: time.Second}
		result, err := loadResult(t.Context(), opts, buildDefuddleOptions(opts))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, server.URL+"/article/child") || !strings.Contains(result.Content, server.URL+"/article/photo.jpg") {
			t.Fatalf("auto=%t content=%s", auto, result.Content)
		}
	}
}

func TestCLIStageContextsAreIndependent(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	fetch, stopFetch := buildContext(parent, time.Hour)
	stopFetch()
	if !errors.Is(fetch.Err(), context.Canceled) {
		t.Fatal("fetch child still alive")
	}
	render, stopRender := buildContext(parent, time.Hour)
	defer stopRender()
	if render.Err() != nil || parent.Err() != nil {
		t.Fatal("fetch cancellation affected independent stage")
	}
	cancel()
	if !errors.Is(render.Err(), context.Canceled) {
		t.Fatal("parent cancellation did not reach render")
	}
}

func TestCLIAutoRenderFallbackAndRenderedBase(t *testing.T) {
	for _, tc := range []struct {
		name      string
		renderErr error
	}{
		{"cap", defuddle.ErrTooLarge},
		{"deadline", errors.Join(defuddle.ErrTimeout, context.DeadlineExceeded)},
		{"success", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := &ParseOptions{Source: "https://source.example/start"}
			document := fetchedDocument{HTML: remediationShell, URL: "https://final.example/article"}
			var result *defuddle.Result
			_, _, err := captureOutput(t, func() error {
				var err error
				result, err = parseAutoDocument(t.Context(), opts, buildDefuddleOptions(opts), document, func(ctx context.Context, _ *ParseOptions) (string, error) {
					if ctx.Err() != nil {
						t.Fatal("render parent expired")
					}
					return `<html><body><article><p>` + strings.Repeat("Detailed readable prose. ", 30) + `<a href="child">Child link</a></p></article></body></html>`, tc.renderErr
				})
				return err
			})
			if err != nil || result == nil {
				t.Fatalf("err=%v", err)
			}
			if tc.renderErr == nil && !strings.Contains(result.Content, "https://source.example/child") {
				t.Fatalf("rendered base: %s", result.Content)
			}
			if exitCodeFor(defuddle.ErrTooLarge) != exitValidation {
				t.Fatal("explicit cap error exit code")
			}
		})
	}
}

func TestCLIAutoRenderParentCancellationIsTerminal(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	opts := &ParseOptions{Source: "https://example.com"}
	result, err := parseAutoDocument(parent, opts, buildDefuddleOptions(opts), fetchedDocument{HTML: remediationShell, URL: opts.Source}, func(context.Context, *ParseOptions) (string, error) { cancel(); return "", defuddle.ErrTooLarge })
	if result != nil || !errors.Is(err, context.Canceled) || errors.Is(err, defuddle.ErrTimeout) {
		t.Fatalf("result=%v err=%v", result, err)
	}
	if err := opts.run(parent); !errors.Is(err, context.Canceled) {
		t.Fatalf("command cancellation=%v", err)
	}
	if _, err := renderToHTML(parent, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("render cancellation=%v", err)
	}
}

func TestCLIAutoRenderStaticFallbackRetainsFinalBase(t *testing.T) {
	opts := &ParseOptions{Source: "https://source.example/start"}
	// The empty root and JavaScript requirement trigger shell detection while
	// noscript content provides a useful static fallback with a relative asset.
	document := fetchedDocument{HTML: strings.Replace(remediationShell, "You need to enable JavaScript to run this app.", "You need to enable JavaScript to run this app. <a href=\"child\">Fallback link</a>", 1), URL: "https://final.example/article/final"}
	var result *defuddle.Result
	_, stderr, err := captureOutput(t, func() error {
		var err error
		result, err = parseAutoDocument(t.Context(), opts, buildDefuddleOptions(opts), document, func(context.Context, *ParseOptions) (string, error) { return "", defuddle.ErrTooLarge })
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "auto-render skipped") {
		t.Fatalf("shell was not rendered: %s", stderr)
	}
	if result.Domain != "final.example" {
		t.Fatalf("static fallback domain=%q", result.Domain)
	}
}

func TestCLIKongRunPropagatesParentCancellation(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"defuddle", "parse", "https://example.com", "--render"}
	t.Cleanup(func() { os.Args = originalArgs })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := run(ctx)
	if !errors.Is(err, context.Canceled) || errors.Is(err, defuddle.ErrTimeout) {
		t.Fatalf("Kong command error=%v", err)
	}
}
