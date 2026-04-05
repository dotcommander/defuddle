// Package text provides text analysis utilities for content extraction.
package text

import "unicode"

// CountWords counts words in plain text, handling CJK characters correctly.
// Each CJK character (Han, Hangul, Hiragana, Katakana) counts as one word.
// Latin and other scripts are counted by whitespace boundaries.
// Go's native rune iteration handles supplementary planes (CJK Extension B+)
// that the TypeScript charCodeAt approach cannot.
func CountWords(text string) int {
	count := 0
	inWord := false

	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			if inWord {
				count++
				inWord = false
			}
		case isCJK(r):
			if inWord {
				count++
				inWord = false
			}
			count++ // Each CJK character is one word
		default:
			inWord = true
		}
	}

	if inWord {
		count++
	}

	return count
}

// isCJK returns true if the rune is a CJK character that should be counted
// individually as a word. Uses Go's unicode range tables which include
// supplementary plane characters (Extension B at U+20000+).
func isCJK(r rune) bool {
	return unicode.In(r,
		unicode.Han,      // CJK Unified Ideographs + Extensions A-G
		unicode.Hangul,   // Korean
		unicode.Hiragana, // Japanese
		unicode.Katakana, // Japanese
	)
}
