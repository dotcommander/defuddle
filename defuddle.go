// Package defuddle provides web content extraction and demuddling capabilities.
package defuddle

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaptinlin/defuddle-go/extractors"
	"github.com/kaptinlin/defuddle-go/internal/constants"
	"github.com/kaptinlin/defuddle-go/internal/debug"
	"github.com/kaptinlin/defuddle-go/internal/markdown"
	"github.com/kaptinlin/defuddle-go/internal/metadata"
	"github.com/kaptinlin/defuddle-go/internal/scoring"
	"github.com/kaptinlin/defuddle-go/internal/standardize"
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
func ParseFromURL(ctx context.Context, url string, options *Options) (*Result, error) {
	if options == nil {
		options = &Options{}
	}

	// Set URL in options if not already set
	if options.URL == "" {
		options.URL = url
	}

	// Create HTTP client and make request
	client := options.Client
	if client == nil {
		client = requests.New(
			requests.WithUserAgent("Mozilla/5.0 (compatible; Defuddle/1.0; +https://github.com/kaptinlin/defuddle-go)"),
			requests.WithTimeout(30*time.Second),
		)
	}
	resp, err := client.Get(url).Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer func() {
		if closeErr := resp.Close(); closeErr != nil {
			slog.Warn("Failed to close response", "error", closeErr)
		}
	}()

	html := resp.String()

	// Create Defuddle instance and parse
	defuddle, err := NewDefuddle(html, options)
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
	d.removeHiddenElements(workingDoc)

	// Remove non-content blocks by scoring
	scoring.ScoreAndRemove(workingDoc, d.debug)

	// Remove clutter using selectors
	if options.RemoveExactSelectors || options.RemovePartialSelectors {
		d.removeBySelector(workingDoc, options.RemoveExactSelectors, options.RemovePartialSelectors)
	}

	// Normalize the main content
	standardize.Content(mainContent, extractedMetadata, workingDoc, d.debug)

	content, _ := mainContent.Html()
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

// removeBySelector removes elements by exact and partial selectors
func (d *Defuddle) removeBySelector(doc *goquery.Document, removeExact, removePartial bool) {
	if removeExact {
		exactSelectors := constants.GetExactSelectors()
		for _, selector := range exactSelectors {
			doc.Find(selector).Remove()
		}
	}

	if removePartial {
		testAttributes := constants.GetTestAttributes()
		partialSelectors := constants.GetPartialSelectors()

		doc.Find("*").Each(func(_ int, element *goquery.Selection) {
			for _, attr := range testAttributes {
				value, exists := element.Attr(attr)
				if exists && value != "" {
					lowerValue := strings.ToLower(value)
					for _, pattern := range partialSelectors {
						if strings.Contains(lowerValue, strings.ToLower(pattern)) {
							element.Remove()
							return
						}
					}
				}
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
	}

	// Apply instance options if they exist (...this.options)
	if d.options != nil {
		// Copy all values from instance options, including false values
		options.Debug = d.options.Debug
		if d.options.URL != "" {
			options.URL = d.options.URL
		}
		options.Markdown = d.options.Markdown
		options.SeparateMarkdown = d.options.SeparateMarkdown

		// For boolean options that can override defaults, always apply them
		options.RemoveExactSelectors = d.options.RemoveExactSelectors
		options.RemovePartialSelectors = d.options.RemovePartialSelectors
		options.RemoveImages = d.options.RemoveImages
		options.ProcessCode = d.options.ProcessCode
		options.ProcessImages = d.options.ProcessImages
		options.ProcessHeadings = d.options.ProcessHeadings
		options.ProcessMath = d.options.ProcessMath
		options.ProcessFootnotes = d.options.ProcessFootnotes
		options.ProcessRoles = d.options.ProcessRoles

		// Copy pointer fields
		if d.options.CodeOptions != nil {
			options.CodeOptions = d.options.CodeOptions
		}
		if d.options.ImageOptions != nil {
			options.ImageOptions = d.options.ImageOptions
		}
		if d.options.HeadingOptions != nil {
			options.HeadingOptions = d.options.HeadingOptions
		}
		if d.options.MathOptions != nil {
			options.MathOptions = d.options.MathOptions
		}
		if d.options.FootnoteOptions != nil {
			options.FootnoteOptions = d.options.FootnoteOptions
		}
		if d.options.RoleOptions != nil {
			options.RoleOptions = d.options.RoleOptions
		}
	}

	// Apply override options if they exist (...overrideOptions)
	if overrideOptions != nil {
		// Copy all values from override options, including false values
		options.Debug = overrideOptions.Debug
		if overrideOptions.URL != "" {
			options.URL = overrideOptions.URL
		}
		options.Markdown = overrideOptions.Markdown
		options.SeparateMarkdown = overrideOptions.SeparateMarkdown

		// Override boolean options (these will override any previous values)
		options.RemoveExactSelectors = overrideOptions.RemoveExactSelectors
		options.RemovePartialSelectors = overrideOptions.RemovePartialSelectors
		options.RemoveImages = overrideOptions.RemoveImages
		options.ProcessCode = overrideOptions.ProcessCode
		options.ProcessImages = overrideOptions.ProcessImages
		options.ProcessHeadings = overrideOptions.ProcessHeadings
		options.ProcessMath = overrideOptions.ProcessMath
		options.ProcessFootnotes = overrideOptions.ProcessFootnotes
		options.ProcessRoles = overrideOptions.ProcessRoles

		// Copy pointer fields
		if overrideOptions.CodeOptions != nil {
			options.CodeOptions = overrideOptions.CodeOptions
		}
		if overrideOptions.ImageOptions != nil {
			options.ImageOptions = overrideOptions.ImageOptions
		}
		if overrideOptions.HeadingOptions != nil {
			options.HeadingOptions = overrideOptions.HeadingOptions
		}
		if overrideOptions.MathOptions != nil {
			options.MathOptions = overrideOptions.MathOptions
		}
		if overrideOptions.FootnoteOptions != nil {
			options.FootnoteOptions = overrideOptions.FootnoteOptions
		}
		if overrideOptions.RoleOptions != nil {
			options.RoleOptions = overrideOptions.RoleOptions
		}
	}

	return options
}

// countWords counts words in HTML content
func (d *Defuddle) countWords(content string) int {
	// Parse HTML content to extract text
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		// Fallback: count words in raw content
		text := strings.TrimSpace(content)
		if text == "" {
			return 0
		}
		words := strings.Fields(text)
		return len(words)
	}

	// Get text content, removing extra whitespace
	text := strings.TrimSpace(doc.Text())
	if text == "" {
		return 0
	}

	return len(strings.Fields(text))
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
