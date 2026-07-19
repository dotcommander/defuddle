// Package elements provides enhanced element processing functionality
// This module handles code block processing including syntax highlighting,
// language detection, and code formatting
package elements

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns for language detection and code normalization.
var (
	highlighterPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^language-(\w+)$`),
		regexp.MustCompile(`^lang-(\w+)$`),
		regexp.MustCompile(`^(\w+)-code$`),
		regexp.MustCompile(`^code-(\w+)$`),
		regexp.MustCompile(`^syntax-(\w+)$`),
		regexp.MustCompile(`^code-snippet__(\w+)$`),
		regexp.MustCompile(`^highlight-(\w+)$`),
		regexp.MustCompile(`^(\w+)-snippet$`),
		regexp.MustCompile(`(?:^|\s)(?:language|lang|brush|syntax)-(\w+)(?:\s|$)`),
	}

	codeThreeNewlinesRe = regexp.MustCompile(`\n{3,}`)
	codeLeadingNlRe     = regexp.MustCompile(`^\n+`)
	codeTrailingNlRe    = regexp.MustCompile(`\n+$`)

	codeLanguages = map[string]bool{
		"abap": true, "actionscript": true, "ada": true, "adoc": true, "agda": true, "antlr4": true,
		"applescript": true, "arduino": true, "armasm": true, "asciidoc": true, "aspnet": true, "atom": true,
		"bash": true, "batch": true, "c": true, "clojure": true, "cmake": true, "cobol": true,
		"coffeescript": true, "cpp": true, "c++": true, "crystal": true, "csharp": true, "cs": true,
		"dart": true, "django": true, "dockerfile": true, "dotnet": true, "elixir": true, "elm": true,
		"erlang": true, "fortran": true, "fsharp": true, "gdscript": true, "gitignore": true, "glsl": true,
		"golang": true, "go": true, "gradle": true, "graphql": true, "groovy": true, "haskell": true,
		"hs": true, "haxe": true, "hlsl": true, "html": true, "idris": true, "java": true,
		"javascript": true, "js": true, "jsx": true, "jsdoc": true, "json": true, "jsonp": true,
		"julia": true, "kotlin": true, "latex": true, "lisp": true, "elisp": true, "livescript": true,
		"lua": true, "makefile": true, "markdown": true, "md": true, "markup": true, "masm": true,
		"mathml": true, "matlab": true, "mongodb": true, "mysql": true, "nasm": true, "nginx": true,
		"nim": true, "nix": true, "objc": true, "ocaml": true, "pascal": true, "perl": true,
		"php": true, "postgresql": true, "powershell": true, "prolog": true, "puppet": true, "python": true,
		"regex": true, "rss": true, "ruby": true, "rb": true, "rust": true, "scala": true,
		"scheme": true, "shell": true, "sh": true, "solidity": true, "sparql": true, "sql": true,
		"ssml": true, "svg": true, "swift": true, "tcl": true, "terraform": true, "tex": true,
		"toml": true, "typescript": true, "ts": true, "tsx": true, "unrealscript": true, "verilog": true,
		"vhdl": true, "webassembly": true, "wasm": true, "xml": true, "yaml": true, "yml": true,
		"zig": true,
	}
)

/*
TypeScript source code (code.ts, 319 lines):

This module provides code block processing functionality including:
- Language detection from class names and content analysis
- Code formatting and normalization
- Syntax highlighting preparation
- Code block structure optimization

Key functions:
- processCodeBlocks(): Main processing function for all code blocks
- detectLanguage(): Language detection from various sources
- formatCodeBlock(): Code formatting and structure optimization
- normalizeCodeContent(): Content normalization and cleanup

Original TypeScript implementation:
const HIGHLIGHTER_PATTERNS = [
	/^language-(\w+)$/,          // language-javascript
	/^lang-(\w+)$/,              // lang-javascript
	/^(\w+)-code$/,              // javascript-code
	/^code-(\w+)$/,              // code-javascript
	/^syntax-(\w+)$/,            // syntax-javascript
	/^code-snippet__(\w+)$/,     // code-snippet__javascript
	/^highlight-(\w+)$/,         // highlight-javascript
	/^(\w+)-snippet$/,           // javascript-snippet
	/(?:^|\s)(?:language|lang|brush|syntax)-(\w+)(?:\s|$)/i
];

const CODE_LANGUAGES = new Set([
	'javascript', 'js', 'jsx', 'typescript', 'ts', 'tsx',
	'python', 'java', 'c', 'cpp', 'c++', 'csharp', 'cs',
	// ... extensive language list
]);

export const codeBlockRules = [
	{
		selector: [
			'pre',
			'div[class*="prismjs"]',
			'.syntaxhighlighter',
			'.highlight',
			'.highlight-source',
			'.wp-block-syntaxhighlighter-code',
			'.wp-block-code',
			'div[class*="language-"]'
		].join(', '),
		element: 'pre',
		transform: (el: Element, doc: Document): Element => {
			// Processing logic here
		}
	}
];
*/

// CodeBlockProcessor handles code block processing and enhancement
// TypeScript original code:
//
//	class CodeBlockProcessor {
//	  constructor(private document: Document) {}
//	}
type CodeBlockProcessor struct {
	doc *goquery.Document
}

// CodeBlockProcessingOptions contains options for code block processing
// TypeScript original code:
//
//	interface CodeBlockOptions {
//	  detectLanguage?: boolean;
//	  formatCode?: boolean;
//	  addLineNumbers?: boolean;
//	  enableSyntaxHighlight?: boolean;
//	  wrapInPre?: boolean;
//	}
type CodeBlockProcessingOptions struct {
	DetectLanguage bool
	FormatCode     bool
}

// DefaultCodeBlockProcessingOptions returns default options for code block processing
// TypeScript original code:
//
//	const defaultOptions: CodeBlockOptions = {
//	  detectLanguage: true,
//	  formatCode: true,
//	  addLineNumbers: false,
//	  enableSyntaxHighlight: true,
//	  wrapInPre: true
//	};
func DefaultCodeBlockProcessingOptions() *CodeBlockProcessingOptions {
	return &CodeBlockProcessingOptions{
		DetectLanguage: true,
		FormatCode:     true,
	}
}

// NewCodeBlockProcessor creates a new code block processor
// TypeScript original code:
// constructor(private doc: Document) {}
func NewCodeBlockProcessor(doc *goquery.Document) *CodeBlockProcessor {
	return &CodeBlockProcessor{
		doc: doc,
	}
}

// ProcessCodeBlocks processes all code blocks in the document
// TypeScript original code:
// export const codeBlockRules = [
//
//	{
//	  selector: [
//	    'pre',
//	    'div[class*="prismjs"]',
//	    '.syntaxhighlighter',
//	    '.highlight',
//	    '.highlight-source',
//	    '.wp-block-syntaxhighlighter-code',
//	    '.wp-block-code',
//	    'div[class*="language-"]'
//	  ].join(', '),
//	  element: 'pre',
//	  transform: (el: Element, doc: Document): Element => {
//	    // Processing logic here
//	  }
//	}
//
// ];
func (p *CodeBlockProcessor) ProcessCodeBlocks(options *CodeBlockProcessingOptions) {
	if options == nil {
		options = DefaultCodeBlockProcessingOptions()
	}

	slog.Debug("processing code blocks", "detectLanguage", options.DetectLanguage, "formatCode", options.FormatCode)

	// Process code blocks with the same selector logic as TypeScript
	selector := []string{
		"pre",
		"div[class*=\"prismjs\"]",
		".syntaxhighlighter",
		".highlight",
		".highlight-source",
		".wp-block-syntaxhighlighter-code",
		".wp-block-code",
		"div[class*=\"language-\"]",
	}

	combinedSelector := strings.Join(selector, ", ")
	slog.Debug("using code block selector", "selector", combinedSelector)

	var processedCount int
	p.doc.Find(combinedSelector).Each(func(_ int, s *goquery.Selection) {
		p.processCodeBlock(s, options)
		processedCount++
	})

	slog.Info("code blocks processed", "count", processedCount)
}
