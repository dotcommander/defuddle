// Package elements provides enhanced element processing functionality.
// This module handles image processing: picture collapse, lazy-load resolution,
// span→figure conversion, and caption normalization — matching the TypeScript
// defuddle imageRules transforms.
package elements

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns for image processing (matching TS patterns).
var (
	b64DataURLRe      = regexp.MustCompile(`^data:image/([^;]+);base64,`)
	srcsetPatternRe   = regexp.MustCompile(`\.(jpg|jpeg|png|webp)\s+\d`)
	srcPatternRe      = regexp.MustCompile(`(?i)^\s*\S+\.(jpg|jpeg|png|webp)\S*\s*$`)
	imageURLRe        = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|webp|gif|avif)(\?.*)?$`)
	widthPatternRe    = regexp.MustCompile(`\s(\d+)w`)
	dprPatternRe      = regexp.MustCompile(`dpr=(\d+(?:\.\d+)?)`)
	urlPatternRe      = regexp.MustCompile(`^([^\s]+)`)
	srcsetEntryRe     = regexp.MustCompile(`(.+?)\s+(\d+(?:\.\d+)?[wx])`)
	filenamePatternRe = regexp.MustCompile(`(?i)^[\w\-./\\]+\.(jpg|jpeg|png|gif|webp|svg)$`)
	datePatternRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	imageExtRe        = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|webp|gif)(\?.*)?$`)

	trackingPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)pixel\.gif`),
		regexp.MustCompile(`(?i)1x1\.gif`),
		regexp.MustCompile(`(?i)tracking\.gif`),
		regexp.MustCompile(`(?i)analytics`),
		regexp.MustCompile(`(?i)metrics`),
		regexp.MustCompile(`(?i)beacon`),
	}
)

// ImageProcessor handles image processing and enhancement.
type ImageProcessor struct {
	doc *goquery.Document
}

// ImageProcessingOptions contains options for image processing.
type ImageProcessingOptions struct {
	EnableLazyLoading bool
	EnableResponsive  bool
	GenerateAltText   bool
	OptimizeImages    bool
	RemoveSmallImages bool
	MinImageWidth     int
	MinImageHeight    int
	MaxImageWidth     int
	MaxImageHeight    int
}

// DefaultImageProcessingOptions returns default options for image processing.
func DefaultImageProcessingOptions() *ImageProcessingOptions {
	return &ImageProcessingOptions{
		EnableLazyLoading: true,
		EnableResponsive:  true,
		GenerateAltText:   true,
		OptimizeImages:    true,
		RemoveSmallImages: true,
		MinImageWidth:     50,
		MinImageHeight:    50,
		MaxImageWidth:     1200,
		MaxImageHeight:    800,
	}
}

// NewImageProcessor creates a new image processor.
func NewImageProcessor(doc *goquery.Document) *ImageProcessor {
	return &ImageProcessor{doc: doc}
}

// --- Public API ---

// ProcessImages processes all images in the document.
func ProcessImages(doc *goquery.Document, options *ImageProcessingOptions) {
	processor := NewImageProcessor(doc)
	processor.ProcessImages(options)
}

// ProcessImagesInScope processes images within the given container element,
// applying content-cleanup transforms matching the TypeScript imageRules.
func ProcessImagesInScope(scope *goquery.Selection, options *ImageProcessingOptions) {
	if options == nil {
		options = DefaultImageProcessingOptions()
	}

	var p ImageProcessor

	// Apply image transform rules in order (matching TS imageRules array).
	p.transformPictures(scope)
	p.transformUniImages(scope)
	p.transformLazyImages(scope)
	p.transformSpanImages(scope)
	p.transformFigures(scope)

	// Remove small/decorative images if enabled.
	if options.RemoveSmallImages {
		scope.Find("img").Each(func(_ int, s *goquery.Selection) {
			if p.shouldRemoveSmallImage(s, options) {
				s.Remove()
			}
		})
	}
}

// ProcessImages applies all image transforms to the document.
func (p *ImageProcessor) ProcessImages(options *ImageProcessingOptions) {
	if options == nil {
		options = DefaultImageProcessingOptions()
	}
	ProcessImagesInScope(p.doc.Selection, options)
}
