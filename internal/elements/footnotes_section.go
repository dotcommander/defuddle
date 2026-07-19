package elements

// TypeScript original code:
// // Insert footnote section at appropriate location
// const insertLocation = options.sectionLocation || 'end';
//
//	switch (insertLocation) {
//	  case 'end':
//	    document.body.appendChild(footnoteSection);
//	    break;
//	  case 'after-content':
//	    const content = document.querySelector('main, article, .content');
//	    if (content) {
//	      content.insertAdjacentElement('afterend', footnoteSection);
//	    }
//	    break;
//	}
//
// insertFootnoteSection inserts the footnote section into the document
func (p *FootnoteProcessor) insertFootnoteSection(html string, options *FootnoteProcessingOptions) {
	switch options.SectionLocation {
	case "end":
		// Append to body
		p.doc.Find("body").AppendHtml(html)
	case "after-content":
		// Insert after main content
		contentArea := p.doc.Find("main, article, .content").First()
		if contentArea.Length() > 0 {
			contentArea.AfterHtml(html)
		} else {
			p.doc.Find("body").AppendHtml(html)
		}
	default:
		// Default to end
		p.doc.Find("body").AppendHtml(html)
	}
}

// GetFootnotes returns all footnotes found in the document
// TypeScript original code:
//
//	getFootnotes(): Footnote[] {
//	  return this.footnotes;
//	}
func (p *FootnoteProcessor) GetFootnotes() []*Footnote {
	return p.ProcessFootnotes(DefaultFootnoteProcessingOptions())
}

// HasFootnotes checks if the document has footnotes
// TypeScript original code:
//
//	hasFootnotes(): boolean {
//	  return this.footnotes.length > 0;
//	}
func (p *FootnoteProcessor) HasFootnotes() bool {
	footnotes := p.GetFootnotes()
	return len(footnotes) > 0
}
