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

		// Strip dangerous URL schemes in href/src/action
		if key == "href" || key == "src" || key == "action" {
			if isDangerousURL(attr.Val) {
				continue
			}
		}

		cleaned = append(cleaned, attr)
	}
	n.Attr = cleaned
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
	if strings.HasPrefix(lower, "vbscript:") {
		return true
	}
	return false
}
