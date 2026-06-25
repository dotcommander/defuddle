package defuddle

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Table is a structured representation of an HTML <table>, suitable for JSON
// output. Headers holds the column labels (from <thead> <th> cells, falling
// back to the first row that contains <th> cells); Rows holds each body row's
// cell text in document order.
type Table struct {
	Caption string     `json:"caption,omitempty"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// ExtractTables parses an HTML fragment and returns every <table> it contains
// as a structured Table. Cell text is trimmed and internal whitespace
// collapsed to single spaces. Headers come from the table's <thead> <th> cells
// when present, otherwise from the first non-thead row that contains <th>
// cells; that header row is then excluded from Rows.
func ExtractTables(htmlContent string) ([]Table, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}
	var tables []Table
	doc.Find("table").Each(func(_ int, t *goquery.Selection) {
		tables = append(tables, extractTable(t))
	})
	return tables, nil
}

func extractTable(t *goquery.Selection) Table {
	tbl := Table{Headers: []string{}, Rows: [][]string{}}
	tbl.Caption = collapseWhitespace(t.ChildrenFiltered("caption").Text())

	t.Find("thead th").Each(func(_ int, c *goquery.Selection) {
		tbl.Headers = append(tbl.Headers, collapseWhitespace(c.Text()))
	})

	headerRow := -1
	t.Find("tr").Each(func(i int, tr *goquery.Selection) {
		if tr.Closest("thead").Length() > 0 {
			return // header cells already captured above (or empty thead)
		}
		// First body row of <th> becomes the header when none found in <thead>.
		if len(tbl.Headers) == 0 && headerRow == -1 && tr.ChildrenFiltered("th").Length() > 0 {
			tr.ChildrenFiltered("th").Each(func(_ int, c *goquery.Selection) {
				tbl.Headers = append(tbl.Headers, collapseWhitespace(c.Text()))
			})
			headerRow = i
			return
		}
		var cells []string
		tr.ChildrenFiltered("td,th").Each(func(_ int, c *goquery.Selection) {
			cells = append(cells, collapseWhitespace(c.Text()))
		})
		if len(cells) > 0 {
			tbl.Rows = append(tbl.Rows, cells)
		}
	})
	return tbl
}

// collapseWhitespace trims and collapses all runs of whitespace to single spaces.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
