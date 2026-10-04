// Package defuddle provides web content extraction and demuddling capabilities.
package defuddle

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/debug"
)

var errInvalidRequestURL = errors.New("URL must be absolute HTTP(S)")

// Defuddle represents a document parser instance
type Defuddle struct {
	rawHTML  string // immutable source for each parse
	doc      *goquery.Document
	options  *Options
	debug    bool
	debugger *debug.Debugger

	// Secondary conversation normalization must use the generic pipeline.
	skipExtractors bool
	engine         extractionEngine // invocation-local test seam; nil uses Trafilatura
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

// Parse parses the document using the site extractor or Trafilatura.
func (d *Defuddle) Parse(ctx context.Context) (*Result, error) {
	return d.parseInternal(ctx, nil)
}
