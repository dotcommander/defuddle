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
func renderToHTML(opts *ParseOptions) (string, error) {
	renderCtx, cancel := buildContext(opts.RenderTimeout)
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

// renderAndParse drives the chromedp render stage, then feeds the rendered HTML
// into the UNCHANGED library entrypoint defuddle.ParseFromString. The render
// deadline comes from opts.RenderTimeout, independent of the fetch --timeout.
func renderAndParse(ctx context.Context, opts *ParseOptions, defuddleOpts *defuddle.Options) (*defuddle.Result, error) {
	html, err := renderToHTML(opts)
	if err != nil {
		return nil, err
	}
	return defuddle.ParseFromString(ctx, html, defuddleOpts)
}
