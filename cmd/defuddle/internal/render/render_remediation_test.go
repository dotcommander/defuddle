package render

import (
	"context"
	"errors"
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
