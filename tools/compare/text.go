package main

import (
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type structure struct {
	Headings   int `json:"headings"`
	Paragraphs int `json:"paragraphs"`
	Links      int `json:"links"`
	Images     int `json:"images"`
	CodeBlocks int `json:"code_blocks"`
	Tables     int `json:"tables"`
}

func normalizeHTML(content string) (string, structure, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", structure{}, err
	}
	doc.Find("script,style,noscript,template").Remove()
	s := structure{doc.Find("h1,h2,h3,h4,h5,h6").Length(), doc.Find("p").Length(), doc.Find("a").Length(), doc.Find("img").Length(), doc.Find("pre").Length(), doc.Find("table").Length()}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		block := n.Type == html.ElementNode && isBlock(n.Data)
		if block {
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if block {
			b.WriteByte(' ')
		}
	}
	for _, n := range doc.Nodes {
		walk(n)
	}
	return strings.Join(strings.Fields(b.String()), " "), s, nil
}

func isBlock(tag string) bool {
	switch tag {
	case "p", "div", "article", "section", "header", "footer", "aside", "nav", "h1", "h2", "h3", "h4", "h5", "h6", "li", "ul", "ol", "pre", "br", "hr", "td", "th", "tr", "table", "blockquote", "figure", "figcaption":
		return true
	}
	return false
}

func tokenSet(text string) map[string]bool {
	tokens := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	set := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		set[token] = true
	}
	return set
}

// Agreement ignores order and frequency; it cannot establish extraction quality.
func jaccard(a, b string) float64 {
	x, y := tokenSet(a), tokenSet(b)
	union, intersection := len(x), 0
	for token := range y {
		if x[token] {
			intersection++
		} else {
			union++
		}
	}
	if union == 0 {
		return 1
	}
	return float64(intersection) / float64(union)
}
