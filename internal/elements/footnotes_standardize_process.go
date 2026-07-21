package elements

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// StandardizeFootnotes rewrites all inline references and footnote definitions
// into the canonical form: <sup id="fnref:N"><a href="#fn:N">N</a></sup> for
// references and <div id="footnotes"><ol><li id="fn:N">…</li></ol></div> for
// definitions. It is the Go port of the TypeScript standardizeFootnotes method.
// TypeScript original code:
//
//	standardizeFootnotes(element: any) {
//	  const footnotes = this.collectFootnotes(element);
//	  const refs = element.querySelectorAll(FOOTNOTE_INLINE_REFERENCES);
//	  const supGroups = new Map();
//	  refs.forEach(el => { ... supGroups / replaceWith ... });
//	  supGroups.forEach((refs, container) => { ... });
//	  // rebuild list, remove originals, append new div#footnotes
//	}
func (p *FootnoteProcessor) StandardizeFootnotes(scope *goquery.Selection) {
	footnotes := p.collectFootnoteDefinitions(scope)
	if len(footnotes) == 0 {
		return
	}

	// supGroups maps a *html.Node (shared <sup> container) to grouped refs.
	// We store the Selection alongside to avoid needing a FindNodes API.
	type supRef struct {
		number int
		refID  string
	}
	type supGroup struct {
		sel  *goquery.Selection
		refs []supRef
	}
	supGroups := make(map[*html.Node]*supGroup)
	supOrder := make([]*html.Node, 0) // preserve insertion order

	scope.Find(FootnoteInlineReferences).Each(func(_ int, el *goquery.Selection) {
		node := el.Get(0)
		if node == nil {
			return
		}

		footnoteID := p.extractInlineFootnoteID(el)
		if footnoteID == "" {
			return
		}

		// Find matching collected footnote by originalID
		var matchNum int
		var matchEntry *footnoteEntry
		for num, entry := range footnotes {
			if entry.originalID == strings.ToLower(footnoteID) {
				matchNum = num
				matchEntry = entry
				break
			}
		}
		if matchEntry == nil {
			return
		}

		// Assign ref ID (e.g. fnref:1 or fnref:1-2 for multi-reference)
		var refID string
		if len(matchEntry.refs) > 0 {
			refID = fmt.Sprintf("fnref:%d-%d", matchNum, len(matchEntry.refs)+1)
		} else {
			refID = fmt.Sprintf("fnref:%d", matchNum)
		}
		matchEntry.refs = append(matchEntry.refs, refID)

		container := p.findOuterFootnoteContainer(el)
		containerNode := container.Get(0)

		if goquery.NodeName(container) == "sup" {
			// Group under the shared sup node
			if _, exists := supGroups[containerNode]; !exists {
				supOrder = append(supOrder, containerNode)
				supGroups[containerNode] = &supGroup{sel: container}
			}
			supGroups[containerNode].refs = append(supGroups[containerNode].refs, supRef{matchNum, refID})
		} else {
			// Replace container directly
			container.ReplaceWithHtml(p.createFootnoteRefHTML(matchNum, refID))
		}
	})

	// Replace each grouped sup with individual <sup> elements
	for _, containerNode := range supOrder {
		g := supGroups[containerNode]
		var b strings.Builder
		for _, r := range g.refs {
			b.WriteString(p.createFootnoteRefHTML(r.number, r.refID))
		}
		g.sel.ReplaceWithHtml(b.String())
	}

	// Remove original footnote lists before appending new ones
	scope.Find(FootnoteListSelectors).Remove()

	// Build new canonical list
	var listHTML strings.Builder
	listHTML.WriteString(`<div id="footnotes"><ol>`)
	for i := 1; i <= len(footnotes); i++ {
		entry, ok := footnotes[i]
		if !ok {
			continue
		}
		listHTML.WriteString(p.createFootnoteItemHTML(i, entry.content, entry.refs))
	}
	listHTML.WriteString(`</ol></div>`)

	scope.AppendHtml(listHTML.String())
}
