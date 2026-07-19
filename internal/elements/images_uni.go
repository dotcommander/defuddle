package elements

import (
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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

		b.WriteString(uniImageFigcaption(el))

		b.WriteString("</figure>")
		el.ReplaceWithHtml(b.String())
	})
}

// uniImageFigcaption returns the <figcaption>…</figcaption> HTML for a
// uni-image element, or "" when there is no caption worth rendering.
func uniImageFigcaption(el *goquery.Selection) string {
	figcaptionEl := el.Find("figcaption").First()
	if figcaptionEl.Length() == 0 {
		return ""
	}
	captionText := strings.TrimSpace(figcaptionEl.Text())
	if len(captionText) <= 5 {
		return ""
	}
	if richTextP := figcaptionEl.Find(".rich-text p").First(); richTextP.Length() > 0 {
		inner, _ := richTextP.Html()
		return "<figcaption>" + inner + "</figcaption>"
	}
	return "<figcaption>" + captionText + "</figcaption>"
}
