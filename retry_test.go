package defuddle

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wordRepeat builds a string of n repetitions of the given word phrase,
// each wrapped in a <p> tag.  It is used to produce controlled word counts
// without relying on strings.Repeat on multi-word strings.
func wordRepeat(phrase string, n int) string {
	var b strings.Builder
	for range n {
		b.WriteString("<p>")
		b.WriteString(phrase)
		b.WriteString("</p>\n")
	}
	return b.String()
}

// TestRetryPartialSelectors verifies Retry 1: when the main body of text lives
// inside an element whose class matches a partial-selector pattern (e.g.
// "sidebar-content"), the first pass strips it and falls below 200 words.
// The retry with RemovePartialSelectors=false must recover the content,
// producing > 2x the original word count so it is accepted.
func TestRetryPartialSelectors(t *testing.T) {
	t.Parallel()

	// "sidebar-content" is in the PartialSelectors list.
	// Place the bulk of the article text inside a div whose class contains
	// that token.  The outer <main> is selected as mainContent; the inner
	// sidebar-content div is a descendant and therefore NOT protected from
	// removal by the partial-selector pass.
	bulkContent := wordRepeat("important article word", 60) // ~180 words stripped by retry-1 trigger
	html := `<!DOCTYPE html><html><head><title>Retry Partial Test</title></head><body>
<main>
  <h1>Article Title</h1>
  <div class="sidebar-content">` + bulkContent + `</div>
  <p>short</p>
</main>
</body></html>`

	d, err := NewDefuddle(html, &Options{})
	require.NoError(t, err)

	result, err := d.Parse(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	// After retry the bulk content is recovered and the word count is high.
	assert.Greater(t, result.WordCount, 100,
		"partial-selector retry should have recovered the bulk content (got %d words)", result.WordCount)
}

// TestRetryHiddenElements verifies Retry 2: content hidden via display:none is
// stripped in the first two passes.  When word count stays < 50 after Retry 1,
// Retry 2 disables hidden-element removal and recovers the text.
func TestRetryHiddenElements(t *testing.T) {
	t.Parallel()

	// Wrap the bulk text in a display:none div so that the hidden-element pass
	// strips it.  There is no partial-selector match so Retry 1 does not help;
	// Retry 2 (RemoveHiddenElements=false) is the recovery path.
	// Use a class name that is NOT in the partial selector list.
	bulkContent := wordRepeat("revealed article word", 40) // 40 paragraphs × 3 words = 120 words
	html := `<!DOCTYPE html><html><head><title>Hidden Content Test</title></head><body>
<main>
  <h1>Hidden Content Article</h1>
  <div style="display:none" class="js-enhanced-content">` + bulkContent + `</div>
  <p>tiny</p>
</main>
</body></html>`

	d, err := NewDefuddle(html, &Options{})
	require.NoError(t, err)

	result, err := d.Parse(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	// Retry 2 should restore the hidden content, producing a meaningful word count.
	assert.Greater(t, result.WordCount, 50,
		"hidden-element retry should have recovered content (got %d words)", result.WordCount)
}

// TestRetryIndexPage verifies Retry 3: a listing page where every pass returns
// very few words.  Retries 1 and 2 require 2x improvement and are rejected.
// Retry 3 accepts ANY improvement, so the meager content is kept.
func TestRetryIndexPage(t *testing.T) {
	t.Parallel()

	// This page has minimal per-item text.  The scoring pass and selector passes
	// each leave only a handful of words.  No single retry doubles the count,
	// but Retry 3 accepts anything better than the current result.
	html := `<!DOCTYPE html><html><head><title>Index Page</title></head><body>
<main>
  <ul>
    <li><a href="/a">Post alpha</a></li>
    <li><a href="/b">Post beta</a></li>
    <li><a href="/c">Post gamma</a></li>
  </ul>
</main>
</body></html>`

	d, err := NewDefuddle(html, &Options{})
	require.NoError(t, err)

	result, err := d.Parse(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	// Something must be returned — Retry 3 ensures we never return nothing.
	assert.Greater(t, result.WordCount, 0,
		"index-page retry should return at least some content (got %d words)", result.WordCount)
}

// TestNoRetryNeeded confirms that a page with >= 200 clean words triggers no
// retry.  The single parse is sufficient and the result has high word count.
func TestNoRetryNeeded(t *testing.T) {
	t.Parallel()

	// 70 paragraphs × 8 words each ≈ 560 words — well above the 200-word threshold.
	bulkContent := wordRepeat("clean article content without any issues here", 70)
	html := `<!DOCTYPE html><html><head><title>Long Article</title></head><body>
<article>
  <h1>A Well-Written Article</h1>` + bulkContent + `</article>
</body></html>`

	d, err := NewDefuddle(html, &Options{})
	require.NoError(t, err)

	result, err := d.Parse(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Greater(t, result.WordCount, 200,
		"content-rich page should produce > 200 words without retry (got %d words)", result.WordCount)
}

// TestRetryNoImprovement verifies that retry 1 is rejected when disabling
// partial selectors does not change the word count. The sidebar-content div
// is outside <article> so it is never part of mainContent in either pass.
// Both passes produce identical word counts → retry rejected (matching TS:
// retryResult.wordCount > result.wordCount).
func TestRetryNoImprovement(t *testing.T) {
	t.Parallel()

	// Main article text: 24 paragraphs × 5 words = 120 words (clean, not matched by selectors).
	mainArticle := wordRepeat("main article body text here", 24)

	// Extra content in a partial-selector-matched div OUTSIDE the article.
	// It is never part of mainContent, so removing/keeping the partial selector
	// has no effect on the final word count.
	extraContent := wordRepeat("extra sidebar text word here", 8)

	html := `<!DOCTYPE html><html><head><title>Threshold Test</title></head><body>
<article>
  <h1>Threshold Test Article</h1>` + mainArticle + `</article>
<div class="sidebar-content">` + extraContent + `</div>
</body></html>`

	d, err := NewDefuddle(html, &Options{})
	require.NoError(t, err)

	result, err := d.Parse(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	// Both passes return the same article content; retry is rejected.
	assert.Less(t, result.WordCount, 200,
		"content outside mainContent should not inflate word count (got %d words)", result.WordCount)
	assert.Greater(t, result.WordCount, 50,
		"article content should still be present (got %d words)", result.WordCount)
}
