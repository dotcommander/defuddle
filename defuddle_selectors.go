package defuddle

import "github.com/dotcommander/defuddle/internal/standardize"

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
	if src.Headers != nil {
		dst.Headers = src.Headers.Clone()
	}
	if src.Client != nil {
		dst.Client = src.Client
	}
	if src.MaxConcurrency > 0 {
		dst.MaxConcurrency = src.MaxConcurrency
	}
}

func standardizeOptions(options *Options) standardize.Options {
	if options == nil {
		return standardize.Options{}
	}
	return standardize.Options{
		ProcessCode:      options.ProcessCode,
		ProcessImages:    options.ProcessImages,
		ProcessHeadings:  options.ProcessHeadings,
		ProcessMath:      options.ProcessMath,
		ProcessFootnotes: options.ProcessFootnotes,
		ProcessRoles:     options.ProcessRoles,
		CodeOptions:      options.CodeOptions,
		ImageOptions:     options.ImageOptions,
		HeadingOptions:   options.HeadingOptions,
		MathOptions:      options.MathOptions,
		FootnoteOptions:  options.FootnoteOptions,
		RoleOptions:      options.RoleOptions,
	}
}

// extractionBypassed preserves the all-false legacy no-clutter-removal mode.
func extractionBypassed(o *Options) bool {
	return !BoolDefault(o.RemoveExactSelectors, true) && !BoolDefault(o.RemovePartialSelectors, true) && !BoolDefault(o.RemoveHiddenElements, true) && !BoolDefault(o.RemoveLowScoring, true) && !BoolDefault(o.RemoveContentPatterns, true)
}
