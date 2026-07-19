package elements

import "github.com/PuerkitoBio/goquery"

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
