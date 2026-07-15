// Package shell classifies a fetched HTML page as static, ambiguous, or a
// likely JavaScript shell using pure-HTML heuristics (no browser, no chromedp).
// The CLI's --render-auto path escalates to a headless-Chrome render ONLY for a
// LikelyShell verdict; Static and Ambiguous pages are parsed as-fetched.
//
// The staged heuristic is adapted from the reference JS-shell detector: enough
// visible content => static; several independent content blocks => ambiguous
// (never escalate); otherwise corroborating signals (a JS-required notice, an
// empty SPA root container, a script-heavy body) promote to likely_shell.
package shell

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Classification is the detector verdict. Only LikelyShell triggers a render.
type Classification string

const (
	Static      Classification = "static"       // sufficient real content; parse as-fetched
	Ambiguous   Classification = "ambiguous"    // unclear; do NOT escalate (fail safe)
	LikelyShell Classification = "likely_shell" // JS-rendered shell; escalate to a browser
)

// Tunable thresholds for the staged heuristic (named, not magic).
const (
	// minVisibleText: at/above this many chars of real content a page is
	// presumed static unless it also announces a JS requirement or is a
	// script-dominated empty shell.
	minVisibleText = 200
	// meaningfulBlockChars: a content-tag node (or a shell root) counts as a
	// real "block" at/above this many trimmed chars.
	meaningfulBlockChars = 25
	// ambiguousBlocks: strictly more than this many meaningful blocks => the
	// page has real structure; classify Ambiguous and never escalate.
	ambiguousBlocks = 2
	// strongScriptRatio / weakScriptRatio: script length (inline text + src
	// attrs) relative to content length. strong is a standalone shell signal;
	// weak needs a corroborator.
	strongScriptRatio = 8
	weakScriptRatio   = 3
)

// contentTags hold human-visible article content; their text is "visible text".
var contentTags = []string{
	"p", "article", "main", "section",
	"h1", "h2", "h3", "h4", "h5", "h6",
	"li", "td", "th", "dd", "dt", "blockquote",
}

// shellSelectors match the empty root containers SPA frameworks mount into.
const shellSelectors = "#root, #app, #__next, #___gatsby, [data-reactroot], [ng-app], [data-server-rendered]"

// jsRequiredPhrases mark a page whose visible text tells the user to enable JS.
var jsRequiredPhrases = []string{
	"enable javascript", "javascript is required",
	"requires javascript", "javascript to run", "please enable js",
}

// Classify returns the detector verdict for an HTML document. Unparseable input
// returns Static (fail safe: never escalate on garbage the browser can't fix).
func Classify(html string) Classification {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return Static
	}

	// Capture script-derived signals BEFORE stripping scripts/noscript so the
	// removed subtrees don't pollute the visible-text and shell-root measures.
	scriptLen := scriptTextLen(doc)
	noscriptJS := noscriptMentionsJS(doc)
	doc.Find("script, style, noscript").Remove()

	shellMarker := hasEmptyShellRoot(doc)
	visible, blocks := visibleText(doc)
	visibleLen := len(visible)
	bodyLen := len(strings.TrimSpace(doc.Find("body").Text()))

	// contentLen is the better of semantic-tag text and total (script-free) body
	// text, so div-only pages aren't mistaken for empty shells.
	contentLen := visibleLen
	if bodyLen > contentLen {
		contentLen = bodyLen
	}

	// jsNoticeInBody: the rendered content itself is a "turn on JavaScript"
	// message — a definitive shell tell regardless of length.
	jsNoticeInBody := containsAny(strings.ToLower(visible), jsRequiredPhrases)
	jsRequired := noscriptJS || jsNoticeInBody
	strongScript := contentLen == 0 || scriptLen >= strongScriptRatio*max(contentLen, 1)
	weakScript := contentLen > 0 && scriptLen >= weakScriptRatio*contentLen

	// Enough real content and no JS-shell tell => static.
	if contentLen >= minVisibleText && !jsRequired && (!shellMarker || !strongScript) {
		return Static
	}
	// The rendered content IS a JS-required notice => definitely a shell.
	if jsNoticeInBody {
		return LikelyShell
	}
	// Several independent content blocks => real structure; never escalate.
	if blocks > ambiguousBlocks {
		return Ambiguous
	}
	// Corroborated shell signals promote to likely_shell.
	switch {
	case shellMarker && (jsRequired || weakScript || strongScript):
		return LikelyShell
	case jsRequired && strongScript:
		return LikelyShell
	case scriptLen > 0 && strongScript && blocks == 0:
		return LikelyShell
	default:
		return Ambiguous
	}
}

// visibleText concatenates trimmed text from content tags and counts how many
// hold at least meaningfulBlockChars of text. Nested tags over-count length and
// blocks, which biases toward Static/Ambiguous (fewer escalations) — the safe
// direction.
func visibleText(doc *goquery.Document) (string, int) {
	var b strings.Builder
	blocks := 0
	doc.Find(strings.Join(contentTags, ",")).Each(func(_ int, s *goquery.Selection) {
		t := strings.TrimSpace(s.Text())
		if t == "" {
			return
		}
		if len(t) >= meaningfulBlockChars {
			blocks++
		}
		b.WriteString(t)
		b.WriteByte(' ')
	})
	return strings.TrimSpace(b.String()), blocks
}

// scriptTextLen sums inline <script> text plus external src lengths, a proxy for
// how script-heavy the page is relative to its rendered content.
func scriptTextLen(doc *goquery.Document) int {
	total := 0
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		total += len(s.Text())
		if src, ok := s.Attr("src"); ok {
			total += len(src)
		}
	})
	return total
}

// noscriptMentionsJS reports whether any <noscript> mentions JavaScript — the
// classic "enable JavaScript" fallback a shell renders for no-JS clients.
func noscriptMentionsJS(doc *goquery.Document) bool {
	found := false
	doc.Find("noscript").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if strings.Contains(strings.ToLower(s.Text()), "javascript") {
			found = true
			return false
		}
		return true
	})
	return found
}

// hasEmptyShellRoot reports whether a known SPA root container is present and
// effectively empty (a mount point with no server-rendered content).
func hasEmptyShellRoot(doc *goquery.Document) bool {
	found := false
	doc.Find(shellSelectors).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if len(strings.TrimSpace(s.Text())) < meaningfulBlockChars {
			found = true
			return false
		}
		return true
	})
	return found
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
