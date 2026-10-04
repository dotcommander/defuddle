package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	defuddle "github.com/dotcommander/defuddle"
	"github.com/dotcommander/defuddle/extractors"
	trafilatura "github.com/markusmobius/go-trafilatura/v2"
	"golang.org/x/net/html"
)

type metadata struct {
	Title   string   `json:"title"`
	Authors []string `json:"authors"`
	Date    string   `json:"date"`
}

type extraction struct {
	HTML     string
	Metadata metadata
}

type engineExtractor func(context.Context, string, string, *url.URL) (extraction, error)

// Recovery belongs to one synchronous engine invocation, never the whole stream.
func extractPage(ctx context.Context, id, engine, input string, pageURL *url.URL, extractor engineExtractor) (result extraction, err error) {
	defer func() {
		if cause := recover(); cause != nil {
			result = extraction{}
			message := fmt.Sprint(cause)
			if len(message) > 256 {
				message = message[:256] + "..."
			}
			err = fmt.Errorf("fixture %q engine %q panicked: %q", id, engine, message)
		}
	}()
	return extractor(ctx, engine, input, pageURL)
}

func extract(ctx context.Context, engine, input string, pageURL *url.URL) (extraction, error) {
	if err := ctx.Err(); err != nil {
		return extraction{}, err
	}
	switch engine {
	case "defuddle":
		extractors.InitializeBuiltins()
		r, err := defuddle.ParseFromString(ctx, input, &defuddle.Options{URL: pageURL.String()})
		if err != nil {
			return extraction{}, err
		}
		return extraction{r.Content, metadata{r.Title, authors(r.Author), r.Published}}, nil
	case "trafilatura":
		r, err := trafilatura.Extract(strings.NewReader(input), trafilatura.Options{OriginalURL: pageURL, ExcludeComments: true, EnableFallback: true, IncludeLinks: true, IncludeImages: true})
		if err != nil {
			return extraction{}, err
		}
		if r == nil || r.ContentNode == nil {
			return extraction{}, fmt.Errorf("empty extraction")
		}
		var b strings.Builder
		if err := html.Render(&b, r.ContentNode); err != nil {
			return extraction{}, err
		}
		date := ""
		if !r.Metadata.Date.IsZero() {
			date = r.Metadata.Date.Format(time.RFC3339)
		}
		return extraction{b.String(), metadata{r.Metadata.Title, authors(r.Metadata.Author), date}}, nil
	case "readability":
		r, err := readability.FromReader(strings.NewReader(input), pageURL)
		if err != nil {
			return extraction{}, err
		}
		var b strings.Builder
		if err := r.RenderHTML(&b); err != nil {
			return extraction{}, err
		}
		date := ""
		if published, err := r.PublishedTime(); err == nil && !published.IsZero() {
			date = published.Format(time.RFC3339)
		}
		return extraction{b.String(), metadata{r.Title(), authors(r.Byline()), date}}, nil
	default:
		return extraction{}, fmt.Errorf("unknown engine: %s", engine)
	}
}

func authors(byline string) []string {
	if byline == "" {
		return []string{}
	}
	return []string{byline}
}
