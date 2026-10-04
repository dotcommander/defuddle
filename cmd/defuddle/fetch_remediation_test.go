package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dotcommander/defuddle"
)

var (
	errRemediationTransport          = errors.New("transport/read failure")
	errRemediationUnrelatedTransport = errors.New("unrelated transport failure")
)

func TestFetchHTMLCharsetAndFinalURL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, header, prefix string }{
		{"declared", "text/html; charset=windows-1252", ""},
		{"sniffed", "text/html", `<meta charset="windows-1252">`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/start" {
					http.Redirect(w, r, "/article/final", http.StatusFound)
					return
				}
				w.Header().Set("Content-Type", tc.header)
				_, _ = fmt.Fprint(w, tc.prefix+"<p>caf\xe9 \x80</p>")
			}))
			defer server.Close()
			doc, err := fetchHTML(t.Context(), server.URL+"/start", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(doc.HTML, "café €") || doc.URL != server.URL+"/article/final" {
				t.Fatalf("document = %#v", doc)
			}
		})
	}
}

func TestFetchHTMLDefaultAndOverrideHeaders(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, ua string }{{"default", ""}, {"override", "CLI/1"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wantUA := tc.ua
				if wantUA == "" {
					wantUA = fmt.Sprintf("Mozilla/5.0 (compatible; Defuddle/%s; +https://github.com/dotcommander/defuddle)", defuddle.Version)
				}
				if r.Header.Get("User-Agent") != wantUA || r.Header.Get("X-Test") != "yes" {
					t.Errorf("headers = %v", r.Header)
				}
				_, _ = io.WriteString(w, fixtureHTML)
			}))
			defer server.Close()
			headers := http.Header{"X-Test": []string{"yes"}}
			if tc.ua != "" {
				headers.Set("User-Agent", tc.ua)
			}
			if _, err := fetchHTML(t.Context(), server.URL, nil, headers); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFetchHTMLDownloadedByteCap(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=windows-1252")
		size := maxInputSize
		if r.URL.Path == "/over" {
			size++
		}
		_, _ = io.WriteString(w, strings.Repeat("\xe9", size))
	}))
	defer server.Close()
	doc, err := fetchHTML(t.Context(), server.URL+"/exact", nil, nil)
	if err != nil || len(doc.HTML) != 2*maxInputSize {
		t.Fatalf("at cap length=%d err=%v", len(doc.HTML), err)
	}
	doc, err = fetchHTML(t.Context(), server.URL+"/over", nil, nil)
	if !errors.Is(err, defuddle.ErrTooLarge) || doc.HTML != "" {
		t.Fatalf("over cap doc=%#v err=%v", doc, err)
	}
}

type remediationTransport func(*http.Request) (*http.Response, error)

func (f remediationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type remediationReader func([]byte) (int, error)

func (f remediationReader) Read(p []byte) (int, error) { return f(p) }

func TestFetchHTMLContextIdentityTransportAndBody(t *testing.T) {
	t.Parallel()
	for _, deadline := range []bool{false, true} {
		for _, bodyFailure := range []bool{false, true} {
			t.Run(fmt.Sprintf("deadline=%t/body=%t", deadline, bodyFailure), func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithCancel(t.Context())
				if deadline {
					ctx, cancel = context.WithTimeout(t.Context(), time.Millisecond)
				}
				defer cancel()
				fail := func() error {
					if deadline {
						<-ctx.Done()
					} else {
						cancel()
					}
					return errRemediationTransport
				}
				client := &http.Client{Transport: remediationTransport(func(r *http.Request) (*http.Response, error) {
					if !bodyFailure {
						return nil, fail()
					}
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(remediationReader(func([]byte) (int, error) { return 0, fail() })), Request: r}, nil
				})}
				_, err := fetchHTML(ctx, "https://example.com", client, nil)
				want := context.Canceled
				if deadline {
					want = context.DeadlineExceeded
				}
				if !errors.Is(err, want) || errors.Is(err, defuddle.ErrTimeout) != deadline || exitCodeFor(err) != exitCancelled {
					t.Fatalf("err=%v", err)
				}
			})
		}
	}
}

func TestFetchHTMLUnrelatedNetworkFailurePreserved(t *testing.T) {
	t.Parallel()
	sentinel := errRemediationUnrelatedTransport
	client := &http.Client{Transport: remediationTransport(func(*http.Request) (*http.Response, error) { return nil, sentinel })}
	_, err := fetchHTML(t.Context(), "https://example.com", client, nil)
	if !errors.Is(err, sentinel) || errors.Is(err, defuddle.ErrTimeout) || errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
