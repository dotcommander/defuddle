package defuddle

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToUTF8(t *testing.T) {
	t.Parallel()

	t.Run("utf8 input passthrough", func(t *testing.T) {
		t.Parallel()
		input := []byte("<p>Hello, world!</p>")
		got, err := toUTF8(input, "text/html; charset=utf-8")
		require.NoError(t, err)
		assert.Equal(t, string(input), got)
	})

	t.Run("latin-1 decoded to utf8", func(t *testing.T) {
		t.Parallel()
		// 0xE9 is 'é' in ISO-8859-1; in UTF-8 it would be 0xC3 0xA9
		input := []byte("caf\xe9")
		got, err := toUTF8(input, "text/html; charset=iso-8859-1")
		require.NoError(t, err)
		assert.Equal(t, "café", got)
	})

	t.Run("empty content-type assumes utf8", func(t *testing.T) {
		t.Parallel()
		input := []byte("<p>plain utf-8</p>")
		got, err := toUTF8(input, "")
		require.NoError(t, err)
		assert.Equal(t, string(input), got)
	})

	t.Run("empty body returns empty string", func(t *testing.T) {
		t.Parallel()
		got, err := toUTF8([]byte{}, "text/html; charset=utf-8")
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("windows-1252 smart quotes decoded to utf8", func(t *testing.T) {
		t.Parallel()
		// 0x93 = left double quotation mark, 0x94 = right double quotation mark in Windows-1252
		input := []byte("\x93Hello\x94")
		got, err := toUTF8(input, "text/html; charset=windows-1252")
		require.NoError(t, err)
		// Windows-1252 0x93/0x94 map to U+201C / U+201D (curly double quotes)
		assert.Contains(t, got, "\u201c", "expected left curly quote U+201C")
		assert.Contains(t, got, "\u201d", "expected right curly quote U+201D")
		assert.Contains(t, got, "Hello")
	})

	t.Run("utf8 bom passthrough with explicit charset", func(t *testing.T) {
		t.Parallel()
		// When an explicit charset=utf-8 is given, charset.NewReader does NOT
		// strip the BOM — the U+FEFF codepoint is preserved in the output.
		// This test documents the actual behaviour so future callers know they
		// may need to trim strings.TrimPrefix(s, "\ufeff") themselves.
		body := append([]byte{0xEF, 0xBB, 0xBF}, "<p>content</p>"...)
		got, err := toUTF8(body, "text/html; charset=utf-8")
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(got, "\ufeff"),
			"BOM is preserved as U+FEFF when charset=utf-8 is explicit")
		assert.Contains(t, got, "<p>content</p>",
			"body content follows the BOM")
	})
}
