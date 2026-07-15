# Changelog

Defuddle Go is a port of the [Defuddle](https://github.com/kepano/defuddle) TypeScript library that extracts clean, readable content from web pages. This changelog covers the Go port; releases are also published on the [GitHub releases page](https://github.com/dotcommander/defuddle/releases).

All notable changes to this project will be documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

---

## [v0.13.0] — 2026-07-14

### Added

- `Options.Headers` for request headers shared by `ParseFromURL` and
  `ParseFromURLs`, including conditional cache validators.

### Changed

- `Options.Client` now accepts the standard library's `*http.Client`, removing
  the public dependency on `github.com/kaptinlin/requests`. This is a breaking
  API change for callers that supplied a custom requests client.
- The CLI now uses Kong instead of Cobra while preserving the `parse` (`p`),
  `batch`, and `extractors` commands, their flags, and typed exit behavior.
- Release automation now verifies the CLI module with `GOWORK=off` after it is
  pinned to the newly published library version and before its module tag is
  created.

### Removed

- The library's `github.com/kaptinlin/requests` dependency and the CLI's Cobra
  dependency.

---

## [v0.12.0] — 2026-07-02

### Added

- Typed CLI exit codes for usage, input, network, render, and extraction
  failures.
- `--render-auto` to detect JavaScript shell pages and render them only when
  needed, with a static-HTML fallback when Chrome is unavailable.

### Changed

- Updated library dependencies and the Go toolchain requirement to 1.26.4.

---

## [v0.11.0] — 2026-06-24

### Added
- `ExtractTables(html string) ([]Table, error)` plus the `Table` type (`{Caption, Headers, Rows}`): parse an HTML fragment into structured tables, reading header text directly from `<thead>`/`<th>` cells (falling back to the first row of `<th>` cells).
- CLI `--tables-json`: emit every table detected in the parsed content as structured JSON (`[{caption, headers, rows}]`), so consumers can select columns by header name instead of by position.
- CLI `--render-wait-for <css-selector>`: block the `--render` snapshot until a node matching the selector becomes visible (bounded by the render timeout) — for tables hydrated by JS after the load event.
- CLI `--render-settle <dur>`: extra settle delay applied after load before the `--render` snapshot, for SPAs that have no stable selector to wait on.

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
