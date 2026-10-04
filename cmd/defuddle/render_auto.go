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
// missing Chrome or an independent render error falls back to
// parsing the already-fetched plain HTML (with a one-line stderr note), so an
// auto run without Chrome still produces best-effort output instead of exit 5.
// Caller cancellation remains terminal.
func autoRenderAndParse(ctx context.Context, opts *ParseOptions, defuddleOpts *defuddle.Options) (*defuddle.Result, error) {
	fetchCtx, cancel := buildContext(ctx, opts.Timeout)
	document, err := fetchHTML(fetchCtx, opts.Source, defuddleOpts.Client, defuddleOpts.Headers)
	cancel()
	if err != nil {
		return nil, err
	}

	return parseAutoDocument(ctx, opts, defuddleOpts, document, renderToHTML)
}

// parseAutoDocument keeps render failure fallback separate from caller cancellation.
func parseAutoDocument(ctx context.Context, opts *ParseOptions, defuddleOpts *defuddle.Options, document fetchedDocument, renderHTML func(context.Context, *ParseOptions) (string, error)) (*defuddle.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	html := document.HTML
	parseOptions := *defuddleOpts
	parseOptions.URL = document.URL
	if shell.Classify(html) == shell.LikelyShell {
		if rendered, rerr := renderHTML(ctx, opts); rerr != nil {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			fmt.Fprintf(os.Stderr, "defuddle: auto-render skipped (%v); parsing fetched HTML\n", rerr)
		} else {
			html = rendered
			parseOptions.URL = defuddleOpts.URL
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return defuddle.ParseFromString(ctx, html, &parseOptions)
}
