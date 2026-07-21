package metadata

import (
	"cmp"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// getSite extracts site name
// JavaScript original code:
//
//	private static getSite(doc: Document, schemaOrgData: any, metaTags: MetaTagItem[]): string {
//	  return (
//	    this.getSchemaProperty(schemaOrgData, 'publisher.name') ||
//	    this.getMetaContent(metaTags, "property", "og:site_name") ||
//	    this.getSchemaProperty(schemaOrgData, 'WebSite.name') ||
//	    this.getSchemaProperty(schemaOrgData, 'sourceOrganization.name') ||
//	    this.getMetaContent(metaTags, "name", "copyright") ||
//	    this.getSchemaProperty(schemaOrgData, 'copyrightHolder.name') ||
//	    this.getSchemaProperty(schemaOrgData, 'isPartOf.name') ||
//	    this.getMetaContent(metaTags, "name", "application-name") ||
//	    this.getAuthor(doc, schemaOrgData, metaTags) ||
//	    ''
//	  );
//	}
func getSite(doc *goquery.Document, schemaOrgData any, metaTags []MetaTag) string {
	// Use cmp.Or for cleaner fallback chain (Go 1.22+)
	return cmp.Or(
		getSchemaProperty(schemaOrgData, "publisher.name"),
		getMetaContent(metaTags, "property", "og:site_name"),
		getSchemaProperty(schemaOrgData, "WebSite.name"),
		getSchemaProperty(schemaOrgData, "sourceOrganization.name"),
		getMetaContent(metaTags, "name", "copyright"),
		getSchemaProperty(schemaOrgData, "copyrightHolder.name"),
		getSchemaProperty(schemaOrgData, "isPartOf.name"),
		getMetaContent(metaTags, "name", "application-name"),
		getAuthor(doc, schemaOrgData, metaTags),
	)
}

// getTitle extracts title
// JavaScript original code:
//
//	private static getTitle(doc: Document, schemaOrgData: any, metaTags: MetaTagItem[]): string {
//	  const rawTitle = (
//	    this.getMetaContent(metaTags, "property", "og:title") ||
//	    this.getMetaContent(metaTags, "name", "twitter:title") ||
//	    this.getSchemaProperty(schemaOrgData, 'headline') ||
//	    this.getMetaContent(metaTags, "name", "title") ||
//	    this.getMetaContent(metaTags, "name", "sailthru.title") ||
//	    doc.querySelector('title')?.textContent?.trim() ||
//	    ''
//	  );
//
//	  return this.cleanTitle(rawTitle, this.getSite(doc, schemaOrgData, metaTags));
//	}
func getTitle(doc *goquery.Document, schemaOrgData any, metaTags []MetaTag) string {
	// Use cmp.Or for cleaner fallback chain
	rawTitle := getMetaContent(metaTags, "property", "og:title")
	if rawTitle == "" {
		rawTitle = getMetaContent(metaTags, "name", "twitter:title")
	}
	if rawTitle == "" {
		rawTitle = getSchemaProperty(schemaOrgData, "headline")
	}
	if rawTitle == "" {
		rawTitle = getMetaContent(metaTags, "name", "title")
	}
	if rawTitle == "" {
		rawTitle = getMetaContent(metaTags, "name", "sailthru.title")
	}
	if rawTitle == "" {
		titleEl := doc.Find("title").First()
		if titleEl.Length() > 0 {
			rawTitle = strings.TrimSpace(titleEl.Text())
		}
	}

	return cleanTitle(rawTitle, getSite(doc, schemaOrgData, metaTags))
}

// cleanTitle removes site name from title
// JavaScript original code:
//
//	private static cleanTitle(title: string, siteName: string): string {
//	  if (!title || !siteName) return title;
//
//	  // Remove site name if it exists
//	  const siteNameEscaped = siteName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
//	  const patterns = [
//	    `\\s*[\\|\\-–—]\\s*${siteNameEscaped}\\s*$`, // Title | Site Name
//	    `^\\s*${siteNameEscaped}\\s*[\\|\\-–—]\\s*`, // Site Name | Title
//	  ];
//
//	  for (const pattern of patterns) {
//	    const regex = new RegExp(pattern, 'i');
//	    if (regex.test(title)) {
//	      title = title.replace(regex, '');
//	      break;
//	    }
//	  }
//
//	  return title.trim();
//	}
func cleanTitle(title, siteName string) string {
	if title == "" || siteName == "" {
		return title
	}

	// Remove site name if it exists
	siteNameEscaped := regexp.QuoteMeta(siteName)
	patterns := []string{
		`\s*[\|\-–—]\s*` + siteNameEscaped + `\s*$`, // Title | Site Name
		`^\s*` + siteNameEscaped + `\s*[\|\-–—]\s*`, // Site Name | Title
	}

	for _, pattern := range patterns {
		regex := regexp.MustCompile(`(?i)` + pattern)
		if regex.MatchString(title) {
			title = regex.ReplaceAllString(title, "")
			break
		}
	}

	return strings.TrimSpace(title)
}
