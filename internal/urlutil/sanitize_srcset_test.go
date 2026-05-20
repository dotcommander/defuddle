package urlutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeUnsafe_StripsDangerousURLs_ExtraAttrs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		html string
		attr string
	}{
		{"javascript poster", `<div><video poster="javascript:alert(1)" src="vid.mp4"></video></div>`, "poster"},
		{"data:text/html poster", `<div><video poster="data:text/html,<script>x</script>" src="vid.mp4"></video></div>`, "poster"},
		{"vbscript poster", `<div><video poster="vbscript:MsgBox(1)" src="vid.mp4"></video></div>`, "poster"},
		{"javascript data-src", `<div><img data-src="javascript:alert(1)" alt="x"></div>`, "data-src"},
		{"case insensitive poster", `<div><video poster="JAVASCRIPT:alert(1)" src="vid.mp4"></video></div>`, "poster"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sel := parseSelection(t, tt.html)
			SanitizeUnsafe(sel)
			out, _ := sel.Html()
			lower := strings.ToLower(out)
			assert.NotContains(t, lower, tt.attr+`="javascript`)
			assert.NotContains(t, lower, tt.attr+`="data:text/html`)
			assert.NotContains(t, lower, tt.attr+`="vbscript`)
		})
	}
}

func TestSanitizeUnsafe_PreservesSafeExtraAttrs(t *testing.T) {
	t.Parallel()

	html := `<div>
		<video poster="thumb.jpg" src="movie.mp4"></video>
		<img data-src="https://cdn.example.com/lazy.jpg" alt="lazy">
		<img data-src="/relative/path.png" alt="rel">
	</div>`

	sel := parseSelection(t, html)
	SanitizeUnsafe(sel)
	out, _ := sel.Html()

	assert.Contains(t, out, `poster="thumb.jpg"`)
	assert.Contains(t, out, `data-src="https://cdn.example.com/lazy.jpg"`)
	assert.Contains(t, out, `data-src="/relative/path.png"`)
}

func TestSanitizeUnsafe_SrcsetFiltersDangerousEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		html    string
		wantHas []string // substrings that must remain
		wantNot []string // lowercased substrings that must be absent
	}{
		{
			name:    "all dangerous strips attribute",
			html:    `<div><img srcset="javascript:alert(1) 1x, vbscript:x 2x" alt="x"></div>`,
			wantNot: []string{`srcset="`, "javascript", "vbscript"},
		},
		{
			name:    "any dangerous drops whole attribute",
			html:    `<div><img srcset="javascript:alert(1) 1x, /safe.jpg 2x" alt="x"></div>`,
			wantNot: []string{`srcset="`, "javascript"},
		},
		{
			name:    "all safe preserved",
			html:    `<div><img srcset="a.jpg 1x, b.jpg 2x, https://cdn.example.com/c.jpg 3x" alt="x"></div>`,
			wantHas: []string{"a.jpg 1x", "b.jpg 2x", "https://cdn.example.com/c.jpg 3x"},
		},
		{
			name:    "width descriptors preserved",
			html:    `<div><img srcset="small.jpg 100w, large.jpg 200w" alt="x"></div>`,
			wantHas: []string{"small.jpg 100w", "large.jpg 200w"},
		},
		{
			name:    "data-srcset dangerous stripped",
			html:    `<div><img data-srcset="javascript:alert(1) 1x" alt="x"></div>`,
			wantNot: []string{`data-srcset="`, "javascript"},
		},
		{
			name:    "data-srcset any dangerous drops all",
			html:    `<div><img data-srcset="vbscript:x 1x, /ok.jpg 2x" alt="x"></div>`,
			wantNot: []string{`data-srcset="`, "vbscript"},
		},
		{
			name:    "case insensitive dangerous scheme",
			html:    `<div><img srcset="JAVASCRIPT:alert(1) 1x, /ok.jpg 2x" alt="x"></div>`,
			wantNot: []string{`srcset="`, "javascript"},
		},
		{
			name:    "data:image preserved in srcset",
			html:    `<div><img srcset="data:image/png;base64,AAAA 1x, /large.png 2x" alt="x"></div>`,
			wantHas: []string{"data:image/png;base64,AAAA 1x", "/large.png 2x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sel := parseSelection(t, tt.html)
			SanitizeUnsafe(sel)
			out, _ := sel.Html()
			lower := strings.ToLower(out)
			for _, want := range tt.wantHas {
				assert.Contains(t, out, want, "expected %q in %q", want, out)
			}
			for _, notWant := range tt.wantNot {
				assert.NotContains(t, lower, notWant, "did not expect %q in %q", notWant, lower)
			}
		})
	}
}

func TestSanitizeSrcset(t *testing.T) {
	t.Parallel()

	// Defensive shortcut: if any dangerous scheme appears anywhere in the
	// value the whole attribute is dropped, since WHATWG srcset allows commas
	// inside data: URLs that a naive split cannot disambiguate safely.
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"all safe", "a.jpg 1x, b.jpg 2x", "a.jpg 1x, b.jpg 2x"},
		{"all dangerous", "javascript:alert(1) 1x, vbscript:x 2x", ""},
		{"mixed any-dangerous drops all", "javascript:alert(1) 1x, /safe.jpg 2x", ""},
		{"empty", "", ""},
		{"single safe no descriptor", "image.jpg", "image.jpg"},
		{"single dangerous no descriptor", "javascript:alert(1)", ""},
		{"whitespace before dangerous", "  javascript:alert(1) 1x", ""},
		{"data:image safe", "data:image/png;base64,abc 1x", "data:image/png;base64,abc 1x"},
		{"data:text/html anywhere drops all", "data:text/html,x 1x, ok.jpg 2x", ""},
		{"width descriptors", "small.jpg 100w, large.jpg 200w", "small.jpg 100w, large.jpg 200w"},
		{"case-insensitive dangerous", "JAVASCRIPT:x 1x, ok.jpg 2x", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, sanitizeSrcset(tt.in))
		})
	}
}
