package defuddle

import "errors"

// Sentinel errors for caller-branching logic via errors.Is().
var (
	// ErrNotHTML is returned when the fetched content is not HTML.
	ErrNotHTML = errors.New("defuddle: content is not HTML")

	// ErrTooLarge is returned when the fetched content exceeds the size limit.
	ErrTooLarge = errors.New("defuddle: content exceeds size limit")

	// ErrTimeout is returned when a fetch operation times out.
	ErrTimeout = errors.New("defuddle: request timed out")

	// ErrNoContent is returned when no main content could be extracted.
	ErrNoContent = errors.New("defuddle: no content extracted")
)
