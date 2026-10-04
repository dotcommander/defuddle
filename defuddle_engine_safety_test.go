package defuddle

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	trafilatura "github.com/markusmobius/go-trafilatura/v2"
	"golang.org/x/net/html"
)

var errEngineAfterCancellation = errors.New("failure after cancellation")

func TestEngineSafetyMetadataAndConfiguration(t *testing.T) {
	t.Parallel()
	d, err := NewDefuddle(`<head><title>Defuddle title</title><base href="https://cdn.example/dir/"><meta name="author" content="Defuddle author"></head><body><p>正文中文</p></body>`, &Options{URL: "https://page.example/a"})
	if err != nil {
		t.Fatal(err)
	}
	var firstConfig *trafilatura.Config
	d.engine = func(_ *html.Node, o trafilatura.Options) (*trafilatura.ExtractResult, error) {
		if !o.ExcludeComments || !o.EnableFallback || !o.IncludeLinks || !o.IncludeImages || o.ExcludeTables {
			t.Fatal("incorrect upstream configuration")
		}
		if o.Config == firstConfig {
			t.Fatal("configuration reused")
		}
		firstConfig = o.Config
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(`<p onclick="evil()">正文中文<a href="rel">safe</a><a href="javascript:evil()">unsafe</a><script>evil()</script></p>`))
		r := &trafilatura.ExtractResult{ContentNode: doc.Find("body").Get(0)}
		r.Metadata.Title = "Upstream title"
		r.Metadata.Author = "Upstream author"
		r.Metadata.Date = time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
		return r, nil
	}
	for range 2 {
		r, err := d.Parse(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if r.Title != "Defuddle title" || r.Author != "Defuddle author" || r.Published != "2025-01-02T03:04:05Z" || r.WordCount < 4 || !strings.Contains(r.Content, `href="https://cdn.example/dir/rel"`) || strings.Contains(r.Content, "onclick") || strings.Contains(r.Content, "javascript:") || strings.Contains(r.Content, "<script") {
			t.Fatalf("%+v", r)
		}
	}
}

func TestEngineCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d, _ := NewDefuddle(`<p>article</p>`, nil)
	if _, err := d.Parse(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)
	d.engine = func(*html.Node, trafilatura.Options) (*trafilatura.ExtractResult, error) {
		cancel()
		return nil, errEngineAfterCancellation
	}
	if _, err := d.Parse(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestEngineConcurrentParses(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			r := engineParse(t, `<article><p>`+strings.Repeat(engineProse, 3)+`<code>concurrent value</code></p></article>`, nil)
			if !strings.Contains(r.Content, "concurrent value") || strings.Contains(r.Content, "DFR") {
				t.Error(r.Content)
			}
		})
	}
	wg.Wait()
}

func TestEngineSelectedRootSafetyAndURLs(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{"script", "style", "object"} {
		r := engineParse(t, `<body><`+tag+`>unsafe root</`+tag+`></body>`, &Options{ContentSelector: tag})
		if strings.Contains(r.Content, "<"+tag) || strings.Contains(r.Content, "unsafe root") {
			t.Fatal(r.Content)
		}
	}
	for _, tc := range []struct{ selector, source, want string }{
		{"a", `<a href="rel" onclick="evil()">selected</a>`, `href="https://example.org/assets/rel"`},
		{"img", `<img src="pic.png" onerror="evil()">`, `src="https://example.org/assets/pic.png"`},
	} {
		r := engineParse(t, `<head><base href="/assets/"></head><body>`+tc.source+`</body>`, &Options{URL: "https://example.org/page", ContentSelector: tc.selector})
		if !strings.Contains(r.Content, tc.want) || strings.Contains(r.Content, "evil") {
			t.Fatal(r.Content)
		}
	}
	r := engineParse(t, `<head><base href="/assets/"></head><body><article><p>`+strings.Repeat(engineProse, 4)+`<a href="rel">ordinary link</a></p></article></body>`, &Options{URL: "https://example.org/page"})
	if !strings.Contains(r.Content, `href="https://example.org/assets/rel"`) {
		t.Fatal(r.Content)
	}
}

func TestEngineDocumentBaseWithoutPageURL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, base, wantHref, wantSrc string }{
		{"absolute base", `<base href="https://example.org/assets/">`, "https://example.org/assets/next", "https://example.org/assets/pic.png"},
		{"no base", "", "next", "pic.png"},
		{"relative base without page", `<base href="/assets/">`, "next", "pic.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := `<head>` + tc.base + `</head><body><article><a href="next">link</a><img src="pic.png"></article></body>`
			r := engineParse(t, source, &Options{ContentSelector: "article"})
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(r.Content))
			if err != nil {
				t.Fatal(err)
			}
			if doc.Find("a").AttrOr("href", "") != tc.wantHref || doc.Find("img").AttrOr("src", "") != tc.wantSrc {
				t.Fatal(r.Content)
			}
		})
	}
	d, err := NewDefuddle(`<head><base href="https://example.org/assets/"></head><body><p>`+engineProse+`</p></body>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.engine = func(_ *html.Node, o trafilatura.Options) (*trafilatura.ExtractResult, error) {
		if o.OriginalURL == nil || o.OriginalURL.String() != "https://example.org/assets/" {
			t.Fatal("upstream missed document base")
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<p>link <a href="next">selected link</a></p>`))
		if err != nil {
			t.Fatal(err)
		}
		return &trafilatura.ExtractResult{ContentNode: doc.Find("body").Get(0)}, nil
	}
	r, err := d.Parse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Content, `href="https://example.org/assets/next"`) {
		t.Fatal(r.Content)
	}
}
