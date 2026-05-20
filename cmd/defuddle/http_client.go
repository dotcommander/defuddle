// Package main: HTTP client construction for the parse command.
//
// buildHTTPClient assembles a *requests.Client from CLI fetch flags
// (--user-agent, --header, --proxy, --timeout). It returns nil when all
// flags are unset so callers can fall through to defuddle's built-in
// default client (preserves prior behavior).
package main

import (
	"fmt"
	"time"

	"github.com/kaptinlin/requests"
)

// buildHTTPClient returns a *requests.Client configured from CLI fetch flags,
// or nil if no fetch flag overrides are present (so the defuddle library uses
// its hardened default client).
//
// timeout has a non-zero default (30s) from the CLI flag definition; it is
// always applied to the constructed client when other flags trigger creation,
// but does not itself force creation when no other flag is set (defuddle's
// default client already uses 30s and the request context carries the
// timeout for cancellation).
func buildHTTPClient(userAgent string, headers []string, proxy string, timeout time.Duration) (*requests.Client, error) {
	// Validate headers up front; surface parse errors before constructing the client.
	parsed := make([][2]string, 0, len(headers))
	for _, h := range headers {
		k, v, err := parseHeader(h)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, [2]string{k, v})
	}

	// If no fetch-shaping flags are set, return nil and let defuddle use its default client.
	if userAgent == "" && len(parsed) == 0 && proxy == "" {
		return nil, nil
	}

	opts := make([]requests.ClientOption, 0, 4+len(parsed))
	if userAgent != "" {
		opts = append(opts, requests.WithUserAgent(userAgent))
	}
	for _, kv := range parsed {
		opts = append(opts, requests.WithHeader(kv[0], kv[1]))
	}
	if proxy != "" {
		opts = append(opts, requests.WithProxy(proxy))
	}
	if timeout > 0 {
		opts = append(opts, requests.WithTimeout(timeout))
	}

	client := requests.New(opts...)
	if client == nil {
		return nil, fmt.Errorf("failed to construct HTTP client")
	}
	return client, nil
}
