package elements

import (
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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
