package defuddle

import (
	"errors"
	"fmt"
)

// Sentinel errors for caller-branching logic via errors.Is().
var (
	// ErrNotHTML is returned when the fetched content is not HTML.
	ErrNotHTML = errors.New("defuddle: content is not HTML")

	// ErrTooLarge is returned when the fetched content exceeds the size limit.
	ErrTooLarge = errors.New("defuddle: content exceeds size limit")

	// ErrTimeout is returned when a fetch operation times out.
	ErrTimeout = errors.New("defuddle: request timed out")

	// ErrHTTPStatus is returned when a fetched URL responds with a non-2xx HTTP status.
	ErrHTTPStatus = errors.New("defuddle: unexpected HTTP status")
)

// HTTPStatusError reports a non-success HTTP response from ParseFromURL.
type HTTPStatusError struct {
	URL        string
	Status     string
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return ErrHTTPStatus.Error()
	}
	if e.URL == "" {
		return fmt.Sprintf("%s: %s", ErrHTTPStatus, e.Status)
	}
	return fmt.Sprintf("%s for %s: %s", ErrHTTPStatus, e.URL, e.Status)
}

func (e *HTTPStatusError) Unwrap() error {
	return ErrHTTPStatus
}
