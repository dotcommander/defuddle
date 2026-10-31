package defuddle

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/metadata"
	"github.com/dotcommander/defuddle/internal/standardize"
)

// recoverSemanticArticle only arbitrates when the engine kept the article's
// paragraphs but introduced outside text or dropped its sole heading. It does
// not change site extractors, explicit selectors, or ambiguous article pages.
func recoverSemanticArticle(article, extracted *goquery.Selection, meta *metadata.Metadata, sourceTitle string, doc *goquery.Document, options *Options) (*goquery.Selection, bool) {
	if article == nil || extracted == nil {
		return nil, false
	}
	if article.Find("article,nav,aside,header,footer,[role=navigation],[role=complementary]").Length() != 0 || article.Find("h1").Length() != 1 {
		return nil, false
	}
	// Never restore text that the paragraph guard cannot account for. Generic
	// wrappers can contain promotions or hidden text without semantic tags.
	unaccounted := article.Clone()
	unaccounted.Find("h1,p").Remove()
	if normalizedArticleText(unaccounted.Text()) != "" {
		return nil, false
	}
	paragraphs := article.Find("p")
	if paragraphs.Length() == 0 {
		return nil, false
	}
	paragraphText := normalizedArticleText(paragraphs.Text())
	extractedText := normalizedArticleText(extracted.Text())
	heading := normalizedArticleText(article.Find("h1").First().Text())
	if paragraphText == "" || heading == "" || !strings.Contains(extractedText, paragraphText) {
		return nil, false
	}
	// Require all article paragraphs in order; any remaining engine text must
	// be the heading or outside material. A missing heading also warrants recovery.
	remainder := strings.Replace(extractedText, paragraphText, "", 1)
	remainder = strings.Replace(remainder, heading, "", 1)
	outside := strings.TrimSpace(remainder) != ""
	missingHeading := !strings.Contains(extractedText, heading)
	if !outside && !missingHeading {
		return nil, false
	}
	candidate := article.Clone()
	standardOptions := standardizeOptions(options)
	standardOptions.ProcessHeadings = true
	candidateMeta := *meta
	candidateMeta.Title = sourceTitle
	standardize.ContentWithOptions(candidate, &candidateMeta, doc, standardOptions, false)
	return candidate, true
}

func normalizedArticleText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
