package constants

import (
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
)

// FootnoteInlineReferences are selectors for footnotes and citations
// JavaScript original code:
// export const FOOTNOTE_INLINE_REFERENCES = [
//
//	'sup.reference',
//	'cite.ltx_cite',
//	'sup[id^="fnr"]',
//	'span[id^="fnr"]',
//	'span[class*="footnote_ref"]',
//	'span.footnote-link',
//	'a.citation',
//	'a[id^="ref-link"]',
//	'a[href^="#fn"]',
//	'a[href^="#cite"]',
//	'a[href^="#reference"]',
//	'a[href^="#footnote"]',
//	'a[href^="#r"]', // Common in academic papers
//	'a[href^="#b"]', // Common for bibliography references
//	'a[href*="cite_note"]',
//	'a[href*="cite_ref"]',
//	'a.footnote-anchor', // Substack
//	'span.footnote-hovercard-target a', // Substack
//	'a[role="doc-biblioref"]', // Science.org
//	'a[id^="fnref"]',
//	'a[id^="ref-link"]', // Nature.com
//
// ].join(',');
var FootnoteInlineReferences = []string{
	"sup.reference",
	"cite.ltx_cite",
	`sup[id^="fnr"]`,
	`span[id^="fnr"]`,
	`span[class*="footnote_ref"]`,
	`span[class*="footnote-ref"]`,
	"span.footnote-link",
	"a.citation",
	`a[id^="ref-link"]`,
	`a[href^="#fn"]`,
	`a[href^="#cite"]`,
	`a[href^="#reference"]`,
	`a[href^="#footnote"]`,
	`a[href^="#r"]`, // Common in academic papers
	`a[href^="#b"]`, // Common for bibliography references
	`a[href*="cite_note"]`,
	`a[href*="cite_ref"]`,
	"a.footnote-anchor",                // Substack
	"span.footnote-hovercard-target a", // Substack
	`a[role="doc-biblioref"]`,          // Science.org
	`a[id^="fnref"]`,
	`a[id^="ref-link"]`, // Nature.com
	"sup.footnoteref",   // Wikidot
}

// FootnoteListSelectors are selectors for footnote lists
// JavaScript original code:
// export const FOOTNOTE_LIST_SELECTORS = [
//
//	'div.footnote ol',
//	'div.footnotes ol',
//	'div[role="doc-endnotes"]',
//	'div[role="doc-footnotes"]',
//	'ol.footnotes-list',
//	'ol.footnotes',
//	'ol.references',
//	'ol[class*="article-references"]',
//	'section.footnotes ol',
//	'section[role="doc-endnotes"]',
//	'section[role="doc-footnotes"]',
//	'section[role="doc-bibliography"]',
//	'ul.footnotes-list',
//	'ul.ltx_biblist',
//	'div.footnote[data-component-name="FootnoteToDOM"]' // Substack
//
// ].join(',');
var FootnoteListSelectors = []string{
	"div.footnote ol",
	"div.footnotes ol",
	`div[role="doc-endnotes"]`,
	`div[role="doc-footnotes"]`,
	"ol.footnotes-list",
	"ol.footnotes",
	"ol.references",
	`ol[class*="article-references"]`,
	"section.footnotes ol",
	`section[role="doc-endnotes"]`,
	`section[role="doc-footnotes"]`,
	`section[role="doc-bibliography"]`,
	"ul.footnotes-list",
	"ul.ltx_biblist",
	`div.footnote[data-component-name="FootnoteToDOM"]`, // Substack
	"div.footnotes-footer",                              // Wikidot
	"div.footnote-definitions",
	"#footnotes", // standardizeFootnotes output container
}

// FootnoteInlineMatcher is a single pre-compiled cascadia matcher equivalent to
// cascadia.MustCompile(strings.Join(FootnoteInlineReferences, ",")). Compiled
// once at package init to avoid per-element recompilation in the scoring hot path.
var FootnoteInlineMatcher = compileCombined(FootnoteInlineReferences)

// FootnoteListMatcher is a single pre-compiled cascadia matcher equivalent to
// cascadia.MustCompile(strings.Join(FootnoteListSelectors, ",")). Same rationale.
var FootnoteListMatcher = compileCombined(FootnoteListSelectors)

// compileCombined joins selectors with "," and compiles via cascadia.MustCompile.
// Panics at package init on invalid selectors — fails fast on typos.
// Returns cascadia.Selector (implements goquery.Matcher) for use with FindMatcher/IsMatcher.
func compileCombined(selectors []string) cascadia.Selector {
	return cascadia.MustCompile(strings.Join(selectors, ","))
}

// GetEntryPointElements returns the entry point elements slice
func GetEntryPointElements() []string {
	return EntryPointElements
}

// GetMobileWidth returns the mobile width threshold
func GetMobileWidth() int {
	return MobileWidth
}

// GetBlockElements returns the block elements slice
func GetBlockElements() []string {
	return BlockElements
}

// IsPreserveElement checks if an element should be preserved
func IsPreserveElement(tagName string) bool {
	return PreserveElements[tagName]
}

// IsInlineElement checks if an element is inline
func IsInlineElement(tagName string) bool {
	return InlineElements[tagName]
}

// GetExactSelectors returns the exact selectors slice
func GetExactSelectors() []string {
	return ExactSelectors
}

// GetTestAttributes returns the test attributes slice
func GetTestAttributes() []string {
	return TestAttributes
}

// GetPartialSelectors returns the partial selectors slice
func GetPartialSelectors() []string {
	return PartialSelectors
}

// partialSelectorRegex is a pre-compiled combined regex for O(n) partial selector matching.
// Built once at package init from all partial selectors.
var partialSelectorRegex = compilePartialSelectorRegex()

func compilePartialSelectorRegex() *regexp.Regexp {
	escaped := make([]string, len(PartialSelectors))
	for i, s := range PartialSelectors {
		escaped[i] = regexp.QuoteMeta(strings.ToLower(s))
	}
	return regexp.MustCompile(`(?i)` + strings.Join(escaped, "|"))
}

// GetPartialSelectorRegex returns the pre-compiled combined regex for partial selector matching.
func GetPartialSelectorRegex() *regexp.Regexp {
	return partialSelectorRegex
}

// GetFootnoteInlineReferences returns the footnote inline reference selectors
func GetFootnoteInlineReferences() []string {
	return FootnoteInlineReferences
}

// GetFootnoteListSelectors returns the footnote list selectors
func GetFootnoteListSelectors() []string {
	return FootnoteListSelectors
}
