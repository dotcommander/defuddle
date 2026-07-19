// Package scoring provides content scoring functionality for the defuddle content extraction system.
// It implements algorithms to score DOM elements based on content quality and relevance.
package scoring

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns for content scoring.
var (
	dateRe   = regexp.MustCompile(`(?i)\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\s+\d{1,2},?\s+\d{4}\b`)
	authorRe = regexp.MustCompile(`(?i)\b(?:by|written by|author:)\s+[A-Za-z\s]+\b`)

	// Social media profile URL pattern — used to detect author bios.
	// Go regexp doesn't support lookaheads, so we match broadly here
	// and filter out intent/share URLs in the calling code.
	socialProfileRe = regexp.MustCompile(`(?i)(linkedin\.com/(in|company)/|twitter\.com/\w|x\.com/\w|facebook\.com/\w|instagram\.com/\w|threads\.net/\w|mastodon\.\w)`)

	// Date pattern for detecting standalone bylines (no leading \b because
	// textContent can concatenate adjacent elements without whitespace)
	bylineDateRe = regexp.MustCompile(`(?i)(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\s+\d{1,2}`)

	// Author attribution pattern — case-sensitive "By" + capitalized name
	bylineAuthorRe = regexp.MustCompile(`\bBy\s+[A-Z]`)

	// Sentence-ending punctuation for prose detection
	sentenceEndRe = regexp.MustCompile(`[.?!]`)
)

// Pre-compiled word-boundary regexes for navigation indicator matching.
// Using \b prevents false positives like "share" matching inside "shareholders".
var navigationIndicatorRegexes = compileNavigationRegexes()

// navigationHeadingPattern is a combined regex for heading text matching in isLikelyContent.
var navigationHeadingPattern = compileNavigationHeadingPattern()

func compileNavigationRegexes() []*regexp.Regexp {
	regexes := make([]*regexp.Regexp, len(navigationIndicators))
	for i, indicator := range navigationIndicators {
		escaped := regexp.QuoteMeta(indicator)
		escaped = strings.ReplaceAll(escaped, `\ `, `\s+`)
		regexes[i] = regexp.MustCompile(`(?i)\b` + escaped + `\b`)
	}
	return regexes
}

func compileNavigationHeadingPattern() *regexp.Regexp {
	patterns := make([]string, len(navigationIndicators))
	for i, indicator := range navigationIndicators {
		escaped := regexp.QuoteMeta(indicator)
		escaped = strings.ReplaceAll(escaped, `\ `, `\s+`)
		patterns[i] = `\b` + escaped + `\b`
	}
	return regexp.MustCompile(`(?i)` + strings.Join(patterns, "|"))
}

// Scoring bonus/penalty constants for ScoreElement.
const (
	scoreParagraphBonus       = 10.0 // per paragraph
	scoreImageDensityFactor   = 3.0  // multiplied by image/word density
	scoreRightSideBonus       = 5.0  // right-aligned elements
	scoreDateBonus            = 10.0 // element contains a recognisable date
	scoreAuthorBonus          = 10.0 // element contains an author attribution
	scoreContentClassBonus    = 15.0 // element class includes content/article/post
	scoreFootnoteBonus        = 10.0 // element contains footnote references
	scoreNestedTablePenalty   = 5.0  // per nested table
	scoreCenterCellBonus      = 10.0 // td that is a centre cell in a layout table
	scoreContentTableMinWidth = 400  // pixel width threshold for content-layout tables
	scoreLinkDensityCap       = 0.5  // cap on link-text/total-text ratio
)

// Word-count thresholds used in isLikelyContent.
const (
	contentMinWords           = 100 // sufficient alone to signal content
	contentMinWordsWithBlocks = 50  // sufficient with 2+ content blocks
	contentMinWordsSmall      = 30  // sufficient with 1+ content block
	contentMinWordsProse      = 10  // sufficient with sentence-ending punct + low link density
)

// ContentScore represents a scored element
// JavaScript original code:
//
//	export interface ContentScore {
//	  score: number;
//	  element: Element;
//	}
type ContentScore struct {
	Score   float64
	Element *goquery.Selection
}
