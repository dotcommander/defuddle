package elements

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// --- Transform 4: span:has(img) → figure ---

// transformSpanImages extracts images from spans and optionally wraps with figure+caption.
func (p *ImageProcessor) transformSpanImages(scope *goquery.Selection) {
	scope.Find("span").Each(func(_ int, el *goquery.Selection) {
		if !containsImage(el) {
			return
		}

		// Preserve spans that contain multiple images/media elements.
		if el.Find("img").Length() > 1 || el.Find("picture").Length() > 1 || el.Find("video").Length() > 1 {
			return
		}

		imgEl := findMainImage(el)
		if imgEl == nil || imgEl.Length() == 0 {
			return
		}

		caption := findCaption(el)
		hasCaption := caption != nil && caption.Length() > 0 && hasMeaningfulCaption(caption)

		var excludedCaption *html.Node
		if hasCaption && isDescendantNode(el.Get(0), caption.Get(0)) {
			excludedCaption = caption.Get(0)
		}

		// Preserve spans that contain meaningful surrounding prose outside any caption.
		if hasTextOutside(el.Get(0), excludedCaption) {
			return
		}

		imgHTML, err := goquery.OuterHtml(imgEl)
		if err != nil {
			return
		}

		if hasCaption {
			captionHTML, _ := caption.Html()
			if !isDescendantNode(el.Get(0), caption.Get(0)) {
				caption.Remove()
			}
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
		if !isDescendantNode(el.Get(0), caption.Get(0)) {
			caption.Remove()
		}
		el.ReplaceWithHtml("<figure>" + imgHTML + "<figcaption>" + captionHTML + "</figcaption></figure>")
	})
}

// isDescendantNode reports whether node is equal to or a descendant of ancestor.
func isDescendantNode(ancestor, node *html.Node) bool {
	if ancestor == nil || node == nil {
		return false
	}
	for cur := node; cur != nil; cur = cur.Parent {
		if cur == ancestor {
			return true
		}
	}
	return false
}

// hasTextOutside reports whether root contains any non-whitespace text node
// outside the excluded subtree.
func hasTextOutside(root, excluded *html.Node) bool {
	if root == nil || root == excluded {
		return false
	}
	if root.Type == html.TextNode {
		return strings.TrimSpace(root.Data) != ""
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if hasTextOutside(c, excluded) {
			return true
		}
	}
	return false
}
