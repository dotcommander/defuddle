// Package urlutil provides URL resolution and sanitization for extracted content.
package urlutil

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ResolveRelativeURLs resolves all relative URLs in the element against baseURL.
// It handles href, src, srcset, poster, and data-src attributes.
// docBaseHref overrides the base URL if a <base href> tag was present.
func ResolveRelativeURLs(element *goquery.Selection, pageURL string, docBaseHref string) {
	baseURL := EffectiveBaseURL(pageURL, docBaseHref)
	if baseURL == nil {
		return
	}
	element.Each(func(_ int, el *goquery.Selection) { resolveElementURLs(el, baseURL) })

	element.Find("*").Each(func(_ int, el *goquery.Selection) {
		resolveElementURLs(el, baseURL)
	})
}

// EffectiveBaseURL composes a document base with its page URL. An absolute
// document base also works for HTML supplied without a page URL (such as stdin).
// Relative document bases require a page URL; otherwise references stay intact.
func EffectiveBaseURL(pageURL, docBaseHref string) *url.URL {
	var page *url.URL
	if pageURL != "" {
		page, _ = url.Parse(pageURL)
	}
	if docBaseHref != "" {
		reference, err := url.Parse(docBaseHref)
		if err != nil {
			return nil
		}
		if reference.IsAbs() {
			return reference
		}
		if page != nil {
			return page.ResolveReference(reference)
		}
		return nil
	}
	return page
}

// urlAttrs are the element attributes that contain a single resolvable URL.
var urlAttrs = []string{"href", "src", "poster", "data-src", "action"}

// resolveElementURLs resolves every single-URL attribute and the srcset on el
// against baseURL.
func resolveElementURLs(el *goquery.Selection, baseURL *url.URL) {
	for _, attr := range urlAttrs {
		val, exists := el.Attr(attr)
		if !exists || val == "" {
			continue
		}
		if resolved := resolveURL(val, baseURL); resolved != val {
			el.SetAttr(attr, resolved)
		}
	}

	// Both eager and lazy image candidates use the same URL/descriptor grammar.
	for _, attr := range []string{"srcset", "data-srcset"} {
		if value, exists := el.Attr(attr); exists && value != "" {
			el.SetAttr(attr, resolveSrcset(value, baseURL))
		}
	}
}

// ExtractBaseHref finds the <base href="..."> value from the document.
func ExtractBaseHref(doc *goquery.Document) string {
	base := doc.Find("base[href]").First()
	if base.Length() == 0 {
		return ""
	}
	href, _ := base.Attr("href")
	return href
}

// resolveURL resolves a single URL reference against a base URL.
// Returns the original string if it's already absolute or unparseable.
func resolveURL(raw string, base *url.URL) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "#" || strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "javascript:") || strings.HasPrefix(raw, "mailto:") {
		return raw
	}

	ref, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	// Already absolute
	if ref.IsAbs() {
		return raw
	}

	resolved := base.ResolveReference(ref)
	return resolved.String()
}

// resolveSrcset resolves URLs in an HTML srcset attribute value.
// Format: "url1 1x, url2 2x" or "url1 300w, url2 600w"
func resolveSrcset(srcset string, base *url.URL) string {
	candidates := parseSrcsetCandidates(srcset)
	resolved := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		// URL tokens may contain commas (notably data URLs); descriptors begin
		// at the first ASCII whitespace. Keep the descriptor bytes intact.
		end := 0
		for end < len(candidate.raw) && !isASCIIWhitespace(candidate.raw[end]) {
			end++
		}
		resolved = append(resolved, resolveURL(candidate.raw[:end], base)+candidate.raw[end:])
	}
	return strings.Join(resolved, ", ")
}
