// Package defuddle provides web content extraction and demuddling capabilities.
package defuddle

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
	"golang.org/x/sync/errgroup"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/extractors"
	"github.com/dotcommander/defuddle/internal/constants"
	"github.com/dotcommander/defuddle/internal/debug"
	"github.com/dotcommander/defuddle/internal/markdown"
	"github.com/dotcommander/defuddle/internal/metadata"
	"github.com/dotcommander/defuddle/internal/removals"
	"github.com/dotcommander/defuddle/internal/scoring"
	"github.com/dotcommander/defuddle/internal/standardize"
	"github.com/dotcommander/defuddle/internal/text"
	"github.com/dotcommander/defuddle/internal/urlutil"
	"github.com/kaptinlin/requests"
)

// headingTags lists the HTML heading tag names used for heading detection.
var headingTags = []string{"h1", "h2", "h3", "h4", "h5", "h6"}

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

// maxResponseSize is the maximum HTML body size (5 MB).
const maxResponseSize = 5 * 1024 * 1024

// ParseFromURL fetches content from a URL and parses it.
// This corresponds to Node.js usage: Defuddle(htmlOrDom, url?, options?)
func ParseFromURL(ctx context.Context, url string, options *Options) (*Result, error) {
	if options == nil {
		options = &Options{}
	}

	// Set URL in options if not already set
	if options.URL == "" {
		options.URL = url
	}

	// Create HTTP client with hardened defaults
	client := options.Client
	if client == nil {
		client = requests.New(
			requests.WithUserAgent(fmt.Sprintf("Mozilla/5.0 (compatible; Defuddle/%s; +https://github.com/dotcommander/defuddle)", Version)),
			requests.WithTimeout(10*time.Second),
		)
	}
	resp, err := client.Get(url).Send(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("fetch %s: %w", url, ErrTimeout)
		}
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer func() {
		if closeErr := resp.Close(); closeErr != nil {
			slog.Warn("Failed to close response", "error", closeErr)
		}
	}()

	// Validate content type — reject non-HTML responses
	ct := resp.ContentType()
	if ct != "" && !strings.Contains(ct, "html") && !strings.Contains(ct, "xml") && !strings.Contains(ct, "text/") {
		return nil, fmt.Errorf("fetch %s: content-type %q: %w", url, ct, ErrNotHTML)
	}

	// Enforce size limit
	rawBody := resp.Body()
	if len(rawBody) > maxResponseSize {
		return nil, fmt.Errorf("fetch %s: response %d bytes: %w", url, len(rawBody), ErrTooLarge)
	}

	// Detect and convert charset to UTF-8
	body, err := toUTF8(rawBody, ct)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: charset conversion: %w", url, err)
	}

	// Create Defuddle instance and parse
	defuddle, err := NewDefuddle(body, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Defuddle instance: %w", err)
	}

	return defuddle.Parse(ctx)
}

// ParseFromString parses HTML content directly from a string
// This is useful when you already have the HTML content (e.g., from browser automation)
func ParseFromString(ctx context.Context, html string, options *Options) (*Result, error) {
	if options == nil {
		options = &Options{}
	}

	// Create Defuddle instance and parse
	defuddle, err := NewDefuddle(html, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Defuddle instance: %w", err)
	}

	return defuddle.Parse(ctx)
}

// URLResult pairs a URL with its extraction result or error.
type URLResult struct {
	URL    string
	Result *Result
	Err    error
}

// ParseFromURLs fetches and parses multiple URLs concurrently.
// MaxConcurrency in options controls parallelism (default 5).
func ParseFromURLs(ctx context.Context, urls []string, options *Options) []URLResult {
	if options == nil {
		options = &Options{}
	}
	limit := options.MaxConcurrency
	if limit <= 0 {
		limit = 5
	}

	results := make([]URLResult, len(urls))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	for i, u := range urls {
		// each slot owns its own error; never short-circuit the group
		g.Go(func() error {
			// Copy options per URL so URL field doesn't collide
			opts := *options
			opts.URL = u
			result, err := ParseFromURL(gctx, u, &opts)
			results[i] = URLResult{URL: u, Result: result, Err: err}
			return nil
		})
	}
	_ = g.Wait()
	return results
}

// parseInternal performs the actual parsing work
func (d *Defuddle) parseInternal(ctx context.Context, overrideOptions *Options) (*Result, error) {
	startTime := time.Now()

	// Merge options with defaults
	options := d.mergeOptions(overrideOptions)

	// Extract schema.org data
	schemaOrgData := d.extractSchemaOrgData()

	// Collect meta tags
	metaTags := d.collectMetaTags()

	// Get base URL for metadata extraction
	baseURL := options.URL

	// Extract metadata
	extractedMetadata := metadata.Extract(d.doc, schemaOrgData, metaTags, baseURL)

	// Initialize debug tracking
	if d.debugger.IsEnabled() {
		d.debugger.StartTimer("total_parsing")
		d.debugger.SetStatistics(debug.Statistics{
			OriginalElementCount: d.doc.Find("*").Length(),
		})
	}

	// Remove all images before extractor check (TS applies to doc before extractor)
	if options.RemoveImages {
		d.removeAllImages(d.doc)
	}

	// Try site-specific extractor first
	if result := d.tryExtractor(ctx, options, extractedMetadata, schemaOrgData, metaTags, startTime); result != nil {
		return result, nil
	}

	// Re-parse from stored HTML to get a fresh mutable document.
	// (goquery has no Clone method; the TypeScript version uses doc.cloneNode(true))
	workingDoc, err := d.prepareWorkingDoc()
	if err != nil {
		return nil, err
	}

	// Find small images in fresh document, excluding lazy-loaded ones
	smallImages := d.findSmallImages(workingDoc)

	// Use explicit content selector if provided
	var mainContent *goquery.Selection
	if options.ContentSelector != "" {
		sel := workingDoc.Find(options.ContentSelector).First()
		if sel.Length() > 0 {
			mainContent = sel
		}
	}

	// Fall back to automatic content detection
	if mainContent == nil {
		mainContent = d.findMainContent(workingDoc)
	}

	if mainContent == nil {
		// Fallback to body content
		body := workingDoc.Find("body")
		content, _ := body.Html()
		wordCount := d.countWordsInSelection(body)
		parseTime := time.Since(startTime).Milliseconds()

		result := &Result{
			Metadata: buildMetadata(extractedMetadata, schemaOrgData, wordCount, parseTime),
			Content:  content,
			MetaTags: metaTags,
		}

		// Add debug info if enabled (fallback case)
		if d.debugger.IsEnabled() {
			d.debugger.EndTimer("total_parsing")
			d.debugger.AddProcessingStep("fallback", "Used fallback body content extraction", 1, "No main content found")
			result.DebugInfo = d.debugger.GetInfo()
		}

		return result, nil
	}

	d.runRemovalPipeline(workingDoc, mainContent, smallImages, options)

	// Normalize the main content
	standardize.Content(mainContent, extractedMetadata, workingDoc, d.debug)

	// Resolve relative URLs against page URL
	if options.URL != "" {
		docBaseHref := urlutil.ExtractBaseHref(workingDoc)
		urlutil.ResolveRelativeURLs(mainContent, options.URL, docBaseHref)
	}

	// Strip unsafe elements and attributes (XSS safety)
	urlutil.SanitizeUnsafe(mainContent)

	content, _ := goquery.OuterHtml(mainContent)
	wordCount := d.countWordsInSelection(mainContent)
	parseTime := time.Since(startTime).Milliseconds()

	// Convert to Markdown if requested
	var contentMarkdown *string
	if options.Markdown || options.SeparateMarkdown {
		if markdownContent, err := d.convertHTMLToMarkdown(content); err == nil {
			contentMarkdown = &markdownContent
		} else if d.debug {
			slog.Debug("Failed to convert to Markdown", "error", err)
		}
	}

	result := &Result{
		Metadata:        buildMetadata(extractedMetadata, schemaOrgData, wordCount, parseTime),
		Content:         content,
		ContentMarkdown: contentMarkdown,
		MetaTags:        metaTags,
	}

	// Add debug info if enabled
	if d.debugger.IsEnabled() {
		d.debugger.EndTimer("total_parsing")
		d.debugger.AddProcessingStep("standard_parsing", "Used standard content extraction algorithm", 1, "")

		// Update final statistics
		finalStats := debug.Statistics{
			OriginalElementCount: d.doc.Find("*").Length(),
			FinalElementCount:    workingDoc.Find("*").Length(),
			WordCount:            wordCount,
			CharacterCount:       len(content),
			ImageCount:           workingDoc.Find("img").Length(),
			LinkCount:            workingDoc.Find("a").Length(),
		}
		finalStats.RemovedElementCount = finalStats.OriginalElementCount - finalStats.FinalElementCount
		d.debugger.SetStatistics(finalStats)

		result.DebugInfo = d.debugger.GetInfo()
	}

	return result, nil
}

// tryExtractor attempts to use a site-specific extractor. Returns nil if no extractor matches.
func (d *Defuddle) tryExtractor(
	ctx context.Context,
	options *Options,
	extractedMetadata *metadata.Metadata,
	schemaOrgData any,
	metaTags []MetaTag,
	startTime time.Time,
) *Result {
	ext := extractors.FindExtractor(d.doc, options.URL, schemaOrgData)
	if ext == nil || !ext.CanExtract() {
		return nil
	}

	// Inject secondary Defuddle pass for conversation extractors
	if setter, ok := ext.(extractors.ContentProcessorSetter); ok {
		setter.SetContentProcessor(func(html string) (*extractors.ContentProcessResult, error) {
			tempDoc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
			if err != nil {
				return nil, err
			}
			tempDefuddle := &Defuddle{rawHTML: html, doc: tempDoc, debugger: d.debugger}
			tempResult, err := tempDefuddle.parseInternal(ctx, options)
			if err != nil {
				return nil, err
			}
			return &extractors.ContentProcessResult{
				Content:   tempResult.Content,
				WordCount: tempResult.WordCount,
			}, nil
		})
	}

	d.debugger.SetExtractorUsed(ext.Name())
	extracted := ext.Extract()

	// Get site name from extractor variables or use metadata
	siteName := extractedMetadata.Site
	if extracted.Variables != nil {
		if site, exists := extracted.Variables["site"]; exists {
			siteName = site
		}
	}

	extractorType := strings.ToLower(strings.TrimSuffix(ext.Name(), "Extractor"))

	// buildMetadata uses extractedMetadata.Site; override with siteName after.
	meta := buildMetadata(extractedMetadata, schemaOrgData, d.countWords(extracted.ContentHTML), time.Since(startTime).Milliseconds())
	meta.Site = siteName
	result := &Result{
		Metadata:      meta,
		Content:       extracted.ContentHTML,
		ExtractorType: &extractorType,
		Variables:     extracted.Variables,
		MetaTags:      metaTags,
	}

	// Override metadata from extractor variables
	if extracted.Variables != nil {
		if v, ok := extracted.Variables["title"]; ok && v != "" {
			result.Title = v
		}
		if v, ok := extracted.Variables["author"]; ok && v != "" {
			result.Author = v
		}
		if v, ok := extracted.Variables["published"]; ok && v != "" {
			result.Published = v
		}
		if v, ok := extracted.Variables["description"]; ok && v != "" {
			result.Description = v
		}
		if v, ok := extracted.Variables["image"]; ok && v != "" {
			result.Image = v
		}
	}

	if d.debugger.IsEnabled() {
		d.debugger.EndTimer("total_parsing")
		d.debugger.AddProcessingStep("extractor", "Used site-specific extractor: "+ext.Name(), 1, "")
		result.DebugInfo = d.debugger.GetInfo()
	}

	return result
}

// removeBySelector removes elements by exact and partial selectors.
// mainContent, footnote lists, and heading elements/anchors are protected from removal.
// mainContent protection applies in both branches via scoring.IsProtectedNode; footnote-list
// and heading protections apply only in the removePartial branch.
func (d *Defuddle) removeBySelector(doc *goquery.Document, removeExact, removePartial bool, mainContent *goquery.Selection) {
	if removeExact {
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

	if removePartial {
		testAttributes := constants.GetTestAttributes()
		partialRegex := constants.GetPartialSelectorRegex()

		// Only query elements that have at least one test attribute
		attrSelector := make([]string, len(testAttributes))
		for i, attr := range testAttributes {
			attrSelector[i] = "[" + attr + "]"
		}
		combinedSelector := strings.Join(attrSelector, ",")

		doc.Find(combinedSelector).Each(func(_ int, element *goquery.Selection) {
			if scoring.IsProtectedNode(element, mainContent) {
				return
			}
			// Protect footnote lists and their parents (element itself or any descendant matches)
			if element.IsMatcher(constants.FootnoteListMatcher) ||
				element.FindMatcher(constants.FootnoteListMatcher).Length() > 0 {
				return
			}
			// Skip heading elements — their IDs often match partial selectors
			tag := goquery.NodeName(element)
			if slices.Contains(headingTags, tag) {
				return
			}
			// Skip anchor links inside headings
			if element.Closest(headingSelector).Length() > 0 {
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
}

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
func (d *Defuddle) runRemovalPipeline(workingDoc *goquery.Document, mainContent *goquery.Selection, smallImages map[string]bool, options *Options) {
	d.removeSmallImages(workingDoc, smallImages)

	if options.RemoveImages {
		d.removeAllImages(workingDoc)
	}

	if BoolDefault(options.RemoveHiddenElements, true) {
		d.removeHiddenElements(workingDoc)
	}

	if BoolDefault(options.RemoveLowScoring, true) {
		scoring.ScoreAndRemove(workingDoc, d.debug, mainContent)
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

// convertHTMLToMarkdown converts HTML content to Markdown
func (d *Defuddle) convertHTMLToMarkdown(htmlContent string) (string, error) {
	return markdown.ConvertHTML(htmlContent)
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
