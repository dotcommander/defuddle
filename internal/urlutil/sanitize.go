package urlutil

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// unsafeElements are HTML elements that should be stripped for XSS safety.
var unsafeElements = []string{"object", "embed", "applet", "frame", "frameset"}

// eventHandlerPrefix matches on* event attributes (onclick, onerror, etc.).
const eventHandlerPrefix = "on"

// executableDataPrefixes are data URI media types that browsers may execute
// as active content. Both single-URL and multi-URL attributes use this list so
// their security policy cannot drift.
var executableDataPrefixes = []string{
	"data:text/html",
	"data:text/javascript",
	"data:image/svg+xml",
	"data:application/javascript",
	"data:application/xhtml+xml",
}

// singleURLAttrs are attributes whose value is a single URL. Dangerous-scheme
// values cause the whole attribute to be stripped.
var singleURLAttrs = map[string]struct{}{
	"href":     {},
	"src":      {},
	"action":   {},
	"poster":   {},
	"data-src": {},
}

// srcsetAttrs are attributes whose value is a comma-separated list of
// "URL [descriptor]" entries. Dangerous entries are dropped; safe entries are
// preserved. The attribute is stripped entirely only if no safe entries remain.
var srcsetAttrs = map[string]struct{}{
	"srcset":      {},
	"data-srcset": {},
}

// SanitizeUnsafe strips dangerous elements, event handlers, and unsafe URLs
// from the extracted content to prevent XSS when the HTML is rendered.
func SanitizeUnsafe(element *goquery.Selection) {
	// Remove unsafe elements entirely
	for _, tag := range unsafeElements {
		element.Find(tag).Remove()
	}

	// Walk all elements: strip event handlers, srcdoc, and dangerous URLs
	element.Find("*").Each(func(_ int, el *goquery.Selection) {
		node := el.Get(0)
		sanitizeNode(node)
	})
}

// sanitizeNode removes dangerous attributes from a single html.Node.
func sanitizeNode(n *html.Node) {
	if n.Type != html.ElementNode {
		return
	}

	cleaned := n.Attr[:0]
	for _, attr := range n.Attr {
		if kept, ok := keepAttr(attr); ok {
			cleaned = append(cleaned, kept)
		}
	}
	n.Attr = cleaned
}

// keepAttr reports whether an attribute survives sanitization, returning the
// (possibly rewritten) attribute to keep. Drops on* event handlers, srcdoc,
// dangerous single-URL schemes, and fully-dangerous srcset values; rewrites a
// srcset attribute to its safe subset.
func keepAttr(attr html.Attribute) (html.Attribute, bool) {
	key := strings.ToLower(attr.Key)

	// Strip on* event handlers (onclick, onerror, onload, etc.)
	if strings.HasPrefix(key, eventHandlerPrefix) && len(key) > 2 {
		return attr, false
	}

	// Strip srcdoc (can embed arbitrary HTML)
	if key == "srcdoc" {
		return attr, false
	}

	// Strip dangerous URL schemes in single-URL attributes
	if _, ok := singleURLAttrs[key]; ok {
		if isDangerousURL(attr.Val) {
			return attr, false
		}
	}

	// Filter dangerous entries from srcset-style comma-separated lists
	if _, ok := srcsetAttrs[key]; ok {
		safe := sanitizeSrcset(attr.Val)
		if safe == "" {
			return attr, false
		}
		attr.Val = safe
	}

	return attr, true
}

// sanitizeSrcset filters a comma-separated srcset value, dropping any entry
// whose URL portion uses a dangerous scheme. Returns "" if no safe entries
// remain so the caller can strip the attribute entirely.
//
// Defensive shortcut: if any dangerous scheme substring appears anywhere in
// the raw value, the entire attribute is stripped. WHATWG srcset parsing
// allows commas inside data: URLs, which a naive split cannot disambiguate;
// rather than risk a partial-strip that smuggles a dangerous URL through,
// drop the whole attribute when any dangerous marker is present.
func sanitizeSrcset(raw string) string {
	if hasDangerousScheme(raw) {
		return ""
	}
	entries := strings.Split(raw, ",")
	kept := entries[:0]
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		// First whitespace-separated token is the URL; remainder is the descriptor.
		url := trimmed
		if i := strings.IndexAny(trimmed, " \t\n\r\f"); i >= 0 {
			url = trimmed[:i]
		}
		if isDangerousURL(url) {
			continue
		}
		kept = append(kept, entry)
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, ",")
}

// hasDangerousScheme returns true if any dangerous URL scheme marker appears
// anywhere in the value (case-insensitive). Used as a defensive shortcut for
// multi-URL attributes where exact parsing is ambiguous.
func hasDangerousScheme(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "javascript:") ||
		strings.Contains(lower, "vbscript:") ||
		containsExecutableDataPrefix(lower)
}

// isDangerousURL returns true if the URL uses a scheme that can execute code.
func isDangerousURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)

	return strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "vbscript:") ||
		hasExecutableDataPrefix(lower)
}

func containsExecutableDataPrefix(value string) bool {
	for _, prefix := range executableDataPrefixes {
		if strings.Contains(value, prefix) {
			return true
		}
	}
	return false
}

func hasExecutableDataPrefix(value string) bool {
	for _, prefix := range executableDataPrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
