package extractors

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Precompiled regex for performance
var paragraphRegex = regexp.MustCompile(`<p[^>]*>[\s\S]*?</p>`)

// ContentProcessResult holds the result of running a secondary content processing pass.
type ContentProcessResult struct {
	Content   string
	WordCount int
}

// ContentProcessorFunc is a function that processes raw HTML through a secondary
// Defuddle pass, returning cleaned content and word count. This breaks the import
// cycle between the root defuddle package and the extractors package.
type ContentProcessorFunc func(html string) (*ContentProcessResult, error)

// ContentProcessorSetter is implemented by extractors that support a secondary
// Defuddle processing pass on their output.
type ContentProcessorSetter interface {
	SetContentProcessor(fn ContentProcessorFunc)
}

// ConversationMessage represents a single message in a conversation
// Corresponding to TypeScript interface ConversationMessage
type ConversationMessage struct {
	Author    string         `json:"author"`
	Content   string         `json:"content"`
	Timestamp string         `json:"timestamp,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// ConversationMetadata represents metadata about the conversation
// Corresponding to TypeScript interface ConversationMetadata
type ConversationMetadata struct {
	Title        string `json:"title"`
	Site         string `json:"site"`
	URL          string `json:"url"`
	MessageCount int    `json:"messageCount"`
	Description  string `json:"description"`
}

// Footnote represents a footnote in the conversation
// Corresponding to TypeScript interface Footnote
type Footnote struct {
	URL  string `json:"url"`
	Text string `json:"text"`
}

// ConversationExtractor defines the interface for conversation extractors
// TypeScript original code:
//
//	export abstract class ConversationExtractor extends BaseExtractor {
//		protected abstract extractMessages(): ConversationMessage[];
//		protected abstract getMetadata(): ConversationMetadata;
//		protected getFootnotes(): Footnote[] {
//			return [];
//		}
//	}
type ConversationExtractor interface {
	BaseExtractor
	ExtractMessages() []ConversationMessage
	GetMetadata() ConversationMetadata
	GetFootnotes() []Footnote
}

// ConversationExtractorBase provides common functionality for conversation extractors
// Implementation corresponding to TypeScript ConversationExtractor abstract class
type ConversationExtractorBase struct {
	*ExtractorBase
	contentProcessor ContentProcessorFunc
	cachedMessages   []ConversationMessage // set by ExtractWithDefuddle to avoid double extraction
}

// NewConversationExtractorBase creates a new conversation extractor base
// TypeScript original code:
//
//	constructor(document: Document, url: string, schemaOrgData?: any) {
//		super(document, url, schemaOrgData);
//	}
func NewConversationExtractorBase(document *goquery.Document, url string, schemaOrgData any) *ConversationExtractorBase {
	return &ConversationExtractorBase{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
}

// SetContentProcessor sets the function used for secondary Defuddle processing.
func (c *ConversationExtractorBase) SetContentProcessor(fn ContentProcessorFunc) {
	c.contentProcessor = fn
}

// TruncateTitle truncates s to max runes, appending "..." if truncated.
// Uses []rune conversion to avoid splitting multi-byte characters.
func TruncateTitle(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// CreateContentHTML creates formatted HTML content from messages and footnotes
// TypeScript original code:
//
//	protected createContentHtml(messages: ConversationMessage[], footnotes: Footnote[]): string {
//		const messagesHtml = messages.map((message, index) => {
//			const timestampHtml = message.timestamp ?
//				`<div class="message-timestamp">${message.timestamp}</div>` : '';
//
//			// Check if content already has paragraph tags
//			const hasParagraphs = /<p[^>]*>[\s\S]*?<\/p>/i.test(message.content);
//			const contentHtml = hasParagraphs ? message.content : `<p>${message.content}</p>`;
//
//			// Add metadata to data attributes
//			const dataAttributes = message.metadata ?
//				Object.entries(message.metadata)
//					.map(([key, value]) => `data-${key}="${value}"`)
//					.join(' ') : '';
//
//			return `
//			<div class="message message-${message.author.toLowerCase()}" ${dataAttributes}>
//				<div class="message-header">
//					<p class="message-author"><strong>${message.author}</strong></p>
//					${timestampHtml}
//				</div>
//				<div class="message-content">
//					${contentHtml}
//				</div>
//			</div>${index < messages.length - 1 ? '\n<hr>' : ''}`;
//		}).join('\n').trim();
//
//		// Add footnotes section if we have any
//		const footnotesHtml = footnotes.length > 0 ? `
//			<div id="footnotes">
//				<ol>
//					${footnotes.map((footnote, index) => `
//						<li class="footnote" id="fn:${index + 1}">
//							<p>
//								<a href="${footnote.url}" target="_blank">${footnote.text}</a>&nbsp;<a href="#fnref:${index + 1}" class="footnote-backref">↩</a>
//							</p>
//						</li>
//					`).join('')}
//				</ol>
//			</div>` : '';
//
//		return `${messagesHtml}\n${footnotesHtml}`.trim();
//	}
func (c *ConversationExtractorBase) CreateContentHTML(messages []ConversationMessage, footnotes []Footnote) string {
	var messagesHTML strings.Builder

	for i, message := range messages {
		timestampHTML := ""
		if message.Timestamp != "" {
			timestampHTML = fmt.Sprintf(`<div class="message-timestamp">%s</div>`, message.Timestamp)
		}

		// Check if content already has paragraph tags
		hasParagraphs := paragraphRegex.MatchString(message.Content)
		contentHTML := message.Content
		if !hasParagraphs {
			contentHTML = fmt.Sprintf("<p>%s</p>", message.Content)
		}

		// Add metadata to data attributes
		var dataAttributes strings.Builder
		if message.Metadata != nil {
			for key, value := range message.Metadata {
				fmt.Fprintf(&dataAttributes, ` data-%s="%v"`, key, value)
			}
		}

		authorLower := strings.ToLower(message.Author)
		messageHTML := fmt.Sprintf(`
			<div class="message message-%s"%s>
				<div class="message-header">
					<p class="message-author"><strong>%s</strong></p>
					%s
				</div>
				<div class="message-content">
					%s
				</div>
			</div>`, authorLower, dataAttributes.String(), message.Author, timestampHTML, contentHTML)

		messagesHTML.WriteString(messageHTML)

		if i < len(messages)-1 {
			messagesHTML.WriteString("\n<hr>")
		}
	}

	// Add footnotes section if we have any
	footnotesHTML := conversationFootnotesHTML(footnotes)

	result := messagesHTML.String()
	if footnotesHTML != "" {
		result += "\n" + footnotesHTML
	}

	return strings.TrimSpace(result)
}
