package elements

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// --- Transform 3: Lazy-loaded image de-lazification ---

// transformLazyImages resolves lazy-loaded images by promoting data-src/data-srcset,
// removing base64 placeholders, and scanning attributes for image URLs.
func (p *ImageProcessor) transformLazyImages(scope *goquery.Selection) {
	scope.Find(`img[data-src], img[data-srcset], img[loading="lazy"], img.lazy, img.lazyload`).Each(func(_ int, el *goquery.Selection) {
		src := strings.TrimSpace(el.AttrOr("src", ""))

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
			if strings.TrimSpace(el.AttrOr("srcset", "")) == "" {
				el.SetAttr("srcset", dataSrcset)
			}
		}

		// Scan all attributes for image URLs without overwriting valid src/srcset
		promoteImageURLAttrs(el)

		// Clean up lazy-loading artifacts
		el.RemoveClass("lazy")
		el.RemoveClass("lazyload")
		el.RemoveAttr("data-ll-status")
		el.RemoveAttr("data-src")
		el.RemoveAttr("data-srcset")
		el.RemoveAttr("loading")
	})
}

// promoteImageURLAttrs scans an img's non-standard attributes for image URLs
// (skipping src/srcset/alt and JSON-ish values) and promotes a match to
// srcset or src only when the target attribute is absent/empty/placeholder.
func promoteImageURLAttrs(el *goquery.Selection) {
	if el.Length() == 0 {
		return
	}
	currentSrc := strings.TrimSpace(el.AttrOr("src", ""))
	hasValidSrc := currentSrc != "" && !isBase64Placeholder(currentSrc) && !isSVGDataURL(currentSrc)
	hasSrcset := strings.TrimSpace(el.AttrOr("srcset", "")) != ""

	node := el.Get(0)
	for _, attr := range node.Attr {
		if attr.Key == "src" || attr.Key == "srcset" || attr.Key == "alt" {
			continue
		}
		if len(attr.Val) > 0 && (attr.Val[0] == '{' || attr.Val[0] == '[') {
			continue
		}
		if srcsetPatternRe.MatchString(attr.Val) {
			if !hasSrcset {
				el.SetAttr("srcset", attr.Val)
				hasSrcset = true
			}
		} else if srcPatternRe.MatchString(attr.Val) {
			if !hasValidSrc {
				el.SetAttr("src", attr.Val)
				hasValidSrc = true
			}
		}
	}
}
