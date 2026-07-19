package elements

import (
	"fmt"
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
// // Try to find definition in common footnote areas
// const footnoteSections = element.querySelectorAll(
//
//	'.footnotes, .notes, .references, .endnotes, [class*="footnote"]'
//
// );
//
//	for (const section of footnoteSections) {
//	  const definition = section.querySelector(`[id*="${key}"], [data-footnote="${key}"]`);
//	  if (definition) {
//	    return definition;
//	  }
//	}
//
// findFootnoteDefinition finds a footnote definition by key
func (p *FootnoteProcessor) findFootnoteDefinition(key string) *goquery.Selection {
	// Try various ID patterns
	selectors := []string{
		"#footnote-" + key,
		"#fn-" + key,
		"#fn:" + key,
		"#note-" + key,
		"#ref-" + key,
		fmt.Sprintf("[data-footnote='%s']", key),
		fmt.Sprintf("[data-note='%s']", key),
	}

	for _, selector := range selectors {
		if def := p.doc.Find(selector).First(); def.Length() > 0 {
			return def
		}
	}

	// Try to find in footnote sections by text content.
	// Compile patterns once per call rather than inside the nested loop.
	keyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\.`),
		regexp.MustCompile(`^\[` + regexp.QuoteMeta(key) + `\]`),
		regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\)`),
	}
	var found *goquery.Selection
	p.doc.Find(".footnotes, .notes, .references, .endnotes").Each(func(_ int, section *goquery.Selection) {
		if found != nil {
			return
		}
		section.Find("li, div, p").Each(func(_ int, el *goquery.Selection) {
			if found != nil {
				return
			}
			text := el.Text()
			for _, re := range keyPatterns {
				if re.MatchString(text) {
					found = el
					return
				}
			}
		})
	})

	return found
}

// linkFootnotes links footnote references to their definitions
// TypeScript original code:
// // Every footnote reference should be a sup element with an anchor inside
// // e.g. <sup id="fnref:1"><a href="#fn:1">1</a></sup>
//
//	createFootnoteReference(footnoteNumber: string, refId: string): any {
//	  const sup = this.doc.createElement('sup');
//	  sup.id = refId;
//	  const link = this.doc.createElement('a');
//	  link.href = `#fn:${footnoteNumber}`;
//	  link.textContent = footnoteNumber;
//	  sup.appendChild(link);
//	  return sup;
//	}
func (p *FootnoteProcessor) linkFootnotes(footnotes []*Footnote, options *FootnoteProcessingOptions) {
	for _, footnote := range footnotes {
		if footnote.Reference == nil || footnote.Definition == nil ||
			footnote.Reference.Length() == 0 || footnote.Definition.Length() == 0 {
			continue
		}

		// Ensure reference has proper structure
		if !footnote.Reference.Parent().Is("sup") {
			// Wrap in sup if not already
			footnote.Reference.WrapHtml("<sup></sup>")
		}

		// Set reference attributes
		refID := fmt.Sprintf("%sref:%d", options.FootnotePrefix, footnote.Number)
		defID := fmt.Sprintf("%s:%d", options.FootnotePrefix, footnote.Number)

		footnote.Reference.Parent().SetAttr("id", refID)
		footnote.Reference.SetAttr("href", "#"+defID)

		// Set definition attributes
		footnote.Definition.SetAttr("id", defID)

		// Add backlink to definition
		backlink := fmt.Sprintf(`<a href="#%s" class="footnote-backref">↩</a>`, refID)
		footnote.Definition.AppendHtml(backlink)

		footnote.Linked = true
	}
}
