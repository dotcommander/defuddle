package defuddle

import (
	"log/slog"
	"strconv"

	"github.com/PuerkitoBio/goquery"
)

// findSmallImages finds small images that should be removed
// JavaScript original code:
//
//	private findSmallImages(doc: Document): Set<string> {
//	  // ... (processes img and svg elements, checks dimensions)
//	}
func (d *Defuddle) findSmallImages(doc *goquery.Document) map[string]bool {
	const minDimension = 33
	smallImages := make(map[string]bool)
	processedCount := 0

	// Process img and svg elements
	doc.Find("img, svg").Each(func(_ int, element *goquery.Selection) {
		// Get dimensions from attributes
		widthStr, _ := element.Attr("width")
		heightStr, _ := element.Attr("height")

		width := 0
		height := 0

		if widthStr != "" {
			if w, err := strconv.Atoi(widthStr); err == nil {
				width = w
			}
		}

		if heightStr != "" {
			if h, err := strconv.Atoi(heightStr); err == nil {
				height = h
			}
		}

		// Check if dimensions are small
		if (width > 0 && width < minDimension) || (height > 0 && height < minDimension) {
			identifier := d.getElementIdentifier(element)
			if identifier != "" {
				smallImages[identifier] = true
				processedCount++
			}
		}
	})

	if d.debug {
		slog.Debug("Found small images", "count", processedCount)
	}

	return smallImages
}

// removeSmallImages removes small images from the document
// JavaScript original code:
//
//	private removeSmallImages(doc: Document, smallImages: Set<string>) {
//	  // ... (removes elements matching smallImages set)
//	}
func (d *Defuddle) removeSmallImages(doc *goquery.Document, smallImages map[string]bool) {
	removedCount := 0

	doc.Find("img, svg").Each(func(_ int, element *goquery.Selection) {
		identifier := d.getElementIdentifier(element)
		if identifier != "" && smallImages[identifier] {
			element.Remove()
			removedCount++
		}
	})

	if d.debug {
		slog.Debug("Removed small images", "count", removedCount)
	}
}

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

// getElementIdentifier creates a unique identifier for an element
// JavaScript original code:
//
//	private getElementIdentifier(element: Element): string | null {
//	  // ... (creates identifier from src, id, viewBox, or class)
//	}
func (d *Defuddle) getElementIdentifier(element *goquery.Selection) string {
	tagName := goquery.NodeName(element)
	if tagName == "img" {
		// For lazy-loaded images, use data-src as identifier if available
		if dataSrc, exists := element.Attr("data-src"); exists && dataSrc != "" {
			return "src:" + dataSrc
		}

		if src, exists := element.Attr("src"); exists && src != "" {
			return "src:" + src
		}

		if srcset, exists := element.Attr("srcset"); exists && srcset != "" {
			return "srcset:" + srcset
		}

		if dataSrcset, exists := element.Attr("data-srcset"); exists && dataSrcset != "" {
			return "srcset:" + dataSrcset
		}
	}

	if id, exists := element.Attr("id"); exists && id != "" {
		return "id:" + id
	}

	if tagName == "svg" {
		if viewBox, exists := element.Attr("viewBox"); exists && viewBox != "" {
			return "viewBox:" + viewBox
		}
	}

	if className, exists := element.Attr("class"); exists && className != "" {
		return "class:" + className
	}

	return ""
}
