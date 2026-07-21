package metadata

import (
	"cmp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// getAuthor extracts author information
// JavaScript original code:
//
//	private static getAuthor(doc: Document, schemaOrgData: any, metaTags: MetaTagItem[]): string {
//	  let authorsString: string | undefined;
//
//	  // Meta tags - typically expect a single string, possibly comma-separated
//	  authorsString = this.getMetaContent(metaTags, "name", "sailthru.author") ||
//	    this.getMetaContent(metaTags, "property", "author") ||
//	    this.getMetaContent(metaTags, "name", "author") ||
//	    this.getMetaContent(metaTags, "name", "byl") ||
//	    this.getMetaContent(metaTags, "name", "authorList");
//	  if (authorsString) return authorsString;
//
//	  // 2. Schema.org data - deduplicate if it's a list
//	  let schemaAuthors = this.getSchemaProperty(schemaOrgData, 'author.name') ||
//	    this.getSchemaProperty(schemaOrgData, 'author.[].name');
//
//	  if (schemaAuthors) {
//	    const parts = schemaAuthors.split(',')
//	      .map(part => part.trim().replace(/,$/, '').trim())
//	      .filter(Boolean);
//	    if (parts.length > 0) {
//	      let uniqueSchemaAuthors = [...new Set(parts)];
//	      if (uniqueSchemaAuthors.length > 10) {
//	        uniqueSchemaAuthors = uniqueSchemaAuthors.slice(0, 10);
//	      }
//	      return uniqueSchemaAuthors.join(', ');
//	    }
//	  }
//
//	  // 3. DOM elements
//	  const collectedAuthorsFromDOM: string[] = [];
//	  const addDomAuthor = (value: string | null | undefined) => {
//	    if (!value) return;
//	    value.split(',').forEach(namePart => {
//	      const cleanedName = namePart.trim().replace(/,$/, '').trim();
//	      const lowerCleanedName = cleanedName.toLowerCase();
//	      if (cleanedName && lowerCleanedName !== 'author' && lowerCleanedName !== 'authors') {
//	        collectedAuthorsFromDOM.push(cleanedName);
//	      }
//	    });
//	  };
//
//	  const domAuthorSelectors = [
//	    '[itemprop="author"]',
//	    '.author',
//	    '[href*="author"]',
//	    '.authors a',
//	  ];
//
//	  domAuthorSelectors.forEach(selector => {
//	    doc.querySelectorAll(selector).forEach(el => {
//	      addDomAuthor(el.textContent);
//	    });
//	  });
//
//	  if (collectedAuthorsFromDOM.length > 0) {
//	    let uniqueAuthors = [...new Set(collectedAuthorsFromDOM.map(name => name.trim()).filter(Boolean))];
//	    if (uniqueAuthors.length > 0) {
//	      if (uniqueAuthors.length > 10) {
//	        uniqueAuthors = uniqueAuthors.slice(0, 10);
//	      }
//	      return uniqueAuthors.join(', ');
//	    }
//	  }
//
//	  // 4. Fallback meta tags and schema properties (less direct for author names)
//	  authorsString = this.getMetaContent(metaTags, "name", "copyright") ||
//	    this.getSchemaProperty(schemaOrgData, 'copyrightHolder.name') ||
//	    this.getMetaContent(metaTags, "property", "og:site_name") ||
//	    this.getSchemaProperty(schemaOrgData, 'publisher.name') ||
//	    this.getSchemaProperty(schemaOrgData, 'sourceOrganization.name') ||
//	    this.getSchemaProperty(schemaOrgData, 'isPartOf.name') ||
//	    this.getMetaContent(metaTags, "name", "twitter:creator") ||
//	    this.getMetaContent(metaTags, "name", "application-name");
//	  if (authorsString) return authorsString;
//
//	  return '';
//	}
func getAuthor(doc *goquery.Document, schemaOrgData any, metaTags []MetaTag) string {
	// Research paper meta tags: citation_author, dc.creator (support multi-value)
	citationAuthors := getMetaContents(metaTags, "name", "citation_author")
	if len(citationAuthors) == 0 {
		citationAuthors = getMetaContents(metaTags, "property", "dc.creator")
	}
	if len(citationAuthors) > 0 {
		reversed := make([]string, 0, len(citationAuthors))
		for _, s := range citationAuthors {
			// Reverse "LastName, FirstName" → "FirstName LastName"
			if m := nameReversalRe.FindStringSubmatch(s); m != nil {
				reversed = append(reversed, strings.TrimSpace(m[2])+" "+strings.TrimSpace(m[1]))
			} else {
				reversed = append(reversed, strings.TrimSpace(s))
			}
		}
		return strings.Join(reversed, ", ")
	}

	// Standard meta tags - use cmp.Or for cleaner fallback chain (Go 1.22+)
	authors := cmp.Or(
		getMetaContent(metaTags, "name", "sailthru.author"),
		getMetaContent(metaTags, "property", "author"),
		getMetaContent(metaTags, "name", "author"),
		getMetaContent(metaTags, "name", "byl"),
		getMetaContent(metaTags, "name", "authorList"),
	)
	if authors != "" {
		return authors
	}

	// Schema.org data - deduplicate if it's a list
	schemaAuthors := cmp.Or(
		getSchemaProperty(schemaOrgData, "author.name"),
		getSchemaProperty(schemaOrgData, "author.[].name"),
	)

	if schemaAuthors != "" {
		parts := strings.Split(schemaAuthors, ",")
		var cleanParts []string
		for _, part := range parts {
			cleaned := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(part), ","))
			if cleaned != "" {
				cleanParts = append(cleanParts, cleaned)
			}
		}
		if joined := joinUniqueAuthors(cleanParts); joined != "" {
			return joined
		}
	}

	// DOM elements
	var domAuthors []string
	addDomAuthor := func(value string) {
		if value == "" {
			return
		}
		for namePart := range strings.SplitSeq(value, ",") {
			cleanedName := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(namePart), ","))
			lowerCleanedName := strings.ToLower(cleanedName)
			if cleanedName != "" && lowerCleanedName != "author" && lowerCleanedName != "authors" {
				domAuthors = append(domAuthors, cleanedName)
			}
		}
	}

	domAuthorSelectors := []string{
		`[itemprop="author"]`,
		".author",
		`[href*="author"]`,
		".authors a",
	}

	for _, selector := range domAuthorSelectors {
		doc.Find(selector).Each(func(_ int, el *goquery.Selection) {
			addDomAuthor(strings.TrimSpace(el.Text()))
		})
	}

	if len(domAuthors) > 0 {
		var cleanAuthors []string
		for _, name := range domAuthors {
			trimmed := strings.TrimSpace(name)
			if trimmed != "" {
				cleanAuthors = append(cleanAuthors, trimmed)
			}
		}
		if joined := joinUniqueAuthors(cleanAuthors); joined != "" {
			return joined
		}
	}

	// Fallback meta tags and schema properties - use cmp.Or (Go 1.22+)
	return cmp.Or(
		getMetaContent(metaTags, "name", "copyright"),
		getSchemaProperty(schemaOrgData, "copyrightHolder.name"),
		getMetaContent(metaTags, "property", "og:site_name"),
		getSchemaProperty(schemaOrgData, "publisher.name"),
		getSchemaProperty(schemaOrgData, "sourceOrganization.name"),
		getSchemaProperty(schemaOrgData, "isPartOf.name"),
		getMetaContent(metaTags, "name", "twitter:creator"),
		getMetaContent(metaTags, "name", "application-name"),
	)
}

// joinUniqueAuthors deduplicates names (preserving order), caps the result at 10,
// and joins with ", ". Returns "" when names is empty.
func joinUniqueAuthors(names []string) string {
	unique := removeDuplicates(names)
	if len(unique) == 0 {
		return ""
	}
	if len(unique) > 10 {
		unique = unique[:10]
	}
	return strings.Join(unique, ", ")
}
