package main

// Tests for the typed exit-code contract (exitCodeFor). Each case wraps a
// sentinel the way its real error path does and asserts the documented code,
// plus the nil (success) and unclassified (fallback) cases.

import (
	"context"
	"fmt"
	"io/fs"
	"testing"

	"github.com/dotcommander/defuddle"
	"github.com/dotcommander/defuddle/cmd/defuddle/internal/render"
	"github.com/stretchr/testify/assert"
)

func TestExitCodeFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil is success", nil, exitOK},
		{"unclassified falls back", assert.AnError, exitError},
		{"invalid header is validation", fmt.Errorf("wrap: %w", ErrInvalidHeaderFormat), exitValidation},
		{"directory traversal is validation", ErrDirectoryTraversal, exitValidation},
		{"no urls is validation", ErrNoURLs, exitValidation},
		{"property not found is validation", fmt.Errorf("x: %w", ErrPropertyNotFound), exitValidation},
		{"parse usage is validation", ErrParseUsage, exitValidation},
		{"invalid match url is validation", ErrInvalidMatchURL, exitValidation},
		{"invalid concurrency is validation", ErrInvalidConcurrency, exitValidation},
		{"not html is validation", defuddle.ErrNotHTML, exitValidation},
		{"too large is validation", fmt.Errorf("read stdin: %w", defuddle.ErrTooLarge), exitValidation},
		{"missing file is not_found", fmt.Errorf("error reading file: %w", fs.ErrNotExist), exitNotFound},
		{"http status is upstream", fmt.Errorf("error loading content: %w", defuddle.ErrHTTPStatus), exitUpstream},
		{"not modified is upstream", defuddle.ErrNotModified, exitUpstream},
		{"chrome not found is precondition", fmt.Errorf("%w: exec", render.ErrChromeNotFound), exitPrecondition},
		{"timeout is cancelled", defuddle.ErrTimeout, exitCancelled},
		{"context deadline is cancelled", fmt.Errorf("render x timed out: %w", context.DeadlineExceeded), exitCancelled},
		{"context canceled is cancelled", context.Canceled, exitCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, exitCodeFor(tt.err))
		})
	}
}
