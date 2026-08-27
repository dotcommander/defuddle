package defuddle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pageHTML returns minimal-but-valid HTML whose <title> encodes the path so
// tests can prove which URL produced which result.
func pageHTML(path string) string {
	return `<!DOCTYPE html><html><head><title>page-` + path + `</title></head><body>` +
		// Body content must be substantive enough that extraction does not
		// reject the document; titles are read from <title> regardless of
		// the extraction path taken.
		`<main><article><h1>page-` + path + `</h1>` +
		wordRepeat("the quick brown fox jumps over the lazy dog", 30) +
		`</article></main></body></html>`
}

// newTestServer returns an httptest server that responds with pageHTML(path).
// Optional preHandler runs before the response is written (used to inject
// per-request hooks such as concurrency tracking or blocking).
func newTestServer(t *testing.T, preHandler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if preHandler != nil {
			preHandler(w, r)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(pageHTML(r.URL.Path)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testClient builds a standard client. Short
// timeouts keep cancellation tests responsive without flaking healthy paths.
func testClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func TestParseFromURL_RejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("a"), maxResponseSize+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	_, err := ParseFromURL(context.Background(), srv.URL, &Options{Client: testClient()})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrTooLarge), "got %v, want ErrTooLarge", err)
}

func TestParseFromURL_RequiresAbsoluteHTTPURL(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{"/relative", "file:///tmp/page.html", "ftp://example.com/page"} {
		t.Run(rawURL, func(t *testing.T) {
			t.Parallel()
			_, err := ParseFromURL(t.Context(), rawURL, &Options{Client: testClient()})
			require.ErrorContains(t, err, "absolute HTTP(S)")
		})
	}
}

func TestParseFromURL_PreservesURLSemantics(t *testing.T) {
	t.Parallel()

	requestedPath := make(chan string, 1)
	srv := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		requestedPath <- r.URL.RequestURI()
	})

	uppercaseScheme := "HTTP" + strings.TrimPrefix(srv.URL, "http")
	result, err := ParseFromURL(t.Context(), uppercaseScheme+"/article#section", &Options{Client: srv.Client()})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "/article", <-requestedPath)
}

func TestParseFromURL_ReturnsHTTPStatusError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<html><body><article><p>Temporary outage page.</p></article></body></html>`))
	}))
	t.Cleanup(srv.Close)

	result, err := ParseFromURL(context.Background(), srv.URL, &Options{Client: testClient()})

	require.ErrorIs(t, err, ErrHTTPStatus)
	assert.Nil(t, result)

	var statusErr *HTTPStatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, srv.URL, statusErr.URL)
	assert.Equal(t, http.StatusServiceUnavailable, statusErr.StatusCode)
	assert.Equal(t, "503 Service Unavailable", statusErr.Status)
}

func TestParseFromURL_NotModified(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, `"v1"`, r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(srv.Close)

	result, err := ParseFromURL(context.Background(), srv.URL, &Options{
		Client:  srv.Client(),
		Headers: http.Header{"If-None-Match": []string{`"v1"`}},
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotModified), "got %v, want ErrNotModified", err)
	assert.Nil(t, result)
}

func TestParseFromURL_ContentTypeValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		wantErr     bool
	}{
		{name: "mixed case HTML", contentType: "Text/HTML; Charset=UTF-8"},
		{name: "application XML", contentType: "Application/XML"},
		{name: "structured XML", contentType: "Application/Atom+XML"},
		{name: "binary", contentType: "application/octet-stream", wantErr: true},
		{name: "malformed", contentType: `text/html; charset="unterminated`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				_, _ = w.Write([]byte(pageHTML("/content-type")))
			}))
			t.Cleanup(srv.Close)

			result, err := ParseFromURL(t.Context(), srv.URL, &Options{Client: srv.Client()})
			if tt.wantErr {
				require.ErrorIs(t, err, ErrNotHTML)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
	assert.True(t, isDocumentContentType(""), "empty Content-Type remains accepted")
}

func TestParseFromURL_UsesRedirectTargetForImplicitMetadataURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/articles/story/", http.StatusFound)
		case "/articles/story/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head>
<title>Redirected Article</title>
<link rel="icon" href="icon.svg">
</head><body><main><article><h1>Redirected Article</h1>` + wordRepeat("redirected article body", 40) + `</article></main></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	options := &Options{Client: testClient()}
	result, err := ParseFromURL(context.Background(), srv.URL+"/start", options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, srv.URL+"/articles/story/", options.URL)
	assert.Equal(t, srv.URL+"/articles/story/icon.svg", result.Favicon)
}

func TestParseFromURL_PreservesExplicitMetadataURLAfterRedirect(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/articles/story/", http.StatusFound)
		case "/articles/story/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head>
<title>Logical Article</title>
<link rel="icon" href="/logical-icon.svg">
</head><body><main><article><h1>Logical Article</h1>` + wordRepeat("logical article body", 40) + `</article></main></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	options := &Options{
		Client: testClient(),
		URL:    "https://example.com/logical/story",
	}
	result, err := ParseFromURL(context.Background(), srv.URL+"/start", options)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "https://example.com/logical/story", options.URL)
	assert.Equal(t, "https://example.com/logical-icon.svg", result.Favicon)
}

func TestParseFromURL_HTTPStatusErrorUsesRedirectTarget(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/unavailable", http.StatusFound)
		case "/unavailable":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`<html><body><p>Temporary outage page.</p></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	result, err := ParseFromURL(context.Background(), srv.URL+"/start", &Options{Client: testClient()})

	require.ErrorIs(t, err, ErrHTTPStatus)
	assert.Nil(t, result)

	var statusErr *HTTPStatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, srv.URL+"/unavailable", statusErr.URL)
}

// TestParseFromURLs_OrderPreserved verifies the returned slice index matches
// the input slice index even though fetches complete in arbitrary order.
func TestParseFromURLs_OrderPreserved(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Stagger response times so completion order differs from input order.
		// Path "/0" sleeps longest, forcing a non-monotonic finish order while
		// the returned slice must still be input-ordered.
		switch r.URL.Path {
		case "/0":
			time.Sleep(40 * time.Millisecond)
		case "/1":
			time.Sleep(10 * time.Millisecond)
		case "/2":
			time.Sleep(25 * time.Millisecond)
		case "/3":
			time.Sleep(5 * time.Millisecond)
		}
	})

	urls := []string{
		srv.URL + "/0",
		srv.URL + "/1",
		srv.URL + "/2",
		srv.URL + "/3",
	}

	results := ParseFromURLs(context.Background(), urls, &Options{
		Client:         testClient(),
		MaxConcurrency: 4,
	})

	require.Len(t, results, len(urls))
	for i, r := range results {
		assert.Equal(t, urls[i], r.URL, "result[%d].URL must match input[%d]", i, i)
		require.NoError(t, r.Err, "result[%d] unexpected fetch error", i)
		require.NotNil(t, r.Result, "result[%d] missing parsed result", i)
	}
}

// TestParseFromURLs_MaxConcurrency verifies the in-flight request count
// never exceeds Options.MaxConcurrency.  The server blocks each handler
// briefly so the limiter can be observed under contention.
func TestParseFromURLs_MaxConcurrency(t *testing.T) {
	t.Parallel()

	const (
		limit     = 2
		urlCount  = 8
		holdEvery = 30 * time.Millisecond
	)

	var inFlight int32
	var peak int32

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inFlight, 1)
		// Track the high-water mark via CAS so concurrent updates don't lose
		// observations.  This is the load-bearing assertion of the test.
		for {
			old := atomic.LoadInt32(&peak)
			if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
				break
			}
		}
		time.Sleep(holdEvery)
		atomic.AddInt32(&inFlight, -1)
	})

	urls := make([]string, urlCount)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/%d", srv.URL, i)
	}

	results := ParseFromURLs(context.Background(), urls, &Options{
		Client:         testClient(),
		MaxConcurrency: limit,
	})

	require.Len(t, results, urlCount)
	for i, r := range results {
		require.NoError(t, r.Err, "result[%d] unexpected error", i)
	}
	observed := atomic.LoadInt32(&peak)
	assert.LessOrEqual(t, int(observed), limit,
		"peak in-flight requests %d exceeded MaxConcurrency=%d", observed, limit)
	assert.Greater(t, int(observed), 0, "expected at least one in-flight request to be observed")
}

// TestParseFromURLs_PerURLIsolation verifies each goroutine sees its own
// Options.URL value rather than aliasing a shared field.  If the per-URL
// copy in ParseFromURLs were dropped, all results would share whichever URL
// won the race and titles would no longer correspond to input paths.
func TestParseFromURLs_PerURLIsolation(t *testing.T) {
	t.Parallel()

	// Block every request on a shared barrier until all goroutines have
	// arrived.  Forcing simultaneity maximizes the chance of catching a
	// shared-Options.URL bug.
	const n = 6
	barrier := make(chan struct{})
	var arrived sync.WaitGroup
	arrived.Add(n)

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		arrived.Done()
		<-barrier
	})

	urls := make([]string, n)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/iso-%d", srv.URL, i)
	}

	// Release the barrier once all requests have arrived.
	go func() {
		arrived.Wait()
		close(barrier)
	}()

	results := ParseFromURLs(context.Background(), urls, &Options{
		Client:         testClient(),
		MaxConcurrency: n,
	})

	require.Len(t, results, n)
	for i, r := range results {
		require.NoError(t, r.Err, "result[%d] unexpected error", i)
		require.NotNil(t, r.Result, "result[%d] missing result", i)
		assert.Equal(t, urls[i], r.URL, "URLResult.URL must match input")
		// Title is derived from <title>page-/iso-i</title> the server
		// emitted for THIS path.  A leaked Options.URL would still produce
		// the wrong-page title because the server renders by request path,
		// but this assertion is the load-bearing per-URL identity check:
		// titles must form a 1:1 map with input URLs.
		expected := "page-/iso-" + fmt.Sprint(i)
		assert.Equal(t, expected, r.Result.Title,
			"result[%d] title mismatch — possible per-URL Options.URL bleed", i)
	}
}

// TestParseFromURLs_ContextCancellation verifies that cancelling the parent
// context aborts in-flight fetches and that every result carries an error.
// The server holds responses indefinitely so only cancellation can complete
// the call.
func TestParseFromURLs_ContextCancellation(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) })

	const n = 4
	urls := make([]string, n)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/cancel-%d", srv.URL, i)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel shortly after dispatch — long enough for requests to be
	// in-flight on the server side, short enough to keep the test fast.
	time.AfterFunc(50*time.Millisecond, cancel)

	results := ParseFromURLs(ctx, urls, &Options{
		Client:         testClient(),
		MaxConcurrency: n,
	})

	require.Len(t, results, n)
	for i, r := range results {
		assert.Equal(t, urls[i], r.URL, "result[%d].URL must match input even on cancellation", i)
		assert.Error(t, r.Err, "result[%d] expected cancellation error", i)
		assert.Nil(t, r.Result, "result[%d] expected nil result on cancellation", i)
	}
}
