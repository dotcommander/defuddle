package defuddle

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
	"github.com/dotcommander/defuddle/internal/urlutil"
	"golang.org/x/net/html"
)

func (m *richManifest) protectFootnotes(root *goquery.Selection) {
	ids := make(map[string][]*html.Node)
	root.Find("[id]").Each(func(_ int, s *goquery.Selection) {
		id := s.AttrOr("id", "")
		if id != "" {
			ids[id] = append(ids[id], s.Get(0))
		}
	})
	type reference struct {
		selection *goquery.Selection
		id        string
	}
	var refs []reference
	root.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href := s.AttrOr("href", "")
		if !strings.HasPrefix(href, "#") || len(href) < 2 {
			return
		}
		id := href[1:]
		nodes := ids[id]
		if len(nodes) != 1 {
			return
		}
		def := goquery.NewDocumentFromNode(nodes[0]).Selection
		supported := s.IsMatcher(constants.FootnoteInlineMatcher) || s.ParentsMatcher(constants.FootnoteInlineMatcher).Length() > 0 || def.ParentsMatcher(constants.FootnoteListMatcher).Length() > 0 || def.IsMatcher(constants.FootnoteListMatcher)
		if !supported || def.Get(0) == s.Get(0) || s.ParentsFiltered("li").IsSelection(def) {
			return
		}
		if goquery.NodeName(def) != "li" && def.ParentsMatcher(constants.FootnoteListMatcher).Length() == 0 && def.AttrOr("role", "") != "doc-footnote" && !def.IsMatcher(constants.FootnoteListMatcher) {
			return
		}
		refs = append(refs, reference{s, id})
		if _, ok := m.definitions[id]; !ok {
			clone := cloneRichNode(nodes[0])
			urlutil.SanitizeUnsafe(goquery.NewDocumentFromNode(clone).Selection)
			m.definitions[id] = clone
		}
	})
	// Saved definitions contain already-processed code and math; never project them.
	for id := range m.definitions {
		n := ids[id][0]
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}
	for _, ref := range refs {
		if ref.selection.Get(0).Parent != nil {
			// Keep the local backlink target on the supported reference wrapper.
			// Protect a shared wrapper only when it contains this one reference;
			// multiple references must remain separate occurrences.
			wrapper := ref.selection.Parent()
			if goquery.NodeName(wrapper) == "sup" && wrapper.Find("a[href]").Length() == 1 {
				m.protect(wrapper, ref.id)
			} else {
				m.protect(ref.selection, ref.id)
			}
		}
	}
}
