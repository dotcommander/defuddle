package extractors

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// getVideoData extracts video data from schema.org structured data
// TypeScript original code:
//
//	private getVideoData(): any {
//		if (!this.schemaOrgData) return {};
//
//		const videoData = Array.isArray(this.schemaOrgData)
//			? this.schemaOrgData.find(item => item['@type'] === 'VideoObject')
//			: this.schemaOrgData['@type'] === 'VideoObject' ? this.schemaOrgData : null;
//
//		return videoData || {};
//	}
func (y *YouTubeExtractor) getVideoData() map[string]any {
	if videoData := y.getVideoDataFromSchema(y.schemaOrgData); len(videoData) > 0 {
		if _, ok := videoData["description"].(string); ok {
			return videoData
		}
		if scriptData := y.getVideoDataFromLDJSONScripts(); len(scriptData) > 0 {
			return scriptData
		}
		return videoData
	}

	return y.getVideoDataFromLDJSONScripts()
}

func (y *YouTubeExtractor) getVideoDataFromSchema(schemaOrgData any) map[string]any {
	if schemaOrgData == nil {
		slog.Debug("YouTube extractor: no schema.org data available")
		return make(map[string]any)
	}

	// Handle both single object and array of objects
	switch data := schemaOrgData.(type) {
	case []any:
		if obj := videoObjectFromArray(data); obj != nil {
			slog.Debug("YouTube extractor: found VideoObject in array", "hasVideoData", true)
			return obj
		}
		slog.Debug("YouTube extractor: no VideoObject found in schema.org array")
	case map[string]any:
		// Check if it's a VideoObject
		if itemType, exists := data["@type"]; exists && itemType == "VideoObject" {
			slog.Debug("YouTube extractor: found VideoObject", "hasVideoData", true)
			return data
		}
		if itemType, exists := data["@type"]; exists {
			slog.Debug("YouTube extractor: schema.org data is not VideoObject", "type", itemType)
		}
	default:
		slog.Debug("YouTube extractor: unexpected schema.org data type", "type", fmt.Sprintf("%T", data))
	}

	return make(map[string]any)
}

// videoObjectFromArray returns the first VideoObject map in data, or nil.
func videoObjectFromArray(data []any) map[string]any {
	for _, item := range data {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if itemType, exists := itemMap["@type"]; exists && itemType == "VideoObject" {
			return itemMap
		}
	}
	return nil
}

func (y *YouTubeExtractor) getVideoDataFromLDJSONScripts() map[string]any {
	videoID := y.getVideoID()
	var fallback map[string]any

	y.document.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var decoded any
		if err := json.Unmarshal([]byte(s.Text()), &decoded); err != nil {
			return true
		}

		for _, item := range flattenJSONLD(decoded) {
			if !isVideoObjectForID(item, videoID) {
				continue
			}
			if desc, _ := item["description"].(string); desc != "" {
				fallback = item
				return false
			}
			if _, hasComment := item["comment"]; hasComment || fallback == nil {
				fallback = item
			}
		}

		return true
	})

	if fallback != nil {
		return fallback
	}

	return y.videoDataFromOGMeta(videoID)
}

// videoDataFromOGMeta builds minimal video data from og: meta tags, but only when
// og:url matches videoID. Returns an empty map otherwise.
func (y *YouTubeExtractor) videoDataFromOGMeta(videoID string) map[string]any {
	if videoID == "" {
		return make(map[string]any)
	}

	ogURL := y.document.Find(`meta[property="og:url"]`).AttrOr("content", "")
	if !strings.Contains(ogURL, videoID) {
		return make(map[string]any)
	}

	videoData := make(map[string]any)
	if title := y.document.Find(`meta[property="og:title"]`).AttrOr("content", ""); title != "" {
		videoData["name"] = title
	}
	if description := y.document.Find(`meta[property="og:description"]`).AttrOr("content", ""); description != "" {
		videoData["description"] = description
	}
	if image := y.document.Find(`meta[property="og:image"]`).AttrOr("content", ""); image != "" {
		videoData["thumbnailUrl"] = []any{image}
	}
	return videoData
}

func flattenJSONLD(value any) []map[string]any {
	switch v := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			out = append(out, flattenJSONLD(item)...)
		}
		return out
	case map[string]any:
		out := make([]map[string]any, 0, 1)
		if graph, ok := v["@graph"]; ok {
			out = append(out, flattenJSONLD(graph)...)
		}
		out = append(out, v)
		return out
	default:
		return nil
	}
}

func isVideoObjectForID(item map[string]any, videoID string) bool {
	if itemType, _ := item["@type"].(string); itemType != "VideoObject" {
		return false
	}
	if videoID == "" {
		return true
	}
	for _, key := range []string{"@id", "url", "embedUrl"} {
		if value, _ := item[key].(string); strings.Contains(value, videoID) {
			return true
		}
	}
	return false
}

// getVideoID extracts the video ID from the URL
// TypeScript original code:
//
//	private getVideoId(): string {
//		const urlParams = new URLSearchParams(new URL(this.url).search);
//		return urlParams.get('v') || '';
//	}
func (y *YouTubeExtractor) getVideoID() string {
	parsedURL, err := url.Parse(y.url)
	if err != nil {
		slog.Warn("YouTube extractor: failed to parse URL", "url", y.url, "error", err)
		return ""
	}

	// For youtube.com/watch?v=...
	if strings.Contains(parsedURL.Host, "youtube.com") {
		videoID := parsedURL.Query().Get("v")
		slog.Debug("YouTube extractor: extracted video ID from youtube.com", "videoId", videoID)
		return videoID
	}

	// For youtu.be/...
	if strings.Contains(parsedURL.Host, "youtu.be") {
		path := strings.TrimPrefix(parsedURL.Path, "/")
		slog.Debug("YouTube extractor: extracted video ID from youtu.be", "videoId", path)
		return path
	}

	slog.Debug("YouTube extractor: no video ID found in URL", "host", parsedURL.Host)
	return ""
}
