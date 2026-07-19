package defuddle

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html/charset"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/metadata"
	"github.com/dotcommander/defuddle/internal/removals"
	"github.com/dotcommander/defuddle/internal/scoring"
	"github.com/dotcommander/defuddle/internal/text"
)

// buildMetadata constructs the Metadata struct from extracted fields.
// All three Result-building sites use identical Metadata fields — this
// helper eliminates the duplication.
func buildMetadata(m *metadata.Metadata, schemaOrgData any, wordCount int, parseTime int64) Metadata {
	return Metadata{
		Title:         m.Title,
		Description:   m.Description,
		Domain:        m.Domain,
		Favicon:       m.Favicon,
		Image:         m.Image,
		Language:      m.Language,
		ParseTime:     parseTime,
		Published:     m.Published,
		Author:        m.Author,
		Site:          m.Site,
		SchemaOrgData: schemaOrgData,
		WordCount:     wordCount,
	}
}

// prepareWorkingDoc re-parses the raw HTML into a fresh mutable document,
// then applies shadow-DOM flattening and React SSR streaming resolution.
func (d *Defuddle) prepareWorkingDoc() (*goquery.Document, error) {
	workingDoc, err := goquery.NewDocumentFromReader(strings.NewReader(d.rawHTML))
	if err != nil {
		return nil, fmt.Errorf("failed to re-parse HTML for processing: %w", err)
	}
	flattenShadowDOM(workingDoc)
	resolveReactStreaming(workingDoc)
	return workingDoc, nil
}

// runRemovalPipeline applies the full removal pipeline to workingDoc:
// small-image removal, hidden elements, low-scoring blocks, clutter
// selectors, and content patterns. mainContent is protected throughout.
func (d *Defuddle) runRemovalPipeline(ctx context.Context, workingDoc *goquery.Document, mainContent *goquery.Selection, smallImages map[string]bool, options *Options) {
	d.removeSmallImages(workingDoc, smallImages)

	if options.RemoveImages {
		d.removeAllImages(workingDoc)
	}

	if BoolDefault(options.RemoveHiddenElements, true) {
		d.removeHiddenElements(workingDoc)
	}

	if BoolDefault(options.RemoveLowScoring, true) {
		scoring.ScoreAndRemove(ctx, workingDoc, d.debug, mainContent)
	}

	removeExact := BoolDefault(options.RemoveExactSelectors, true)
	removePartial := BoolDefault(options.RemovePartialSelectors, true)
	if removeExact || removePartial {
		d.removeBySelector(workingDoc, removeExact, removePartial, mainContent)
	}

	if BoolDefault(options.RemoveContentPatterns, true) {
		removals.RemoveByContentPattern(mainContent, workingDoc, d.debug, options.URL)
	}
}

// countWordsInSelection counts words in a goquery Selection's text content,
// with CJK-aware counting. This avoids the HTML serialize → re-parse round-trip
// of countWords when the caller already holds a Selection.
func (d *Defuddle) countWordsInSelection(sel *goquery.Selection) int {
	if sel == nil || sel.Length() == 0 {
		return 0
	}
	return text.CountWords(strings.TrimSpace(sel.Text()))
}

// countWords counts words in HTML content, with CJK-aware counting.
// Prefer countWordsInSelection when a *goquery.Selection is already in scope.
func (d *Defuddle) countWords(content string) int {
	// Parse HTML content to extract text
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		// Fallback: count words in raw content
		return text.CountWords(strings.TrimSpace(content))
	}

	return text.CountWords(strings.TrimSpace(doc.Text()))
}

// collectMetaTags collects meta tags from the document
func (d *Defuddle) collectMetaTags() []MetaTag {
	var metaTags []MetaTag

	d.doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		name, nameExists := s.Attr("name")
		property, propertyExists := s.Attr("property")
		content, contentExists := s.Attr("content")

		if contentExists && content != "" {
			metaTag := MetaTag{
				Content: &content,
			}
			if nameExists {
				metaTag.Name = &name
			}
			if propertyExists {
				metaTag.Property = &property
			}
			metaTags = append(metaTags, metaTag)
		}
	})

	return metaTags
}

// toUTF8 converts raw bytes to a UTF-8 string using charset detection.
// It inspects both the Content-Type header and the HTML content itself
// (meta charset, BOM) to determine the source encoding.
func toUTF8(body []byte, contentType string) (string, error) {
	r, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		// If charset detection fails, assume UTF-8 (best effort)
		return string(body), nil //nolint:nilerr
	}
	utf8Body, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(utf8Body), nil
}
