package defuddle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildScoringPage wraps content in a minimal page that has no semantic
// elements (article/main), forcing the scoring path in findMainContent.
func buildScoringPage(title, bodyHTML string) string {
	return fmt.Sprintf(`<html><head><title>%s</title></head><body>%s</body></html>`, title, bodyHTML)
}

func TestScoringIntegration(t *testing.T) {
	t.Parallel()

	t.Run("deeply nested div found by scoring", func(t *testing.T) {
		t.Parallel()
		// Build a deeply nested div tree with real content at the leaf.
		// No article/main, so scoring is the only path.
		paragraphs := strings.Repeat(
			"<p>The quick brown fox jumps over the lazy dog. "+
				"Pack my box with five dozen liquor jugs. "+
				"How vexingly quick daft zebras jump.</p>\n",
			8,
		)
		bodyHTML := fmt.Sprintf(
			`<div class="outer"><div class="middle"><div class="inner">%s</div></div></div>`,
			paragraphs,
		)
		html := buildScoringPage("Nested Div Test", bodyHTML)

		d, err := NewDefuddle(html, nil)
		require.NoError(t, err)

		result, err := d.Parse(context.Background())
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Greater(t, result.WordCount, 0,
			"scoring should find content inside nested divs")
		assert.Contains(t, result.Content, "quick brown fox",
			"extracted content should include paragraph text")
	})

	t.Run("div with most text wins over sparse siblings", func(t *testing.T) {
		t.Parallel()
		// One content-rich div alongside several low-text divs.
		richContent := strings.Repeat(
			"<p>Scoring algorithms evaluate text density and paragraph count "+
				"to identify the most relevant content block on a page.</p>\n",
			10,
		)
		bodyHTML := fmt.Sprintf(`
<div class="nav">Home About Contact</div>
<div class="sidebar">Tags: foo bar baz</div>
<div class="content">%s</div>
<div class="footer">Copyright 2024</div>
`, richContent)
		html := buildScoringPage("Multi-Div Test", bodyHTML)

		d, err := NewDefuddle(html, nil)
		require.NoError(t, err)

		result, err := d.Parse(context.Background())
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Greater(t, result.WordCount, 30,
			"word count should reflect the rich content div")
		assert.Contains(t, result.Content, "text density",
			"content from the rich div should be present")
	})

	t.Run("page with only nav header footer still returns result", func(t *testing.T) {
		t.Parallel()
		// All blocks are clutter. Parse must not return nil — it falls back
		// gracefully rather than crashing or returning an error.
		bodyHTML := `
<nav>Home | About | Contact | Blog | Portfolio</nav>
<header>My Website — Established 2010</header>
<footer>Privacy Policy | Terms of Service | Cookie Settings</footer>
`
		html := buildScoringPage("Clutter Only", bodyHTML)

		d, err := NewDefuddle(html, nil)
		require.NoError(t, err)

		result, err := d.Parse(context.Background())
		require.NoError(t, err)
		require.NotNil(t, result, "Parse must never return nil result for valid HTML")
	})

	t.Run("single high-scoring div selected over many low-scoring divs", func(t *testing.T) {
		t.Parallel()
		// Build many near-empty divs and one substantive one.
		var sb strings.Builder
		for i := range 15 {
			sb.WriteString(fmt.Sprintf(`<div class="filler-%d">word%d</div>`, i, i))
		}
		winnerContent := strings.Repeat(
			"<p>Content scoring selects the element whose text density, "+
				"paragraph structure, and link ratio best match article content. "+
				"This paragraph provides enough signal to rank above the fillers.</p>\n",
			6,
		)
		sb.WriteString(fmt.Sprintf(`<div class="winner">%s</div>`, winnerContent))

		html := buildScoringPage("Winner Div Test", sb.String())

		d, err := NewDefuddle(html, nil)
		require.NoError(t, err)

		result, err := d.Parse(context.Background())
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Greater(t, result.WordCount, 20,
			"the high-scoring div should dominate the word count")
		assert.Contains(t, result.Content, "text density",
			"winner div content should be present in extraction")
	})
}
