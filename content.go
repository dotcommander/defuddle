package defuddle

import (
	"log/slog"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/internal/constants"
	"github.com/kaptinlin/defuddle-go/internal/scoring"
	"github.com/kaptinlin/defuddle-go/internal/text"
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
//
// contentCandidate represents a scored entry point match.
type contentCandidate struct {
	element       *goquery.Selection
	score         float64
	selectorIndex int
}

func (d *Defuddle) findMainContent(doc *goquery.Document) *goquery.Selection {
	entryPoints := constants.GetEntryPointElements()
	var candidates []contentCandidate

	// Score ALL matches from ALL entry point selectors
	for i, selector := range entryPoints {
		doc.Find(selector).Each(func(_ int, element *goquery.Selection) {
			// Base score from selector priority (earlier = higher)
			score := float64(len(entryPoints)-i) * 40
			// Add content-based score
			score += scoring.ScoreElement(element)
			candidates = append(candidates, contentCandidate{
				element:       element,
				score:         score,
				selectorIndex: i,
			})
		})
	}

	if len(candidates) == 0 {
		// Fall back to scoring block elements
		scoredContent := d.findContentByScoring(doc)
		if scoredContent != nil {
			if d.debug {
				slog.Debug("Found main content using scoring")
			}
			return scoredContent
		}
		return nil
	}

	// Sort by score descending
	sort.Slice(candidates, func(a, b int) bool {
		return candidates[a].score > candidates[b].score
	})

	if d.debug {
		for _, c := range candidates {
			tag := goquery.NodeName(c.element)
			cls := c.element.AttrOr("class", "")
			id := c.element.AttrOr("id", "")
			slog.Debug("Content candidate",
				"tag", tag, "class", cls, "id", id,
				"score", c.score, "selectorIndex", c.selectorIndex)
		}
	}

	// If we only matched body, try table-based detection
	if len(candidates) == 1 && strings.EqualFold(goquery.NodeName(candidates[0].element), "body") {
		tableContent := d.findTableBasedContent(doc)
		if tableContent != nil {
			if d.debug {
				slog.Debug("Found main content using table-based detection")
			}
			return tableContent
		}
	}

	// If the top candidate contains a child candidate that matched a
	// higher-priority selector (lower index), prefer the more specific child.
	// This prevents e.g. <main> from winning over a contained <article>
	// just because sibling noise inflates the parent's content score.
	top := candidates[0]
	best := top

	// Don't descend into child on listing pages (multiple articles)
	articleCount := top.element.Find("article").Length()
	if articleCount < 3 {
		for i := 1; i < len(candidates); i++ {
			child := candidates[i]
			childText := strings.TrimSpace(child.element.Text())
			childWords := text.CountWords(childText)
			if child.selectorIndex < best.selectorIndex && scoring.NodeContains(best.element, child.element) && childWords > 50 {
				best = child
			}
		}
	}

	if d.debug {
		tag := goquery.NodeName(best.element)
		slog.Debug("Selected main content", "tag", tag, "score", best.score)
	}

	return best.element
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
