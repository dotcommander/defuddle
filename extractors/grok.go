package extractors

import (
	"log/slog"
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns for Grok extraction.
var (
	grokTitleSuffixRe = regexp.MustCompile(`\s-\s*Grok$`)
	grokLinkRe        = regexp.MustCompile(`(?i)<a\s+(?:[^>]*?\s+)?href="([^"]*)"[^>]*>(.*?)</a>`)
	grokHTTPRe        = regexp.MustCompile(`(?i)^https?://`)
)

// TypeScript original code:
// import { ConversationExtractor } from './_conversation';
// import { ConversationMessage, ConversationMetadata, Footnote } from '../types/extractors';
//
//	export class GrokExtractor extends ConversationExtractor {
//		// Note: This selector relies heavily on CSS utility classes and may break if Grok's UI changes.
//		private messageContainerSelector = '.relative.group.flex.flex-col.justify-center.w-full';
//		private messageBubbles: NodeListOf<Element> | null;
//		private footnotes: Footnote[];
//		private footnoteCounter: number;
//
//		constructor(document: Document, url: string) {
//			super(document, url);
//			this.messageBubbles = document.querySelectorAll(this.messageContainerSelector);
//			this.footnotes = [];
//			this.footnoteCounter = 0;
//		}
//	}
//
// grokMessageContainerSelector is the primary CSS selector for Grok message
// bubbles. Relies on Grok's utility-class chain and may break if Grok's UI
// changes. Held as a package-level const because it never varies per-instance.
const grokMessageContainerSelector = ".relative.group.flex.flex-col.justify-center.w-full"

// GrokExtractor handles Grok (X.AI) conversation content extraction.
type GrokExtractor struct {
	*ConversationExtractorBase
	messageBubbles  *goquery.Selection
	footnotes       []Footnote
	footnoteCounter int
}

// NewGrokExtractor creates a new Grok extractor
// TypeScript original code:
//
//	constructor(document: Document, url: string) {
//		super(document, url);
//		// Note: This selector relies heavily on CSS utility classes and may break if Grok's UI changes.
//		this.messageContainerSelector = '.relative.group.flex.flex-col.justify-center.w-full';
//		this.messageBubbles = document.querySelectorAll(this.messageContainerSelector);
//		this.footnotes = [];
//		this.footnoteCounter = 0;
//	}
func NewGrokExtractor(document *goquery.Document, urlStr string, schemaOrgData any) *GrokExtractor {
	messageBubbles := document.Find(grokMessageContainerSelector)

	// Fallback selectors if primary ones don't work
	if messageBubbles.Length() == 0 {
		slog.Debug("Grok extractor: trying fallback selectors")
		// Try generic message fallbacks first, then Grok-specific ones.
		grokFallbacks := make([]string, len(genericMessageFallbacks)+2)
		copy(grokFallbacks, genericMessageFallbacks)
		grokFallbacks[len(genericMessageFallbacks)] = "div[class*='conversation']"
		grokFallbacks[len(genericMessageFallbacks)+1] = "div[class*='bubble']"
		messageBubbles = firstMatchingSelection(document, grokFallbacks)
		if messageBubbles.Length() > 0 {
			slog.Debug("Grok extractor: found bubbles with fallback", "count", messageBubbles.Length())
		}
	}

	slog.Debug("Grok extractor initialized",
		"messageBubblesFound", messageBubbles.Length(),
		"url", urlStr,
		"selector", grokMessageContainerSelector)

	return &GrokExtractor{
		ConversationExtractorBase: NewConversationExtractorBase(document, urlStr, schemaOrgData),
		messageBubbles:            messageBubbles,
		footnotes:                 make([]Footnote, 0),
		footnoteCounter:           0,
	}
}

// CanExtract checks if the extractor can extract content
// TypeScript original code:
//
//	canExtract(): boolean {
//		return !!this.messageBubbles && this.messageBubbles.length > 0;
//	}
func (g *GrokExtractor) CanExtract() bool {
	return canExtractFromSelection(g.messageBubbles, "Grok", "messageBubblesCount")
}

// Name returns the name of the extractor
func (g *GrokExtractor) Name() string {
	return "GrokExtractor"
}

// Extract extracts the Grok conversation
// TypeScript original code:
//
//	extract(): ExtractorResult {
//		return this.extractWithDefuddle(this);
//	}
func (g *GrokExtractor) Extract() *ExtractorResult {
	slog.Debug("Grok extractor starting extraction", "url", g.url)
	return g.ExtractWithDefuddle(g)
}
