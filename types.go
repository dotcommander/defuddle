package defuddle

import (
	"github.com/kaptinlin/defuddle-go/internal/debug"
	"github.com/kaptinlin/defuddle-go/internal/elements"
	"github.com/kaptinlin/defuddle-go/internal/metadata"
	"github.com/kaptinlin/requests"
)

// MetaTag represents a meta tag item from HTML
// This is an alias to the internal metadata.MetaTag type
type MetaTag = metadata.MetaTag

// Options represents configuration options for Defuddle parsing
// JavaScript original code:
//
//	export interface DefuddleOptions {
//	  debug?: boolean;
//	  url?: string;
//	  markdown?: boolean;
//	  separateMarkdown?: boolean;
//	  removeExactSelectors?: boolean;
//	  removePartialSelectors?: boolean;
//	}
type Options struct {
	// Enable debug logging
	Debug bool `json:"debug,omitempty"`

	// URL of the page being parsed
	URL string `json:"url,omitempty"`

	// Convert output to Markdown
	Markdown bool `json:"markdown,omitempty"`

	// Include Markdown in the response
	SeparateMarkdown bool `json:"separateMarkdown,omitempty"`

	// Whether to remove elements matching exact selectors like ads, social buttons, etc.
	// nil = true (default). Use PtrBool(false) to disable.
	RemoveExactSelectors *bool `json:"removeExactSelectors,omitempty"`

	// Whether to remove elements matching partial selectors like ads, social buttons, etc.
	// nil = true (default). Use PtrBool(false) to disable.
	RemovePartialSelectors *bool `json:"removePartialSelectors,omitempty"`

	// Remove images from the extracted content
	// Defaults to false.
	RemoveImages bool `json:"removeImages,omitempty"`

	// Whether to remove hidden elements (display:none, Tailwind hidden classes).
	// nil = true (default). Use PtrBool(false) to disable.
	RemoveHiddenElements *bool `json:"removeHiddenElements,omitempty"`

	// Whether to remove low-scoring non-content blocks.
	// nil = true (default). Use PtrBool(false) to disable.
	RemoveLowScoring *bool `json:"removeLowScoring,omitempty"`

	// Whether to remove content patterns (boilerplate, breadcrumbs, etc.).
	// nil = true (default). Use PtrBool(false) to disable.
	RemoveContentPatterns *bool `json:"removeContentPatterns,omitempty"`

	// CSS selector to use for content extraction instead of auto-detection.
	ContentSelector string `json:"contentSelector,omitempty"`

	// Element processing options
	ProcessCode      bool                                 `json:"processCode,omitempty"`
	ProcessImages    bool                                 `json:"processImages,omitempty"`
	ProcessHeadings  bool                                 `json:"processHeadings,omitempty"`
	ProcessMath      bool                                 `json:"processMath,omitempty"`
	ProcessFootnotes bool                                 `json:"processFootnotes,omitempty"`
	ProcessRoles     bool                                 `json:"processRoles,omitempty"`
	CodeOptions      *elements.CodeBlockProcessingOptions `json:"codeOptions,omitempty"`
	ImageOptions     *elements.ImageProcessingOptions     `json:"imageOptions,omitempty"`
	HeadingOptions   *elements.HeadingProcessingOptions   `json:"headingOptions,omitempty"`
	MathOptions      *elements.MathProcessingOptions      `json:"mathOptions,omitempty"`
	FootnoteOptions  *elements.FootnoteProcessingOptions  `json:"footnoteOptions,omitempty"`
	RoleOptions      *elements.RoleProcessingOptions      `json:"roleOptions,omitempty"`

	// Client is a custom HTTP client for fetching URLs.
	// If nil, a default client with standard User-Agent and 30s timeout is created.
	Client *requests.Client `json:"-"`

	// MaxConcurrency limits parallel URL fetches in ParseFromURLs.
	// Defaults to 5 if zero.
	MaxConcurrency int `json:"maxConcurrency,omitempty"`
}

// Metadata represents extracted metadata from a document
// This is an alias to the internal metadata.Metadata type
type Metadata = metadata.Metadata

// Result represents the complete response from Defuddle parsing
// JavaScript original code:
//
//	export interface DefuddleResponse extends DefuddleMetadata {
//	  content: string;
//	  contentMarkdown?: string;
//	  extractorType?: string;
//	  metaTags?: MetaTagItem[];
//	}
type Result struct {
	Metadata
	Content         string            `json:"content"`
	ContentMarkdown *string           `json:"contentMarkdown,omitempty"`
	ExtractorType   *string           `json:"extractorType,omitempty"`
	Variables       map[string]string `json:"variables,omitempty"`
	MetaTags        []MetaTag         `json:"metaTags,omitempty"`
	DebugInfo       *debug.Info       `json:"debugInfo,omitempty"`
}

// PtrBool returns a pointer to the given bool value.
// Use this to explicitly set *bool fields in Options (e.g., PtrBool(false) to disable defaults).
func PtrBool(v bool) *bool { return &v }

// BoolDefault returns the value pointed to by b, or defaultVal if b is nil.
func BoolDefault(b *bool, defaultVal bool) bool {
	if b == nil {
		return defaultVal
	}
	return *b
}

// ExtractorVariables represents variables extracted by site-specific extractors
// JavaScript original code:
//
//	export interface ExtractorVariables {
//	  [key: string]: string;
//	}
type ExtractorVariables map[string]string

// ExtractedContent represents content extracted by site-specific extractors
// JavaScript original code:
//
//	export interface ExtractedContent {
//	  title?: string;
//	  author?: string;
//	  published?: string;
//	  content?: string;
//	  contentHtml?: string;
//	  variables?: ExtractorVariables;
//	}
type ExtractedContent struct {
	Title       *string             `json:"title,omitempty"`
	Author      *string             `json:"author,omitempty"`
	Published   *string             `json:"published,omitempty"`
	Content     *string             `json:"content,omitempty"`
	ContentHTML *string             `json:"contentHtml,omitempty"`
	Variables   *ExtractorVariables `json:"variables,omitempty"`
}
