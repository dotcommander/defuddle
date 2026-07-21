package elements

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
//
//	const generateId = (key: string): string => {
//	  return `fn-${key}`;
//	};
//
// generateFootnoteID generates a footnote ID
func (p *FootnoteProcessor) generateFootnoteID(key string, options *FootnoteProcessingOptions) string {
	prefix := options.FootnotePrefix
	if prefix == "" {
		prefix = "fn"
	}
	return fmt.Sprintf("%s-%s", prefix, key)
}

// findOuterFootnoteContainer walks up from el through span/sup ancestors,
// returning the outermost span or sup encountered.
// TypeScript original code:
//
//	findOuterFootnoteContainer(el: any): any {
//	  let current = el;
//	  let parent = el.parentElement;
//	  while (parent && (parent.tagName === 'span' || parent.tagName === 'sup')) {
//	    current = parent;
//	    parent = parent.parentElement;
//	  }
//	  return current;
//	}
func (p *FootnoteProcessor) findOuterFootnoteContainer(s *goquery.Selection) *goquery.Selection {
	current := s
	for {
		parent := current.Parent()
		if parent.Length() == 0 {
			break
		}
		tag := goquery.NodeName(parent)
		if tag != "span" && tag != "sup" {
			break
		}
		current = parent
	}
	return current
}

// createFootnoteRefHTML returns a standardized inline reference element.
// TypeScript original code:
//
//	createFootnoteReference(footnoteNumber, refId) {
//	  const sup = createElement('sup'); sup.id = refId;
//	  const link = createElement('a'); link.href = `#fn:${footnoteNumber}`;
//	  link.textContent = footnoteNumber;
//	  sup.appendChild(link); return sup;
//	}
func (p *FootnoteProcessor) createFootnoteRefHTML(number int, refID string) string {
	return fmt.Sprintf(`<sup id="%s"><a href="#fn:%d">%d</a></sup>`, refID, number, number)
}

// createFootnoteItemHTML returns a standardized footnote list item.
// TypeScript original code (createFootnoteItem):
//
//	newItem.id = `fn:${footnoteNumber}`;
//	// copy paragraphs from content, append backlinks
func (p *FootnoteProcessor) createFootnoteItemHTML(number int, content *goquery.Selection, refs []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<li id="fn:%d">`, number)

	// Get paragraphs from content element
	paragraphs := content.Find("p")
	if paragraphs.Length() == 0 {
		// Wrap raw innerHTML in a paragraph
		inner, _ := content.Html()
		b.WriteString("<p>")
		b.WriteString(inner)
	} else {
		// Copy first paragraph; others follow after
		paragraphs.Each(func(i int, par *goquery.Selection) {
			inner, _ := par.Html()
			b.WriteString("<p>")
			b.WriteString(inner)
			if i < paragraphs.Length()-1 {
				b.WriteString("</p>")
			}
			// Leave last paragraph open so backlinks are appended inside it
		})
	}

	// Append back-links into the last paragraph
	for i, refID := range refs {
		fmt.Fprintf(&b, `<a href="#%s" title="return to article" class="footnote-backref">↩</a>`, refID)
		if i < len(refs)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("</p>")
	b.WriteString("</li>")
	return b.String()
}
