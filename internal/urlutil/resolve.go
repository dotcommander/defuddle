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
	base := docBaseHref
	if base == "" {
		base = pageURL
	}
	if base == "" {
		return
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return
	}

	// Attributes that contain a single URL
	urlAttrs := []string{"href", "src", "poster", "data-src", "data-srcset", "action"}

	element.Find("*").Each(func(_ int, el *goquery.Selection) {
		for _, attr := range urlAttrs {
			val, exists := el.Attr(attr)
			if !exists || val == "" {
				continue
			}
			if resolved := resolveURL(val, baseURL); resolved != val {
				el.SetAttr(attr, resolved)
			}
		}

		// Handle srcset (comma-separated list of "url descriptor" pairs)
		if srcset, exists := el.Attr("srcset"); exists && srcset != "" {
			el.SetAttr("srcset", resolveSrcset(srcset, baseURL))
		}
	})
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
	entries := strings.Split(srcset, ",")
	resolved := make([]string, 0, len(entries))

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Split into URL and optional descriptor (e.g. "1x", "300w")
		parts := strings.Fields(entry)
		if len(parts) == 0 {
			continue
		}

		parts[0] = resolveURL(parts[0], base)
		resolved = append(resolved, strings.Join(parts, " "))
	}

	return strings.Join(resolved, ", ")
}
