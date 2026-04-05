// Package elements provides enhanced element processing functionality.
// This module handles image processing: picture collapse, lazy-load resolution,
// span→figure conversion, and caption normalization — matching the TypeScript
// defuddle imageRules transforms.
package elements

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
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

// --- Transform 1: Picture element collapse ---

// transformPictures collapses <picture> elements by selecting the best
// <source> and applying its srcset to the <img>, then removing sources.
func (p *ImageProcessor) transformPictures(scope *goquery.Selection) {
	scope.Find("picture").Each(func(_ int, el *goquery.Selection) {
		// Collect source elements
		var sources []*goquery.Selection
		el.Find("source").Each(func(_ int, src *goquery.Selection) {
			sources = append(sources, src)
		})

		img := el.Find("img").First()

		if img.Length() == 0 {
			// No img fallback — try to create one from best source
			best := selectBestSource(sources)
			if best != nil {
				if srcset, ok := best.Attr("srcset"); ok && srcset != "" {
					firstURL := extractFirstURLFromSrcset(srcset)
					if firstURL != "" && isValidImageURL(firstURL) {
						el.SetHtml(`<img src="` + firstURL + `" srcset="` + srcset + `"/>`)
					}
				}
			}
			return
		}

		var bestSrcset, bestSrc string
		if len(sources) > 0 {
			best := selectBestSource(sources)
			if best != nil {
				if srcset, ok := best.Attr("srcset"); ok && srcset != "" {
					bestSrcset = srcset
					bestSrc = extractFirstURLFromSrcset(bestSrcset)
				}
			}
		}

		if bestSrcset != "" {
			img.SetAttr("srcset", bestSrcset)
		}
		if bestSrc != "" && isValidImageURL(bestSrc) {
			img.SetAttr("src", bestSrc)
		} else {
			currentSrc := img.AttrOr("src", "")
			if currentSrc == "" || !isValidImageURL(currentSrc) {
				// Try extracting from img's own srcset or bestSrcset
				imgSrcset := img.AttrOr("srcset", bestSrcset)
				if firstURL := extractFirstURLFromSrcset(imgSrcset); firstURL != "" && isValidImageURL(firstURL) {
					img.SetAttr("src", firstURL)
				}
			}
		}

		// Remove all source elements
		for _, src := range sources {
			src.Remove()
		}
	})
}

// --- Transform 2: uni-image-full-width → figure ---

// dataLoadingDesktop is the JSON structure for uni-image data-loading attribute.
type dataLoadingDesktop struct {
	Desktop string `json:"desktop"`
}

// transformUniImages converts uni-image-full-width custom elements to figures.
func (p *ImageProcessor) transformUniImages(scope *goquery.Selection) {
	scope.Find("uni-image-full-width").Each(func(_ int, el *goquery.Selection) {
		originalImg := el.Find("img").First()
		if originalImg.Length() == 0 {
			return
		}

		bestSrc := originalImg.AttrOr("src", "")
		if dataLoading, ok := originalImg.Attr("data-loading"); ok {
			var parsed dataLoadingDesktop
			if json.Unmarshal([]byte(dataLoading), &parsed) == nil && parsed.Desktop != "" && isValidImageURL(parsed.Desktop) {
				bestSrc = parsed.Desktop
			}
		}

		if bestSrc == "" || !isValidImageURL(bestSrc) {
			return
		}

		altText := originalImg.AttrOr("alt", "")
		if altText == "" {
			altText, _ = el.Attr("alt-text")
		}

		var b strings.Builder
		b.WriteString(`<figure><img src="`)
		b.WriteString(bestSrc)
		b.WriteString(`"`)
		if altText != "" {
			b.WriteString(` alt="`)
			b.WriteString(altText)
			b.WriteString(`"`)
		}
		b.WriteString(`/>`)

		figcaptionEl := el.Find("figcaption").First()
		if figcaptionEl.Length() > 0 {
			captionText := strings.TrimSpace(figcaptionEl.Text())
			if len(captionText) > 5 {
				richTextP := figcaptionEl.Find(".rich-text p").First()
				if richTextP.Length() > 0 {
					inner, _ := richTextP.Html()
					b.WriteString("<figcaption>")
					b.WriteString(inner)
					b.WriteString("</figcaption>")
				} else {
					b.WriteString("<figcaption>")
					b.WriteString(captionText)
					b.WriteString("</figcaption>")
				}
			}
		}

		b.WriteString("</figure>")
		el.ReplaceWithHtml(b.String())
	})
}

// --- Transform 3: Lazy-loaded image de-lazification ---

// transformLazyImages resolves lazy-loaded images by promoting data-src/data-srcset,
// removing base64 placeholders, and scanning attributes for image URLs.
func (p *ImageProcessor) transformLazyImages(scope *goquery.Selection) {
	scope.Find(`img[data-src], img[data-srcset], img[loading="lazy"], img.lazy, img.lazyload`).Each(func(_ int, el *goquery.Selection) {
		src := el.AttrOr("src", "")

		// Remove base64 placeholder if a better source exists
		if isBase64Placeholder(src) && hasBetterImageSource(el) {
			el.RemoveAttr("src")
			src = ""
		}

		// Promote data-src → src
		dataSrc, _ := el.Attr("data-src")
		if dataSrc != "" && src == "" {
			el.SetAttr("src", dataSrc)
		}

		// Promote data-srcset → srcset
		dataSrcset, _ := el.Attr("data-srcset")
		if dataSrcset != "" {
			if _, hasSrcset := el.Attr("srcset"); !hasSrcset {
				el.SetAttr("srcset", dataSrcset)
			}
		}

		// Scan all attributes for image URLs
		if el.Length() > 0 {
			node := el.Get(0)
			for _, attr := range node.Attr {
				if attr.Key == "src" || attr.Key == "srcset" || attr.Key == "alt" {
					continue
				}
				if len(attr.Val) > 0 && (attr.Val[0] == '{' || attr.Val[0] == '[') {
					continue
				}
				if srcsetPatternRe.MatchString(attr.Val) {
					el.SetAttr("srcset", attr.Val)
				} else if srcPatternRe.MatchString(attr.Val) {
					el.SetAttr("src", attr.Val)
				}
			}
		}

		// Clean up lazy-loading artifacts
		el.RemoveClass("lazy")
		el.RemoveClass("lazyload")
		el.RemoveAttr("data-ll-status")
		el.RemoveAttr("data-src")
		el.RemoveAttr("data-srcset")
		el.RemoveAttr("loading")
	})
}

// --- Transform 4: span:has(img) → figure ---

// transformSpanImages extracts images from spans and optionally wraps with figure+caption.
func (p *ImageProcessor) transformSpanImages(scope *goquery.Selection) {
	scope.Find("span").Each(func(_ int, el *goquery.Selection) {
		if !containsImage(el) {
			return
		}

		imgEl := findMainImage(el)
		if imgEl == nil || imgEl.Length() == 0 {
			return
		}

		imgHTML, err := goquery.OuterHtml(imgEl)
		if err != nil {
			return
		}

		caption := findCaption(el)
		if caption != nil && caption.Length() > 0 && hasMeaningfulCaption(caption) {
			captionHTML, _ := caption.Html()
			el.ReplaceWithHtml("<figure>" + imgHTML + "<figcaption>" + captionHTML + "</figcaption></figure>")
		} else {
			el.ReplaceWithHtml(imgHTML)
		}
	})
}

// --- Transform 5: figure/caption normalization ---

// transformFigures normalizes figure elements and paragraphs with caption children.
func (p *ImageProcessor) transformFigures(scope *goquery.Selection) {
	scope.Find(`figure, p`).Each(func(_ int, el *goquery.Selection) {
		// For <p>, only match if it has a child with a caption class
		if goquery.NodeName(el) == "p" {
			if el.Find(`[class*="caption"]`).Length() == 0 {
				return
			}
		}

		if !containsImage(el) {
			return
		}

		imgEl := findMainImage(el)
		if imgEl == nil || imgEl.Length() == 0 {
			return
		}

		caption := findCaption(el)
		if caption == nil || caption.Length() == 0 || !hasMeaningfulCaption(caption) {
			return
		}

		// Re-find current image (may have been modified by earlier rules)
		currentImg := findMainImage(el)
		if currentImg == nil || currentImg.Length() == 0 {
			return
		}

		imgHTML, err := goquery.OuterHtml(currentImg)
		if err != nil {
			return
		}
		captionHTML, _ := caption.Html()
		el.ReplaceWithHtml("<figure>" + imgHTML + "<figcaption>" + captionHTML + "</figcaption></figure>")
	})
}

// --- Helper functions ---

// isBase64Placeholder checks if a src is a small base64 placeholder image.
func isBase64Placeholder(src string) bool {
	match := b64DataURLRe.FindStringSubmatch(src)
	if match == nil {
		return false
	}
	// SVG images can be meaningful even when small
	if match[1] == "svg+xml" {
		return false
	}
	// Base64 portion after the data URL prefix
	b64Length := len(src) - len(match[0])
	return b64Length < 133
}

// isSVGDataURL checks if a src is an SVG data URL.
func isSVGDataURL(src string) bool {
	return strings.HasPrefix(src, "data:image/svg+xml")
}

// isValidImageURL checks if a URL is a valid image source (not data: URL, not empty).
func isValidImageURL(src string) bool {
	if strings.HasPrefix(src, "data:") {
		return false
	}
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	return imageURLRe.MatchString(src) ||
		strings.Contains(src, "image") ||
		strings.Contains(src, "img") ||
		strings.Contains(src, "photo")
}

// hasBetterImageSource checks if an element has alternative image sources
// beyond its current src (data-src, data-srcset, or other image-URL attrs).
func hasBetterImageSource(el *goquery.Selection) bool {
	if _, ok := el.Attr("data-src"); ok {
		return true
	}
	if _, ok := el.Attr("data-srcset"); ok {
		return true
	}
	if el.Length() == 0 {
		return false
	}
	for _, attr := range el.Get(0).Attr {
		if attr.Key == "src" {
			continue
		}
		if imageExtRe.MatchString(attr.Val) {
			return true
		}
	}
	return false
}

// extractFirstURLFromSrcset extracts the first valid image URL from a srcset string.
// Handles URLs containing commas (e.g. Substack CDN).
func extractFirstURLFromSrcset(srcset string) string {
	srcset = strings.TrimSpace(srcset)
	if srcset == "" {
		return ""
	}

	matches := srcsetEntryRe.FindAllStringSubmatchIndex(srcset, -1)
	lastEnd := 0
	for _, m := range matches {
		// m[2]:m[3] is submatch 1 (the URL part)
		urlPart := srcset[m[2]:m[3]]
		if lastEnd > 0 {
			// Trim leading comma separator from previous entry
			urlPart = strings.TrimLeft(urlPart, ", ")
		}
		urlPart = strings.TrimSpace(urlPart)
		lastEnd = m[1]

		if urlPart == "" || isSVGDataURL(urlPart) {
			continue
		}
		return urlPart
	}

	// Fallback: first non-whitespace token
	if m := urlPatternRe.FindString(srcset); m != "" && !isSVGDataURL(m) {
		return m
	}
	return ""
}

// selectBestSource picks the best <source> element from a picture's sources.
// Prefers sources without media queries, then highest resolution.
func selectBestSource(sources []*goquery.Selection) *goquery.Selection {
	if len(sources) == 0 {
		return nil
	}
	if len(sources) == 1 {
		return sources[0]
	}

	// Prefer source without media query (default/fallback)
	for _, src := range sources {
		if _, hasMedia := src.Attr("media"); !hasMedia {
			return src
		}
	}

	// Find highest resolution source
	var bestSource *goquery.Selection
	var maxResolution float64

	for _, src := range sources {
		srcset, ok := src.Attr("srcset")
		if !ok || srcset == "" {
			continue
		}

		wm := widthPatternRe.FindStringSubmatch(srcset)
		if wm == nil {
			continue
		}
		width, err := strconv.Atoi(wm[1])
		if err != nil {
			continue
		}

		dpr := 1.0
		if dm := dprPatternRe.FindStringSubmatch(srcset); dm != nil {
			if parsed, err := strconv.ParseFloat(dm[1], 64); err == nil {
				dpr = parsed
			}
		}

		resolution := float64(width) * dpr
		if resolution > maxResolution {
			maxResolution = resolution
			bestSource = src
		}
	}

	if bestSource != nil {
		return bestSource
	}
	return sources[0]
}

// containsImage checks if an element contains any image-related children.
func containsImage(el *goquery.Selection) bool {
	return el.Find("img, video, picture, source").Length() > 0
}

// findMainImage finds the most relevant image element, skipping placeholders.
func findMainImage(el *goquery.Selection) *goquery.Selection {
	// Check for picture elements first
	if pic := el.Find("picture").First(); pic.Length() > 0 {
		return pic
	}

	// Find non-placeholder imgs
	imgs := el.Find("img")
	totalImgs := imgs.Length()
	var best *goquery.Selection
	imgs.Each(func(_ int, img *goquery.Selection) {
		if best != nil {
			return
		}
		src := img.AttrOr("src", "")
		if isSVGDataURL(src) || isBase64Placeholder(src) {
			return
		}
		// Skip empty-alt images if there are multiple (likely decorative)
		alt := strings.TrimSpace(img.AttrOr("alt", ""))
		if alt == "" && totalImgs > 1 {
			return
		}
		best = img
	})
	if best != nil {
		return best
	}

	// Video fallback
	if vid := el.Find("video").First(); vid.Length() > 0 {
		return vid
	}

	// Source fallback
	if src := el.Find("source").First(); src.Length() > 0 {
		return src
	}

	// Last resort: any image element
	if mediaEl := el.Find("img, picture, source, video").First(); mediaEl.Length() > 0 {
		return mediaEl
	}
	return nil
}

// captionSelectors is the combined selector for caption-like elements.
const captionSelectors = `[class*="caption"], [class*="description"], [class*="credit"], [class*="text"], [class*="image-caption"], [class*="photo-caption"]`

// findCaption finds a caption element near an image.
func findCaption(el *goquery.Selection) *goquery.Selection {
	// Check for figcaption
	if fc := el.Find("figcaption").First(); fc.Length() > 0 {
		return fc
	}

	// Check for caption-class elements (skip image elements)
	var found *goquery.Selection
	el.Find(captionSelectors).Each(func(_ int, capEl *goquery.Selection) {
		if found != nil {
			return
		}
		tag := goquery.NodeName(capEl)
		if tag == "img" || tag == "video" || tag == "picture" || tag == "source" {
			return
		}
		if text := strings.TrimSpace(capEl.Text()); text != "" {
			found = capEl
		}
	})
	if found != nil {
		return found
	}

	// Check sibling elements with caption classes
	if el.Length() > 0 && el.Get(0).Parent != nil {
		parent := el.Parent()
		parent.Children().Each(func(_ int, sib *goquery.Selection) {
			if found != nil {
				return
			}
			if sib.Get(0) == el.Get(0) {
				return
			}
			class := strings.ToLower(sib.AttrOr("class", ""))
			if strings.Contains(class, "caption") || strings.Contains(class, "credit") || strings.Contains(class, "description") {
				if text := strings.TrimSpace(sib.Text()); text != "" {
					found = sib
				}
			}
		})
	}
	if found != nil {
		return found
	}

	// Check text elements following images within the element
	el.Find("img").Each(func(_ int, img *goquery.Selection) {
		if found != nil {
			return
		}
		// Walk next siblings of the img's parent context
		for sib := img.Get(0).NextSibling; sib != nil; sib = sib.NextSibling {
			if sib.Type != html.ElementNode {
				continue
			}
			switch sib.Data {
			case "em", "strong", "span", "i", "b", "small", "cite":
				sel := el.FindSelection(goquery.NewDocumentFromNode(sib).Selection)
				if sel.Length() == 0 {
					// Construct selection from the node directly
					sel = goquery.NewDocumentFromNode(sib).Find(sib.Data).First()
				}
				text := strings.TrimSpace(extractNodeText(sib))
				if text != "" && sel.Length() > 0 {
					found = sel
					return
				}
			}
		}
	})

	return found
}

// hasMeaningfulCaption checks if a caption element has meaningful text content.
func hasMeaningfulCaption(caption *goquery.Selection) bool {
	text := strings.TrimSpace(caption.Text())
	if len(text) < 10 {
		return false
	}
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return false
	}
	if filenamePatternRe.MatchString(text) {
		return false
	}
	if matched, _ := regexp.MatchString(`^\d+$`, text); matched {
		return false
	}
	if datePatternRe.MatchString(text) {
		return false
	}
	return true
}

// extractNodeText extracts text from an html.Node tree.
func extractNodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractNodeText(c))
	}
	return sb.String()
}

// --- Image removal (retained from original) ---

// isDecorativeImage determines if an image is decorative/small.
func (p *ImageProcessor) isDecorativeImage(s *goquery.Selection, src string) bool {
	if width, hasWidth := s.Attr("width"); hasWidth {
		if w, err := strconv.Atoi(width); err == nil && w < 50 {
			return true
		}
	}
	if height, hasHeight := s.Attr("height"); hasHeight {
		if h, err := strconv.Atoi(height); err == nil && h < 50 {
			return true
		}
	}
	if class, hasClass := s.Attr("class"); hasClass {
		classLower := strings.ToLower(class)
		for _, dc := range []string{"icon", "avatar", "emoji", "bullet", "decoration", "logo-small"} {
			if strings.Contains(classLower, dc) {
				return true
			}
		}
	}
	return p.isTrackingPixel(src)
}

// isTrackingPixel determines if an image is a tracking pixel.
func (p *ImageProcessor) isTrackingPixel(src string) bool {
	if src == "" {
		return false
	}
	for _, re := range trackingPatterns {
		if re.MatchString(src) {
			return true
		}
	}
	return false
}

// shouldRemoveSmallImage determines if a small image should be removed.
func (p *ImageProcessor) shouldRemoveSmallImage(s *goquery.Selection, options *ImageProcessingOptions) bool {
	if width, hasWidth := s.Attr("width"); hasWidth {
		if w, err := strconv.Atoi(width); err == nil && w > 0 && w < options.MinImageWidth {
			return true
		}
	}
	if height, hasHeight := s.Attr("height"); hasHeight {
		if h, err := strconv.Atoi(height); err == nil && h > 0 && h < options.MinImageHeight {
			return true
		}
	}
	if p.isImportantImage(s) {
		return false
	}
	src := s.AttrOr("src", "")
	return p.isTrackingPixel(src) || p.isDecorativeImage(s, src)
}

// isImportantImage determines if an image is important (shouldn't be removed).
func (p *ImageProcessor) isImportantImage(s *goquery.Selection) bool {
	figure := s.Closest("figure")
	if figure.Length() > 0 && figure.HasClass("featured") {
		return true
	}

	// Check if it's one of the first 3 images in the document
	if p.doc != nil {
		idx := -1
		p.doc.Find("img").EachWithBreak(func(i int, img *goquery.Selection) bool {
			if img.Get(0) == s.Get(0) {
				idx = i
				return false
			}
			return true
		})
		if idx >= 0 && idx < 3 {
			return true
		}
	}

	alt := s.AttrOr("alt", "")
	if len(alt) > 20 {
		return true
	}

	mainContent := s.Closest("article, main, .content, .post")
	return mainContent.Length() > 0
}
