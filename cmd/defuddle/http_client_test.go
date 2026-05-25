package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dotcommander/defuddle"
)

func TestParseHeader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        string
		wantKey   string
		wantVal   string
		wantError bool
	}{
		{"simple", "X-Foo: bar", "X-Foo", "bar", false},
		{"trims whitespace", "  Authorization  :   Bearer abc  ", "Authorization", "Bearer abc", false},
		{"value with colon", "X-Url: https://example.com/p", "X-Url", "https://example.com/p", false},
		{"empty value allowed", "X-Empty:", "X-Empty", "", false},
		{"missing colon", "no-colon-here", "", "", true},
		{"empty header name", ": value", "", "", true},
		{"blank header name", "   : value", "", "", true},
		{"empty string", "", "", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k, v, err := parseHeader(tc.in)
			if tc.wantError {
				if err == nil {
					t.Fatalf("parseHeader(%q) expected error, got nil", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseHeader(%q) unexpected error: %v", tc.in, err)
			}
			if k != tc.wantKey || v != tc.wantVal {
				t.Fatalf("parseHeader(%q) = (%q, %q); want (%q, %q)", tc.in, k, v, tc.wantKey, tc.wantVal)
			}
		})
	}
}

func TestBuildHTTPClient_NoFlagsReturnsNil(t *testing.T) {
	t.Parallel()
	c, err := buildHTTPClient("", nil, "", 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c != nil {
		t.Fatalf("expected nil client when no fetch flags set, got %v", c)
	}
}

func TestBuildHTTPClient_InvalidHeader(t *testing.T) {
	t.Parallel()
	_, err := buildHTTPClient("", []string{"no-colon"}, "", 30*time.Second)
	if err == nil {
		t.Fatalf("expected error for invalid header, got nil")
	}
}

// TestParseFromURL_UsesCustomClient verifies that a *requests.Client built
// from CLI flags (User-Agent, custom header) actually reaches the upstream
// request when passed via defuddle.Options.Client.
func TestParseFromURL_UsesCustomClient(t *testing.T) {
	t.Parallel()

	var gotUA, gotXTest string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotXTest = r.Header.Get("X-Test")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>t</title></head><body><article><p>hello world from test fixture, long enough to score as content.</p></article></body></html>`))
	}))
	defer srv.Close()

	const customUA = "DefuddleCLITest/1.0"
	client, err := buildHTTPClient(customUA, []string{"X-Test: yes"}, "", 5*time.Second)
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	if client == nil {
		t.Fatalf("expected non-nil client when flags set")
	}

	opts := &defuddle.Options{Client: client}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := defuddle.ParseFromURL(ctx, srv.URL, opts); err != nil {
		t.Fatalf("ParseFromURL: %v", err)
	}

	if gotUA != customUA {
		t.Errorf("User-Agent: got %q, want %q", gotUA, customUA)
	}
	if gotXTest != "yes" {
		t.Errorf("X-Test header: got %q, want %q", gotXTest, "yes")
	}
}

// TestParseFromURL_DefaultsPreservedWhenNoFlags ensures that when
// buildHTTPClient returns nil (no fetch flags), defuddle falls back to its
// own default client (UA starts with "Mozilla/5.0").
func TestParseFromURL_DefaultsPreservedWhenNoFlags(t *testing.T) {
	t.Parallel()

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>t</title></head><body><article><p>hello world from test fixture, long enough to score as content.</p></article></body></html>`))
	}))
	defer srv.Close()

	client, err := buildHTTPClient("", nil, "", 30*time.Second)
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	if client != nil {
		t.Fatalf("expected nil client for no-flags case, got %v", client)
	}

	opts := &defuddle.Options{Client: client} // nil — defuddle uses its default

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := defuddle.ParseFromURL(ctx, srv.URL, opts); err != nil {
		t.Fatalf("ParseFromURL: %v", err)
	}

	if !strings.HasPrefix(gotUA, "Mozilla/5.0") {
		t.Errorf("default User-Agent: got %q, want prefix %q", gotUA, "Mozilla/5.0")
	}
}
