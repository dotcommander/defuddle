package defuddle

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/extractors"
	"github.com/dotcommander/defuddle/internal/debug"
	"github.com/dotcommander/defuddle/internal/markdown"
	"github.com/dotcommander/defuddle/internal/metadata"
	"github.com/dotcommander/defuddle/internal/standardize"
	"github.com/dotcommander/defuddle/internal/urlutil"
)

// selectMainContent returns the main content element: the explicit
// ContentSelector match if provided and present, otherwise automatic detection
// (which may return nil).
func (d *Defuddle) selectMainContent(workingDoc *goquery.Document, options *Options) *goquery.Selection {
	if options.ContentSelector != "" {
		if sel := workingDoc.Find(options.ContentSelector).First(); sel.Length() > 0 {
			return sel
		}
	}
	return d.findMainContent(workingDoc)
}

// parseInternal performs the actual parsing work.
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

	// Select main content: explicit selector if provided, else auto-detect
	mainContent := d.selectMainContent(workingDoc, options)

	if mainContent == nil {
		// Fallback to body content
		body := workingDoc.Find("body")
		content := sanitizeHTMLFragment(selectionHTML(body))
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

	d.runRemovalPipeline(ctx, workingDoc, mainContent, smallImages, options)

	// Normalize the main content
	standardize.ContentWithOptions(mainContent, extractedMetadata, workingDoc, standardizeOptions(options), d.debug)

	// Resolve relative URLs against page URL
	if options.URL != "" {
		docBaseHref := urlutil.ExtractBaseHref(workingDoc)
		urlutil.ResolveRelativeURLs(mainContent, options.URL, docBaseHref)
	}

	// Strip unsafe elements and attributes (XSS safety)
	urlutil.SanitizeUnsafe(mainContent)

	content := selectionOuterHTML(mainContent)
	wordCount := d.countWordsInSelection(mainContent)
	parseTime := time.Since(startTime).Milliseconds()

	// Convert to Markdown if requested
	var contentMarkdown *string
	if options.wantsMarkdown() {
		if markdownContent, err := markdown.ConvertHTML(content); err == nil {
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
	contentHTML := sanitizeHTMLFragment(extracted.ContentHTML)
	meta := buildMetadata(extractedMetadata, schemaOrgData, d.countWords(contentHTML), time.Since(startTime).Milliseconds())
	meta.Site = siteName
	result := &Result{
		Metadata:      meta,
		Content:       contentHTML,
		ExtractorType: &extractorType,
		Variables:     extracted.Variables,
		MetaTags:      metaTags,
	}

	// Override metadata from extractor variables
	if extracted.Variables != nil {
		setIfPresent := func(key string, dst *string) {
			if v, ok := extracted.Variables[key]; ok && v != "" {
				*dst = v
			}
		}
		setIfPresent("title", &result.Title)
		setIfPresent("author", &result.Author)
		setIfPresent("published", &result.Published)
		setIfPresent("description", &result.Description)
		setIfPresent("image", &result.Image)
	}

	if options.wantsMarkdown() {
		if md, err := markdown.ConvertHTML(contentHTML); err == nil {
			result.ContentMarkdown = &md
		} else if d.debug {
			slog.Debug("Failed to convert extractor output to Markdown", "error", err)
		}
	}

	if d.debugger.IsEnabled() {
		d.debugger.EndTimer("total_parsing")
		d.debugger.AddProcessingStep("extractor", "Used site-specific extractor: "+ext.Name(), 1, "")
		result.DebugInfo = d.debugger.GetInfo()
	}

	return result
}

func selectionHTML(sel *goquery.Selection) string {
	content, _ := sel.Html()
	return content
}

func selectionOuterHTML(sel *goquery.Selection) string {
	content, _ := goquery.OuterHtml(sel)
	return content
}

func sanitizeHTMLFragment(content string) string {
	if content == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + content + "</div>"))
	if err != nil {
		return content
	}
	root := doc.Find("body > div").First()
	urlutil.SanitizeUnsafe(root)
	return selectionHTML(root)
}
