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

// parseInternal owns dispatch; explicit selection and bypass precede site engines.
func (d *Defuddle) parseInternal(ctx context.Context, overrideOptions *Options) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	start := time.Now()
	doc, err := d.prepareWorkingDoc()
	if err != nil {
		return nil, err
	}
	local := *d
	local.doc = doc
	d = &local
	options := d.mergeOptions(overrideOptions)
	d.debugger = debug.NewDebugger(options.Debug)
	d.debug = options.Debug
	schema := d.extractSchemaOrgData()
	tags := d.collectMetaTags()
	meta := metadata.Extract(doc, schema, tags, options.URL)
	base := urlutil.ExtractBaseHref(doc)
	d.debugger.StartTimer("total_parsing")
	if options.RemoveImages {
		d.removeAllImages(doc)
	}
	root := doc.Find("body").First()
	selected := false
	if options.ContentSelector != "" {
		if match := doc.Find(options.ContentSelector).First(); match.Length() > 0 {
			root = match
			selected = true
		}
	}
	if selected || extractionBypassed(options) {
		standardize.ContentWithOptions(root, meta, doc, standardizeOptions(options), false)
		d.debugger.AddProcessingStep("extraction_bypass", "Processed selected subtree or body", 1, "")
		return d.finalize(ctx, root, options, meta, schema, tags, base, start)
	}
	if result := d.tryExtractor(ctx, options, meta, schema, tags, start); result != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return result, nil
	}
	standardize.ContentWithOptions(root, meta, doc, standardizeOptions(options), false)
	snapshot := root.Clone()
	manifest, err := prepareRichContent(doc, root)
	if err != nil {
		return nil, err
	}
	// Resolve remaining ordinary references before upstream URL normalization.
	// Rich local relationships were already captured with their original hrefs.
	urlutil.ResolveRelativeURLs(root, options.URL, base)
	upstream, reason := callExtraction(d.engine, doc, options)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var content *goquery.Selection
	if reason == "" {
		fillEngineMetadata(meta, upstream)
		content, err = manifest.restore(upstream.ContentNode)
		if err != nil {
			reason = "rich-content: " + err.Error()
		} else if strings.TrimSpace(content.Text()) == "" && content.Find("img,math,table,pre").Length() == 0 {
			reason = "unusable output"
		}
	}
	if reason != "" {
		content = snapshot
		d.debugger.AddProcessingStep("extraction_recovery", "Used marker-free body snapshot", 1, reason)
	} else {
		d.debugger.AddProcessingStep("trafilatura", "Used Trafilatura extraction", 1, "")
	}
	return d.finalize(ctx, content, options, meta, schema, tags, base, start)
}

func (d *Defuddle) finalize(ctx context.Context, content *goquery.Selection, options *Options, meta *metadata.Metadata, schema any, tags []MetaTag, base string, start time.Time) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if options.RemoveImages {
		content.Find("img,svg,picture,video,canvas").Remove()
	}
	urlutil.ResolveRelativeURLs(content, options.URL, base)
	urlutil.SanitizeUnsafe(content)
	html, err := goquery.OuterHtml(content)
	if err != nil {
		return nil, err
	}
	result := &Result{Metadata: buildMetadata(meta, schema, d.countWordsInSelection(content), time.Since(start).Milliseconds()), Content: html, MetaTags: tags}
	if options.wantsMarkdown() {
		if md, err := markdown.ConvertHTML(html); err == nil {
			result.ContentMarkdown = &md
		} else if d.debug {
			slog.Debug("Failed to convert to Markdown", "error", err)
		}
	}
	d.debugger.EndTimer("total_parsing")
	if d.debugger.IsEnabled() {
		result.DebugInfo = d.debugger.GetInfo()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
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
	if d.skipExtractors {
		return nil
	}
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
			tempDefuddle := &Defuddle{rawHTML: html, doc: tempDoc, debugger: d.debugger, debug: d.debug, skipExtractors: true}
			// The conversation extractor has already selected this message.
			// Apply inherited formatting and safety without extracting it again.
			messageOptions := *options
			messageOptions.ContentSelector = "body"
			tempResult, err := tempDefuddle.parseInternal(ctx, &messageOptions)
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
	if extracted == nil {
		return nil
	}

	// Get site name from extractor variables or use metadata
	siteName := extractedMetadata.Site
	if extracted.Variables != nil {
		if site, exists := extracted.Variables["site"]; exists {
			siteName = site
		}
	}

	extractorType := strings.ToLower(strings.TrimSuffix(ext.Name(), "Extractor"))

	// buildMetadata uses extractedMetadata.Site; override with siteName after.
	fragment, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + extracted.ContentHTML + "</div>"))
	if err != nil {
		return nil
	}
	output := fragment.Find("body > div").First()
	// Conversation content has already passed its enabled processors once.
	if _, conversation := ext.(extractors.ContentProcessorSetter); !conversation {
		standardize.ContentWithOptions(output, extractedMetadata, fragment, standardizeOptions(options), false)
	}
	if options.RemoveImages {
		output.Find("img,svg,picture,video,canvas").Remove()
	}
	if options.URL != "" {
		urlutil.ResolveRelativeURLs(output, options.URL, urlutil.ExtractBaseHref(d.doc))
	}
	urlutil.SanitizeUnsafe(output)
	contentHTML, err := output.Html()
	if err != nil {
		return nil
	}
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
