// Package constants provides configuration constants and selectors for the defuddle content extraction system.
// It includes CSS selectors for finding main content, removing unwanted elements, and processing footnotes.
package constants

// EntryPointElements are the elements that will be used to find the main content
// JavaScript original code:
// export const ENTRY_POINT_ELEMENTS = [
//
//	'#post',
//	'.post-content',
//	'.article-content',
//	'#article-content',
//	'.article_post',
//	'.article-wrapper',
//	'.entry-content',
//	'.content-article',
//	'.post',
//	'.markdown-body',
//	'article',
//	'[role="article"]',
//	'main',
//	'[role="main"]',
//	'body' // ensures there is always a match
//
// ];
var EntryPointElements = []string{
	"#post",
	".post-content",
	".post-body",
	".article-content",
	"#article-content",
	".article_post",
	".article-wrapper",
	".entry-content",
	".content-article",
	".instapaper_body",
	".post",
	".markdown-body",
	"article",
	`[role="article"]`,
	"main",
	`[role="main"]`,
	".article-body",
	"#content",
	"body", // ensures there is always a match
}

// MobileWidth is the width threshold for mobile styles
// JavaScript original code:
// export const MOBILE_WIDTH = 600;
const MobileWidth = 600

// BlockElements are HTML block-level elements
// JavaScript original code:
// export const BLOCK_ELEMENTS = ['div', 'section', 'article', 'main', 'aside', 'header', 'footer', 'nav', 'content'];
var BlockElements = []string{
	"div", "section", "article", "main", "aside", "header", "footer", "nav", "content",
}

// PreserveElements are elements that should not be unwrapped
// JavaScript original code:
// export const PRESERVE_ELEMENTS = new Set([
//
//	'pre', 'code', 'table', 'thead', 'tbody', 'tr', 'td', 'th',
//	'ul', 'ol', 'li', 'dl', 'dt', 'dd',
//	'figure', 'figcaption', 'picture',
//	'details', 'summary',
//	'blockquote',
//	'form', 'fieldset'
//
// ]);
var PreserveElements = map[string]bool{
	"pre": true, "code": true, "table": true, "thead": true, "tbody": true, "tr": true, "td": true, "th": true,
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"figure": true, "figcaption": true, "picture": true,
	"details": true, "summary": true,
	"blockquote": true,
	"form":       true, "fieldset": true,
}

// InlineElements are inline elements that should not be unwrapped
// JavaScript original code:
// export const INLINE_ELEMENTS = new Set([
//
//	'a', 'span', 'strong', 'em', 'i', 'b', 'u', 'code', 'br', 'small',
//	'sub', 'sup', 'mark', 'date', 'del', 'ins', 'q', 'abbr', 'cite', 'relative-time', 'time',
//	'font'
//
// ]);
var InlineElements = map[string]bool{
	"a": true, "span": true, "strong": true, "em": true, "i": true, "b": true, "u": true, "code": true, "br": true, "small": true,
	"sub": true, "sup": true, "mark": true, "date": true, "del": true, "ins": true, "q": true, "abbr": true, "cite": true, "relative-time": true, "time": true,
	"font": true,
}
