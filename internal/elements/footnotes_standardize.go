package elements

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// collectFootnoteDefinitions collects all footnote definitions from a scope.
// It mirrors the TypeScript collectFootnotes method, returning a sequential
// number → footnoteEntry map so callers can rebuild a canonical list.
// TypeScript original code:
//
//	collectFootnotes(element: any): FootnoteCollection {
//	  const footnotes: FootnoteCollection = {};
//	  let footnoteCount = 1;
//	  const processedIds = new Set<string>();
//	  const footnoteLists = element.querySelectorAll(FOOTNOTE_LIST_SELECTORS);
//	  footnoteLists.forEach((list: any) => { ... });
//	  return footnotes;
//	}
func (p *FootnoteProcessor) collectFootnoteDefinitions(scope *goquery.Selection) map[int]*footnoteEntry {
	result := make(map[int]*footnoteEntry)
	count := 1
	processedIDs := make(map[string]bool)

	scope.Find(FootnoteListSelectors).Each(func(_ int, list *goquery.Selection) {
		// Substack: individual footnote divs with no parent list
		if goquery.NodeName(list) == "div" {
			if _, ok := list.Attr("data-component-name"); ok {
				p.collectSubstackFootnote(list, result, processedIDs, &count)
				return
			}
		}
		p.collectListFootnotes(list, result, processedIDs, &count)
	})

	return result
}

// collectSubstackFootnote handles Substack-format footnote divs.
func (p *FootnoteProcessor) collectSubstackFootnote(
	list *goquery.Selection,
	result map[int]*footnoteEntry,
	seen map[string]bool,
	count *int,
) {
	anchor := list.Find("a.footnote-number").First()
	content := list.Find(".footnote-content").First()
	if anchor.Length() == 0 || content.Length() == 0 {
		return
	}
	id, _ := anchor.Attr("id")
	id = strings.ToLower(strings.TrimPrefix(id, "footnote-"))
	if id == "" || seen[id] {
		return
	}
	result[*count] = &footnoteEntry{content: content, originalID: id}
	seen[id] = true
	*count++
}

// collectListFootnotes handles standard OL/UL footnote lists.
func (p *FootnoteProcessor) collectListFootnotes(
	list *goquery.Selection,
	result map[int]*footnoteEntry,
	seen map[string]bool,
	count *int,
) {
	list.Find(`li, div[role="listitem"]`).Each(func(_ int, li *goquery.Selection) {
		id, content := p.extractListItemIDAndContent(li)
		if id == "" || seen[id] {
			return
		}
		if content == nil {
			content = li
		}
		result[*count] = &footnoteEntry{content: content, originalID: id}
		seen[id] = true
		*count++
	})
}

// extractListItemIDAndContent derives the canonical ID and content element from an li.
// TypeScript original code (inside collectFootnotes items.forEach):
//
//	const citationsDiv = li.querySelector('.citations');
//	if (citationsDiv?.id?.toLowerCase().startsWith('r')) { ... }
//	else if (li.id.toLowerCase().startsWith('bib.bib')) { ... }
//	else if (li.id.toLowerCase().startsWith('fn:')) { ... }
//	...
func (p *FootnoteProcessor) extractListItemIDAndContent(li *goquery.Selection) (string, *goquery.Selection) {
	// Citations div (Science.org)
	citDiv := li.Find(".citations").First()
	if citDiv.Length() > 0 {
		citID, _ := citDiv.Attr("id")
		if strings.HasPrefix(strings.ToLower(citID), "r") {
			citContent := citDiv.Find(".citation-content").First()
			if citContent.Length() > 0 {
				return strings.ToLower(citID), citContent
			}
			return strings.ToLower(citID), citDiv
		}
	}

	rawID, _ := li.Attr("id")
	lower := strings.ToLower(rawID)
	var id string

	switch {
	case strings.HasPrefix(lower, "bib.bib"):
		id = strings.TrimPrefix(lower, "bib.bib")
	case strings.HasPrefix(lower, "fn:"):
		id = strings.TrimPrefix(lower, "fn:")
	case strings.HasPrefix(lower, "fn"):
		id = strings.TrimPrefix(lower, "fn")
	default:
		if dc, exists := li.Attr("data-counter"); exists {
			// Nature.com: strip trailing dot
			id = strings.ToLower(reDotTrailing.ReplaceAllString(dc, ""))
		} else {
			// Wikipedia: cite_note-<suffix>
			raw := strings.Split(rawID, "/")
			last := raw[len(raw)-1]
			if m := reCiteNoteSuffix.FindStringSubmatch(last); m != nil {
				id = strings.ToLower(m[1])
			} else {
				id = strings.ToLower(rawID)
			}
		}
	}

	return id, li
}

// extractInlineFootnoteID derives the canonical footnote ID from an inline
// reference element, matching all site-specific patterns from the TypeScript.
// TypeScript original code: the large if-else chain inside standardizeFootnotes.
func (p *FootnoteProcessor) extractInlineFootnoteID(el *goquery.Selection) string {
	// Nature.com: a[id^="ref-link"]
	if id, _ := el.Attr("id"); strings.HasPrefix(id, "ref-link") {
		return strings.TrimSpace(el.Text())
	}

	// Science.org: a[role="doc-biblioref"]
	if role, _ := el.Attr("role"); role == "doc-biblioref" {
		if xmlRid, ok := el.Attr("data-xml-rid"); ok && xmlRid != "" {
			return xmlRid
		}
		if href, ok := el.Attr("href"); ok && strings.HasPrefix(href, "#core-R") {
			return strings.TrimPrefix(href, "#core-")
		}
	}

	// Substack: a.footnote-anchor
	if el.HasClass("footnote-anchor") {
		if id, _ := el.Attr("id"); id != "" {
			return strings.ToLower(strings.TrimPrefix(id, "footnote-anchor-"))
		}
	}

	// ArXiv: cite.ltx_cite
	if goquery.NodeName(el) == "cite" && el.HasClass("ltx_cite") {
		link := el.Find("a").First()
		if link.Length() > 0 {
			if href, ok := link.Attr("href"); ok {
				parts := strings.Split(href, "/")
				last := parts[len(parts)-1]
				if m := reBibBib.FindStringSubmatch(last); m != nil {
					return strings.ToLower(m[1])
				}
			}
		}
		return ""
	}

	// Wikipedia: sup.reference
	if goquery.NodeName(el) == "sup" && el.HasClass("reference") {
		var found string
		el.Find("a").Each(func(_ int, a *goquery.Selection) {
			if found != "" {
				return
			}
			if href, ok := a.Attr("href"); ok {
				parts := strings.Split(href, "/")
				last := parts[len(parts)-1]
				if m := reCiteNoteRef.FindStringSubmatch(last); m != nil {
					found = strings.ToLower(m[1])
				}
			}
		})
		return found
	}

	// sup[id^="fnref:"]
	if tag := goquery.NodeName(el); tag == "sup" {
		if id, _ := el.Attr("id"); strings.HasPrefix(id, "fnref:") {
			return strings.ToLower(strings.TrimPrefix(id, "fnref:"))
		}
		// sup[id^="fnr"]
		if id, _ := el.Attr("id"); strings.HasPrefix(id, "fnr") {
			return strings.ToLower(strings.TrimPrefix(id, "fnr"))
		}
	}

	// LessWrong / span.footnote-reference
	if el.HasClass("footnote-reference") || el.HasClass("footnote_ref") || el.HasClass("footnote-ref") {
		if fnID, ok := el.Attr("data-footnote-id"); ok && fnID != "" {
			return fnID
		}
		if id, _ := el.Attr("id"); strings.HasPrefix(id, "fnref") {
			return strings.ToLower(strings.TrimPrefix(id, "fnref"))
		}
	}

	// span.footnote-link
	if el.HasClass("footnote-link") {
		if fnID, ok := el.Attr("data-footnote-id"); ok && fnID != "" {
			return fnID
		}
	}

	// a.citation
	if goquery.NodeName(el) == "a" && el.HasClass("citation") {
		return strings.TrimSpace(el.Text())
	}

	// a[id^="fnref"]
	if goquery.NodeName(el) == "a" {
		if id, _ := el.Attr("id"); strings.HasPrefix(id, "fnref") {
			return strings.ToLower(strings.TrimPrefix(id, "fnref"))
		}
	}

	// Default: use href
	if href, ok := el.Attr("href"); ok && href != "" {
		id := strings.TrimPrefix(href, "#")
		return strings.ToLower(id)
	}

	return ""
}
