// Package main: single-shot HTML fetch for ordinary and automatic rendering paths.
//
// fetchHTML performs one plain GET and returns decoded HTML with its final URL so
// the shell detector can classify the page before deciding whether to escalate
// to a browser render. It deliberately mirrors the library's internal fetch
// hardening (size cap via readCapped, HTTP-status / content-type / timeout
// typing via the exported defuddle sentinels) so the exit-code contract holds
// on the auto path. It cannot call the library's own fetch (unexported) and the
// CLI must not add exported library symbols (would break the standalone CLI
// build until a library release — see CLAUDE.md false-green caveat), hence this
// small self-contained mirror.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/dotcommander/defuddle"
	"golang.org/x/net/html/charset"
)

type fetchedDocument struct {
	HTML string
	URL  string
}

// fetchHTML GETs rawURL and returns the response body. client carries the CLI's
// --user-agent/--header/--proxy/--timeout overrides (may be nil, in which case
// the library-equivalent 30s client is used; context also bounds the fetch).
func fetchHTML(ctx context.Context, rawURL string, client *http.Client, headers http.Header) (fetchedDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fetchedDocument{}, fmt.Errorf("fetch %s: %w", rawURL, err)
	}

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", fmt.Sprintf("Mozilla/5.0 (compatible; Defuddle/%s; +https://github.com/dotcommander/defuddle)", defuddle.Version))
	}
	httpClient := client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fetchedDocument{}, fetchContextError(rawURL, ctx.Err())
		}
		return fetchedDocument{}, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return fetchedDocument{}, defuddle.ErrNotModified
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fetchedDocument{}, fmt.Errorf("fetch %s: %s: %w", rawURL, resp.Status, defuddle.ErrHTTPStatus)
	}

	ct := resp.Header.Get("Content-Type")
	if !isDocumentContentType(ct) {
		return fetchedDocument{}, fmt.Errorf("fetch %s: content-type %q: %w", rawURL, ct, defuddle.ErrNotHTML)
	}

	body, err := readCapped(resp.Body, rawURL)
	if err != nil {
		if ctx.Err() != nil {
			return fetchedDocument{}, fetchContextError(rawURL, ctx.Err())
		}
		return fetchedDocument{}, err
	}
	html := string(body)
	reader, decodeErr := charset.NewReader(bytes.NewReader(body), ct)
	if decodeErr == nil {
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return fetchedDocument{}, fmt.Errorf("decoding %s: %w", rawURL, err)
		}
		html = string(decoded)
	}
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return fetchedDocument{HTML: html, URL: finalURL}, nil
}

func isDocumentContentType(contentType string) bool {
	if contentType == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "text/") ||
		mediaType == "application/xml" ||
		strings.HasSuffix(mediaType, "+xml")
}

func fetchContextError(rawURL string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		err = errors.Join(defuddle.ErrTimeout, err)
	}
	return fmt.Errorf("fetch %s: %w", rawURL, err)
}
