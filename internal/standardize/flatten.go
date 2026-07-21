package standardize

import (
	"regexp"
)

var (
	semanticClassRe = regexp.MustCompile(`(?:article|main|content|footnote|reference|bibliography)`)
	wrapperClassRe  = regexp.MustCompile(`(?:wrapper|container|layout|row|col|grid|flex|outer|inner|content-area)`)
	// additionalBlockElements supplements constants.GetBlockElements() with heading/list elements
	// that should be treated as block-level during flattening and empty-element cleanup.
	additionalBlockElements = []string{"p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "pre", "blockquote", "figure"}
)
