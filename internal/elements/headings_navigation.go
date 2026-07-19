package elements

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
// // Remove navigation elements
//
//	const toRemove = Array.from(clone.querySelectorAll('*')).filter(child => {
//	  if (child.tagName.toLowerCase() === 'a') {
//	    const href = child.getAttribute('href');
//	    return href?.includes('#') || href?.startsWith('#');
//	  }
//	  if (child.classList.contains('anchor')) {
//	    return true;
//	  }
//	  if (child.tagName.toLowerCase() === 'button') {
//	    return true;
//	  }
//	  if ((child.tagName.toLowerCase() === 'span' || child.tagName.toLowerCase() === 'div') &&
//	    child.querySelector('a[href^="#"]')) {
//	    return true;
//	  }
//	  return false;
//	});
//
// toRemove.forEach(element => element.remove());
// removeNavigationElements removes navigation elements from heading
func (p *HeadingProcessor) removeNavigationElements(s *goquery.Selection) {
	var toRemove []*goquery.Selection

	s.Find("*").Each(func(_ int, child *goquery.Selection) {
		shouldRemove := false

		// Remove anchor links with hash
		if child.Is("a") {
			href, hasHref := child.Attr("href")
			if hasHref && (strings.Contains(href, "#") || strings.HasPrefix(href, "#")) {
				shouldRemove = true
			}
		}

		// Remove elements with anchor class
		if child.HasClass("anchor") {
			shouldRemove = true
		}

		// Remove buttons
		if child.Is("button") {
			shouldRemove = true
		}

		// Remove spans/divs containing anchor links
		if child.Is("span, div") {
			anchor := child.Find("a[href^=\"#\"]")
			if anchor.Length() > 0 {
				shouldRemove = true
			}
		}

		if shouldRemove {
			toRemove = append(toRemove, child)
		}
	})

	// Remove collected elements
	for _, element := range toRemove {
		element.Remove()
	}
}
