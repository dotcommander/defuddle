package extractors

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYouTubeTranscriptExtractor_ExtractsEscapedDeduplicatedCues(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, `<transcript><text start="0">First   &amp; &lt;second&gt;</text><text start="1">First
&amp; &lt;second&gt;</text><text start="2">Next</text><text start="3">First &amp; &lt;second&gt;</text></transcript>`)
	ext := NewYouTubeTranscriptExtractor(doc, "https://www.youtube.com/api/timedtext?v=abc", nil)
	result := ext.Extract()

	require.True(t, ext.CanExtract())
	require.NotNil(t, result)
	assert.Equal(t, `<p>First &amp; &lt;second&gt; Next First &amp; &lt;second&gt;</p>`, result.ContentHTML)
	assert.Equal(t, 3, result.ExtractedContent["cueCount"])
	assert.Equal(t, false, result.ExtractedContent["truncated"])
}

func TestYouTubeTranscriptExtractor_CanExtractRequiresTranscriptTextCues(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, `<html><body><p>Not a transcript</p><text>Or a cue</text></body></html>`)
	ext := NewYouTubeTranscriptExtractor(doc, "https://www.youtube.com/api/timedtext?v=abc", nil)

	assert.False(t, ext.CanExtract())
}

func TestYouTubeTranscriptExtractor_BlankCuesDoNotBreakDeduplication(t *testing.T) {
	t.Parallel()

	doc := newTestDoc(t, `<transcript><text>A</text><text>   </text><text>A</text></transcript>`)
	ext := NewYouTubeTranscriptExtractor(doc, "https://www.youtube.com/api/timedtext?v=abc", nil)

	require.True(t, ext.CanExtract())
	assert.Equal(t, `<p>A</p>`, ext.Extract().ContentHTML)

	blankDoc := newTestDoc(t, `<transcript><text>   </text></transcript>`)
	assert.False(t, NewYouTubeTranscriptExtractor(blankDoc, "https://www.youtube.com/api/timedtext?v=abc", nil).CanExtract())
}

func TestYouTubeTranscriptExtractor_BoundsTotalText(t *testing.T) {
	t.Parallel()

	boundaryCue := strings.Repeat("a", maxYouTubeTranscriptBytes)
	doc := newTestDoc(t, `<transcript><text>`+boundaryCue+`</text><text>overflow</text></transcript>`)
	result := NewYouTubeTranscriptExtractor(doc, "https://www.youtube.com/api/timedtext?v=abc", nil).Extract()
	assert.Equal(t, 1, result.ExtractedContent["cueCount"])
	assert.Equal(t, true, result.ExtractedContent["truncated"])
	assert.Contains(t, result.ContentHTML, "Transcript truncated")

	oversizedCue := strings.Repeat("b", maxYouTubeTranscriptBytes+1)
	oversizedDoc := newTestDoc(t, `<transcript><text>`+oversizedCue+`</text></transcript>`)
	oversized := NewYouTubeTranscriptExtractor(oversizedDoc, "https://www.youtube.com/api/timedtext?v=abc", nil).Extract()
	assert.Equal(t, 0, oversized.ExtractedContent["cueCount"])
	assert.Equal(t, true, oversized.ExtractedContent["truncated"])
	assert.Equal(t, `<p><em>Transcript truncated at extraction limit.</em></p>`, oversized.ContentHTML)
}

func TestYouTubeTranscriptExtractor_NormalizesBeforeTruncationAccounting(t *testing.T) {
	t.Parallel()

	boundaryCue := strings.Repeat("a", maxYouTubeTranscriptBytes)
	duplicateDoc := newTestDoc(t, `<transcript><text>`+boundaryCue+`</text><text>`+boundaryCue+`</text></transcript>`)
	duplicate := NewYouTubeTranscriptExtractor(duplicateDoc, "https://www.youtube.com/api/timedtext?v=abc", nil).Extract()
	assert.Equal(t, 1, duplicate.ExtractedContent["cueCount"])
	assert.Equal(t, false, duplicate.ExtractedContent["truncated"])

	spacedDoc := newTestDoc(t, `<transcript><text>A`+strings.Repeat(" ", maxYouTubeTranscriptBytes)+`B</text></transcript>`)
	spaced := NewYouTubeTranscriptExtractor(spacedDoc, "https://www.youtube.com/api/timedtext?v=abc", nil).Extract()
	assert.Equal(t, `<p>A B</p>`, spaced.ContentHTML)
	assert.Equal(t, false, spaced.ExtractedContent["truncated"])
}
