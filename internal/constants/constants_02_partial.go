package constants

import (
	"slices"
)

// PartialSelectors are removal patterns tested against attributes above
// Case insensitive, partial matches allowed
// JavaScript original code: (first part of PARTIAL_SELECTORS array)
var PartialSelectors = slices.Concat(partialSelectorsA, partialSelectorsB)
