package elements

import (
	"fmt"
	"strconv"
	"strings"
)

// TypeScript original code:
// let footnoteCount = 1;
//
//	footnotes.forEach((footnote, index) => {
//	  footnote.number = footnoteCount++;
//	  // Update reference text
//	  if (footnote.reference) {
//	    footnote.reference.textContent = footnote.number.toString();
//	  }
//	});
//
// numberFootnotes assigns numbers to footnotes
func (p *FootnoteProcessor) numberFootnotes(footnotes []*Footnote, _ *FootnoteProcessingOptions) {
	for i, footnote := range footnotes {
		footnote.Number = i + 1

		// Update reference text
		if footnote.Reference != nil && footnote.Reference.Length() > 0 {
			footnote.Reference.SetText(strconv.Itoa(footnote.Number))
		}
	}
}

// improveAccessibility improves footnote accessibility
// TypeScript original code:
// // Add ARIA attributes for screen readers
// reference.setAttribute('aria-describedby', definitionId);
// reference.setAttribute('role', 'doc-noteref');
// definition.setAttribute('role', 'doc-endnote');
// definition.setAttribute('aria-label', `Footnote ${footnote.number}`);
func (p *FootnoteProcessor) improveAccessibility(footnotes []*Footnote) {
	for _, footnote := range footnotes {
		if footnote.Reference != nil && footnote.Reference.Length() > 0 {
			footnote.Reference.SetAttr("role", "doc-noteref")
			footnote.Reference.SetAttr("aria-describedby", footnote.ID)
		}

		if footnote.Definition != nil && footnote.Definition.Length() > 0 {
			footnote.Definition.SetAttr("role", "doc-endnote")
			footnote.Definition.SetAttr("aria-label", fmt.Sprintf("Footnote %d", footnote.Number))
		}
	}
}

// generateFootnoteSection generates a footnote section
// TypeScript original code:
// createFootnoteItem(
//
//	footnoteNumber: number,
//	content: string | any,
//	refs: string[]
//
//	): any {
//	  const doc = typeof content === 'string' ? this.doc : content.ownerDocument;
//	  const newItem = doc.createElement('li');
//	  newItem.className = 'footnote';
//	  newItem.id = `fn:${footnoteNumber}`;
//
//	  // Handle content
//	  if (typeof content === 'string') {
//	    const paragraph = doc.createElement('p');
//	    paragraph.innerHTML = content;
//	    newItem.appendChild(paragraph);
//	  }
//
//	  // Add backlink(s) to the last paragraph
//	  const lastParagraph = newItem.querySelector('p:last-of-type') || newItem;
//	  refs.forEach((refId, index) => {
//	    const backlink = doc.createElement('a');
//	    backlink.href = `#${refId}`;
//	    backlink.title = 'return to article';
//	    backlink.className = 'footnote-backref';
//	    backlink.innerHTML = '↩';
//	    lastParagraph.appendChild(backlink);
//	  });
//
//	  return newItem;
//	}
func (p *FootnoteProcessor) generateFootnoteSection(footnotes []*Footnote, options *FootnoteProcessingOptions) {
	if len(footnotes) == 0 {
		return
	}

	// Create footnote section HTML
	var sectionHTML strings.Builder
	fmt.Fprintf(&sectionHTML, `<div class="footnotes">
<h2>%s</h2>
<ol>`, options.SectionTitle)

	for _, footnote := range footnotes {
		if footnote.Content == "" {
			continue
		}

		defID := fmt.Sprintf("%s:%d", options.FootnotePrefix, footnote.Number)
		refID := fmt.Sprintf("%sref:%d", options.FootnotePrefix, footnote.Number)

		fmt.Fprintf(&sectionHTML, `
<li id="%s" class="footnote">
<p>%s <a href="#%s" class="footnote-backref" title="return to article">↩</a></p>
</li>`, defID, footnote.Content, refID)
	}

	sectionHTML.WriteString(`
</ol>
</div>`)

	// Insert the section
	p.insertFootnoteSection(sectionHTML.String(), options)
}
