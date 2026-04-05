package defuddle

import (
	"log/slog"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/internal/constants"
	"github.com/kaptinlin/defuddle-go/internal/scoring"
)

// findMainContent finds the main content element
// JavaScript original code:
//
//	private findMainContent(doc: Document): Element | null {
//	  // Try entry point elements first
//	  for (const selector of ENTRY_POINT_ELEMENTS) {
//	    const element = doc.querySelector(selector);
//	    if (element) {
//	      return element;
//	    }
//	  }
//
//	  // Try table-based content
//	  const tableContent = this.findTableBasedContent(doc);
//	  if (tableContent) {
//	    return tableContent;
//	  }
//
//	  // Try content scoring
//	  const scoredContent = this.findContentByScoring(doc);
//	  if (scoredContent) {
//	    return scoredContent;
//	  }
//
//	  return null;
//	}
func (d *Defuddle) findMainContent(doc *goquery.Document) *goquery.Selection {
	// Try entry point elements first
	entryPoints := constants.GetEntryPointElements()
	for _, selector := range entryPoints {
		element := doc.Find(selector).First()
		if element.Length() > 0 {
			if d.debug {
				slog.Debug("Found main content using entry point", "selector", selector)
			}
			return element
		}
	}

	// Try table-based content
	tableContent := d.findTableBasedContent(doc)
	if tableContent != nil {
		if d.debug {
			slog.Debug("Found main content using table-based detection")
		}
		return tableContent
	}

	// Try content scoring
	scoredContent := d.findContentByScoring(doc)
	if scoredContent != nil {
		if d.debug {
			slog.Debug("Found main content using scoring")
		}
		return scoredContent
	}

	return nil
}

// findTableBasedContent finds content in table-based layouts
// JavaScript original code:
//
//	private findTableBasedContent(doc: Document): Element | null {
//	  const tables = doc.querySelectorAll('table');
//	  let bestTable: Element | null = null;
//	  let bestScore = 0;
//
//	  tables.forEach(table => {
//	    const cells = table.querySelectorAll('td');
//	    cells.forEach(cell => {
//	      const score = ContentScorer.scoreElement(cell);
//	      if (score > bestScore) {
//	        bestScore = score;
//	        bestTable = cell;
//	      }
//	    });
//	  });
//
//	  return bestScore > 50 ? bestTable : null;
//	}
func (d *Defuddle) findTableBasedContent(doc *goquery.Document) *goquery.Selection {
	var bestElement *goquery.Selection
	bestScore := 0.0

	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		table.Find("td").Each(func(_ int, cell *goquery.Selection) {
			score := scoring.ScoreElement(cell)
			if score > bestScore {
				bestScore = score
				bestElement = cell
			}
		})
	})

	if bestScore > 50 {
		return bestElement
	}
	return nil
}

// findContentByScoring finds content using scoring algorithm
// JavaScript original code:
//
//	private findContentByScoring(doc: Document): Element | null {
//	  const candidates = doc.querySelectorAll('div, section, article, main');
//	  const elements = Array.from(candidates);
//	  return ContentScorer.findBestElement(elements, 50);
//	}
func (d *Defuddle) findContentByScoring(doc *goquery.Document) *goquery.Selection {
	var candidates []*goquery.Selection
	doc.Find("div, section, article, main").Each(func(_ int, s *goquery.Selection) {
		candidates = append(candidates, s)
	})

	return scoring.FindBestElement(candidates, 50)
}
