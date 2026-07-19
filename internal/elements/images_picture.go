package elements

import "github.com/PuerkitoBio/goquery"

// --- Transform 1: Picture element collapse ---

// transformPictures collapses <picture> elements by selecting the best
// <source> and applying its srcset to the <img>, then removing sources.
func (p *ImageProcessor) transformPictures(scope *goquery.Selection) {
	scope.Find("picture").Each(func(_ int, el *goquery.Selection) {
		transformPicture(el)
	})
}

// transformPicture rewrites one <picture> element: it promotes the best <source>
// srcset/src onto the fallback <img> (creating one if absent) and drops the sources.
func transformPicture(el *goquery.Selection) {
	// Collect source elements
	var sources []*goquery.Selection
	el.Find("source").Each(func(_ int, src *goquery.Selection) {
		sources = append(sources, src)
	})

	img := el.Find("img").First()

	if img.Length() == 0 {
		// No img fallback — try to create one from best source
		if srcset, firstURL := bestSourceSrcset(sources); firstURL != "" && isValidImageURL(firstURL) {
			el.SetHtml(`<img src="` + firstURL + `" srcset="` + srcset + `"/>`)
		}
		return
	}

	var bestSrcset, bestSrc string
	if len(sources) > 0 {
		bestSrcset, bestSrc = bestSourceSrcset(sources)
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
}

// bestSourceSrcset returns the srcset of the best <source> in sources and the
// first URL within it, or empty strings if there is no usable source.
func bestSourceSrcset(sources []*goquery.Selection) (srcset, firstURL string) {
	best := selectBestSource(sources)
	if best == nil {
		return "", ""
	}
	if s, ok := best.Attr("srcset"); ok && s != "" {
		return s, extractFirstURLFromSrcset(s)
	}
	return "", ""
}
