// Package main: single-shot HTML fetch for the --render-auto path.
//
// fetchHTML performs one plain GET of a URL and returns the body as a string so
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
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/dotcommander/defuddle"
	"github.com/kaptinlin/requests"
)

// fetchHTML GETs rawURL and returns the response body. client carries the CLI's
// --user-agent/--header/--proxy/--timeout overrides (may be nil, in which case
// http.DefaultClient is used and the request context bounds the fetch).
func fetchHTML(ctx context.Context, rawURL string, client *requests.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", rawURL, err)
	}

	httpClient := http.DefaultClient
	if client != nil {
		if client.Headers != nil {
			for key, values := range *client.Headers {
				for _, value := range values {
					req.Header.Add(key, value)
				}
			}
		}
		for _, cookie := range client.Cookies {
			req.AddCookie(cookie)
		}
		if client.HTTPClient != nil {
			httpClient = client.HTTPClient
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("fetch %s: %w", rawURL, defuddle.ErrTimeout)
		}
		return "", fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return "", defuddle.ErrNotModified
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("fetch %s: %s: %w", rawURL, resp.Status, defuddle.ErrHTTPStatus)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "html") && !strings.Contains(ct, "xml") && !strings.Contains(ct, "text/") {
		return "", fmt.Errorf("fetch %s: content-type %q: %w", rawURL, ct, defuddle.ErrNotHTML)
	}

	body, err := readCapped(resp.Body, rawURL)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
