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

	slog.Debug("YouTube extractor: using title from document", "title", title)
	return title
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
	data := y.parseInlineJSON("ytInitialPlayerResponse")
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
// {...} JSON object that follows it, or nil if absent or unparseable.
func parseGlobalJSON(text, globalName string) map[string]any {
	idx := strings.Index(text, globalName)
	if idx == -1 {
		return nil
	}
	// Find opening brace after the variable name
	startIndex := strings.IndexByte(text[idx:], '{')
	if startIndex == -1 {
		return nil
	}
	startIndex += idx

	// Brace-balance to find matching closing brace
	depth := 0
	for i := startIndex; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				jsonText := text[startIndex : i+1]
				var parsed map[string]any
				if err := json.Unmarshal([]byte(jsonText), &parsed); err != nil {
					slog.Debug("YouTube: failed to parse inline JSON", "error", err)
					return nil
				}
				return parsed
			}
		}
	}
	return nil
}
