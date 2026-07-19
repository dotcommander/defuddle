package extractors

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// conversationFootnotesHTML renders the numbered footnotes <div> for a
// conversation, or "" when there are no footnotes.
func conversationFootnotesHTML(footnotes []Footnote) string {
	footnotesHTML := ""
	if len(footnotes) > 0 {
		var footnotesBuilder strings.Builder
		footnotesBuilder.WriteString(`
			<div id="footnotes">
				<ol>`)

		for i, footnote := range footnotes {
			footnoteNum := i + 1
			footnoteHTML := fmt.Sprintf(`
						<li class="footnote" id="fn:%d">
							<p>
								<a href="%s" target="_blank">%s</a>&nbsp;<a href="#fnref:%d" class="footnote-backref">↩</a>
							</p>
						</li>`, footnoteNum, footnote.URL, footnote.Text, footnoteNum)
			footnotesBuilder.WriteString(footnoteHTML)
		}

		footnotesBuilder.WriteString(`
				</ol>
			</div>`)
		footnotesHTML = footnotesBuilder.String()
	}
	return footnotesHTML
}

// ExtractWithDefuddle extracts conversation content similar to TypeScript implementation
// TypeScript original code:
//
//	extract(): ExtractorResult {
//		const messages = this.extractMessages();
//		const metadata = this.getMetadata();
//		const footnotes = this.getFootnotes();
//		const rawContentHtml = this.createContentHtml(messages, footnotes);
//
//		// Create a temporary document to run Defuddle on our content
//		const tempDoc = document.implementation.createHTMLDocument();
//		const container = tempDoc.createElement('article');
//		container.innerHTML = rawContentHtml;
//		tempDoc.body.appendChild(container);
//
//		// Run Defuddle on our formatted content
//		const defuddled = new Defuddle(tempDoc).parse();
//		const contentHtml = defuddled.content;
//
//		return {
//			content: contentHtml,
//			contentHtml: contentHtml,
//			extractedContent: {
//				messageCount: messages.length.toString(),
//			},
//			variables: {
//				title: metadata.title || 'Conversation',
//				site: metadata.site,
//				description: metadata.description || `${metadata.site} conversation with ${messages.length} messages`,
//				wordCount: defuddled.wordCount?.toString() || '',
//			}
//		};
//	}
func (c *ConversationExtractorBase) ExtractWithDefuddle(extractor ConversationExtractor) *ExtractorResult {
	messages := extractor.ExtractMessages()
	// Cache so GetMetadata implementations can read len without a second extraction pass.
	c.cachedMessages = messages
	metadata := extractor.GetMetadata()
	footnotes := extractor.GetFootnotes()
	rawContentHTML := c.CreateContentHTML(messages, footnotes)

	contentHTML := rawContentHTML
	wordCount := ""

	// Run secondary Defuddle pass if a content processor was injected
	if c.contentProcessor != nil {
		wrappedHTML := "<article>" + rawContentHTML + "</article>"
		if result, err := c.contentProcessor(wrappedHTML); err == nil {
			contentHTML = result.Content
			wordCount = strconv.Itoa(result.WordCount)
		}
	}

	description := metadata.Description
	if description == "" {
		description = fmt.Sprintf("%s conversation with %d messages", metadata.Site, len(messages))
	}

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"messageCount": strconv.Itoa(len(messages)),
		},
		Variables: map[string]string{
			"title":       metadata.Title,
			"site":        metadata.Site,
			"description": description,
			"wordCount":   wordCount,
		},
	}
}

// conversationMetadata builds ConversationMetadata for a chat extractor, with the
// shared "<site> conversation with N messages" description template.
func conversationMetadata(site, title, url string, messageCount int) ConversationMetadata {
	return ConversationMetadata{
		Title:        title,
		Site:         site,
		URL:          url,
		MessageCount: messageCount,
		Description:  fmt.Sprintf("%s conversation with %d messages", site, messageCount),
	}
}

// canExtractFromSelection reports whether sel is non-empty (the conversation
// extractors' shared CanExtract test) and logs the decision under countLabel.
func canExtractFromSelection(sel *goquery.Selection, site, countLabel string) bool {
	canExtract := sel.Length() > 0
	slog.Debug(site+" extractor can extract check", "canExtract", canExtract, countLabel, sel.Length())
	return canExtract
}
