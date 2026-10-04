package defuddle

import (
	"github.com/PuerkitoBio/goquery"
	"log/slog"
)

// removeAllImages removes all images from the document
// Implements the removeImages option from TypeScript version
func (d *Defuddle) removeAllImages(doc *goquery.Document) {
	removedCount := 0

	doc.Find("img, svg, picture, video, canvas").Each(func(_ int, element *goquery.Selection) {
		element.Remove()
		removedCount++
	})

	if d.debug {
		slog.Debug("Removed all images", "count", removedCount)
	}
}
