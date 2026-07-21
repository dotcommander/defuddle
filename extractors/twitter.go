package extractors

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// Pre-compiled regex patterns for Twitter extraction.
var (
	twitterImageNameRe = regexp.MustCompile(`&name=\w+$`)
	twitterStatusRe    = regexp.MustCompile(`status/(\d+)`)
)

// TwitterExtractor handles Twitter/X content extraction
// TypeScript original code:
// import { BaseExtractor } from './_base';
// import { ExtractorResult } from '../types/extractors';
//
//	export class TwitterExtractor extends BaseExtractor {
//		private mainTweet: Element | null = null;
//		private threadTweets: Element[] = [];
//
//		constructor(document: Document, url: string) {
//			super(document, url);
//
//			// Get all tweets from the timeline
//			const timeline = document.querySelector('[aria-label="Timeline: Conversation"]');
//			if (!timeline) {
//				// Try to find a single tweet if not in timeline view
//				const singleTweet = document.querySelector('article[data-testid="tweet"]');
//				if (singleTweet) {
//					this.mainTweet = singleTweet;
//				}
//				return;
//			}
//
//			// Get all tweets before any section with "Discover more" or similar headings
//			const allTweets = Array.from(timeline.querySelectorAll('article[data-testid="tweet"]'));
//			const firstSection = timeline.querySelector('section, h2')?.parentElement;
//
//			if (firstSection) {
//				// Filter out tweets that appear after the first section
//				allTweets.forEach((tweet, index) => {
//					if (firstSection.compareDocumentPosition(tweet) & Node.DOCUMENT_POSITION_FOLLOWING) {
//						allTweets.splice(index);
//						return false;
//					}
//				});
//			}
//
//			// Set main tweet and thread tweets
//			this.mainTweet = allTweets[0] || null;
//			this.threadTweets = allTweets.slice(1);
//		}
//	}
type TwitterExtractor struct {
	*ExtractorBase
	mainTweet    *goquery.Selection
	threadTweets []*goquery.Selection
}

// UserInfo represents Twitter user information
type UserInfo struct {
	FullName  string
	Handle    string
	Date      string
	Permalink string
}

// NewTwitterExtractor creates a new Twitter extractor
// TypeScript original code:
//
//	constructor(document: Document, url: string) {
//		super(document, url);
//
//		// Get all tweets from the timeline
//		const timeline = document.querySelector('[aria-label="Timeline: Conversation"]');
//		if (!timeline) {
//			// Try to find a single tweet if not in timeline view
//			const singleTweet = document.querySelector('article[data-testid="tweet"]');
//			if (singleTweet) {
//				this.mainTweet = singleTweet;
//			}
//			return;
//		}
//
//		// Get all tweets before any section with "Discover more" or similar headings
//		const allTweets = Array.from(timeline.querySelectorAll('article[data-testid="tweet"]'));
//
//		// Set main tweet and thread tweets
//		if (allTweets.length > 0) {
//			this.mainTweet = allTweets[0];
//			this.threadTweets = allTweets.slice(1);
//		}
//	}
func NewTwitterExtractor(document *goquery.Document, url string, schemaOrgData any) *TwitterExtractor {
	extractor := &TwitterExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
		threadTweets:  make([]*goquery.Selection, 0),
	}

	allTweets := findTweets(document)

	// Set main tweet and thread tweets
	if len(allTweets) > 0 {
		extractor.mainTweet = allTweets[0]
		extractor.threadTweets = allTweets[1:]
	}

	return extractor
}

// findTweets locates the conversation's tweets in document: it prefers the
// timeline (filtering out recommended tweets past a section/h2 boundary) and
// falls back to document-wide tweet selectors when no timeline is found.
func findTweets(document *goquery.Document) []*goquery.Selection {
	// Primary method: Get all tweets from the timeline
	timeline := document.Find(`[aria-label="Timeline: Conversation"]`).First()
	if timeline.Length() == 0 {
		// Fallback: Try alternative timeline selectors
		timelineSelectors := []string{
			`[aria-label*="timeline"]`,
			`[aria-label*="Timeline"]`,
			`main[role="main"]`,
			`section[role="region"]`,
		}

		for _, selector := range timelineSelectors {
			timeline = document.Find(selector).First()
			if timeline.Length() > 0 {
				break
			}
		}
	}

	var allTweets []*goquery.Selection

	if timeline.Length() > 0 {
		// Try to find tweets within the timeline
		timeline.Find(`article[data-testid="tweet"]`).Each(func(_ int, s *goquery.Selection) {
			allTweets = append(allTweets, s)
		})

		// Filter out recommended tweets after section/h2 boundary
		// (e.g. "Discover more" sections). TS uses compareDocumentPosition;
		// Go walks the DOM tree to determine document order.
		if len(allTweets) > 0 {
			sectionBoundary := timeline.Find("section, h2").First()
			if sectionBoundary.Length() > 0 {
				boundaryParent := sectionBoundary.Parent()
				if boundaryParent.Length() > 0 {
					allTweets = filterTweetsBeforeBoundary(timeline, allTweets, boundaryParent)
				}
			}
		}
	}

	// Fallback: Try to find tweets anywhere in the document if timeline method fails
	if len(allTweets) == 0 {
		// Try alternative tweet selectors
		tweetSelectors := []string{
			`article[data-testid="tweet"]`,
			`[data-testid="tweet"]`,
			`.tweet`,
			`article[role="article"]`,
			`div[data-tweet-id]`,
		}

		for _, selector := range tweetSelectors {
			document.Find(selector).Each(func(_ int, s *goquery.Selection) {
				allTweets = append(allTweets, s)
			})
			if len(allTweets) > 0 {
				break
			}
		}
	}

	return allTweets
}

// filterTweetsBeforeBoundary removes tweets that appear after the boundary
// element in document order, matching TypeScript's compareDocumentPosition logic.
func filterTweetsBeforeBoundary(timeline *goquery.Selection, tweets []*goquery.Selection, boundary *goquery.Selection) []*goquery.Selection {
	nodePos := make(map[*html.Node]int)
	pos := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		nodePos[n] = pos
		pos++
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(timeline.Get(0))

	boundaryIdx, ok := nodePos[boundary.Get(0)]
	if !ok {
		return tweets
	}

	cutoff := len(tweets)
	for i, tweet := range tweets {
		tweetIdx, exists := nodePos[tweet.Get(0)]
		if exists && tweetIdx > boundaryIdx {
			cutoff = i
			break
		}
	}

	return tweets[:cutoff]
}

// CanExtract checks if the extractor can extract content
// TypeScript original code:
//
//	canExtract(): boolean {
//		return !!this.mainTweet;
//	}
func (t *TwitterExtractor) CanExtract() bool {
	return t.mainTweet != nil && t.mainTweet.Length() > 0
}

// Name returns the name of the extractor
func (t *TwitterExtractor) Name() string {
	return "TwitterExtractor"
}
