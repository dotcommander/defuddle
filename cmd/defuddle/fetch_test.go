package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotcommander/defuddle"
)

func TestFetchHTML_Success(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, fixtureHTML)
	}))
	defer ts.Close()

	body, err := fetchHTML(t.Context(), ts.URL, nil, nil)
	if err != nil {
		t.Fatalf("fetchHTML: unexpected error: %v", err)
	}
	if body != fixtureHTML {
		t.Fatalf("fetchHTML body mismatch:\n got: %q\nwant: %q", body, fixtureHTML)
	}
}

func TestFetchHTML_HTTPStatusError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	_, err := fetchHTML(t.Context(), ts.URL, nil, nil)
	if !errors.Is(err, defuddle.ErrHTTPStatus) {
		t.Fatalf("fetchHTML on 404: want ErrHTTPStatus, got %v", err)
	}
}

func TestFetchHTML_NonHTMLContentType(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = fmt.Fprint(w, "binary")
	}))
	defer ts.Close()

	_, err := fetchHTML(t.Context(), ts.URL, nil, nil)
	if !errors.Is(err, defuddle.ErrNotHTML) {
		t.Fatalf("fetchHTML on octet-stream: want ErrNotHTML, got %v", err)
	}
}
