package extractors

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// getTitle gets the video title
// TypeScript original code:
//
//	private getTitle(videoData: any): string {
//		if (videoData.name) {
//			return videoData.name;
//		}
//
//		// Fallback to document title
//		let title = this.document.title;
//		// Remove " - YouTube" suffix if present
//		return title.replace(/ - YouTube$/, '');
//	}
func (y *YouTubeExtractor) getTitle(videoData map[string]any) string {
	if name, exists := videoData["name"]; exists {
		if nameStr, ok := name.(string); ok && nameStr != "" {
			slog.Debug("YouTube extractor: using title from schema.org", "title", nameStr)
			return nameStr
		}
	}

	// Fallback to document title
	title := y.document.Find("title").Text()
	// Remove " - YouTube" suffix if present
	title = strings.TrimSuffix(title, " - YouTube")
	if title != "" {
		slog.Debug("YouTube extractor: using title from document", "title", title)
		return title
	}

	if videoDetails := y.playerResponseVideoDetails(); videoDetails != nil {
		if title, _ := videoDetails["title"].(string); title != "" {
			slog.Debug("YouTube extractor: using title from player response", "title", title)
			return title
		}
	}

	slog.Debug("YouTube extractor: no title found")
	return ""
}

// getAuthor resolves the channel name using a 3-level fallback chain matching TS:
// 1. DOM selectors (YouTube-specific owner elements)
// 2. ytInitialPlayerResponse inline JSON
// 3. Schema.org VideoObject author field
func (y *YouTubeExtractor) getAuthor(videoData map[string]any) string {
	if name := y.getChannelNameFromDOM(); name != "" {
		return name
	}
	if name := y.getChannelNameFromPlayerResponse(); name != "" {
		return name
	}
	if author, exists := videoData["author"]; exists {
		if authorStr, ok := author.(string); ok {
			return authorStr
		}
	}
	return ""
}

// getChannelNameFromDOM extracts channel name from YouTube DOM elements.
func (y *YouTubeExtractor) getChannelNameFromDOM() string {
	selectors := []string{
		`ytd-video-owner-renderer #channel-name a[href^="/@"]`,
		`#owner-name a[href^="/@"]`,
	}
	for _, sel := range selectors {
		el := y.document.Find(sel).First()
		if text := strings.TrimSpace(el.Text()); text != "" {
			return text
		}
	}
	// Fallback: microdata itemprop="author"
	authorRoot := y.document.Find(`[itemprop="author"]`).First()
	if authorRoot.Length() == 0 {
		return ""
	}
	if content, exists := authorRoot.Find(`meta[itemprop="name"]`).Attr("content"); exists {
		if v := strings.TrimSpace(content); v != "" {
			return v
		}
	}
	if content, exists := authorRoot.Find(`link[itemprop="name"]`).Attr("content"); exists {
		if v := strings.TrimSpace(content); v != "" {
			return v
		}
	}
	if el := authorRoot.Find(`[itemprop="name"], a, span`).First(); el.Length() > 0 {
		if v := strings.TrimSpace(el.Text()); v != "" {
			return v
		}
	}
	return ""
}

// getChannelNameFromPlayerResponse parses ytInitialPlayerResponse from inline scripts.
func (y *YouTubeExtractor) getChannelNameFromPlayerResponse() string {
	data := y.playerResponse()
	if data == nil {
		return ""
	}
	// videoDetails.author or videoDetails.ownerChannelName
	if vd, ok := data["videoDetails"].(map[string]any); ok {
		if author, ok := vd["author"].(string); ok && author != "" {
			return author
		}
		if owner, ok := vd["ownerChannelName"].(string); ok && owner != "" {
			return owner
		}
	}
	// microformat.playerMicroformatRenderer.ownerChannelName
	if mf, ok := data["microformat"].(map[string]any); ok {
		if pmr, ok := mf["playerMicroformatRenderer"].(map[string]any); ok {
			if owner, ok := pmr["ownerChannelName"].(string); ok && owner != "" {
				return owner
			}
		}
	}
	return ""
}

// playerResponse returns the inline player data when YouTube supplied it with
// the watch page. It is intentionally only a fallback: schema.org and DOM
// values retain their existing precedence.
func (y *YouTubeExtractor) playerResponse() map[string]any {
	return y.parseInlineJSON("ytInitialPlayerResponse")
}

func (y *YouTubeExtractor) playerResponseVideoDetails() map[string]any {
	data := y.playerResponse()
	if data == nil {
		return nil
	}
	videoDetails, _ := data["videoDetails"].(map[string]any)
	return videoDetails
}

func (y *YouTubeExtractor) playerResponseDescription() string {
	data := y.playerResponse()
	if data == nil {
		return ""
	}
	if videoDetails, _ := data["videoDetails"].(map[string]any); videoDetails != nil {
		if description, _ := videoDetails["shortDescription"].(string); strings.TrimSpace(description) != "" {
			return strings.TrimSpace(description)
		}
	}
	microformat, _ := data["microformat"].(map[string]any)
	renderer, _ := microformat["playerMicroformatRenderer"].(map[string]any)
	description, _ := renderer["description"].(map[string]any)
	if text, _ := description["simpleText"].(string); strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}
	return ""
}

func (y *YouTubeExtractor) playerResponsePublished() string {
	if videoDetails := y.playerResponseVideoDetails(); videoDetails != nil {
		for _, key := range []string{"publishDate", "uploadDate"} {
			if date, _ := videoDetails[key].(string); date != "" {
				return date
			}
		}
	}

	data := y.playerResponse()
	if data == nil {
		return ""
	}
	microformat, _ := data["microformat"].(map[string]any)
	renderer, _ := microformat["playerMicroformatRenderer"].(map[string]any)
	for _, key := range []string{"publishDate", "uploadDate"} {
		if date, _ := renderer[key].(string); date != "" {
			return date
		}
	}
	return ""
}

// parseInlineJSON finds a global JS variable assignment and extracts the JSON object.
// Matches TS parseInlineJson: scans script tags for `globalName`, then brace-balances
// to extract the JSON block.
func (y *YouTubeExtractor) parseInlineJSON(globalName string) map[string]any {
	var result map[string]any
	y.document.Find("script").Each(func(_ int, s *goquery.Selection) {
		if result != nil {
			return
		}
		if parsed := parseGlobalJSON(s.Text(), globalName); parsed != nil {
			result = parsed
		}
	})
	return result
}

// parseGlobalJSON finds globalName in text and returns the first balanced
// {...} JSON object that follows it, or nil if absent or unparseable. Braces
// inside JSON strings do not affect object balancing, including escaped quotes.
func parseGlobalJSON(text, globalName string) map[string]any {
	searchFrom := 0
	for searchFrom < len(text) {
		offset := strings.Index(text[searchFrom:], globalName)
		if offset == -1 {
			return nil
		}
		idx := searchFrom + offset
		startOffset := strings.IndexByte(text[idx+len(globalName):], '{')
		if startOffset == -1 {
			return nil
		}
		start := idx + len(globalName) + startOffset
		if end, ok := balancedJSONObjectEnd(text, start); ok {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(text[start:end]), &parsed); err == nil {
				return parsed
			}
			slog.Debug("YouTube: failed to parse inline JSON")
		}
		searchFrom = idx + len(globalName)
	}
	return nil
}

// balancedJSONObjectEnd returns the exclusive end index of the object starting
// at start. JSON string contents, including escaped quotes and braces, are
// skipped while balancing braces.
func balancedJSONObjectEnd(text string, start int) (int, bool) {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		char := text[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inString = false
			}
			continue
		}

		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}
