package elements

import (
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
//
//	transform: (el: Element): Element => {
//	  // Get document from element's owner document
//	  const doc = el.ownerDocument;
//	  if (!doc) {
//	    console.warn('No document available');
//	    return el;
//	  }
//
//	  // Create new heading of same level
//	  const newHeading = doc.createElement(el.tagName);
//
//	  // Copy allowed attributes from original heading
//	  Array.from(el.attributes).forEach(attr => {
//	    if (ALLOWED_ATTRIBUTES.has(attr.name)) {
//	      newHeading.setAttribute(attr.name, attr.value);
//	    }
//	  });
//
//	  // Clone the element so we can modify it without affecting the original
//	  const clone = el.cloneNode(true) as Element;
//	  // Processing logic...
//	}
//
// processHeading processes a single heading element
func (p *HeadingProcessor) processHeading(s *goquery.Selection, options *HeadingProcessingOptions) {
	slog.Debug("processing individual heading", "tag", goquery.NodeName(s))

	if !options.RemoveNavigation {
		return
	}

	// Clone the heading for processing
	clone := s.Clone()

	// Extract navigation text before removing elements
	navigationTexts := p.extractNavigationTexts(clone)

	// Remove navigation elements
	p.removeNavigationElements(clone)

	// Get cleaned text content
	textContent := strings.TrimSpace(clone.Text())

	// If we lost all text content but had navigation text, use that instead
	if textContent == "" && len(navigationTexts) > 0 {
		textContent = navigationTexts[0]
	}

	// Create new heading with cleaned content
	if options.PreserveStructure {
		p.replaceHeadingContent(s, textContent, options)
	} else {
		s.SetText(textContent)
	}

	slog.Debug("cleaned heading", "originalLength", len(s.Text()), "cleanedLength", len(textContent))
}

// extractNavigationTexts extracts text from navigation elements before removal
// TypeScript original code:
// // First extract text from navigation elements before removing them
// const navigationText = new Map<Element, string>();
//
// // Find all navigation elements and store their text content
//
//	Array.from(clone.querySelectorAll('*')).forEach(child => {
//	  let shouldRemove = false;
//
//	  if (child.tagName.toLowerCase() === 'a') {
//	    const href = child.getAttribute('href');
//	    if (href?.includes('#') || href?.startsWith('#')) {
//	      navigationText.set(child, child.textContent?.trim() || '');
//	      shouldRemove = true;
//	    }
//	  }
//	  if (child.classList.contains('anchor')) {
//	    navigationText.set(child, child.textContent?.trim() || '');
//	    shouldRemove = true;
//	  }
//	  if (child.tagName.toLowerCase() === 'button') {
//	    shouldRemove = true;
//	  }
//	  if ((child.tagName.toLowerCase() === 'span' || child.tagName.toLowerCase() === 'div') &&
//	    child.querySelector('a[href^="#"]')) {
//	    const anchor = child.querySelector('a[href^="#"]');
//	    if (anchor) {
//	      navigationText.set(child, anchor.textContent?.trim() || '');
//	    }
//	    shouldRemove = true;
//	  }
//
//	  if (shouldRemove) {
//	    // If this element contains the only text content of its parent,
//	    // store its text to be used for the parent
//	    const parent = child.parentElement;
//	    if (parent && parent !== clone &&
//	      parent.textContent?.trim() === child.textContent?.trim()) {
//	      navigationText.set(parent, child.textContent?.trim() || '');
//	    }
//	  }
//	});
func (p *HeadingProcessor) extractNavigationTexts(s *goquery.Selection) []string {
	var navigationTexts []string
	textMap := make(map[string]bool) // To avoid duplicates

	s.Find("*").Each(func(_ int, child *goquery.Selection) {
		shouldExtract := false
		var extractedText string

		// Check for anchor links with hash
		if child.Is("a") {
			href, hasHref := child.Attr("href")
			if hasHref && (strings.Contains(href, "#") || strings.HasPrefix(href, "#")) {
				extractedText = strings.TrimSpace(child.Text())
				shouldExtract = true
			}
		}

		// Check for anchor class
		if child.HasClass("anchor") {
			extractedText = strings.TrimSpace(child.Text())
			shouldExtract = true
		}

		// Check for buttons
		if child.Is("button") {
			shouldExtract = true // But don't extract text from buttons
		}

		// Check for spans/divs containing anchor links
		if child.Is("span, div") {
			anchor := child.Find("a[href^=\"#\"]").First()
			if anchor.Length() > 0 {
				extractedText = strings.TrimSpace(anchor.Text())
				shouldExtract = true
			}
		}

		// Store navigation text if it's meaningful
		if shouldExtract && extractedText != "" && !textMap[extractedText] {
			navigationTexts = append(navigationTexts, extractedText)
			textMap[extractedText] = true

			// Also check parent-child text relationship like TypeScript
			parent := child.Parent()
			childText := strings.TrimSpace(child.Text())
			parentText := strings.TrimSpace(parent.Text())
			if parentText == childText && !textMap[parentText] {
				navigationTexts = append(navigationTexts, parentText)
				textMap[parentText] = true
			}
		}
	})

	return navigationTexts
}
