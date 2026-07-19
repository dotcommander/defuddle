package removals

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns for content-pattern removal.
var (
	contentDatePattern     = regexp.MustCompile(`(?i)(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\s+\d{1,2}`)
	contentReadTimePattern = regexp.MustCompile(`(?i)\d+\s*min(?:ute)?s?\s+read\b`)
	bylineUppercasePattern = regexp.MustCompile(`^\p{Lu}`)
	startsByPattern        = regexp.MustCompile(`(?i)^by\s+\S`)
	metadataLabelPattern   = regexp.MustCompile(`(?i)^(?:date|published|updated|posted|from|to|subject)\s*:`)

	boilerplatePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^This (?:article|story|piece) (?:appeared|was published|originally appeared) in\b`),
		regexp.MustCompile(`(?i)^A version of this (?:article|story) (?:appeared|was published) in\b`),
		regexp.MustCompile(`(?i)^Originally (?:published|appeared) (?:in|on|at)\b`),
		regexp.MustCompile(`(?i)^Any re-?use permitted\b`),
		regexp.MustCompile(`(?i)^©\s*(?:Copyright\s+)?\d{4}`),
		regexp.MustCompile(`(?i)^Comments?$`),
		regexp.MustCompile(`(?i)^Leave a (?:comment|reply)$`),
	}

	newsletterPattern = regexp.MustCompile(
		`(?i)\bsubscribe\b[\s\S]{0,40}\bnewsletter\b|\bnewsletter\b[\s\S]{0,40}\bsubscribe\b|\bsign[- ]up\b[\s\S]{0,80}\b(?:newsletter|email alert)`,
	)

	relatedHeadingPattern = regexp.MustCompile(
		`(?i)^(?:related (?:posts?|articles?|content|stories|reads?|reading)|you (?:might|may|could) (?:also )?(?:like|enjoy|be interested in)|read (?:next|more|also)|further reading|see also|more (?:from|articles?|posts?|like this)|more to (?:read|explore)|about (?:the )?author)$`,
	)

	breadcrumbLinkPattern = regexp.MustCompile(`^/[a-zA-Z0-9_-]+/?$`)
	parentIndexPattern    = regexp.MustCompile(`(?i)^index\.(html?|php)$`)
	camelBoundary         = regexp.MustCompile(`([a-z])([A-Z])`)

	// Metadata strip patterns — date/number components.
	metadataStripMonth   = regexp.MustCompile(`(?i)\b(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|Jun(?:e)?|Jul(?:y)?|Aug(?:ust)?|Sep(?:t(?:ember)?)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)\b`)
	metadataStripWeekday = regexp.MustCompile(`(?i)\b(?:Mon(?:day)?|Tue(?:s(?:day)?)?|Wed(?:nesday)?|Thu(?:rs(?:day)?)?|Fri(?:day)?|Sat(?:urday)?|Sun(?:day)?)\b`)
	metadataStripNumber  = regexp.MustCompile(`\b\d+(?:st|nd|rd|th)?\b`)

	// Read-time strip patterns (expect empty residual after applying all).
	readTimeStripMin   = regexp.MustCompile(`(?i)\bmin(?:ute)?s?\b`)
	readTimeStripRead  = regexp.MustCompile(`(?i)\bread\b`)
	readTimeStripPunct = regexp.MustCompile(`[/|·•—–\-,.\s]+`)

	// Byline strip patterns (preserve spaces so name words can be identified).
	bylineStripBy    = regexp.MustCompile(`(?i)\bby\b`)
	bylineStripPunct = regexp.MustCompile(`[/|·•—–\-,]+`)

	// Sentence punctuation patterns used across multiple removal functions.
	sentencePunctRe      = regexp.MustCompile(`[.!?]`)
	sentencePunctEndRe   = regexp.MustCompile(`[.!?]$`)
	sentencePunctSpaceRe = regexp.MustCompile(`[.!?]\s`)
)

// RemoveByContentPattern detects and removes boilerplate, metadata, and
// navigational fragments from mainContent. It is a faithful port of the
// TypeScript removeByContentPattern function.
func RemoveByContentPattern(mainContent *goquery.Selection, _ *goquery.Document, debug bool, pageURL string) {
	mainNode := mainContent.Nodes[0]

	removeBreadcrumbList(mainContent, mainNode, debug)
	removePromotionalBanners(mainContent, debug)
	removeHeroHeader(mainContent, mainNode, debug)
	removeSinglePassMetadata(mainContent, mainNode, debug)
	removeStandaloneTimeElements(mainContent, debug)
	removeBlogMetadataLists(mainContent, debug)
	removeSectionBreadcrumbs(mainContent, pageURL, debug)
	removeTrailingExternalLinkLists(mainContent, pageURL, debug)
	removeTrailingRelatedPostsBlock(mainContent, mainNode, debug)
	removeTrailingThinSections(mainContent, debug)
	removeBoilerplateSentences(mainContent, mainNode, debug)
	removeRelatedHeadingSections(mainContent, debug)
	removeRelatedPostCardGrids(mainContent, mainNode, debug)
	removeNewsletterSections(mainContent, mainNode, debug)
}
