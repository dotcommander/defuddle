package urlutil

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseSelection(t *testing.T, html string) *goquery.Selection {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	return doc.Find("body").First()
}

func TestSanitizeUnsafe_RemovesUnsafeElements(t *testing.T) {
	t.Parallel()

	// frameset is omitted — it replaces <body> during HTML parsing and
	// cannot appear inside extracted content.
	for _, tag := range []string{"object", "embed", "applet", "frame"} {
		t.Run(tag, func(t *testing.T) {
			t.Parallel()
			html := "<div><" + tag + ">evil</" + tag + "><p>safe</p></div>"
			sel := parseSelection(t, html)
			SanitizeUnsafe(sel)
			out, _ := sel.Html()
			assert.NotContains(t, out, tag)
			assert.Contains(t, out, "safe")
		})
	}
}

func TestSanitizeUnsafe_StripsEventHandlers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		html string
	}{
		{"onclick", `<div><a href="/ok" onclick="alert(1)">Link</a></div>`},
		{"onerror", `<div><img src="x.jpg" onerror="alert(1)"></div>`},
		{"onload", `<div><body onload="alert(1)"><p>text</p></body></div>`},
		{"onmouseover", `<div><span onmouseover="steal()">hover</span></div>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sel := parseSelection(t, tt.html)
			SanitizeUnsafe(sel)
			out, _ := sel.Html()
			assert.NotContains(t, strings.ToLower(out), tt.name+"=")
		})
	}
}

func TestSanitizeUnsafe_StripsSrcdoc(t *testing.T) {
	t.Parallel()
	html := `<div><iframe srcdoc="<script>alert(1)</script>" src="safe.html"></iframe></div>`
	sel := parseSelection(t, html)
	SanitizeUnsafe(sel)
	out, _ := sel.Html()
	assert.NotContains(t, out, "srcdoc")
	assert.Contains(t, out, `src="safe.html"`)
}

func TestSanitizeUnsafe_StripsDangerousURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		html string
		attr string
	}{
		{"javascript href", `<div><a href="javascript:alert(1)">XSS</a></div>`, "href"},
		{"javascript src", `<div><iframe src="javascript:alert(1)"></iframe></div>`, "src"},
		{"data:text/html src", `<div><iframe src="data:text/html,<script>alert(1)</script>"></iframe></div>`, "src"},
		{"vbscript href", `<div><a href="vbscript:MsgBox('XSS')">VBS</a></div>`, "href"},
		{"javascript action", `<div><form action="javascript:void(0)"></form></div>`, "action"},
		{"case insensitive", `<div><a href="JAVASCRIPT:alert(1)">XSS</a></div>`, "href"},
		{"whitespace prefix", `<div><a href="  javascript:alert(1)">XSS</a></div>`, "href"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sel := parseSelection(t, tt.html)
			SanitizeUnsafe(sel)
			out, _ := sel.Html()
			assert.NotContains(t, strings.ToLower(out), tt.attr+`="javascript`)
			assert.NotContains(t, strings.ToLower(out), tt.attr+`="data:text/html`)
			assert.NotContains(t, strings.ToLower(out), tt.attr+`="vbscript`)
		})
	}
}

func TestSanitizeUnsafe_PreservesSafeContent(t *testing.T) {
	t.Parallel()

	html := `<div>
		<a href="https://example.com" class="link">Safe link</a>
		<img src="photo.jpg" alt="Photo">
		<p style="color: red">Styled text</p>
		<iframe src="https://youtube.com/embed/123"></iframe>
	</div>`

	sel := parseSelection(t, html)
	SanitizeUnsafe(sel)
	out, _ := sel.Html()

	assert.Contains(t, out, `href="https://example.com"`)
	assert.Contains(t, out, `src="photo.jpg"`)
	assert.Contains(t, out, `class="link"`)
	assert.Contains(t, out, `alt="Photo"`)
	assert.Contains(t, out, "Styled text")
	assert.Contains(t, out, `src="https://youtube.com/embed/123"`)
}

func TestIsDangerousURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url       string
		dangerous bool
	}{
		{"javascript:alert(1)", true},
		{"JAVASCRIPT:alert(1)", true},
		{"  javascript:void(0)", true},
		{"data:text/html,<script>alert(1)</script>", true},
		{"data:text/html;base64,abc", true},
		{"vbscript:MsgBox('XSS')", true},
		{"https://example.com", false},
		{"/path/to/page", false},
		{"data:image/png;base64,abc", false},
		{"mailto:user@example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.dangerous, isDangerousURL(tt.url))
		})
	}
}
