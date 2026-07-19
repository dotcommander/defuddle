// Package defuddle provides web content extraction and demuddling capabilities.
package defuddle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/debug"
)

// headingTags lists the HTML heading tag names used for heading detection.
var headingTags = []string{"h1", "h2", "h3", "h4", "h5", "h6"}

var errInvalidRequestURL = errors.New("URL must be absolute HTTP(S)")

// headingSelector is a CSS selector string derived from headingTags.
var headingSelector = strings.Join(headingTags, ", ")

// Defuddle represents a document parser instance
type Defuddle struct {
	rawHTML  string // stored for re-parsing on retry (goquery has no clone)
	doc      *goquery.Document
	options  *Options
	debug    bool
	debugger *debug.Debugger
}

// NewDefuddle creates a new Defuddle instance from HTML content
// JavaScript original code:
//
//	constructor(document: Document, options: DefuddleOptions = {}) {
//	  this.doc = document;
//	  this.options = options;
//	}
func NewDefuddle(html string, options *Options) (*Defuddle, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	debugEnabled := false
	if options != nil {
		debugEnabled = options.Debug
	}
	debugger := debug.NewDebugger(debugEnabled)

	return &Defuddle{
		rawHTML:  html,
		doc:      doc,
		options:  options,
		debug:    debugEnabled,
		debugger: debugger,
	}, nil
}

// Parse extracts the main content from the document
// JavaScript original code:
//
//	parse(): DefuddleResponse {
//	  const result = this.parseInternal();
//	  if (result.wordCount < 200) {
//	    const retryResult = this.parseInternal({ removePartialSelectors: false });
//	    if (retryResult.wordCount > result.wordCount) {
//	      return retryResult;
//	    }
//	  }
//	  return result;
//	}
//
// retryStep describes one retry pass in the Parse retry ladder.
type retryStep struct {
	name    string
	trigger int                       // retry when result.WordCount < trigger
	mutate  func(*Options)            // mutations to apply to a copy of options
	accept  func(prev, next int) bool // accept next result if true
}

// retryLadder defines the ordered retry passes for Parse.
// Predicates are transcribed verbatim from the original logic.
var retryLadder = []retryStep{
	{
		name:    "partial-selectors",
		trigger: 200,
		mutate:  func(o *Options) { o.RemovePartialSelectors = new(bool) },
		accept:  func(prev, next int) bool { return next > prev },
	},
	{
		name:    "hidden-elements",
		trigger: 50,
		mutate:  func(o *Options) { o.RemoveHiddenElements = new(bool) },
		accept:  func(prev, next int) bool { return next > prev*2 },
	},
	{
		name:    "index-page",
		trigger: 50,
		mutate: func(o *Options) {
			o.RemoveLowScoring = new(bool)
			o.RemovePartialSelectors = new(bool)
			o.RemoveContentPatterns = new(bool)
		},
		accept: func(prev, next int) bool { return next > prev },
	},
}

// Parse parses the document and returns the extracted content.
func (d *Defuddle) Parse(ctx context.Context) (*Result, error) {
	// Try first with default settings
	result, err := d.parseInternal(ctx, nil)
	if err != nil {
		return nil, err
	}

	for _, step := range retryLadder {
		if result.WordCount >= step.trigger {
			continue
		}
		if d.debug {
			slog.Debug("Parse: trying retry", "step", step.name, "wordCount", result.WordCount, "trigger", step.trigger)
		}

		retryOpts := &Options{}
		if d.options != nil {
			*retryOpts = *d.options
		}
		step.mutate(retryOpts)

		retryResult, retryErr := d.parseInternal(ctx, retryOpts)
		if retryErr != nil {
			// First retry propagates error; subsequent retries are best-effort.
			if step.trigger == 200 {
				return result, retryErr
			}
			continue
		}

		if step.accept(result.WordCount, retryResult.WordCount) {
			if d.debug {
				slog.Debug("Parse: retry accepted", "step", step.name, "originalWordCount", result.WordCount, "retryWordCount", retryResult.WordCount)
			}
			result = retryResult
		}
	}

	return result, nil
}
