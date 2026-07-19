// Package metadata provides functionality for extracting and processing document metadata.
// It extracts metadata from HTML documents including title, description, author, and Schema.org data.
package metadata

import (
	"cmp"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns.
var (
	arrayIndexRe = regexp.MustCompile(`^\[\d+\]$`)
	// Matches common date formats: YYYY-MM-DD, Month DD YYYY, DD Month YYYY, etc.
	datePatternRe = regexp.MustCompile(`(?i)\b(\d{4}[-/]\d{1,2}[-/]\d{1,2}|\d{1,2}\s+(?:jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\w*\s+\d{4}|(?:jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\w*\s+\d{1,2},?\s+\d{4})\b`)
	// Matches "LastName, FirstName" pattern for name reversal.
	nameReversalRe = regexp.MustCompile(`^(.*),\s(.*)$`)
)

// MetaTag represents a meta tag item from HTML
// JavaScript original code:
//
//	export interface MetaTagItem {
//	  name?: string | null;
//	  property?: string | null;
//	  content: string | null;
//	}
type MetaTag struct {
	Name     *string `json:"name,omitempty"`
	Property *string `json:"property,omitempty"`
	Content  *string `json:"content"`
}

// Metadata represents extracted metadata from a document
// JavaScript original code:
//
//	export interface DefuddleMetadata {
//	  title: string;
//	  description: string;
//	  domain: string;
//	  favicon: string;
//	  image: string;
//	  parseTime: number;
//	  published: string;
//	  author: string;
//	  site: string;
//	  schemaOrgData: any;
//	  wordCount: number;
//	}
type Metadata struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Domain        string `json:"domain"`
	Favicon       string `json:"favicon"`
	Image         string `json:"image"`
	Language      string `json:"language"`
	ParseTime     int64  `json:"parseTime"`
	Published     string `json:"published"`
	Author        string `json:"author"`
	Site          string `json:"site"`
	SchemaOrgData any    `json:"schemaOrgData"`
	WordCount     int    `json:"wordCount"`
}

// Extract extracts metadata from a document
// JavaScript original code:
//
//	static extract(doc: Document, schemaOrgData: any, metaTags: MetaTagItem[]): DefuddleMetadata {
//	  let domain = '';
//	  let url = '';
//
//	  try {
//	    // Try to get URL from document location
//	    url = doc.location?.href || '';
//
//	    // If no URL from location, try other sources
//	    if (!url) {
//	      url = this.getMetaContent(metaTags, "property", "og:url") ||
//	        this.getMetaContent(metaTags, "property", "twitter:url") ||
//	        this.getSchemaProperty(schemaOrgData, 'url') ||
//	        this.getSchemaProperty(schemaOrgData, 'mainEntityOfPage.url') ||
//	        this.getSchemaProperty(schemaOrgData, 'mainEntity.url') ||
//	        this.getSchemaProperty(schemaOrgData, 'WebSite.url') ||
//	        doc.querySelector('link[rel="canonical"]')?.getAttribute('href') || '';
//	    }
//
//	    if (url) {
//	      try {
//	        domain = new URL(url).hostname.replace(/^www\./, '');
//	      } catch (e) {
//	        console.warn('Failed to parse URL:', e);
//	      }
//	    }
//	  } catch (e) {
//	    // If URL parsing fails, try to get from base tag
//	    const baseTag = doc.querySelector('base[href]');
//	    if (baseTag) {
//	      try {
//	        url = baseTag.getAttribute('href') || '';
//	        domain = new URL(url).hostname.replace(/^www\./, '');
//	      } catch (e) {
//	        console.warn('Failed to parse base URL:', e);
//	      }
//	    }
//	  }
//
//	  return {
//	    title: this.getTitle(doc, schemaOrgData, metaTags),
//	    description: this.getDescription(doc, schemaOrgData, metaTags),
//	    domain,
//	    favicon: this.getFavicon(doc, url, metaTags),
//	    image: this.getImage(doc, schemaOrgData, metaTags),
//	    published: this.getPublished(doc, schemaOrgData, metaTags),
//	    author: this.getAuthor(doc, schemaOrgData, metaTags),
//	    site: this.getSite(doc, schemaOrgData, metaTags),
//	    schemaOrgData,
//	    wordCount: 0,
//	    parseTime: 0
//	  };
//	}
func Extract(doc *goquery.Document, schemaOrgData any, metaTags []MetaTag, baseURL string) *Metadata {
	documentURL, domain := resolveDocumentURL(doc, schemaOrgData, metaTags, baseURL)

	return &Metadata{
		Title:         getTitle(doc, schemaOrgData, metaTags),
		Description:   getDescription(doc, schemaOrgData, metaTags),
		Domain:        domain,
		Favicon:       getFavicon(doc, documentURL, metaTags),
		Image:         getImage(doc, schemaOrgData, metaTags),
		Language:      getLanguage(doc, schemaOrgData, metaTags),
		Published:     getPublished(doc, schemaOrgData, metaTags),
		Author:        getAuthor(doc, schemaOrgData, metaTags),
		Site:          getSite(doc, schemaOrgData, metaTags),
		SchemaOrgData: schemaOrgData,
		WordCount:     0,
		ParseTime:     0,
	}
}

// resolveDocumentURL determines the document URL (from baseURL, meta/schema, a
// canonical link, or a base tag) and the domain derived from it.
func resolveDocumentURL(doc *goquery.Document, schemaOrgData any, metaTags []MetaTag, baseURL string) (documentURL, domain string) {
	documentURL = baseURL

	// If no base URL provided, try to extract from meta tags and canonical links
	if documentURL == "" {
		documentURL = cmp.Or(
			getMetaContent(metaTags, "property", "og:url"),
			getMetaContent(metaTags, "property", "twitter:url"),
			getSchemaProperty(schemaOrgData, "url"),
			getSchemaProperty(schemaOrgData, "mainEntityOfPage.url"),
			getSchemaProperty(schemaOrgData, "mainEntity.url"),
			getSchemaProperty(schemaOrgData, "WebSite.url"),
		)
		if documentURL == "" {
			canonical := doc.Find(`link[rel="canonical"]`).First()
			if canonical.Length() > 0 {
				documentURL, _ = canonical.Attr("href")
			}
		}
	}

	// Extract domain from URL
	if documentURL != "" {
		if parsedURL, err := url.Parse(documentURL); err == nil {
			domain = strings.TrimPrefix(parsedURL.Hostname(), "www.")
		}
	}

	// If still no URL, try base tag
	if documentURL == "" {
		baseTag := doc.Find("base[href]").First()
		if baseTag.Length() > 0 {
			if href, exists := baseTag.Attr("href"); exists {
				documentURL = href
				if parsedURL, err := url.Parse(documentURL); err == nil {
					domain = strings.TrimPrefix(parsedURL.Hostname(), "www.")
				}
			}
		}
	}

	return documentURL, domain
}
