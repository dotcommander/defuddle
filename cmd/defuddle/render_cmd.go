package main

import (
	"context"

	"github.com/dotcommander/defuddle"
	"github.com/dotcommander/defuddle/cmd/defuddle/internal/render"
)

// renderToHTML renders opts.Source to fully-rendered HTML under its own
// render-timeout context (independent of the fetch --timeout). Shared by the
// explicit --render path (renderAndParse) and the auto path (autoRenderAndParse)
// so the two never drift.
func renderToHTML(ctx context.Context, opts *ParseOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	renderCtx, cancel := buildContext(ctx, opts.RenderTimeout)
	defer cancel()
	return render.RenderHTML(renderCtx, opts.Source, buildRenderConfig(opts))
}

// buildRenderConfig maps ParseOptions render flags into a render.Config.
func buildRenderConfig(opts *ParseOptions) render.Config {
	return render.Config{
		ChromePath:      opts.ChromePath,
		UserAgent:       opts.RenderUA,
		Wait:            render.WaitStrategy(opts.RenderWait),
		WaitForSelector: opts.RenderWaitFor,
		Settle:          opts.RenderSettle,
		MaxHTMLBytes:    maxInputSize,
	}
}

// effectiveAutoRenderOpts applies the auto-path render defaults. --render-auto
// escalates exactly on JS-shell pages, which hydrate after the load event, so
// an unset --render-wait escalates to networkidle; an explicit --render-wait
// value is honored unchanged on every path.
func effectiveAutoRenderOpts(opts *ParseOptions) *ParseOptions {
	effective := *opts
	if effective.RenderWait == "" {
		effective.RenderWait = "networkidle"
	}
	return &effective
}

// renderAndParse drives the chromedp render stage, then feeds the rendered HTML
// into the UNCHANGED library entrypoint defuddle.ParseFromString. The render
// deadline comes from opts.RenderTimeout, independent of the fetch --timeout.
func renderAndParse(ctx context.Context, opts *ParseOptions, defuddleOpts *defuddle.Options) (*defuddle.Result, error) {
	html, err := renderToHTML(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return defuddle.ParseFromString(ctx, html, defuddleOpts)
}
