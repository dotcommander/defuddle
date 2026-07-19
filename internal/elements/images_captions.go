package elements

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

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
	if fc := el.Find("figcaption").First(); fc.Length() > 0 {
		return fc
	}
	if c := captionByClass(el); c != nil {
		return c
	}
	if c := captionFromSibling(el); c != nil {
		return c
	}
	return captionFromFollowingText(el)
}

// captionByClass finds a non-media descendant matching a caption-class selector.
func captionByClass(el *goquery.Selection) *goquery.Selection {
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
	return found
}

// captionFromSibling finds a sibling element whose class indicates a caption, credit, or description.
func captionFromSibling(el *goquery.Selection) *goquery.Selection {
	if el.Length() == 0 || el.Get(0).Parent == nil {
		return nil
	}
	var found *goquery.Selection
	el.Parent().Children().Each(func(_ int, sib *goquery.Selection) {
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
	return found
}

// captionFromFollowingText finds an inline text element (em/strong/span/...) following an image.
func captionFromFollowingText(el *goquery.Selection) *goquery.Selection {
	var found *goquery.Selection
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
