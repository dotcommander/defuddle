package defuddle

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/urlutil"
	"golang.org/x/net/html"
)

type richOccurrence struct {
	begin, end string
	node       *html.Node
	definition string
}

type richManifest struct {
	prefix      string
	occurrences []richOccurrence
	definitions map[string]*html.Node
}

func cloneRichNode(n *html.Node) *html.Node {
	c := &html.Node{Type: n.Type, DataAtom: n.DataAtom, Data: n.Data, Namespace: n.Namespace, Attr: append([]html.Attribute(nil), n.Attr...)}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.AppendChild(cloneRichNode(child))
	}
	return c
}

// printable per-invocation identities avoid invisible-character cleanup and
// collide with neither source text nor attributes. No registry survives a parse.
func newRichManifest(doc *goquery.Document) (*richManifest, error) {
	source, err := doc.Html()
	if err != nil {
		return nil, err
	}
	for {
		var entropy [16]byte
		if _, err := rand.Read(entropy[:]); err != nil {
			return nil, fmt.Errorf("rich marker identity: %w", err)
		}
		prefix := "DFR" + hex.EncodeToString(entropy[:]) + "X"
		if !strings.Contains(source, prefix) {
			return &richManifest{prefix: prefix, definitions: make(map[string]*html.Node)}, nil
		}
	}
}

func (m *richManifest) protect(sel *goquery.Selection, definition string) {
	n := cloneRichNode(sel.Get(0))
	urlutil.SanitizeUnsafe(goquery.NewDocumentFromNode(n).Selection)
	id := len(m.occurrences)
	// A printable terminator makes identity independent of whitespace. Native
	// fallback can join the end of a block directly to the next prose word.
	begin := fmt.Sprintf("%s%dB:", m.prefix, id)
	end := fmt.Sprintf("%s%dE:", m.prefix, id)
	m.occurrences = append(m.occurrences, richOccurrence{begin: begin, end: end, node: n, definition: definition})
	projection := sel.Text()
	if projection == "" {
		projection = sel.AttrOr("data-latex", sel.AttrOr("data-math", "formula"))
	}
	tag := "span"
	if goquery.NodeName(sel) == "pre" || sel.AttrOr("display", "") == "block" {
		tag = "p"
	}
	replacement := &html.Node{Type: html.ElementNode, Data: tag}
	replacement.AppendChild(&html.Node{Type: html.TextNode, Data: begin + " " + projection + " " + end})
	sel.ReplaceWithNodes(replacement)
}

// Relationship discovery happens before URLs become absolute. Duplicate IDs
// and remote hrefs never establish a definition relationship.
func prepareRichContent(doc *goquery.Document, root *goquery.Selection) (*richManifest, error) {
	m, err := newRichManifest(doc)
	if err != nil {
		return nil, err
	}
	m.protectFootnotes(root)
	selector := `pre, code, math, .katex, .katex-display, .katex-mathml, .katex-html, [data-katex], .MathJax, .mwe-math-element, [class*="mwe-math-"], [data-mathml], [data-math], [data-latex]`
	candidates := root.Find(selector)
	candidates.Each(func(_ int, sel *goquery.Selection) {
		// Preserve outer supported nodes once; nested code/MathML is in the clone.
		if sel.ParentsFiltered(selector).Length() == 0 && sel.Get(0).Parent != nil {
			m.protect(sel, "")
		}
	})
	return m, nil
}
