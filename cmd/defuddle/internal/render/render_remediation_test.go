package render

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/dotcommander/defuddle"
)

func TestRenderedHTMLCapRejectsWithoutPartialUTF8(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, html string
		limit      int
		tooLarge   bool
	}{
		{"exact UTF8", "é", 2, false}, {"split UTF8", "é", 1, true}, {"over ASCII", "abc", 2, true}, {"uncapped", "é", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cappedHTML(tc.html, tc.limit)
			if tc.tooLarge {
				if got != "" || !errors.Is(err, defuddle.ErrTooLarge) {
					t.Fatalf("got=%q err=%v", got, err)
				}
				return
			}
			if got != tc.html || err != nil {
				t.Fatalf("got=%q err=%v", got, err)
			}
		})
	}
}

func TestRenderContextErrorIdentity(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		err := renderContextError("https://example.com", cause)
		if !errors.Is(err, cause) || errors.Is(err, defuddle.ErrTimeout) != errors.Is(cause, context.DeadlineExceeded) {
			t.Fatalf("cause=%v err=%v", cause, err)
		}
	}
}

func TestRenderExpiredParentReturnsDeadlineWithoutChrome(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	html, err := RenderHTML(ctx, "https://example.com", Config{ChromePath: "missing-chrome"})
	if html != "" || !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, defuddle.ErrTimeout) || errors.Is(err, ErrChromeNotFound) {
		t.Fatalf("html=%q err=%v", html, err)
	}
}

func TestBuildExtraHTTPHeaders(t *testing.T) {
	t.Parallel()
	if got := buildExtraHTTPHeaders(nil); got != nil {
		t.Fatalf("expected nil for nil headers, got %v", got)
	}
	got := buildExtraHTTPHeaders(http.Header{
		"Authorization": []string{"Bearer tok"},
		"X-Multi":       []string{"a", "b"},
		"X-Empty":       nil,
	})
	if len(got) != 2 || got["Authorization"] != "Bearer tok" || got["X-Multi"] != "a, b" {
		t.Fatalf("unexpected headers: %#v", got)
	}
}
