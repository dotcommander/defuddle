package extractors

import (
	"html"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
)

// maxYouTubeTranscriptCues bounds an unusually large timed-text response while
// retaining enough content for ordinary videos.
const maxYouTubeTranscriptCues = 10_000

// maxYouTubeTranscriptBytes bounds emitted transcript text for direct library
// callers, whose input is not necessarily subject to the URL fetch body limit.
const maxYouTubeTranscriptBytes = 1 << 20

// YouTubeTranscriptExtractor turns YouTube's timed-text XML response into the
// readable transcript text it carries. It never fetches caption URLs itself.
type YouTubeTranscriptExtractor struct {
	*ExtractorBase
}

// NewYouTubeTranscriptExtractor creates an extractor for a YouTube timed-text response.
func NewYouTubeTranscriptExtractor(document *goquery.Document, url string, schemaOrgData any) *YouTubeTranscriptExtractor {
	return &YouTubeTranscriptExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
}

// CanExtract reports whether the document contains at least one nonblank transcript cue.
func (y *YouTubeTranscriptExtractor) CanExtract() bool {
	hasCue := false
	y.document.Find("transcript text").EachWithBreak(func(_ int, text *goquery.Selection) bool {
		hasCue = strings.TrimSpace(text.Text()) != ""
		return !hasCue
	})
	return hasCue
}

// Name returns the extractor's registry name.
func (y *YouTubeTranscriptExtractor) Name() string {
	return "YouTubeTranscriptExtractor"
}

// Extract returns a bounded, normalized, HTML-escaped transcript.
func (y *YouTubeTranscriptExtractor) Extract() *ExtractorResult {
	cues, truncated := y.cues()
	contentHTML := ""
	if len(cues) > 0 {
		contentHTML = "<p>" + html.EscapeString(strings.Join(cues, " ")) + "</p>"
	}
	if truncated {
		contentHTML += "<p><em>Transcript truncated at extraction limit.</em></p>"
	}

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"cueCount":  len(cues),
			"truncated": truncated,
		},
		Variables: map[string]string{
			"title": "YouTube Transcript",
			"site":  "YouTube",
		},
	}
}

func (y *YouTubeTranscriptExtractor) cues() ([]string, bool) {
	cues := make([]string, 0)
	previous := ""
	totalBytes := 0
	truncated := false
	y.document.Find("transcript text").EachWithBreak(func(_ int, text *goquery.Selection) bool {
		cue, exceedsLimit := normalizeYouTubeCue(text.Text(), maxYouTubeTranscriptBytes)
		if exceedsLimit {
			truncated = true
			return false
		}
		if cue == "" {
			return true
		}
		if cue == previous {
			return true
		}
		if len(cues) == maxYouTubeTranscriptCues {
			truncated = true
			return false
		}
		additionalBytes := len(cue)
		if len(cues) > 0 {
			additionalBytes++
		}
		if additionalBytes > maxYouTubeTranscriptBytes-totalBytes {
			truncated = true
			return false
		}
		cues = append(cues, cue)
		previous = cue
		totalBytes += additionalBytes
		return true
	})
	return cues, truncated
}

func normalizeYouTubeCue(raw string, limit int) (string, bool) {
	var normalized strings.Builder
	if len(raw) < limit {
		normalized.Grow(len(raw))
	} else {
		normalized.Grow(limit)
	}
	pendingSpace := false
	for _, char := range raw {
		if unicode.IsSpace(char) {
			pendingSpace = normalized.Len() > 0
			continue
		}
		additionalBytes := utf8.RuneLen(char)
		if pendingSpace {
			additionalBytes++
		}
		if additionalBytes > limit-normalized.Len() {
			return "", true
		}
		if pendingSpace {
			normalized.WriteByte(' ')
			pendingSpace = false
		}
		normalized.WriteRune(char)
	}
	return normalized.String(), false
}
