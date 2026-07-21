package defuddle

import (
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/constants"
	"github.com/dotcommander/defuddle/internal/scoring"
	"github.com/dotcommander/defuddle/internal/standardize"
)

// removeBySelector removes elements by exact and partial selectors.
// mainContent, footnote lists, and heading elements/anchors are protected from removal.
// mainContent protection applies in both branches via scoring.IsProtectedNode; footnote-list
// and heading protections apply only in the removePartial branch.
func (d *Defuddle) removeBySelector(doc *goquery.Document, removeExact, removePartial bool, mainContent *goquery.Selection) {
	if removeExact {
		d.removeExactSelectors(doc, mainContent)
	}
	if removePartial {
		d.removePartialSelectors(doc, mainContent)
	}
}

// removeExactSelectors removes elements matching the exact removal selectors,
// preserving protected (main-content) nodes.
func (d *Defuddle) removeExactSelectors(doc *goquery.Document, mainContent *goquery.Selection) {
	exactSelectors := constants.GetExactSelectors()
	for _, selector := range exactSelectors {
		doc.Find(selector).Each(func(_ int, el *goquery.Selection) {
			if scoring.IsProtectedNode(el, mainContent) {
				return
			}
			el.Remove()
		})
	}
}

// removePartialSelectors removes elements whose test-attribute values match the
// partial selector regex, preserving protected nodes, footnote lists, and headings.
func (d *Defuddle) removePartialSelectors(doc *goquery.Document, mainContent *goquery.Selection) {
	testAttributes := constants.GetTestAttributes()
	partialRegex := constants.GetPartialSelectorRegex()

	// Only query elements that have at least one test attribute
	attrSelector := make([]string, len(testAttributes))
	for i, attr := range testAttributes {
		attrSelector[i] = "[" + attr + "]"
	}
	combinedSelector := strings.Join(attrSelector, ",")

	doc.Find(combinedSelector).Each(func(_ int, element *goquery.Selection) {
		if isProtectedFromPartialRemoval(element, mainContent) {
			return
		}

		// Combine all test attribute values into one string for single regex test
		var combined strings.Builder
		for _, attr := range testAttributes {
			if value, exists := element.Attr(attr); exists && value != "" {
				combined.WriteString(value)
				combined.WriteByte(' ')
			}
		}
		attrs := strings.ToLower(combined.String())
		if strings.TrimSpace(attrs) == "" {
			return
		}

		if partialRegex.MatchString(attrs) {
			element.Remove()
		}
	})
}

// isProtectedFromPartialRemoval reports whether element must be kept despite
// matching a test-attribute selector: protected content nodes, footnote lists
// (or their parents), headings, and anchors inside headings.
func isProtectedFromPartialRemoval(element, mainContent *goquery.Selection) bool {
	if scoring.IsProtectedNode(element, mainContent) {
		return true
	}
	// Protect footnote lists and their parents (element itself or any descendant matches)
	if element.IsMatcher(constants.FootnoteListMatcher) ||
		element.FindMatcher(constants.FootnoteListMatcher).Length() > 0 {
		return true
	}
	// Skip heading elements — their IDs often match partial selectors
	if slices.Contains(headingTags, goquery.NodeName(element)) {
		return true
	}
	// Skip anchor links inside headings
	return element.Closest(headingSelector).Length() > 0
}

// mergeOptions merges override options with instance options and defaults.
// Mirrors the TypeScript spread pattern:
//
//	const options = { removeExactSelectors: true, ...this.options, ...overrideOptions };
//
// Defaults for *bool fields (all true) are applied at use sites via BoolDefault(field, true).
// nil *bool means "use default"; non-nil means "explicitly set by caller".
func (d *Defuddle) mergeOptions(overrideOptions *Options) *Options {
	options := &Options{}

	// Apply instance options then override options (mirrors JS spread order)
	applyOptions(options, d.options)
	applyOptions(options, overrideOptions)

	return options
}

// applyOptions overlays src onto dst.
// Plain bools and strings are always copied (false/empty is meaningful).
// *bool fields are only copied when non-nil — nil means "not set, use default".
// Empty strings for URL/ContentSelector are skipped to avoid clearing set values.
func applyOptions(dst, src *Options) {
	if src == nil {
		return
	}
	dst.Debug = src.Debug
	if src.URL != "" {
		dst.URL = src.URL
	}
	dst.Markdown = src.Markdown
	dst.SeparateMarkdown = src.SeparateMarkdown
	// Pointer bools: only copy when explicitly set (non-nil)
	if src.RemoveExactSelectors != nil {
		dst.RemoveExactSelectors = src.RemoveExactSelectors
	}
	if src.RemovePartialSelectors != nil {
		dst.RemovePartialSelectors = src.RemovePartialSelectors
	}
	dst.RemoveImages = src.RemoveImages
	if src.RemoveHiddenElements != nil {
		dst.RemoveHiddenElements = src.RemoveHiddenElements
	}
	if src.RemoveLowScoring != nil {
		dst.RemoveLowScoring = src.RemoveLowScoring
	}
	if src.RemoveContentPatterns != nil {
		dst.RemoveContentPatterns = src.RemoveContentPatterns
	}
	if src.ContentSelector != "" {
		dst.ContentSelector = src.ContentSelector
	}
	dst.ProcessCode = src.ProcessCode
	dst.ProcessImages = src.ProcessImages
	dst.ProcessHeadings = src.ProcessHeadings
	dst.ProcessMath = src.ProcessMath
	dst.ProcessFootnotes = src.ProcessFootnotes
	dst.ProcessRoles = src.ProcessRoles
	if src.CodeOptions != nil {
		dst.CodeOptions = src.CodeOptions
	}
	if src.ImageOptions != nil {
		dst.ImageOptions = src.ImageOptions
	}
	if src.HeadingOptions != nil {
		dst.HeadingOptions = src.HeadingOptions
	}
	if src.MathOptions != nil {
		dst.MathOptions = src.MathOptions
	}
	if src.FootnoteOptions != nil {
		dst.FootnoteOptions = src.FootnoteOptions
	}
	if src.RoleOptions != nil {
		dst.RoleOptions = src.RoleOptions
	}
	if src.Client != nil {
		dst.Client = src.Client
	}
	if src.MaxConcurrency > 0 {
		dst.MaxConcurrency = src.MaxConcurrency
	}
}

func standardizeOptions(options *Options) standardize.Options {
	if options == nil {
		return standardize.Options{}
	}
	return standardize.Options{
		ProcessCode:      options.ProcessCode,
		ProcessImages:    options.ProcessImages,
		ProcessHeadings:  options.ProcessHeadings,
		ProcessMath:      options.ProcessMath,
		ProcessFootnotes: options.ProcessFootnotes,
		ProcessRoles:     options.ProcessRoles,
	}
}
