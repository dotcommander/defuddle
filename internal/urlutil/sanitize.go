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
		key := strings.ToLower(attr.Key)

		// Strip on* event handlers (onclick, onerror, onload, etc.)
		if strings.HasPrefix(key, eventHandlerPrefix) && len(key) > 2 {
			continue
		}

		// Strip srcdoc (can embed arbitrary HTML)
		if key == "srcdoc" {
			continue
		}

		// Strip dangerous URL schemes in single-URL attributes
		if _, ok := singleURLAttrs[key]; ok {
			if isDangerousURL(attr.Val) {
				continue
			}
		}

		// Filter dangerous entries from srcset-style comma-separated lists
		if _, ok := srcsetAttrs[key]; ok {
			safe := sanitizeSrcset(attr.Val)
			if safe == "" {
				continue
			}
			attr.Val = safe
		}

		cleaned = append(cleaned, attr)
	}
	n.Attr = cleaned
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
		strings.Contains(lower, "data:text/html") ||
		strings.Contains(lower, "data:text/javascript") ||
		strings.Contains(lower, "data:application/javascript")
}

// isDangerousURL returns true if the URL uses a scheme that can execute code.
func isDangerousURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)

	if strings.HasPrefix(lower, "javascript:") {
		return true
	}
	if strings.HasPrefix(lower, "data:text/html") {
		return true
	}
	if strings.HasPrefix(lower, "data:text/javascript") {
		return true
	}
	if strings.HasPrefix(lower, "data:application/javascript") {
		return true
	}
	if strings.HasPrefix(lower, "vbscript:") {
		return true
	}
	return false
}
