package extractors

import (
	"fmt"
	"log/slog"
	"strings"
)

// getDescription gets the video description
// TypeScript original code:
//
//	private getDescription(videoData: any): string {
//		if (videoData.description) {
//			return videoData.description;
//		}
//
//		// Fallback to description element in DOM
//		const descElement = this.document.querySelector('#description');
//		return descElement ? descElement.textContent || '' : '';
//	}
func (y *YouTubeExtractor) getDescription(videoData map[string]any) string {
	if description, exists := videoData["description"]; exists {
		if descStr, ok := description.(string); ok && descStr != "" {
			slog.Debug("YouTube extractor: using description from schema.org", "descriptionLength", len(descStr))
			return descStr
		}
	}

	// Fallback to description element in DOM
	descElement := y.document.Find("#description").First()
	if descElement.Length() > 0 {
		description := descElement.Text()
		slog.Debug("YouTube extractor: using description from DOM", "descriptionLength", len(description))
		return description
	}

	slog.Debug("YouTube extractor: no description found")
	return ""
}

// getPublished gets the published date
// TypeScript original code:
//
//	private getPublished(videoData: any): string {
//		return videoData.uploadDate || '';
//	}
func (y *YouTubeExtractor) getPublished(videoData map[string]any) string {
	if uploadDate, exists := videoData["uploadDate"]; exists {
		if dateStr, ok := uploadDate.(string); ok {
			return dateStr
		}
	}
	return ""
}

// getThumbnail gets the video thumbnail URL
// TypeScript original code:
//
//	private getThumbnail(videoData: any): string {
//		if (videoData.thumbnailUrl) {
//			return Array.isArray(videoData.thumbnailUrl) ? videoData.thumbnailUrl[0] : videoData.thumbnailUrl;
//		}
//
//		// Generate thumbnail URL from video ID if not found
//		const videoId = this.getVideoId();
//		return videoId ? `https://img.youtube.com/vi/${videoId}/maxresdefault.jpg` : '';
//	}
func (y *YouTubeExtractor) getThumbnail(videoData map[string]any) string {
	if thumbnailURL, exists := videoData["thumbnailUrl"]; exists {
		switch thumb := thumbnailURL.(type) {
		case []any:
			if len(thumb) > 0 {
				if thumbStr, ok := thumb[0].(string); ok {
					slog.Debug("YouTube extractor: using thumbnail from schema.org array", "thumbnailUrl", thumbStr)
					return thumbStr
				}
			}
		case string:
			if thumb != "" {
				slog.Debug("YouTube extractor: using thumbnail from schema.org", "thumbnailUrl", thumb)
				return thumb
			}
		}
	}

	// Generate thumbnail URL from video ID if not found
	videoID := y.getVideoID()
	if videoID != "" {
		generatedThumbnail := fmt.Sprintf("https://img.youtube.com/vi/%s/maxresdefault.jpg", videoID)
		slog.Debug("YouTube extractor: generated thumbnail URL", "thumbnailUrl", generatedThumbnail)
		return generatedThumbnail
	}

	slog.Debug("YouTube extractor: no thumbnail available")
	return ""
}

// formatDescription formats the video description
// TypeScript original code:
//
//	private formatDescription(description: string): string {
//		return `<p>${description.replace(/\n/g, '<br>')}</p>`;
//	}
func (y *YouTubeExtractor) formatDescription(description string) string {
	if description == "" {
		return ""
	}

	// Replace newlines with <br> tags
	formatted := strings.ReplaceAll(description, "\n", "<br>")
	return fmt.Sprintf("<p>%s</p>", formatted)
}

// truncateDescription truncates description for metadata
// TypeScript original code:
//
//	private truncateDescription(description: string): string {
//		if (description.length <= 200) {
//			return description.trim();
//		}
//
//		// Find a good breaking point (end of sentence or word)
//		let truncated = description.substring(0, 200);
//		const lastSpace = truncated.lastIndexOf(' ');
//		if (lastSpace > 150) { // Only use word boundary if it's not too far back
//			truncated = truncated.substring(0, lastSpace);
//		}
//
//		return truncated.trim();
//	}
func (y *YouTubeExtractor) truncateDescription(description string) string {
	if len(description) <= 200 {
		return strings.TrimSpace(description)
	}

	// Find a good breaking point (end of sentence or word)
	truncated := description[:200]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 150 { // Only use word boundary if it's not too far back
		truncated = truncated[:lastSpace]
	}

	return strings.TrimSpace(truncated)
}
