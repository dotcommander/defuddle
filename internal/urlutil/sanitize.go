package urlutil

import (
	"mime"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// unsafeElementNames are active HTML elements that must never survive in
// extracted content. MathML script elements are data, not executable HTML.
var unsafeElementNames = map[string]struct{}{
	"applet":   {},
	"base":     {},
	"embed":    {},
	"frame":    {},
	"frameset": {},
	"noscript": {},
	"object":   {},
	"script":   {},
	"style":    {},
}

// javascriptMIMETypes is the JavaScript MIME type group defined by WHATWG MIME
// Sniffing. Data URLs with any of these exact MIME essences are active content.
var javascriptMIMETypes = map[string]struct{}{
	"application/ecmascript":   {},
	"application/javascript":   {},
	"application/x-ecmascript": {},
	"application/x-javascript": {},
	"text/ecmascript":          {},
	"text/javascript":          {},
	"text/javascript1.0":       {},
	"text/javascript1.1":       {},
	"text/javascript1.2":       {},
	"text/javascript1.3":       {},
	"text/javascript1.4":       {},
	"text/javascript1.5":       {},
	"text/jscript":             {},
	"text/livescript":          {},
	"text/x-ecmascript":        {},
	"text/x-javascript":        {},
}

const eventHandlerPrefix = "on"

var singleURLAttrs = map[string]struct{}{
	"action":     {},
	"data-src":   {},
	"formaction": {},
	"href":       {},
	"poster":     {},
	"src":        {},
	"xlink:href": {},
}

var srcsetAttrs = map[string]struct{}{
	"data-srcset": {},
	"srcset":      {},
}

// SanitizeUnsafe strips dangerous elements, event handlers, and unsafe URLs
// from extracted content. The selected roots and all descendants are covered.
func SanitizeUnsafe(element *goquery.Selection) {
	if element == nil {
		return
	}

	element.Find("*").Each(func(_ int, selected *goquery.Selection) {
		if isUnsafeElement(selected.Get(0)) {
			selected.Remove()
		}
	})
	element.Each(func(_ int, selected *goquery.Selection) {
		if isUnsafeElement(selected.Get(0)) {
			selected.Remove()
			return
		}
		sanitizeNode(selected.Get(0))
	})
	element.Find("*").Each(func(_ int, selected *goquery.Selection) {
		sanitizeNode(selected.Get(0))
	})
}

func isUnsafeElement(n *html.Node) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	name := strings.ToLower(n.Data)
	if name == "script" && n.Namespace == "math" {
		return false
	}
	_, unsafe := unsafeElementNames[name]
	return unsafe
}

func sanitizeNode(n *html.Node) {
	if n == nil || n.Type != html.ElementNode {
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

func keepAttr(attr html.Attribute) (html.Attribute, bool) {
	key := strings.ToLower(attr.Key)
	qualifiedKey := key
	if attr.Namespace != "" {
		qualifiedKey = strings.ToLower(attr.Namespace) + ":" + key
	}

	if strings.HasPrefix(key, eventHandlerPrefix) && len(key) > len(eventHandlerPrefix) {
		return attr, false
	}
	if key == "srcdoc" {
		return attr, false
	}

	if _, ok := singleURLAttrs[qualifiedKey]; !ok {
		_, ok = singleURLAttrs[key]
		if ok && attr.Namespace != "" && qualifiedKey != "xlink:href" {
			ok = false
		}
		if ok && isDangerousURL(attr.Val) {
			return attr, false
		}
	} else if isDangerousURL(attr.Val) {
		return attr, false
	}

	if _, ok := srcsetAttrs[key]; ok {
		safe := sanitizeSrcset(attr.Val)
		if safe == "" {
			return attr, false
		}
		attr.Val = safe
	}

	return attr, true
}

// sanitizeSrcset applies the same URL classifier to every parsed candidate.
// Safe input is returned byte-for-byte; mixed input retains only safe entries.
func sanitizeSrcset(raw string) string {
	candidates := parseSrcsetCandidates(raw)
	if len(candidates) == 0 {
		return ""
	}

	kept := make([]string, 0, len(candidates))
	removed := false
	for _, candidate := range candidates {
		// Classify the whole candidate so ASCII tabs, LF, or CR embedded in an
		// obfuscated scheme are removed only in the detection copy. The raw
		// candidate remains intact when retained.
		if isDangerousURL(candidate.raw) {
			removed = true
			continue
		}
		kept = append(kept, candidate.raw)
	}
	if !removed {
		return raw
	}
	return strings.Join(kept, ", ")
}

type srcsetCandidate struct {
	raw string
}

// parseSrcsetCandidates follows the candidate-token boundaries relevant to URL
// classification, including commas inside data URLs and optional descriptors.
func parseSrcsetCandidates(raw string) []srcsetCandidate {
	var candidates []srcsetCandidate
	for pos := 0; pos < len(raw); {
		for pos < len(raw) && (isASCIIWhitespace(raw[pos]) || raw[pos] == ',') {
			pos++
		}
		if pos >= len(raw) {
			break
		}

		start := pos
		for pos < len(raw) && !isASCIIWhitespace(raw[pos]) {
			pos++
		}
		url := raw[start:pos]
		trimmedURL := strings.TrimRight(url, ",")
		if trimmedURL == "" {
			continue
		}
		if len(trimmedURL) != len(url) {
			candidates = append(candidates, srcsetCandidate{raw: trimmedURL})
			continue
		}

		depth := 0
		for pos < len(raw) {
			switch raw[pos] {
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			case ',':
				if depth == 0 {
					goto candidateDone
				}
			}
			pos++
		}

	candidateDone:
		candidates = append(candidates, srcsetCandidate{
			raw: strings.TrimSpace(raw[start:pos]),
		})
		if pos < len(raw) && raw[pos] == ',' {
			pos++
		}
	}
	return candidates
}

func isASCIIWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func isDangerousURL(raw string) bool {
	normalized := strings.ToLower(normalizeURLForDetection(raw))
	if strings.HasPrefix(normalized, "javascript:") || strings.HasPrefix(normalized, "vbscript:") {
		return true
	}
	if !strings.HasPrefix(normalized, "data:") {
		return false
	}
	return isDangerousDataURL(normalized)
}

// normalizeURLForDetection mirrors the preprocessing browsers apply while
// finding a URL scheme. It is never used to rewrite accepted attribute values.
func normalizeURLForDetection(raw string) string {
	trimmed := strings.TrimSpace(raw)
	return strings.Map(func(r rune) rune {
		switch r {
		case '\t', '\n', '\r':
			return -1
		default:
			return r
		}
	}, trimmed)
}

func isDangerousDataURL(normalized string) bool {
	comma := strings.IndexByte(normalized, ',')
	if comma < 0 {
		return false
	}

	metadata := strings.TrimSpace(normalized[len("data:"):comma])
	if len(metadata) >= len(";base64") && strings.EqualFold(metadata[len(metadata)-len(";base64"):], ";base64") {
		metadata = strings.TrimSpace(metadata[:len(metadata)-len(";base64")])
	}
	if metadata == "" {
		metadata = "text/plain"
	}

	mediaType, _, err := mime.ParseMediaType(metadata)
	if err != nil {
		return false
	}
	essence := strings.ToLower(mediaType)
	if essence == "text/html" || essence == "application/pdf" || essence == "text/xml" || essence == "application/xml" {
		return true
	}
	if strings.HasSuffix(essence, "+xml") {
		return true
	}
	_, dangerousJavaScript := javascriptMIMETypes[essence]
	return dangerousJavaScript
}
