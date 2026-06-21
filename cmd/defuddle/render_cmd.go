package main

import (
	"context"

	"github.com/dotcommander/defuddle"
	"github.com/dotcommander/defuddle/cmd/defuddle/internal/render"
)

// renderAndParse drives the chromedp render stage, then feeds the rendered
// HTML into the UNCHANGED library entrypoint defuddle.ParseFromString. The
// render deadline comes from opts.RenderTimeout (its own bounded context),
// independent of the fetch --timeout.
func renderAndParse(ctx context.Context, opts *ParseOptions, defuddleOpts *defuddle.Options) (*defuddle.Result, error) {
	renderCtx, cancel := buildContext(opts.RenderTimeout)
	defer cancel()

	cfg := render.Config{
		ChromePath:   opts.ChromePath,
		UserAgent:    opts.RenderUA,
		Wait:         render.WaitStrategy(opts.RenderWait),
		MaxHTMLBytes: maxInputSize,
	}
	html, err := render.RenderHTML(renderCtx, opts.Source, cfg)
	if err != nil {
		return nil, err
	}
	return defuddle.ParseFromString(ctx, html, defuddleOpts)
}
