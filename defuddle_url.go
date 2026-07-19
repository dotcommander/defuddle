package defuddle

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// maxResponseSize is the maximum HTML body size (5 MB).
const maxResponseSize = 5 * 1024 * 1024

// ParseFromURL fetches content from a URL and parses it.
// This corresponds to Node.js usage: Defuddle(htmlOrDom, url?, options?)
func ParseFromURL(ctx context.Context, url string, options *Options) (*Result, error) {
	if options == nil {
		options = &Options{}
	}

	useResponseURL := options.URL == ""

	// Set URL in options if not already set
	if options.URL == "" {
		options.URL = url
	}

	// Create HTTP client with hardened defaults
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	fetched, err := fetchCapped(ctx, client, options.Headers, url)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("fetch %s: %w", url, ErrTimeout)
		}
		return nil, err
	}
	if useResponseURL && fetched.URL != "" {
		options.URL = fetched.URL
	}

	// Detect and convert charset to UTF-8
	body, err := toUTF8(fetched.Body, fetched.ContentType)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: charset conversion: %w", url, err)
	}

	// Create Defuddle instance and parse
	defuddle, err := NewDefuddle(body, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Defuddle instance: %w", err)
	}

	return defuddle.Parse(ctx)
}

type fetchResult struct {
	Body        []byte
	ContentType string
	URL         string
}

func fetchCapped(ctx context.Context, client *http.Client, headers http.Header, rawURL string) (*fetchResult, error) {
	reqURL, err := validateRequestURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", rawURL, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", rawURL, err)
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", fmt.Sprintf("Mozilla/5.0 (compatible; Defuddle/%s; +https://github.com/dotcommander/defuddle)", Version))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", rawURL, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("Failed to close response", "error", closeErr)
		}
	}()

	responseURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		responseURL = resp.Request.URL.String()
	}

	if resp.StatusCode == http.StatusNotModified {
		return nil, ErrNotModified
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &HTTPStatusError{
			URL:        responseURL,
			Status:     resp.Status,
			StatusCode: resp.StatusCode,
		}
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "html") && !strings.Contains(ct, "xml") && !strings.Contains(ct, "text/") {
		return nil, fmt.Errorf("fetch %s: content-type %q: %w", rawURL, ct, ErrNotHTML)
	}
	if resp.ContentLength > maxResponseSize {
		return nil, fmt.Errorf("fetch %s: response %d bytes: %w", rawURL, resp.ContentLength, ErrTooLarge)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: read response: %w", rawURL, err)
	}
	if len(body) > maxResponseSize {
		return nil, fmt.Errorf("fetch %s: response exceeds %d bytes: %w", rawURL, maxResponseSize, ErrTooLarge)
	}
	return &fetchResult{Body: body, ContentType: ct, URL: responseURL}, nil
}

func validateRequestURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errInvalidRequestURL, err)
	}
	if !parsed.IsAbs() ||
		(!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) ||
		parsed.Host == "" {
		return "", errInvalidRequestURL
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	return parsed.String(), nil
}

// ParseFromString parses HTML content directly from a string
// This is useful when you already have the HTML content (e.g., from browser automation)
func ParseFromString(ctx context.Context, html string, options *Options) (*Result, error) {
	if options == nil {
		options = &Options{}
	}

	// Create Defuddle instance and parse
	defuddle, err := NewDefuddle(html, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Defuddle instance: %w", err)
	}

	return defuddle.Parse(ctx)
}

// URLResult pairs a URL with its extraction result or error.
type URLResult struct {
	URL    string
	Result *Result
	Err    error
}

// ParseFromURLs fetches and parses multiple URLs concurrently.
// MaxConcurrency in options controls parallelism (default 5).
func ParseFromURLs(ctx context.Context, urls []string, options *Options) []URLResult {
	if options == nil {
		options = &Options{}
	}
	limit := options.MaxConcurrency
	if limit <= 0 {
		limit = 5
	}

	results := make([]URLResult, len(urls))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	for i, u := range urls {
		// each slot owns its own error; never short-circuit the group
		g.Go(func() error {
			// Copy options per URL so URL field doesn't collide
			opts := *options
			opts.URL = u
			result, err := ParseFromURL(gctx, u, &opts)
			results[i] = URLResult{URL: u, Result: result, Err: err}
			return nil
		})
	}
	_ = g.Wait()
	return results
}
