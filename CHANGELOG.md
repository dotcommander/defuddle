# Changelog

Defuddle Go is a port of the [Defuddle](https://github.com/kepano/defuddle) TypeScript library that extracts clean, readable content from web pages. This changelog covers the Go port; releases are also published on the [GitHub releases page](https://github.com/dotcommander/defuddle/releases).

All notable changes to this project will be documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [v0.10.0] — 2026-06-24

### Added

- `ErrNotModified` sentinel: `ParseFromURL` now returns `ErrNotModified` (test with `errors.Is`) when a conditional request — a caller-supplied `If-None-Match` / `If-Modified-Since` header — gets an HTTP 304 response, instead of surfacing it as a generic `HTTPStatusError`.

### Security

- Block `data:image/svg+xml` URIs in the content sanitizer. An SVG data-URI (which can embed `<script>`) in an attribute such as `<img src>` previously passed through `Result.Content` verbatim; it is now stripped, matching the existing handling of other dangerous `data:` subtypes (`data:text/html`, `data:text/javascript`, `data:application/javascript`). Benign image data-URIs (`data:image/png`, etc.) are preserved.

---

## [v0.8.0] — 2026-06-16

### Added

- Opt-in JavaScript rendering for the CLI: `defuddle parse --render` (alias `--js`) renders a page in headless Chrome (via chromedp) before extraction — for JS-heavy / single-page sites that return little or no content from a static fetch. Tune with `--render-wait` (`load` or `networkidle`), `--render-timeout`, `--render-user-agent`, and `--chrome-path`. Requires an existing Chrome/Chromium install; no browser is bundled. Without `--render`, behavior is unchanged (static HTTP fetch, no JS execution).

### Changed

- The CLI is now a separate Go module (`github.com/dotcommander/defuddle/cmd/defuddle`) so the library module stays dependency-light — chromedp never enters the library's module graph. Install the CLI with `go install github.com/dotcommander/defuddle/cmd/defuddle@latest`; import the library with `go get github.com/dotcommander/defuddle`.

### Documentation

- Rewrote the CLI reference (`docs/cli.md`) to cover every command, flag, input/output mode, the JSON output shape, and exit codes.
- Documented `--render` across the README and docs, and added a Documentation index to the README.

---

## [v0.7.3] — 2026-06-10

### Fixed

- Sanitize site-specific extractor output before returning `Result.Content`, matching the generic parser sanitizer path.
- Honor `ProcessCode`, `ProcessImages`, `ProcessHeadings`, `ProcessMath`, `ProcessFootnotes`, and `ProcessRoles` options during standardization.
- Cap `ParseFromURL` response reads before buffering the body, returning `ErrTooLarge` for oversized responses.
- Return structured `ErrHTTPStatus` / `HTTPStatusError` for non-2xx URL fetches instead of parsing error pages.
- Resolve implicit metadata URLs against the final redirect target while preserving an explicit caller-supplied `Options.URL`.
- Sync selected upstream parser fixes from `kepano/defuddle`: ChatGPT split assistant messages, YouTube JSON-LD video metadata selection, markdown link destinations with spaces, and weekday-aware byline cleanup.

### Changed

- `task verify` now runs `govulncheck ./...` through the new `task vuln` gate.

---

## [Unreleased]

---

## [v0.7.2] — 2026-05-29

### Fixed

- `fix(extractors/grok): extract body inner HTML instead of full document wrapper`

### Changed

- `refactor(scoring): single-pass anchor metrics in scoreNonContentBlock`

---

### Fixed

- `fix(metadata): Language field now populated in buildMetadata` — `getLanguage` was called in `metadata.Extract` but the result was not forwarded through `buildMetadata`, causing `Result.Language` to always be empty. The field now correctly reflects the page's `<html lang>`, `content-language` meta, `og:locale`, or Schema.org `inLanguage`.

### Changed

- `docs: rewrite quickstart to lead with CLI and ParseFromURL; add ParseFromString as primary single-call form`
- `docs: rewrite custom extractor example with working CanExtract selector and Variables usage`
- `docs: add Limitations section to README and new docs/limitations.md`
- `docs: add docs/recipes.md with six concrete workflows (batch, vault, RAG, cookies, headless)`
- `docs: rewrite error sentinels in docs/library.md as a table with trigger and handling guidance`
- `docs: document all 12 Metadata fields including Language`
- `defuddle: HTTP client default timeout unified to 30s (was 10s) to match the CLI default; eliminates surprise when ParseFromURL is called without a custom client`

### Removed

- `errors: drop unused ErrNoContent sentinel — it had no triggering code path; callers branching on it via errors.Is were dead code`

---

## [v0.5.3] — 2026-04-25

### Changed

- `ParseFromURLs` now uses `errgroup.WithContext` with `g.SetLimit(limit)` in place of a hand-rolled semaphore and `sync.WaitGroup`. Per-slot error semantics are preserved.
- `internal/standardize`: replaced `\w` regex character class with an `isWordChar` ASCII helper function, eliminating per-call allocations.
- `internal/constants`: `GetInlineElements` and `GetAllowedEmptyElements` now use `slices.Collect(maps.Keys(...))` followed by `slices.Sort` to produce deterministic ordering without manual accumulation.
- `golang.org/x/sync` promoted to a direct module dependency.

### Removed

- GitHub Actions workflow files removed from the repository.

---

## [v0.5.2]

### Changed

- Pre-compiled CSS selectors and regex fast-path: cascadia matchers are now cached at package init, and a combined-alternation regex serves as an O(1) reject filter for footnote scoring, avoiding repeated compilation.

### Fixed

- Subdomain-aware same-site hostname matching: link removal now uses `publicsuffix.EffectiveTLDPlusOne` instead of `strings.TrimPrefix(host, "www.")`, correctly treating `news.example.com` and `example.com` as the same site.

---

## [v0.5.1]

### Changed

- Extractor registry split from a single `registry.go` into per-category files: `registry_conversation.go`, `registry_news.go`, `registry_social.go`, `registry_tech.go`, and `registry_catchall.go`. No behavior change — organizational refactor only.

---

## [v0.5.0]

### Added

- **Wikipedia** extractor (`*.wikipedia.org`) — article body with section structure.
- **Medium** extractor (`medium.com`) — articles with author and publication metadata.
- **NYTimes** extractor (`nytimes.com`) — news articles with author and section metadata.
- **LWN** extractor (`lwn.net`) — Linux Weekly News articles and subscriber content.
- **C2 Wiki** extractor (`c2.com`) — wiki pages.
- **X oEmbed** extractor (`publish.twitter.com`, `publish.x.com`) — embedded tweet markup.
- **LeetCode** extractor (`leetcode.com`) — problem statements and editorial content.
- **Discourse** extractor (DOM-signature, any host) — forum topics and reply threads.
- **LinkedIn** extractor (`linkedin.com`) — posts and articles.
- Upstream extractor sync checker script for tracking parity with the TypeScript library.

---

## Earlier releases

See the [git history](https://github.com/dotcommander/defuddle/commits/main) for changes prior to v0.5.0. Curated notes are not available for those releases.
