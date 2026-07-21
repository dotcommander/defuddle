package elements

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TypeScript original code:
//
//	transform: (el: Element, doc: Document): Element => {
//	  const getCodeLanguage = (element: Element): string => {
//	    const dataLang = element.getAttribute('data-lang') || element.getAttribute('data-language');
//	    if (dataLang) {
//	      return dataLang.toLowerCase();
//	    }
//	    // Check class names for patterns and supported languages
//	    const classNames = Array.from(element.classList || []);
//	    // Pattern matching logic...
//	  };
//
//	  let language = '';
//	  let currentElement: Element | null = el;
//	  while (currentElement && !language) {
//	    language = getCodeLanguage(currentElement);
//	    currentElement = currentElement.parentElement;
//	  }
//	}
//
// processCodeBlock processes a single code block
func (p *CodeBlockProcessor) processCodeBlock(s *goquery.Selection, options *CodeBlockProcessingOptions) {
	slog.Debug("processing individual code block")

	// Detect language using hierarchical approach like TypeScript
	var language string
	if options.DetectLanguage {
		language = p.detectLanguageHierarchical(s)
		if language != "" {
			slog.Debug("detected language", "language", language)
		}
	}

	// Extract content using structured text extraction (TypeScript equivalent)
	content := p.extractStructuredContent(s)
	content = p.normalizeCodeContent(content)

	// Format the code block
	if options.FormatCode {
		p.formatCodeBlock(s, language, content, options)
	}
}

// detectLanguageHierarchical detects language using hierarchical approach like TypeScript
// TypeScript original code:
// let language = ”;
// let currentElement: Element | null = el;
//
//	while (currentElement && !language) {
//	  language = getCodeLanguage(currentElement);
//	  // Also check for code elements within the current element
//	  const codeEl = currentElement.querySelector('code');
//	  if (!language && codeEl) {
//	    language = getCodeLanguage(codeEl);
//	  }
//	  currentElement = currentElement.parentElement;
//	}
func (p *CodeBlockProcessor) detectLanguageHierarchical(s *goquery.Selection) string {
	var language string
	current := s

	// Traverse hierarchy like TypeScript implementation
	for current.Length() > 0 && language == "" {
		language = p.getCodeLanguage(current)

		// Also check for code elements within current element
		if language == "" {
			codeEl := current.Find("code").First()
			if codeEl.Length() > 0 {
				language = p.getCodeLanguage(codeEl)
			}
		}

		current = current.Parent()
	}

	return language
}

// getCodeLanguage extracts language from element attributes and classes
// TypeScript original code:
//
//	const getCodeLanguage = (element: Element): string => {
//	  // Check data-lang attribute first
//	  const dataLang = element.getAttribute('data-lang') || element.getAttribute('data-language');
//	  if (dataLang) {
//	    return dataLang.toLowerCase();
//	  }
//
//	  // Check class names for patterns and supported languages
//	  const classNames = Array.from(element.classList || []);
//
//	  // Check for syntax highlighter specific format
//	  if (element.classList?.contains('syntaxhighlighter')) {
//	    const langClass = classNames.find(c => !['syntaxhighlighter', 'nogutter'].includes(c));
//	    if (langClass && CODE_LANGUAGES.has(langClass.toLowerCase())) {
//	      return langClass.toLowerCase();
//	    }
//	  }
//
//	  // Check patterns
//	  for (const className of classNames) {
//	    for (const pattern of HIGHLIGHTER_PATTERNS) {
//	      const match = className.toLowerCase().match(pattern);
//	      if (match && match[1] && CODE_LANGUAGES.has(match[1].toLowerCase())) {
//	        return match[1].toLowerCase();
//	      }
//	    }
//	  }
//
//	  // If all else fails, check for bare language names
//	  for (const className of classNames) {
//	    if (CODE_LANGUAGES.has(className.toLowerCase())) {
//	      return className.toLowerCase();
//	    }
//	  }
//
//	  return '';
//	};
func (p *CodeBlockProcessor) getCodeLanguage(s *goquery.Selection) string {
	// Check data-lang attribute first
	if dataLang, exists := s.Attr("data-lang"); exists && dataLang != "" {
		return strings.ToLower(dataLang)
	}
	if dataLanguage, exists := s.Attr("data-language"); exists && dataLanguage != "" {
		return strings.ToLower(dataLanguage)
	}

	// Get class names for pattern matching
	class, hasClass := s.Attr("class")
	if !hasClass {
		return ""
	}

	classNames := strings.Fields(class)

	// Check for syntax highlighter specific format
	if slices.Contains(classNames, "syntaxhighlighter") {
		for _, className := range classNames {
			if className != "syntaxhighlighter" && className != "nogutter" {
				langLower := strings.ToLower(className)
				if p.isCodeLanguage(langLower) {
					return langLower
				}
			}
		}
	}

	// Check highlighter patterns (same as TypeScript)
	for _, className := range classNames {
		classLower := strings.ToLower(className)
		for _, re := range highlighterPatterns {
			if matches := re.FindStringSubmatch(classLower); len(matches) > 1 {
				lang := matches[1]
				if p.isCodeLanguage(lang) {
					return lang
				}
			}
		}
	}

	// Check for bare language names
	for _, className := range classNames {
		classLower := strings.ToLower(className)
		if p.isCodeLanguage(classLower) {
			return classLower
		}
	}

	return ""
}
