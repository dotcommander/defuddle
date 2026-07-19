package constants

import (
	"maps"
	"slices"
)

// AllowedEmptyElements are elements that are allowed to be empty
// These are not removed even if they have no content
// JavaScript original code:
// export const ALLOWED_EMPTY_ELEMENTS = new Set([
//
//	'area',
//	'audio',
//	'base',
//	'br',
//	'circle',
//	'col',
//	'defs',
//	'ellipse',
//	'embed',
//	'figure',
//	'g',
//	'hr',
//	'iframe',
//	'img',
//	'input',
//	'line',
//	'link',
//	'mask',
//	'meta',
//	'object',
//	'param',
//	'path',
//	'pattern',
//	'picture',
//	'polygon',
//	'polyline',
//	'rect',
//	'source',
//	'stop',
//	'svg',
//	'td',
//	'th',
//	'track',
//	'use',
//	'video',
//	'wbr'
//
// ]);
var AllowedEmptyElements = map[string]bool{
	"area": true, "audio": true, "base": true, "br": true, "circle": true, "col": true, "defs": true,
	"ellipse": true, "embed": true, "figure": true, "g": true, "hr": true, "iframe": true, "img": true,
	"input": true, "line": true, "link": true, "mask": true, "meta": true, "object": true, "param": true,
	"path": true, "pattern": true, "picture": true, "polygon": true, "polyline": true, "rect": true,
	"source": true, "stop": true, "svg": true, "td": true, "th": true, "track": true, "use": true,
	"video": true, "wbr": true,
}

// AllowedAttributes are attributes to keep
// JavaScript original code:
// export const ALLOWED_ATTRIBUTES = new Set([
//
//	'alt',
//	'allow',
//	'allowfullscreen',
//	'aria-label',
//	'checked',
//	'colspan',
//	'controls',
//	'data-latex',
//	'data-src',
//	'data-srcset',
//	'data-lang',
//	'dir',
//	'display',
//	'frameborder',
//	'headers',
//	'height',
//	'href',
//	'lang',
//	'role',
//	'rowspan',
//	'src',
//	'srcset',
//	'title',
//	'type',
//	'width',
//
//	// MathML attributes
//	'accent',
//	'accentunder',
//	'align',
//	'columnalign',
//	'columnlines',
//	'columnspacing',
//	'columnspan',
//	'data-mjx-texclass',
//	'depth',
//	'displaystyle',
//	'fence',
//	'frame',
//	'framespacing',
//	'linethickness',
//	'lspace',
//	'mathsize',
//	'mathvariant',
//	'maxsize',
//	'minsize',
//	'movablelimits',
//	'notation',
//	'rowalign',
//	'rowlines',
//	'rowspacing',
//	'rowspan',
//	'rspace',
//	'scriptlevel',
//	'separator',
//	'stretchy',
//	'symmetric',
//	'voffset',
//	'xmlns'
//
// ]);
var AllowedAttributes = map[string]bool{
	"alt": true, "allow": true, "allowfullscreen": true, "aria-label": true, "checked": true,
	"colspan": true, "controls": true, "data-latex": true, "data-src": true, "data-srcset": true,
	"data-callout": true, "data-lang": true, "dir": true, "display": true, "frameborder": true, "headers": true,
	"height": true, "href": true, "kind": true, "label": true, "lang": true, "role": true, "rowspan": true, "src": true,
	"srclang": true, "srcset": true, "title": true, "type": true, "width": true,

	// MathML attributes
	"accent": true, "accentunder": true, "align": true, "columnalign": true, "columnlines": true,
	"columnspacing": true, "columnspan": true, "data-mjx-texclass": true, "depth": true,
	"displaystyle": true, "fence": true, "frame": true, "framespacing": true, "linethickness": true,
	"lspace": true, "mathsize": true, "mathvariant": true, "maxsize": true, "minsize": true,
	"movablelimits": true, "notation": true, "rowalign": true, "rowlines": true, "rowspacing": true,
	"rspace": true, "scriptlevel": true, "separator": true, "stretchy": true, "symmetric": true,
	"voffset": true, "xmlns": true,
}

// AllowedAttributesDebug are additional attributes to keep in debug mode
// JavaScript original code:
// export const ALLOWED_ATTRIBUTES_DEBUG = new Set([
//
//	'class',
//	'id',
//
// ]);
var AllowedAttributesDebug = map[string]bool{
	"class": true,
	"id":    true,
}

// IsAllowedEmptyElement checks if an element is allowed to be empty
func IsAllowedEmptyElement(tagName string) bool {
	return AllowedEmptyElements[tagName]
}

// IsAllowedAttribute checks if an attribute is allowed
func IsAllowedAttribute(attrName string) bool {
	return AllowedAttributes[attrName]
}

// IsAllowedAttributeDebug checks if an attribute is allowed in debug mode
func IsAllowedAttributeDebug(attrName string) bool {
	return AllowedAttributesDebug[attrName]
}

// GetInlineElements returns a sorted slice of inline element names.
func GetInlineElements() []string {
	s := slices.Collect(maps.Keys(InlineElements))
	slices.Sort(s)
	return s
}

// GetAllowedEmptyElements returns a sorted slice of allowed empty element names.
func GetAllowedEmptyElements() []string {
	s := slices.Collect(maps.Keys(AllowedEmptyElements))
	slices.Sort(s)
	return s
}
