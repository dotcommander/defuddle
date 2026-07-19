package elements

import (
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// --- Helper functions ---

// isBase64Placeholder checks if a src is a small base64 placeholder image.
func isBase64Placeholder(src string) bool {
	match := b64DataURLRe.FindStringSubmatch(src)
	if match == nil {
		return false
	}
	// SVG images can be meaningful even when small
	if match[1] == "svg+xml" {
		return false
	}
	// Base64 portion after the data URL prefix
	b64Length := len(src) - len(match[0])
	return b64Length < 133
}

// isSVGDataURL checks if a src is an SVG data URL.
func isSVGDataURL(src string) bool {
	return strings.HasPrefix(src, "data:image/svg+xml")
}

// isValidImageURL checks if a URL is a valid image source (not data: URL, not empty).
func isValidImageURL(src string) bool {
	if strings.HasPrefix(src, "data:") {
		return false
	}
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	return imageURLRe.MatchString(src) ||
		strings.Contains(src, "image") ||
		strings.Contains(src, "img") ||
		strings.Contains(src, "photo")
}

// hasBetterImageSource checks if an element has alternative image sources
// beyond its current src (data-src, data-srcset, or other image-URL attrs).
func hasBetterImageSource(el *goquery.Selection) bool {
	if _, ok := el.Attr("data-src"); ok {
		return true
	}
	if _, ok := el.Attr("data-srcset"); ok {
		return true
	}
	if el.Length() == 0 {
		return false
	}
	for _, attr := range el.Get(0).Attr {
		if attr.Key == "src" {
			continue
		}
		if imageExtRe.MatchString(attr.Val) {
			return true
		}
	}
	return false
}

// extractFirstURLFromSrcset extracts the first valid image URL from a srcset string.
// Handles URLs containing commas (e.g. Substack CDN).
func extractFirstURLFromSrcset(srcset string) string {
	srcset = strings.TrimSpace(srcset)
	if srcset == "" {
		return ""
	}

	matches := srcsetEntryRe.FindAllStringSubmatchIndex(srcset, -1)
	lastEnd := 0
	for _, m := range matches {
		// m[2]:m[3] is submatch 1 (the URL part)
		urlPart := srcset[m[2]:m[3]]
		if lastEnd > 0 {
			// Trim leading comma separator from previous entry
			urlPart = strings.TrimLeft(urlPart, ", ")
		}
		urlPart = strings.TrimSpace(urlPart)
		lastEnd = m[1]

		if urlPart == "" || isSVGDataURL(urlPart) {
			continue
		}
		return urlPart
	}

	// Fallback: first non-whitespace token
	if m := urlPatternRe.FindString(srcset); m != "" && !isSVGDataURL(m) {
		return m
	}
	return ""
}

// selectBestSource picks the best <source> element from a picture's sources.
// Prefers sources without media queries, then highest resolution.
func selectBestSource(sources []*goquery.Selection) *goquery.Selection {
	if len(sources) == 0 {
		return nil
	}
	if len(sources) == 1 {
		return sources[0]
	}

	// Prefer source without media query (default/fallback)
	for _, src := range sources {
		if _, hasMedia := src.Attr("media"); !hasMedia {
			return src
		}
	}

	// Find highest resolution source
	var bestSource *goquery.Selection
	var maxResolution float64

	for _, src := range sources {
		srcset, ok := src.Attr("srcset")
		if !ok || srcset == "" {
			continue
		}

		wm := widthPatternRe.FindStringSubmatch(srcset)
		if wm == nil {
			continue
		}
		width, err := strconv.Atoi(wm[1])
		if err != nil {
			continue
		}

		dpr := 1.0
		if dm := dprPatternRe.FindStringSubmatch(srcset); dm != nil {
			if parsed, err := strconv.ParseFloat(dm[1], 64); err == nil {
				dpr = parsed
			}
		}

		resolution := float64(width) * dpr
		if resolution > maxResolution {
			maxResolution = resolution
			bestSource = src
		}
	}

	if bestSource != nil {
		return bestSource
	}
	return sources[0]
}
