package metadata

import (
	"cmp"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// getDescription extracts description
// JavaScript original code:
//
//	private static getDescription(doc: Document, schemaOrgData: any, metaTags: MetaTagItem[]): string {
//	  return (
//	    this.getMetaContent(metaTags, "name", "description") ||
//	    this.getMetaContent(metaTags, "property", "description") ||
//	    this.getMetaContent(metaTags, "property", "og:description") ||
//	    this.getSchemaProperty(schemaOrgData, 'description') ||
//	    this.getMetaContent(metaTags, "name", "twitter:description") ||
//	    this.getMetaContent(metaTags, "name", "sailthru.description") ||
//	    ''
//	  );
//	}
func getDescription(_ *goquery.Document, schemaOrgData any, metaTags []MetaTag) string {
	// Use cmp.Or for cleaner fallback chain (Go 1.22+)
	return cmp.Or(
		getMetaContent(metaTags, "name", "description"),
		getMetaContent(metaTags, "property", "description"),
		getMetaContent(metaTags, "property", "og:description"),
		getSchemaProperty(schemaOrgData, "description"),
		getMetaContent(metaTags, "name", "twitter:description"),
		getMetaContent(metaTags, "name", "sailthru.description"),
	)
}

// getImage extracts image URL
// JavaScript original code:
//
//	private static getImage(doc: Document, schemaOrgData: any, metaTags: MetaTagItem[]): string {
//	  return (
//	    this.getMetaContent(metaTags, "property", "og:image") ||
//	    this.getMetaContent(metaTags, "name", "twitter:image") ||
//	    this.getSchemaProperty(schemaOrgData, 'image.url') ||
//	    this.getSchemaProperty(schemaOrgData, 'image') ||
//	    this.getMetaContent(metaTags, "name", "sailthru.image.full") ||
//	    this.getMetaContent(metaTags, "name", "sailthru.image.thumb") ||
//	    ''
//	  );
//	}
func getImage(_ *goquery.Document, schemaOrgData any, metaTags []MetaTag) string {
	// Use cmp.Or for cleaner fallback chain (Go 1.22+)
	return cmp.Or(
		getMetaContent(metaTags, "property", "og:image"),
		getMetaContent(metaTags, "name", "twitter:image"),
		getSchemaProperty(schemaOrgData, "image.url"),
		getSchemaProperty(schemaOrgData, "image"),
		getMetaContent(metaTags, "name", "sailthru.image.full"),
		getMetaContent(metaTags, "name", "sailthru.image.thumb"),
	)
}

// getLanguage extracts the document language using multiple fallback sources.
// Priority: <html lang> → content-language meta → og:locale → http-equiv → schema.org inLanguage
func getLanguage(doc *goquery.Document, schemaOrgData any, metaTags []MetaTag) string {
	// 1. <html lang="...">
	if lang, exists := doc.Find("html").Attr("lang"); exists {
		if l := strings.TrimSpace(lang); l != "" {
			return normalizeLangCode(l)
		}
	}

	// 2. Content-Language meta tag or og:locale
	if cl := cmp.Or(
		getMetaContent(metaTags, "name", "content-language"),
		getMetaContent(metaTags, "property", "og:locale"),
	); cl != "" {
		return normalizeLangCode(cl)
	}

	// 3. http-equiv Content-Language
	if val, exists := doc.Find(`meta[http-equiv="Content-Language" i]`).Attr("content"); exists {
		if l := strings.TrimSpace(val); l != "" {
			return normalizeLangCode(l)
		}
	}

	// 4. Schema.org inLanguage
	if sl := getSchemaProperty(schemaOrgData, "inLanguage"); sl != "" {
		return normalizeLangCode(sl)
	}

	return ""
}

// normalizeLangCode normalizes language codes to BCP 47 format (e.g. en_US → en-US).
func normalizeLangCode(code string) string {
	return strings.ReplaceAll(code, "_", "-")
}

// getFavicon extracts favicon URL
// JavaScript original code:
//
//	private static getFavicon(doc: Document, baseUrl: string, metaTags: MetaTagItem[]): string {
//	  const favicon = doc.querySelector('link[rel*="icon"]')?.getAttribute('href') ||
//	    this.getMetaContent(metaTags, "name", "msapplication-TileImage") ||
//	    '/favicon.ico';
//
//	  if (favicon.startsWith('http')) {
//	    return favicon;
//	  }
//
//	  if (baseUrl) {
//	    try {
//	      return new URL(favicon, baseUrl).href;
//	    } catch (e) {
//	      return favicon;
//	    }
//	  }
//
//	  return favicon;
//	}
func getFavicon(doc *goquery.Document, baseURL string, metaTags []MetaTag) string {
	// First priority: og:image:favicon meta tag
	if iconFromMeta := getMetaContent(metaTags, "property", "og:image:favicon"); iconFromMeta != "" {
		return iconFromMeta
	}

	favicon := ""
	iconLink := doc.Find(`link[rel*="icon"]`).First()
	if iconLink.Length() > 0 {
		href, exists := iconLink.Attr("href")
		if exists {
			favicon = href
		}
	}

	if favicon == "" {
		favicon = getMetaContent(metaTags, "name", "msapplication-TileImage")
	}

	// Only fall back to /favicon.ico if we have a base URL to resolve against
	if favicon == "" && baseURL != "" {
		favicon = "/favicon.ico"
	}

	return resolveFaviconURL(favicon, baseURL)
}

// resolveFaviconURL returns favicon as an absolute URL, resolving a relative
// value against baseURL when possible.
func resolveFaviconURL(favicon, baseURL string) string {
	if strings.HasPrefix(favicon, "http") {
		return favicon
	}

	if baseURL != "" {
		if parsedBase, err := url.Parse(baseURL); err == nil {
			if resolvedURL, err := parsedBase.Parse(favicon); err == nil {
				return resolvedURL.String()
			}
		}
	}

	return favicon
}
