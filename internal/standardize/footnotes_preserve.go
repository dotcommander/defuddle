package standardize

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
)

// Rewriting an ambiguous list would erase evidence of duplicate targets.
// Preserve this scope as authored rather than choose a definition by position.
func hasAmbiguousFootnoteTargets(root *goquery.Selection) bool {
	ids := make(map[string]int)
	root.Find("[id]").Each(func(_ int, s *goquery.Selection) { ids[s.AttrOr("id", "")]++ })
	ambiguous := false
	root.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href := s.AttrOr("href", "")
		if strings.HasPrefix(href, "#") && ids[strings.TrimPrefix(href, "#")] > 1 && (s.IsMatcher(constants.FootnoteInlineMatcher) || s.ParentsMatcher(constants.FootnoteInlineMatcher).Length() > 0) {
			ambiguous = true
		}
	})
	return ambiguous
}
