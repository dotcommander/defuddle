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
	for _, tag := range []string{"object", "embed", "applet", "frame", "script", "style", "noscript", "base"} {
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

func TestSanitizeUnsafe_PreservesMathMLScript(t *testing.T) {
	t.Parallel()

	sel := parseSelection(t, `<div><math><semantics><annotation-xml encoding="application/xhtml+xml"><script>math data</script></annotation-xml><script>math expression</script></semantics></math><script>alert(1)</script></div>`)
	SanitizeUnsafe(sel)
	out, err := sel.Html()
	require.NoError(t, err)
	assert.NotContains(t, out, "alert(1)")
	assert.Contains(t, out, "math expression")
}

func TestSanitizeUnsafe_SanitizesSelectedRoot(t *testing.T) {
	t.Parallel()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<main onclick="evil()" href="java&#x09;script:evil()"><p>safe</p></main>`))
	require.NoError(t, err)
	root := doc.Find("main")
	SanitizeUnsafe(root)
	_, hasOnclick := root.Attr("onclick")
	_, hasHref := root.Attr("href")
	assert.False(t, hasOnclick)
	assert.False(t, hasHref)
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
		{"data:application/xhtml+xml src", `<div><iframe src="data:application/xhtml+xml,<script>alert(1)</script>"></iframe></div>`, "src"},
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
			_, ok := sel.Find("[" + tt.attr + "]").Attr(tt.attr)
			assert.False(t, ok, "expected dangerous %s attribute to be removed", tt.attr)
			out, _ := sel.Html()
			assert.NotContains(t, strings.ToLower(out), tt.attr+`="javascript`)
			assert.NotContains(t, strings.ToLower(out), tt.attr+`="data:text/html`)
			assert.NotContains(t, strings.ToLower(out), tt.attr+`="vbscript`)
		})
	}
}

func TestSanitizeUnsafe_StripsSVGDataImageOnly(t *testing.T) {
	t.Parallel()

	dangerous := parseSelection(t, `<div><img src="data:image/svg+xml;base64,PHN2Zz48c2NyaXB0PmFsZXJ0KDEpPC9zY3JpcHQ+PC9zdmc="></div>`)
	SanitizeUnsafe(dangerous)
	_, ok := dangerous.Find("img").Attr("src")
	assert.False(t, ok)

	benign := parseSelection(t, `<div><img src="data:image/png;base64,iVBORw0KGgo="></div>`)
	SanitizeUnsafe(benign)
	src, ok := benign.Find("img").Attr("src")
	require.True(t, ok)
	assert.Equal(t, "data:image/png;base64,iVBORw0KGgo=", src)
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

	type testCase struct {
		url       string
		dangerous bool
	}
	tests := make([]testCase, 0, 16+len(javascriptMIMETypes))
	tests = append(tests, []testCase{
		{"javascript:alert(1)", true},
		{"JAVASCRIPT:alert(1)", true},
		{"  javascript:void(0)", true},
		{"\tjava\tscript:alert(1)\r\n", true},
		{"java\nscript:alert(1)", true},
		{"vbscr\ript:MsgBox(1)", true},
		{"data:text/html,<script>alert(1)</script>", true},
		{"data:text/html;base64,abc", true},
		{"data: TEXT/HTML ; charset=utf-8 ;base64,abc", true},
		{"data:application/xhtml+xml,<script>alert(1)</script>", true},
		{"data:text/xml,<x/>", true},
		{"data:application/xml,<x/>", true},
		{"data:application/atom+xml,<x/>", true},
		{"data:image/svg+xml;base64,xxx", true},
		{"data:application/pdf;base64,xxx", true},
		{"data:application/ecmascript,alert(1)", true},
		{"data:text/javascript1.5,alert(1)", true},
		{"data:text/x-javascript,alert(1)", true},
		{"vbscript:MsgBox('XSS')", true},
		{"https://example.com", false},
		{"/path/to/page", false},
		{"data:image/png;base64,xxx", false},
		{"data:image/jpeg;base64,abc", false},
		{"data:text/plain,<b>plain text</b>", false},
		{"data:text/htmlx,<b>not html</b>", false},
		{"mailto:user@example.com", false},
		{"", false},
	}...)

	for mediaType := range javascriptMIMETypes {
		tests = append(tests, testCase{url: "data:" + mediaType + ",alert(1)", dangerous: true})
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.dangerous, isDangerousURL(tt.url))
		})
	}
}

func TestSanitizeUnsafe_StripsFormactionAndXLinkHref(t *testing.T) {
	t.Parallel()

	sel := parseSelection(t, `<div><button formaction="java&#x0A;script:evil()">go</button><svg><a xlink:href="data:text/html,evil"><text>x</text></a></svg></div>`)
	SanitizeUnsafe(sel)
	_, hasFormaction := sel.Find("button").Attr("formaction")
	assert.False(t, hasFormaction)

	link := sel.Find("svg a").Get(0)
	require.NotNil(t, link)
	for _, attr := range link.Attr {
		assert.False(t, attr.Key == "href" || attr.Key == "xlink:href", "unsafe xlink href survived: %#v", attr)
	}
}
