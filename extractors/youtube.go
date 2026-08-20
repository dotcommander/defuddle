package extractors

import (
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// YouTubeExtractor handles YouTube content extraction
// TypeScript original code:
// import { BaseExtractor } from './_base';
// import { ExtractorResult } from '../types/extractors';
//
//	export class YoutubeExtractor extends BaseExtractor {
//		private videoElement: HTMLVideoElement | null;
//		protected override schemaOrgData: any;
//
//		constructor(document: Document, url: string, schemaOrgData?: any) {
//			super(document, url, schemaOrgData);
//			this.videoElement = document.querySelector('video');
//			this.schemaOrgData = schemaOrgData;
//		}
//
//		canExtract(): boolean {
//			return true;
//		}
//
//		extract(): ExtractorResult {
//			const videoData = this.getVideoData();
//			const description = videoData.description || '';
//			const formattedDescription = this.formatDescription(description);
//			const contentHtml = `<iframe width="560" height="315" src="https://www.youtube.com/embed/${this.getVideoId()}?si=_m0qv33lAuJFoGNh" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe><br>${formattedDescription}`;
//
//			return {
//				content: contentHtml,
//				contentHtml: contentHtml,
//				extractedContent: {
//					videoId: this.getVideoId(),
//					author: videoData.author || '',
//				},
//				variables: {
//					title: videoData.name || '',
//					author: videoData.author || '',
//					site: 'YouTube',
//					image: Array.isArray(videoData.thumbnailUrl) ? videoData.thumbnailUrl[0] || '' : '',
//					published: videoData.uploadDate,
//					description: description.slice(0, 200).trim(),
//				}
//			};
//		}
//	}
type YouTubeExtractor struct {
	*ExtractorBase
	videoElement *goquery.Selection
}

// NewYouTubeExtractor creates a new YouTube extractor
// TypeScript original code:
//
//	constructor(document: Document, url: string, schemaOrgData?: any) {
//		super(document, url, schemaOrgData);
//		this.videoElement = document.querySelector('video');
//		this.schemaOrgData = schemaOrgData;
//	}
func NewYouTubeExtractor(document *goquery.Document, url string, schemaOrgData any) *YouTubeExtractor {
	extractor := &YouTubeExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}

	// Find video element
	extractor.videoElement = document.Find("video").First()

	slog.Debug("YouTube extractor initialized",
		"hasVideoElement", extractor.videoElement.Length() > 0,
		"url", url,
		"hasSchemaOrgData", schemaOrgData != nil)

	return extractor
}

// CanExtract checks if the extractor can extract content
// TypeScript original code:
//
//	canExtract(): boolean {
//		return true; // YouTube extractor can always extract
//	}
func (y *YouTubeExtractor) CanExtract() bool {
	canExtract := true // YouTube extractor can always extract
	slog.Debug("YouTube extractor can extract check", "canExtract", canExtract)
	return canExtract
}

// Name returns the name of the extractor
func (y *YouTubeExtractor) Name() string {
	return "YouTubeExtractor"
}

// Extract extracts the YouTube content
// TypeScript original code:
//
//	extract(): ExtractorResult {
//		const videoData = this.getVideoData();
//		const description = videoData.description || '';
//		const formattedDescription = this.formatDescription(description);
//		const contentHtml = `<iframe width="560" height="315" src="https://www.youtube.com/embed/${this.getVideoId()}?si=_m0qv33lAuJFoGNh" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe><br>${formattedDescription}`;
//
//		return {
//			content: contentHtml,
//			contentHtml: contentHtml,
//			extractedContent: {
//				videoId: this.getVideoId(),
//				author: videoData.author || '',
//			},
//			variables: {
//				title: videoData.name || '',
//				author: videoData.author || '',
//				site: 'YouTube',
//				image: Array.isArray(videoData.thumbnailUrl) ? videoData.thumbnailUrl[0] || '' : '',
//				published: videoData.uploadDate,
//				description: description.slice(0, 200).trim(),
//			}
//		};
//	}
func (y *YouTubeExtractor) Extract() *ExtractorResult {
	slog.Debug("YouTube extractor starting extraction", "url", y.url)

	videoData := y.getVideoData()
	description := y.getDescription(videoData)
	formattedDescription := y.formatDescription(description)
	videoID := y.getVideoID()

	// Create iframe content - only if videoID is not empty
	var contentHTML string
	if videoID != "" {
		contentHTML = fmt.Sprintf(
			`<iframe width="560" height="315" src="https://www.youtube.com/embed/%s" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe><br>%s`,
			videoID,
			formattedDescription,
		)
	} else {
		// Fallback content when videoID is empty
		contentHTML = formattedDescription
	}
	contentHTML += y.transcriptLinksHTML()

	title := y.getTitle(videoData)
	author := y.getAuthor(videoData)
	thumbnail := y.getThumbnail(videoData)
	published := y.getPublished(videoData)
	truncatedDescription := y.truncateDescription(description)

	slog.Debug("YouTube extraction completed",
		"videoId", videoID,
		"title", title,
		"author", author,
		"published", published,
		"descriptionLength", len(description))

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"videoId": videoID,
			"author":  author,
		},
		Variables: map[string]string{
			"title":       title,
			"author":      author,
			"site":        "YouTube",
			"image":       thumbnail,
			"published":   published,
			"description": truncatedDescription,
		},
	}
}

type youtubeCaptionTrack struct {
	url   string
	label string
}

func (y *YouTubeExtractor) transcriptLinksHTML() string {
	tracks := y.captionTracks()
	if len(tracks) == 0 {
		return ""
	}

	var content strings.Builder
	for _, track := range tracks {
		fmt.Fprintf(&content, `<p><a href="%s">%s</a></p>`, html.EscapeString(track.url), html.EscapeString(track.label))
	}
	return content.String()
}

func (y *YouTubeExtractor) captionTracks() []youtubeCaptionTrack {
	data := y.playerResponse()
	if data == nil {
		return nil
	}
	captions, _ := data["captions"].(map[string]any)
	trackList, _ := captions["playerCaptionsTracklistRenderer"].(map[string]any)
	tracks, _ := trackList["captionTracks"].([]any)

	result := make([]youtubeCaptionTrack, 0, len(tracks))
	for _, item := range tracks {
		track, _ := item.(map[string]any)
		trackURL, _ := track["baseUrl"].(string)
		if !isSafeYouTubeTranscriptURL(trackURL) {
			continue
		}
		language := captionTrackLanguage(track)
		label := fmt.Sprintf("Transcript (%s)", language)
		if kind, _ := track["kind"].(string); strings.EqualFold(kind, "asr") {
			label = fmt.Sprintf("Transcript (%s, auto-generated)", language)
		}
		result = append(result, youtubeCaptionTrack{url: trackURL, label: label})
	}
	return result
}

func captionTrackLanguage(track map[string]any) string {
	if name, ok := track["name"].(map[string]any); ok {
		if text, _ := name["simpleText"].(string); strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
		if runs, ok := name["runs"].([]any); ok {
			var text strings.Builder
			for _, item := range runs {
				run, _ := item.(map[string]any)
				part, _ := run["text"].(string)
				text.WriteString(part)
			}
			if label := strings.TrimSpace(text.String()); label != "" {
				return label
			}
		}
	}
	if languageCode, _ := track["languageCode"].(string); languageCode != "" {
		return languageCode
	}
	return "unknown language"
}

func isSafeYouTubeTranscriptURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return (host == "youtube.com" || strings.HasSuffix(host, ".youtube.com")) && parsed.Path == "/api/timedtext"
}
