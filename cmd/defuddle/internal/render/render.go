// Package render drives an existing Chrome/Chromium over CDP (chromedp) to
// produce fully-rendered HTML for JS-heavy pages. It bundles no browser; if no
// Chrome binary is found it returns ErrChromeNotFound with remediation guidance.
package render

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// WaitStrategy selects when the rendered HTML snapshot is taken.
type WaitStrategy string

const (
	// WaitLoad snapshots after the page load event.
	WaitLoad WaitStrategy = "load"
	// WaitIdle snapshots after a network-idle settle delay.
	WaitIdle WaitStrategy = "networkidle"
)

// ErrChromeNotFound is returned when chromedp cannot locate a Chrome/Chromium
// binary. The message tells the user how to install Chrome or point at one.
var ErrChromeNotFound = errors.New("chrome/chromium not found: install Google Chrome or Chromium, or set --chrome-path to an existing executable")

// Config holds render-stage settings sourced from CLI flags (never hardcoded).
// Zero values mean "use chromedp defaults" so an unset flag changes nothing.
type Config struct {
	// ChromePath overrides the Chrome executable path. Empty => chromedp auto-detects.
	ChromePath string
	// UserAgent overrides the navigator user-agent. Empty => Chrome default.
	UserAgent string
	// Wait selects the snapshot point. Empty => WaitLoad.
	Wait WaitStrategy
	// IdleDelay is the settle delay applied for WaitIdle. Zero => 500ms when WaitIdle.
	IdleDelay time.Duration
	// MaxHTMLBytes caps the rendered HTML returned. Zero => no extra cap
	// (the library applies its own 5 MiB ceiling downstream).
	MaxHTMLBytes int
}

// RenderHTML navigates Chrome to url under the caller's ctx (which MUST carry
// the nav/render deadline), waits per cfg.Wait, and returns
// document.documentElement.outerHTML. The caller's ctx bounds the whole
// operation; RenderHTML adds no deadline of its own.
func RenderHTML(ctx context.Context, url string, cfg Config) (string, error) {
	allocOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	if cfg.ChromePath != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(cfg.ChromePath))
	}
	if cfg.UserAgent != "" {
		allocOpts = append(allocOpts, chromedp.UserAgent(cfg.UserAgent))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx,
		chromedp.WithErrorf(func(format string, args ...any) {
			msg := fmt.Sprintf(format, args...)
			if strings.Contains(msg, "could not unmarshal event") {
				return
			}
			fmt.Fprintf(os.Stderr, "%s\n", msg)
		}),
	)
	defer cancelBrowser()

	var html string
	tasks := chromedp.Tasks{chromedp.Navigate(url)}
	if cfg.Wait == WaitIdle {
		delay := cfg.IdleDelay
		if delay <= 0 {
			delay = 500 * time.Millisecond
		}
		tasks = append(tasks, chromedp.Sleep(delay))
	}
	tasks = append(tasks, chromedp.OuterHTML("html", &html, chromedp.ByQuery))

	if err := chromedp.Run(browserCtx, tasks); err != nil {
		// chromedp surfaces a missing/unstartable browser as an exec error.
		if isChromeStartFailure(err) {
			return "", fmt.Errorf("%w: %v", ErrChromeNotFound, err)
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("render %s timed out: %w", url, ctx.Err())
		}
		return "", fmt.Errorf("render %s: %w", url, err)
	}

	if cfg.MaxHTMLBytes > 0 && len(html) > cfg.MaxHTMLBytes {
		html = html[:cfg.MaxHTMLBytes]
	}
	return html, nil
}

// isChromeStartFailure reports whether err indicates Chrome could not be
// located or launched (as opposed to a navigation/runtime error).
func isChromeStartFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "exec:") ||
		strings.Contains(msg, "failed to start")
}
