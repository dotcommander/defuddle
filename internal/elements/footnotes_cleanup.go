package elements

// CleanupFootnotes removes duplicate and invalid footnotes.
// TypeScript original code:
//
//	cleanupFootnotes(footnotes: Footnote[]): Footnote[] {
//	  const uniqueFootnotes = new Map();
//	  const cleaned = [];
//
//	  for (const footnote of footnotes) {
//	    if (!uniqueFootnotes.has(footnote.id) && footnote.isValid()) {
//	      uniqueFootnotes.set(footnote.id, footnote);
//	      cleaned.push(footnote);
//	    }
//	  }
//
//	  return cleaned;
//	}
func (p *FootnoteProcessor) CleanupFootnotes(footnotes []*Footnote) []*Footnote {
	seen := make(map[string]bool)
	cleaned := make([]*Footnote, 0, len(footnotes))

	for _, footnote := range footnotes {
		// Skip duplicates and invalid footnotes
		if seen[footnote.ID] || footnote.ID == "" {
			continue
		}

		seen[footnote.ID] = true
		cleaned = append(cleaned, footnote)
	}

	return cleaned
}
