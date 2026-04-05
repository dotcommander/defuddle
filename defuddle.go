// Package defuddle provides web content extraction and demuddling capabilities.
package defuddle

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html/charset"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/extractors"
	"github.com/kaptinlin/defuddle-go/internal/constants"
	"github.com/kaptinlin/defuddle-go/internal/debug"
	"github.com/kaptinlin/defuddle-go/internal/markdown"
	"github.com/kaptinlin/defuddle-go/internal/metadata"
	"github.com/kaptinlin/defuddle-go/internal/scoring"
	"github.com/kaptinlin/defuddle-go/internal/standardize"
	"github.com/kaptinlin/defuddle-go/internal/text"
	"github.com/kaptinlin/defuddle-go/internal/urlutil"
	"github.com/kaptinlin/requests"
)

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
func (d *Defuddle) Parse(ctx context.Context) (*Result, error) {
	// Try first with default settings
	result, err := d.parseInternal(ctx, nil)
	if err != nil {
		return nil, err
	}

	// If result has very little content, try again without clutter removal
	if result.WordCount < 200 {
		if d.debug {
			slog.Debug("Initial parse returned very little content, trying again")
		}

		retryOptions := &Options{}
		if d.options != nil {
			*retryOptions = *d.options
		}
		retryOptions.RemovePartialSelectors = false

		retryResult, retryErr := d.parseInternal(ctx, retryOptions)
		if retryErr != nil {
			return result, retryErr
		}

		// Return the result with more content
		if retryResult.WordCount > result.WordCount {
			if d.debug {
				slog.Debug("Retry produced more content", "originalWordCount", result.WordCount, "retryWordCount", retryResult.WordCount)
			}
			return retryResult, nil
		}
	}

	return result, nil
}

// ParseFromURL fetches content from a URL and parses it
// JavaScript original code:
// // This corresponds to Node.js usage: Defuddle(htmlOrDom, url?, options?)
// maxResponseSize is the maximum HTML body size (5 MB).
const maxResponseSize = 5 * 1024 * 1024

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
			requests.WithUserAgent("Mozilla/5.0 (compatible; Defuddle/1.0; +https://github.com/kaptinlin/defuddle-go)"),
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
	sem := make(chan struct{}, limit)

	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(idx int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Copy options per URL so URL field doesn't collide
			opts := *options
			opts.URL = url
			result, err := ParseFromURL(ctx, url, &opts)
			results[idx] = URLResult{URL: url, Result: result, Err: err}
		}(i, u)
	}
	wg.Wait()
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

	// Try site-specific extractor first, if there is one
	url := options.URL
	extractor := extractors.FindExtractor(d.doc, url, schemaOrgData)
	if extractor != nil && extractor.CanExtract() {
		// Inject secondary Defuddle pass for conversation extractors
		if setter, ok := extractor.(extractors.ContentProcessorSetter); ok {
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

		d.debugger.SetExtractorUsed(extractor.Name())
		extracted := extractor.Extract()
		parseTime := time.Since(startTime).Milliseconds()

		// Get site name from extractor variables or use metadata
		siteName := extractedMetadata.Site
		if extracted.Variables != nil {
			if site, exists := extracted.Variables["site"]; exists {
				siteName = site
			}
		}

		// Create extractor type name (remove "Extractor" suffix)
		extractorType := strings.ToLower(strings.TrimSuffix(extractor.Name(), "Extractor"))

		result := &Result{
			Metadata: Metadata{
				Title:         extractedMetadata.Title,
				Description:   extractedMetadata.Description,
				Domain:        extractedMetadata.Domain,
				Favicon:       extractedMetadata.Favicon,
				Image:         extractedMetadata.Image,
				ParseTime:     parseTime,
				Published:     extractedMetadata.Published,
				Author:        extractedMetadata.Author,
				Site:          siteName,
				SchemaOrgData: schemaOrgData,
				WordCount:     d.countWords(extracted.ContentHTML),
			},
			Content:       extracted.ContentHTML,
			ExtractorType: &extractorType,
			MetaTags:      metaTags,
		}

		// Override metadata from extractor if available
		if extracted.Variables != nil {
			if title, exists := extracted.Variables["title"]; exists && title != "" {
				result.Title = title
			}
			if author, exists := extracted.Variables["author"]; exists && author != "" {
				result.Author = author
			}
			if published, exists := extracted.Variables["published"]; exists && published != "" {
				result.Published = published
			}
			if description, exists := extracted.Variables["description"]; exists && description != "" {
				result.Description = description
			}
			if image, exists := extracted.Variables["image"]; exists && image != "" {
				result.Image = image
			}
		}

		// Add debug info if enabled
		if d.debugger.IsEnabled() {
			d.debugger.EndTimer("total_parsing")
			d.debugger.AddProcessingStep("extractor", "Used site-specific extractor: "+extractor.Name(), 1, "")
			result.DebugInfo = d.debugger.GetInfo()
		}

		return result, nil
	}

	// Re-parse from stored HTML to get a fresh document for mutation
	// (goquery has no Clone method; the TypeScript version uses doc.cloneNode(true))
	workingDoc, err := goquery.NewDocumentFromReader(strings.NewReader(d.rawHTML))
	if err != nil {
		return nil, fmt.Errorf("failed to re-parse HTML for processing: %w", err)
	}

	// Flatten declarative Shadow DOM templates into the main document
	flattenShadowDOM(workingDoc)

	// Resolve React SSR streaming placeholders ($RC boundaries)
	resolveReactStreaming(workingDoc)

	// Evaluate mobile styles and sizes on fresh document
	mobileStyles := d.evaluateMediaQueries()

	// Find small images in fresh document, excluding lazy-loaded ones
	smallImages := d.findSmallImages(workingDoc)

	// Apply mobile styles to document
	d.applyMobileStyles(workingDoc, mobileStyles)

	// Find main content
	mainContent := d.findMainContent(workingDoc)
	if mainContent == nil {
		// Fallback to body content
		content, _ := workingDoc.Find("body").Html()
		wordCount := d.countWords(content)
		parseTime := time.Since(startTime).Milliseconds()

		result := &Result{
			Metadata: Metadata{
				Title:         extractedMetadata.Title,
				Description:   extractedMetadata.Description,
				Domain:        extractedMetadata.Domain,
				Favicon:       extractedMetadata.Favicon,
				Image:         extractedMetadata.Image,
				ParseTime:     parseTime,
				Published:     extractedMetadata.Published,
				Author:        extractedMetadata.Author,
				Site:          extractedMetadata.Site,
				SchemaOrgData: schemaOrgData,
				WordCount:     wordCount,
			},
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

	// Remove small images
	d.removeSmallImages(workingDoc, smallImages)

	// Remove all images if removeImages option is enabled
	if options.RemoveImages {
		d.removeAllImages(workingDoc)
	}

	// Remove hidden elements using computed styles
	if options.RemoveHiddenElements {
		d.removeHiddenElements(workingDoc)
	}

	// Remove non-content blocks by scoring
	if options.RemoveLowScoring {
		scoring.ScoreAndRemove(workingDoc, d.debug, mainContent)
	}

	// Remove clutter using selectors (after scoring — matches TS pipeline order)
	if options.RemoveExactSelectors || options.RemovePartialSelectors {
		d.removeBySelector(workingDoc, options.RemoveExactSelectors, options.RemovePartialSelectors, mainContent)
	}

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
	wordCount := d.countWords(content)
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
		Metadata: Metadata{
			Title:         extractedMetadata.Title,
			Description:   extractedMetadata.Description,
			Domain:        extractedMetadata.Domain,
			Favicon:       extractedMetadata.Favicon,
			Image:         extractedMetadata.Image,
			ParseTime:     parseTime,
			Published:     extractedMetadata.Published,
			Author:        extractedMetadata.Author,
			Site:          extractedMetadata.Site,
			SchemaOrgData: schemaOrgData,
			WordCount:     wordCount,
		},
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

// removeBySelector removes elements by exact and partial selectors.
// mainContent is used to protect the main content element and its ancestors from removal.
func (d *Defuddle) removeBySelector(doc *goquery.Document, removeExact, removePartial bool, mainContent *goquery.Selection) {
	if removeExact {
		exactSelectors := constants.GetExactSelectors()
		for _, selector := range exactSelectors {
			doc.Find(selector).Each(func(_ int, el *goquery.Selection) {
				// Never remove ancestors of main content
				if mainContent != nil && scoring.NodeContains(el, mainContent) {
					return
				}
				// Protect elements inside code blocks
				if el.Closest("pre").Length() > 0 || el.Closest("code").Length() > 0 {
					return
				}
				el.Remove()
			})
		}
	}

	if removePartial {
		testAttributes := constants.GetTestAttributes()
		partialRegex := constants.GetPartialSelectorRegex()

		// Pre-compute footnote list selectors for protection
		footnoteListSelectors := constants.GetFootnoteListSelectors()

		// Only query elements that have at least one test attribute
		attrSelector := make([]string, len(testAttributes))
		for i, attr := range testAttributes {
			attrSelector[i] = "[" + attr + "]"
		}
		combinedSelector := strings.Join(attrSelector, ",")

		doc.Find(combinedSelector).Each(func(_ int, element *goquery.Selection) {
			// Never remove ancestors of main content
			if mainContent != nil && scoring.NodeContains(element, mainContent) {
				return
			}
			// Protect elements inside code blocks
			if element.Closest("pre").Length() > 0 || element.Closest("code").Length() > 0 {
				return
			}
			// Protect footnote lists and their parents
			for _, sel := range footnoteListSelectors {
				if element.Is(sel) || element.Find(sel).Length() > 0 {
					return
				}
			}
			// Skip heading elements — their IDs often match partial selectors
			tag := goquery.NodeName(element)
			if tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" || tag == "h5" || tag == "h6" {
				return
			}
			// Skip anchor links inside headings
			if element.Closest("h1, h2, h3, h4, h5, h6").Length() > 0 {
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

// mergeOptions merges override options with instance options and defaults
// JavaScript original code:
//
//	const options = {
//	  removeExactSelectors: true,
//	  removePartialSelectors: true,
//	  ...this.options,
//	  ...overrideOptions
//	};
func (d *Defuddle) mergeOptions(overrideOptions *Options) *Options {
	// Start with defaults (exactly like TypeScript version)
	options := &Options{
		RemoveExactSelectors:   true,
		RemovePartialSelectors: true,
		RemoveHiddenElements:   true,
		RemoveLowScoring:       true,
		RemoveContentPatterns:  true,
	}

	// Apply instance options then override options (mirrors JS spread order)
	applyOptions(options, d.options)
	applyOptions(options, overrideOptions)

	return options
}

// applyOptions overlays src onto dst, skipping zero-value string fields so
// that defaults set on dst are not accidentally cleared by empty strings.
// All boolean and pointer fields are always copied because false/nil is a
// meaningful caller intent (e.g. RemoveExactSelectors=false disables removal).
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
	dst.RemoveExactSelectors = src.RemoveExactSelectors
	dst.RemovePartialSelectors = src.RemovePartialSelectors
	dst.RemoveImages = src.RemoveImages
	dst.RemoveHiddenElements = src.RemoveHiddenElements
	dst.RemoveLowScoring = src.RemoveLowScoring
	dst.RemoveContentPatterns = src.RemoveContentPatterns
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

// countWords counts words in HTML content, with CJK-aware counting.
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
