package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/dotcommander/defuddle"
)

var errRemediationUnexpected = errors.New("unexpected")

func TestCLIWrappedNetworkAndValidationPrecedence(t *testing.T) {
	t.Parallel()
	network := &url.Error{Op: "Get", URL: "https://example.com", Err: &net.DNSError{Err: "no such host", Name: "example.com"}}
	for _, tc := range []struct {
		err  error
		code int
	}{
		{fmt.Errorf("outer: %w", network), exitUpstream},
		{errors.Join(network, context.Canceled), exitCancelled},
		{errors.Join(network, context.DeadlineExceeded), exitCancelled},
		{errors.Join(network, defuddle.ErrTooLarge), exitValidation},
		{errRemediationUnexpected, exitError},
	} {
		if got := exitCodeFor(tc.err); got != tc.code {
			t.Fatalf("err=%v code=%d want=%d", tc.err, got, tc.code)
		}
	}
}

func TestCLIActualOversizedScannerInputIsValidation(t *testing.T) {
	t.Parallel()
	_, err := scanURLs(strings.NewReader(strings.Repeat("x", maxURLLineSize+1)))
	if !errors.Is(err, bufio.ErrTooLong) || !errors.Is(err, ErrCLIUsage) || exitCodeFor(err) != exitValidation {
		t.Fatalf("err=%v code=%d", err, exitCodeFor(err))
	}
}
