package extractors

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// lwnDateRe matches "Posted Month DD, YYYY" in byline text.
var lwnDateRe = regexp.MustCompile(`(?i)Posted\s+(\w+\s+\d{1,2},\s+\d{4})`)

// lwnBylineAuthorRe extracts the first word after "by " in a byline.
var lwnBylineAuthorRe = regexp.MustCompile(`(?i)by\s+(\w+)`)

// LWNExtractor handles LWN.net article content extraction.
//
// TypeScript original:
//
//	export class LwnExtractor extends BaseExtractor {
//	  canExtract(): boolean {
//	    return !!this.document.querySelector('.PageHeadline') &&
//	      !!this.document.querySelector('.ArticleText');
//	  }
//	  extract(): ExtractorResult { ... }
//	}
type LWNExtractor struct {
	*ExtractorBase
}

// NewLWNExtractor creates a new LWN extractor.
func NewLWNExtractor(document *goquery.Document, url string, schemaOrgData any) *LWNExtractor {
	return &LWNExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
}

// Name returns the extractor identifier.
func (e *LWNExtractor) Name() string { return "LWNExtractor" }

// CanExtract returns true when the LWN page headline and article body are present.
func (e *LWNExtractor) CanExtract() bool {
	doc := e.GetDocument()
	return doc.Find(".PageHeadline").Length() > 0 && doc.Find(".ArticleText").Length() > 0
}

// Extract returns the structured content for an LWN article.
func (e *LWNExtractor) Extract() *ExtractorResult {
	doc := e.GetDocument()

	main := doc.Find(".ArticleText main").First()
	articleContent := ""
	comments := ""
	if main.Length() > 0 {
		articleContent = e.getArticleContent(main)
		comments = e.extractComments(main)
	}
	contentHTML := buildContentHtml("lwn", articleContent, comments)

	byline := strings.TrimSpace(doc.Find(".Byline").First().Text())

	title := strings.TrimSpace(doc.Find(".PageHeadline h1").First().Text())
	author := ""
	if m := lwnBylineAuthorRe.FindStringSubmatch(byline); m != nil {
		author = m[1]
	}
	description, _ := doc.Find(`meta[property="og:description"]`).First().Attr("content")
	published := e.parseDate(byline)

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		Variables: map[string]string{
			"title":       title,
			"author":      author,
			"site":        "LWN.net",
			"published":   published,
			"description": description,
		},
	}
}

// parseDate extracts a publication date from a byline string.
// Returns "YYYY-MM-DD" or empty string on failure.
func (e *LWNExtractor) parseDate(text string) string {
	m := lwnDateRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	// normalise via time.Parse isn't available without importing time — use
	// a manual mapping of the three-letter month prefix to a two-digit number.
	dateStr := m[1]
	months := map[string]string{
		"January": "01", "February": "02", "March": "03", "April": "04",
		"May": "05", "June": "06", "July": "07", "August": "08",
		"September": "09", "October": "10", "November": "11", "December": "12",
	}
	// Try each month name.
	for name, num := range months {
		if strings.HasPrefix(strings.ToLower(dateStr), strings.ToLower(name)) {
			rest := strings.TrimPrefix(dateStr, name+" ")
			parts := strings.SplitN(rest, ", ", 2)
			if len(parts) == 2 {
				var dayNum int
				if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &dayNum); err == nil {
					year := strings.TrimSpace(parts[1])
					return fmt.Sprintf("%s-%s-%02d", year, num, dayNum)
				}
			}
		}
	}
	return ""
}

// getArticleContent clones the main element, strips comment boxes, reply buttons,
// comment anchors, and trailing <hr>/<br clear> separators, then serialises to HTML.
func (e *LWNExtractor) getArticleContent(main *goquery.Selection) string {
	clone := main.Clone()

	// Strip comment input boxes, forms, and named comment anchors.
	clone.Find(`details.CommentBox, form, a[name^="Comm"]`).Remove()

	// Remove trailing <hr> and <br clear="all"> that separate article from comments.
	for {
		last := clone.Children().Last()
		if last.Length() == 0 {
			break
		}
		tag := goquery.NodeName(last)
		_, hasClear := last.Attr("clear")
		if tag == "hr" || (tag == "br" && hasClear) {
			last.Remove()
		} else {
			break
		}
	}

	h, _ := clone.Html()
	return strings.TrimSpace(h)
}

// extractComments collects all CommentBox <details> elements and converts them
// to the shared CommentData format for renderCommentThread.
func (e *LWNExtractor) extractComments(main *goquery.Selection) string {
	var commentData []CommentData

	main.Find("details.CommentBox").Each(func(_ int, box *goquery.Selection) {
		depth := e.getCommentDepth(box, main)
		if data := e.extractCommentData(box, depth); data != nil {
			commentData = append(commentData, *data)
		}
	})

	if len(commentData) == 0 {
		return ""
	}
	return renderCommentThread(commentData)
}

// getCommentDepth counts ancestor CommentBox <details> elements up to root.
func (e *LWNExtractor) getCommentDepth(el *goquery.Selection, root *goquery.Selection) int {
	depth := 0
	parent := el.Parent()
	rootNode := root.Get(0)
	for parent.Length() > 0 && parent.Get(0) != rootNode {
		if goquery.NodeName(parent) == "details" && parent.HasClass("CommentBox") {
			depth++
		}
		parent = parent.Parent()
	}
	return depth
}

// extractCommentData extracts metadata and body HTML from a single CommentBox.
func (e *LWNExtractor) extractCommentData(box *goquery.Selection, depth int) *CommentData {
	poster := box.Find("summary .CommentPoster").First()
	if poster.Length() == 0 {
		return nil
	}

	author := strings.TrimSpace(poster.Find("b").First().Text())
	linkEl := poster.Find(`a[href^="/Articles/"]`).First()
	articlePath, _ := linkEl.Attr("href")
	url := ""
	if articlePath != "" {
		url = "https://lwn.net" + articlePath
	}

	byline := poster.Text()
	date := e.parseDate(byline)

	title := strings.TrimSpace(box.Find("summary h3.CommentTitle").First().Text())

	// Only use title if distinct from the parent comment's title.
	parentTitle := ""
	if parentBox := box.Parent().Closest("details.CommentBox"); parentBox.Length() > 0 {
		parentTitle = strings.TrimSpace(parentBox.Find("summary h3.CommentTitle").First().Text())
	}
	uniqueTitle := ""
	if title != "" && title != parentTitle {
		uniqueTitle = title
	}

	content := e.getCommentContent(box, uniqueTitle)

	return &CommentData{
		Author:  author,
		Date:    date,
		Content: content,
		Depth:   depth,
		URL:     url,
	}
}

// getCommentContent builds the HTML body for a comment box.
func (e *LWNExtractor) getCommentContent(box *goquery.Selection, title string) string {
	var b strings.Builder

	if title != "" {
		fmt.Fprintf(&b, "<p><strong>%s</strong></p>", html.EscapeString(title))
	}

	formatted := box.Find(".FormattedComment").First()
	if formatted.Length() > 0 {
		h, _ := formatted.Html()
		b.WriteString(h)
	} else {
		// Collect direct content nodes, skipping structural elements.
		box.Children().Each(func(_ int, child *goquery.Selection) {
			tag := goquery.NodeName(child)
			switch tag {
			case "summary", "details", "form":
				return
			}
			if child.HasClass("CommentReplyButton") {
				return
			}
			if tag == "a" {
				if name, _ := child.Attr("name"); strings.HasPrefix(name, "CommAnchor") {
					return
				}
			}
			if tag == "p" && strings.TrimSpace(child.Text()) == "" {
				return
			}
			h, _ := goquery.OuterHtml(child)
			b.WriteString(h)
		})
	}

	return b.String()
}
