package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dotcommander/defuddle"
	"github.com/dotcommander/defuddle/cmd/defuddle/internal/shell"
)

// autoRenderAndParse implements the opt-in --render-auto flow: fetch the page's
// plain HTML once, classify it, and escalate to a headless-Chrome render ONLY
// when the page looks like a JS shell. Unlike the explicit --render path, a
// missing Chrome or a render error never hard-fails here — it degrades to
// parsing the already-fetched plain HTML (with a one-line stderr note), so an
// auto run without Chrome still produces best-effort output instead of exit 5.
func autoRenderAndParse(ctx context.Context, opts *ParseOptions, defuddleOpts *defuddle.Options) (*defuddle.Result, error) {
	html, err := fetchHTML(ctx, opts.Source, defuddleOpts.Client, defuddleOpts.Headers)
	if err != nil {
		return nil, err
	}

	if shell.Classify(html) == shell.LikelyShell {
		if rendered, rerr := renderToHTML(opts); rerr != nil {
			fmt.Fprintf(os.Stderr, "defuddle: auto-render skipped (%v); parsing fetched HTML\n", rerr)
		} else {
			html = rendered
		}
	}

	return defuddle.ParseFromString(ctx, html, defuddleOpts)
}
