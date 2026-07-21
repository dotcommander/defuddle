package extractors

import (
	"fmt"
	"log/slog"

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
