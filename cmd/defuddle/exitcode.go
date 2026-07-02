// Package main: typed exit-code contract for agent-friendly control flow.
//
// The CLI classifies its single top-level error (main → rootCmd.Execute) into a
// small, stable set of exit codes so a calling process or agent can branch on
// the failure category without parsing stderr text. Success is 0 and any error
// the mapper does not recognize stays 1 (drop-in compatible with the prior
// "0 or 1" behavior); only recognized failure categories return 2–6.
package main

import (
	"context"
	"errors"
	"io/fs"

	"github.com/dotcommander/defuddle"
	"github.com/dotcommander/defuddle/cmd/defuddle/internal/render"
)

// Exit-code contract. Stable and documented in docs/cli.md ("Exit codes &
// output streams"). Do not renumber — callers depend on these values.
const (
	exitOK           = 0 // success
	exitError        = 1 // unclassified error (fallback)
	exitValidation   = 2 // bad flags/args/input
	exitNotFound     = 3 // source file / input missing
	exitUpstream     = 4 // fetch / HTTP / network failure
	exitPrecondition = 5 // operator action needed (e.g. Chrome not installed)
	exitCancelled    = 6 // context cancelled / timeout
)

// exitCodeFor classifies err into the exit-code contract via errors.Is. It is
// the single source of truth for the process exit status; main calls it once at
// the top-level error boundary. An unrecognized error returns exitError so
// behavior stays drop-in with the prior 0/1 contract.
func exitCodeFor(err error) int {
	if err == nil {
		return exitOK
	}
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, defuddle.ErrTimeout):
		return exitCancelled
	case errors.Is(err, render.ErrChromeNotFound):
		return exitPrecondition
	case errors.Is(err, defuddle.ErrHTTPStatus),
		errors.Is(err, defuddle.ErrNotModified):
		return exitUpstream
	case errors.Is(err, fs.ErrNotExist):
		return exitNotFound
	case errors.Is(err, ErrInvalidHeaderFormat),
		errors.Is(err, ErrDirectoryTraversal),
		errors.Is(err, ErrNoURLs),
		errors.Is(err, ErrPropertyNotFound),
		errors.Is(err, ErrParseUsage),
		errors.Is(err, ErrInvalidMatchURL),
		errors.Is(err, ErrInvalidConcurrency),
		errors.Is(err, defuddle.ErrNotHTML),
		errors.Is(err, defuddle.ErrTooLarge):
		return exitValidation
	default:
		return exitError
	}
}
