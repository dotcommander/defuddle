package metadata

import (
	"cmp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// getPublished extracts publication date
// JavaScript original code:
//
//	private static getPublished(doc: Document, schemaOrgData: any, metaTags: MetaTagItem[]): string {
//	  return (
//	    this.getSchemaProperty(schemaOrgData, 'datePublished') ||
//	    this.getMetaContent(metaTags, "property", "article:published_time") ||
//	    this.getMetaContent(metaTags, "name", "sailthru.date") ||
//	    this.getMetaContent(metaTags, "name", "date") ||
//	    this.getTimeElement(doc) ||
//	    ''
//	  );
//	}
func getPublished(doc *goquery.Document, schemaOrgData any, metaTags []MetaTag) string {
	// Use cmp.Or for cleaner fallback chain matching TS priority order
	result := cmp.Or(
		getSchemaProperty(schemaOrgData, "datePublished"),
		getMetaContent(metaTags, "name", "publishDate"),
		getMetaContent(metaTags, "property", "article:published_time"),
		getAbbrevDatePublished(doc),
		getTimeElement(doc),
		getMetaContent(metaTags, "name", "sailthru.date"),
		getMetaContent(metaTags, "name", "date"),
	)
	if result != "" {
		return result
	}

	// Near-h1 date scanning: check up to 3 siblings after the h1
	h1 := doc.Find("h1").First()
	if h1.Length() > 0 {
		sibling := h1.Next()
		for range 3 {
			if sibling.Length() == 0 {
				break
			}
			text := strings.TrimSpace(sibling.Text())
			if parsed := parseDateText(text); parsed != "" {
				return parsed
			}
			sibling = sibling.Next()
		}
	}

	return ""
}

// getAbbrevDatePublished extracts date from abbr[itemprop="datePublished"] title attr.
func getAbbrevDatePublished(doc *goquery.Document) string {
	abbr := doc.Find(`abbr[itemprop="datePublished"]`).First()
	if abbr.Length() > 0 {
		if title, exists := abbr.Attr("title"); exists {
			return strings.TrimSpace(title)
		}
	}
	return ""
}

// getMetaContent finds meta tag content by attribute and value
// JavaScript original code:
//
//	private static getMetaContent(metaTags: MetaTagItem[], attr: string, value: string): string {
//	  const tag = metaTags.find(tag => tag[attr] === value);
//	  return tag?.content || '';
//	}
func getMetaContent(metaTags []MetaTag, attr, value string) string {
	for _, tag := range metaTags {
		var tagValue *string
		switch attr {
		case "name":
			tagValue = tag.Name
		case "property":
			tagValue = tag.Property
		}
		if tagValue != nil && strings.EqualFold(*tagValue, value) && tag.Content != nil {
			return *tag.Content
		}
	}
	return ""
}

// getMetaContents returns all matching meta tag values (for multi-value meta like citation_author).
func getMetaContents(metaTags []MetaTag, attr, value string) []string {
	var results []string
	for _, tag := range metaTags {
		var tagValue *string
		switch attr {
		case "name":
			tagValue = tag.Name
		case "property":
			tagValue = tag.Property
		}
		if tagValue != nil && strings.EqualFold(*tagValue, value) && tag.Content != nil && *tag.Content != "" {
			results = append(results, *tag.Content)
		}
	}
	return results
}

// parseDateText extracts a date from freeform text near headings.
func parseDateText(text string) string {
	if text == "" || len(text) > 200 {
		return ""
	}
	m := datePatternRe.FindString(text)
	return strings.TrimSpace(m)
}

// getTimeElement extracts time from time elements
// JavaScript original code:
//
//	private static getTimeElement(doc: Document): string {
//	  const timeEl = doc.querySelector('time[datetime]');
//	  return timeEl?.getAttribute('datetime') || '';
//	}
func getTimeElement(doc *goquery.Document) string {
	timeEl := doc.Find("time[datetime]").First()
	if timeEl.Length() > 0 {
		datetime, exists := timeEl.Attr("datetime")
		if exists {
			return datetime
		}
	}
	return ""
}
