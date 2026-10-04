package defuddle

import (
	"errors"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var errRichOccurrence = errors.New("invalid rich-content occurrence")

// Restoration is exact, occurrence-based, and fail closed. A completely absent
// pair denotes excluded content; a partial or duplicated pair is never guessed.
func (m *richManifest) restore(node *html.Node) (*goquery.Selection, error) {
	var buf strings.Builder
	if err := html.Render(&buf, node); err != nil {
		return nil, err
	}
	output := buf.String()
	if err := m.validateTokens(output); err != nil {
		return nil, err
	}
	type selected struct {
		occurrence richOccurrence
		start, end int
	}
	var pairs []selected
	for _, o := range m.occurrences {
		b, e := strings.Count(output, o.begin), strings.Count(output, o.end)
		if b == 0 && e == 0 {
			continue
		}
		if b != 1 || e != 1 {
			return nil, fmt.Errorf("%w: incomplete or duplicate occurrence", errRichOccurrence)
		}
		start, end := strings.Index(output, o.begin), strings.Index(output, o.end)
		if end < start {
			return nil, fmt.Errorf("%w: reversed occurrence", errRichOccurrence)
		}
		pairs = append(pairs, selected{o, start, end + len(o.end)})
	}
	// Sort into surviving document order, also the definition append order.
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j].start < pairs[j-1].start; j-- {
			pairs[j], pairs[j-1] = pairs[j-1], pairs[j]
		}
	}
	cursor := 0
	var restored strings.Builder
	seen := make(map[string]bool)
	var definitions []string
	for _, p := range pairs {
		if p.start < cursor {
			return nil, fmt.Errorf("%w: crossing or nested occurrences", errRichOccurrence)
		}
		restored.WriteString(output[cursor:p.start])
		if err := html.Render(&restored, p.occurrence.node); err != nil {
			return nil, err
		}
		cursor = p.end
		if id := p.occurrence.definition; id != "" && !seen[id] {
			seen[id] = true
			definitions = append(definitions, id)
		}
	}
	restored.WriteString(output[cursor:])
	if strings.Contains(restored.String(), m.prefix) {
		return nil, fmt.Errorf("%w: unknown or residual marker", errRichOccurrence)
	}
	// Definitions need list context to preserve valid rich block children.
	if len(definitions) > 0 {
		restored.WriteString(`<section id="footnotes"><ol>`)
		for _, id := range definitions {
			if err := html.Render(&restored, m.definitions[id]); err != nil {
				return nil, err
			}
		}
		restored.WriteString(`</ol></section>`)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(restored.String()))
	if err != nil {
		return nil, err
	}
	return doc.Find("body").First(), nil
}

// Validate identities before any range is discarded, including tokens inside
// selected projections. A malformed suffix must not be hidden by restoration.
func (m *richManifest) validateTokens(output string) error {
	for pos := 0; pos < len(output); {
		offset := strings.Index(output[pos:], m.prefix)
		if offset < 0 {
			return nil
		}
		start := pos + offset
		token := ""
		for _, o := range m.occurrences {
			for _, known := range []string{o.begin, o.end} {
				if strings.HasPrefix(output[start:], known) {
					token = known
					break
				}
			}
			if token != "" {
				break
			}
		}
		if token == "" {
			return fmt.Errorf("%w: unknown or malformed marker", errRichOccurrence)
		}
		pos = start + len(token)
		// Full identities include their terminator; adjacent ordinary prose
		// does not alter an identity. An inserted suffix before the terminator
		// fails the exact known-token comparison above.
	}
	return nil
}
