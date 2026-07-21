package elements

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
// const CODE_LANGUAGES = new Set([
//
//	'abap', 'actionscript', 'ada', 'adoc', 'agda', 'antlr4',
//	// ... extensive language list ...
//
// ]);
// isCodeLanguage checks if a language is in the supported languages set
func (p *CodeBlockProcessor) isCodeLanguage(lang string) bool {
	return codeLanguages[lang]
}

// ProcessCodeBlocks processes all code blocks in the document (public interface)
// TypeScript original code:
//
//	export function processCodeBlocks(doc: Document, options?: CodeBlockOptions): void {
//	  const processor = new CodeBlockProcessor(doc);
//	  processor.processAllCodeBlocks(options || defaultOptions);
//	}
func ProcessCodeBlocks(doc *goquery.Document, options *CodeBlockProcessingOptions) {
	processor := NewCodeBlockProcessor(doc)
	processor.ProcessCodeBlocks(options)
}

// ProcessCodeBlocksInScope processes code blocks within the given container element.
func ProcessCodeBlocksInScope(scope *goquery.Selection, options *CodeBlockProcessingOptions) {
	processor := &CodeBlockProcessor{}
	if options == nil {
		options = DefaultCodeBlockProcessingOptions()
	}
	selector := strings.Join([]string{
		"pre",
		`div[class*="prismjs"]`,
		".syntaxhighlighter",
		".highlight",
		".highlight-source",
		".wp-block-syntaxhighlighter-code",
		".wp-block-code",
		`div[class*="language-"]`,
	}, ", ")
	scope.Find(selector).Each(func(_ int, s *goquery.Selection) {
		processor.processCodeBlock(s, options)
	})
}
