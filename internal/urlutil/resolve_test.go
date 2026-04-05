package urlutil

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveURL(t *testing.T) {
	t.Parallel()

	base, _ := url.Parse("https://example.com/blog/post.html")

	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{"absolute URL unchanged", "https://other.com/img.png", "https://other.com/img.png"},
		{"relative path", "images/photo.jpg", "https://example.com/blog/images/photo.jpg"},
		{"root-relative path", "/assets/style.css", "https://example.com/assets/style.css"},
		{"protocol-relative", "//cdn.example.com/lib.js", "https://cdn.example.com/lib.js"},
		{"fragment only", "#", "#"},
		{"empty string", "", ""},
		{"data URI", "data:image/png;base64,abc", "data:image/png;base64,abc"},
		{"javascript URI", "javascript:void(0)", "javascript:void(0)"},
		{"mailto URI", "mailto:user@example.com", "mailto:user@example.com"},
		{"whitespace trimmed", "  /path  ", "https://example.com/path"},
		{"parent traversal", "../other/page.html", "https://example.com/other/page.html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := resolveURL(tt.raw, base)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveSrcset(t *testing.T) {
	t.Parallel()

	base, _ := url.Parse("https://example.com/page/")

	tests := []struct {
		name     string
		srcset   string
		expected string
	}{
		{
			"single entry with descriptor",
			"img.jpg 1x",
			"https://example.com/page/img.jpg 1x",
		},
		{
			"multiple entries",
			"small.jpg 300w, large.jpg 600w",
			"https://example.com/page/small.jpg 300w, https://example.com/page/large.jpg 600w",
		},
		{
			"absolute URLs unchanged",
			"https://cdn.com/a.jpg 1x, https://cdn.com/b.jpg 2x",
			"https://cdn.com/a.jpg 1x, https://cdn.com/b.jpg 2x",
		},
		{
			"no descriptor",
			"/images/photo.jpg",
			"https://example.com/images/photo.jpg",
		},
		{
			"empty entries filtered",
			"a.jpg 1x, , b.jpg 2x",
			"https://example.com/page/a.jpg 1x, https://example.com/page/b.jpg 2x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := resolveSrcset(tt.srcset, base)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractBaseHref(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			"base tag present",
			`<html><head><base href="https://example.com/"></head><body></body></html>`,
			"https://example.com/",
		},
		{
			"no base tag",
			`<html><head></head><body></body></html>`,
			"",
		},
		{
			"base tag without href",
			`<html><head><base target="_blank"></head><body></body></html>`,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, ExtractBaseHref(doc))
		})
	}
}

func TestResolveRelativeURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		html        string
		pageURL     string
		docBaseHref string
		checkAttr   string
		checkTag    string
		expected    string
	}{
		{
			"resolves href",
			`<div><a href="/about">About</a></div>`,
			"https://example.com/page",
			"",
			"href",
			"a",
			"https://example.com/about",
		},
		{
			"resolves src",
			`<div><img src="photo.jpg"></div>`,
			"https://example.com/blog/post",
			"",
			"src",
			"img",
			"https://example.com/blog/photo.jpg",
		},
		{
			"resolves data-src",
			`<div><img data-src="/lazy.jpg"></div>`,
			"https://example.com/",
			"",
			"data-src",
			"img",
			"https://example.com/lazy.jpg",
		},
		{
			"resolves poster",
			`<div><video poster="thumb.jpg"></video></div>`,
			"https://example.com/videos/",
			"",
			"poster",
			"video",
			"https://example.com/videos/thumb.jpg",
		},
		{
			"base href overrides page URL",
			`<div><a href="page2.html">Link</a></div>`,
			"https://example.com/old/path",
			"https://cdn.example.com/new/",
			"href",
			"a",
			"https://cdn.example.com/new/page2.html",
		},
		{
			"srcset resolved",
			`<div><img srcset="small.jpg 300w, large.jpg 600w"></div>`,
			"https://example.com/blog/",
			"",
			"srcset",
			"img",
			"https://example.com/blog/small.jpg 300w, https://example.com/blog/large.jpg 600w",
		},
		{
			"no-op when page URL empty",
			`<div><a href="/path">Link</a></div>`,
			"",
			"",
			"href",
			"a",
			"/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
			require.NoError(t, err)

			sel := doc.Find("div").First()
			ResolveRelativeURLs(sel, tt.pageURL, tt.docBaseHref)

			val, exists := sel.Find(tt.checkTag).First().Attr(tt.checkAttr)
			assert.True(t, exists)
			assert.Equal(t, tt.expected, val)
		})
	}
}
