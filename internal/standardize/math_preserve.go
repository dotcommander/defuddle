package standardize

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// Formula scripts are data, but cannot be restored as executable elements.
// Carry their exact text in safe MathML without invoking math normalization.
func preserveMathScriptText(root *goquery.Selection) {
	root.Find("script[type]").Each(func(_ int, s *goquery.Selection) {
		if !strings.HasPrefix(strings.ToLower(s.AttrOr("type", "")), "math/") {
			return
		}
		formula := s.Text()
		math := &html.Node{Type: html.ElementNode, Data: "math", Namespace: "math", Attr: []html.Attribute{{Key: "data-latex", Val: formula}}}
		text := &html.Node{Type: html.ElementNode, Data: "mtext", Namespace: "math"}
		text.AppendChild(&html.Node{Type: html.TextNode, Data: formula})
		math.AppendChild(text)
		s.ReplaceWithNodes(math)
	})
}
