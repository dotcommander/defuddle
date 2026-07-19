# CLI Reference

`defuddle` extracts clean, readable content from web pages — stripping navigation, ads, and clutter. It reads a URL, a local HTML file, or HTML piped on stdin, and emits HTML, Markdown, or JSON.

## Synopsis

```
Usage: defuddle <command> [flags]

Extract and structure content from web pages.

Commands:
  parse (p)       Parse and extract content from a URL, HTML file, or stdin.
    [<source>]    URL or HTML file; reads stdin when omitted.

  extractors      List registered site-specific extractors.

  batch           Parse multiple URLs, output JSONL.

Flags:
  -h, --help       Show context-sensitive help.
      --version    Print version information and quit.

Run "defuddle <command> --help" for more information on a command.
```

There are no persistent/global flags beyond `--version` and `--help`; each command owns its own flags.

## Installation

```bash
go install github.com/dotcommander/defuddle/cmd/defuddle@latest
```

## parse

Extract content from a single source.

### Input: URL, file, or stdin

`parse` selects its input in this order:

1. A positional argument — `defuddle parse <source>`. If it starts with `http://` or `https://` it is fetched as a URL; otherwise it is read as a local file path.
2. `-` as the argument — read HTML from stdin explicitly.
3. No argument, but HTML is piped in — stdin is used automatically.

```bash
defuddle parse https://example.com/article      # URL
defuddle parse ./page.html                       # local file
curl -s https://example.com | defuddle parse     # piped stdin (auto)
curl -s https://example.com | defuddle parse -   # explicit stdin
```

Local input (files and stdin) is capped at **5 MiB**; larger input returns an error wrapping `defuddle.ErrTooLarge` with the source path or `stdin` in the message. URL fetches have their own internal cap enforced by the library.

### Output formats

By default `parse` prints the extracted **HTML** content to stdout. Change the format with:

```bash
defuddle parse URL --markdown          # Markdown (-m; --md is an alias)
defuddle parse URL --json              # full JSON: content + all metadata
defuddle parse URL --property title    # a single field, raw
defuddle parse URL --output out.html   # write to a file instead of stdout (-o)
```

When more than one is set, precedence is `--property` > `--json` > `--markdown` > default HTML. `--markdown` falls back to HTML if no markdown was produced.

`--property` accepts: `content`, `title`, `description`, `domain`, `favicon`, `image`, `author`, `site`, `published`, `wordCount`, `parseTime`, `metaTags`, `schemaOrgData`, `extractorType`, `contentMarkdown`.

See [JSON output](#json-output) for the full object shape.

### JavaScript rendering (opt-in)

By default `parse` fetches the raw server HTML and does **not** execute JavaScript — client-rendered (SPA) pages may come back nearly empty. Pass `--render` (alias `--js`) to render the page in a headless browser first, then extract:

```bash
defuddle parse --render https://example.com/spa-article
```

This requires an existing **Chrome or Chromium** install — defuddle drives it over CDP and bundles no browser. If Chrome is not found, point at one with `--chrome-path`, or install Chrome/Chromium.

```bash
# Wait for the network to settle (good for lazy-loaded content)
defuddle parse --render --render-wait networkidle https://example.com

# Point at a specific browser, cap render time, and set a user agent
defuddle parse --render \
  --chrome-path "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  --render-timeout 45s \
  --render-user-agent "MyBot/1.0" \
  https://example.com
```

`--render-auto` first fetches the static HTML and escalates to Chrome only when
the page looks like a JavaScript shell. If Chrome is unavailable, it falls back
to parsing the static HTML. `--render-wait` accepts `load` (default; snapshot
after the load event) or `networkidle` (snapshot after a brief network-idle
settle). Use `--render-wait-for` for a stable CSS selector, or `--render-settle`
for a fixed post-load delay. Render tuning flags only affect a render path.

### HTTP flags

```bash
# Custom timeout
defuddle parse https://example.com --timeout 60s

# Custom user agent
defuddle parse https://example.com --user-agent "MyBot/1.0"

# Custom headers (repeatable)
defuddle parse https://example.com -H "Authorization: Bearer token123"
defuddle parse https://example.com -H "Cookie: session=abc" -H "Accept-Language: en"

# Route through a proxy
defuddle parse https://example.com --proxy http://localhost:8080
defuddle parse https://example.com --proxy socks5://localhost:1080
```

Headers must use the `Key: Value` form. Invalid headers are rejected before any HTTP request is issued. Proxy URLs accept `http://`, `https://`, and `socks5://` schemes.

### Content control flags

```bash
# Remove all images from output
defuddle parse https://example.com --remove-images

# Force a specific content root (bypass auto-detection)
defuddle parse https://example.com --content-selector "article.post-body"

# Disable all clutter removal (return everything)
defuddle parse https://example.com --no-clutter-removal

# Debug mode (shows removed elements, timings, statistics)
defuddle parse https://example.com --debug
```

### Flag reference (parse)

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--json` | `-j` | bool | false | Output as JSON with metadata and content |
| `--tables-json` | | bool | false | Output detected tables as structured JSON |
| `--markdown` | `-m` | bool | false | Convert content to markdown format |
| `--md` | | bool | false | Alias for `--markdown` |
| `--property` | `-p` | string | | Extract a single property (e.g. `title`, `author`) |
| `--output` | `-o` | string | stdout | Write output to a file |
| `--user-agent` | | string | | Custom user agent string (default: built-in) |
| `--header` | `-H` | string[] | | Custom HTTP headers, `Key: Value` (repeatable) |
| `--timeout` | | duration | 30s | HTTP request timeout |
| `--proxy` | | string | | Proxy URL (`http://`, `https://`, `socks5://`) |
| `--debug` | | bool | false | Enable debug output |
| `--remove-images` | | bool | false | Strip images from content |
| `--content-selector` | | string | | CSS selector for content root (bypasses auto-detection) |
| `--no-clutter-removal` | | bool | false | Disable all clutter removal heuristics |
| `--render` | | bool | false | Render JavaScript via headless Chrome before extracting |
| `--render-auto` | | bool | false | Render only pages detected as JavaScript-heavy |
| `--js` | | bool | false | Alias for `--render` |
| `--render-wait` | | string | load | Render wait strategy: `load` or `networkidle` |
| `--render-wait-for` | | string | | Wait for a visible CSS selector before snapshot |
| `--render-settle` | | duration | 0 | Extra post-load delay before snapshot |
| `--render-user-agent` | | string | | User agent for the render stage (default: Chrome default) |
| `--chrome-path` | | string | | Path to a Chrome/Chromium executable (default: auto-detect) |
| `--render-timeout` | | duration | 30s | Maximum time to spend rendering the page |

## batch

Parse multiple URLs concurrently. Reads one URL per line from stdin (default) or a file, and outputs JSONL — one JSON object per line.

```bash
# From stdin
echo -e "https://example.com/a\nhttps://example.com/b" | defuddle batch

# From file
defuddle batch --input urls.txt

# Control concurrency
defuddle batch --input urls.txt --concurrency 10

# Include markdown in output
defuddle batch --input urls.txt --markdown

# Skip failures instead of stopping
defuddle batch --input urls.txt --continue-on-error

# Bound total batch duration (0 = no overall deadline)
defuddle batch --input urls.txt --timeout 2m

# Save results
defuddle batch --input urls.txt > results.jsonl
```

Blank lines and lines beginning with `#` are skipped. Each input line is bounded at 64 KiB; longer lines surface as an error rather than being silently truncated. Results are written in input order.

### Output format

`batch` writes one JSON object per line (JSONL) to stdout. Successful results emit the full `defuddle.Result` JSON on their own line. With `--continue-on-error`, a failed URL emits a per-line error object instead of aborting:

```json
{"url":"https://example.com/broken","error":"<message>"}
```

Without `--continue-on-error`, the first failure terminates the batch with a non-zero exit.

### Flag reference (batch)

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--input` | `-i` | string | stdin | Read URLs from a file instead of stdin |
| `--concurrency` | `-c` | int | 5 | Maximum concurrent requests |
| `--markdown` | `-m` | bool | false | Include markdown in output |
| `--continue-on-error` | | bool | false | Continue processing on individual URL errors |
| `--timeout` | | duration | 0 | Overall batch deadline (e.g. `30s`, `2m`); `0` disables |

## extractors

List all registered site-specific extractors.

```bash
defuddle extractors
```

Check which extractor matches a URL:

```bash
defuddle extractors --match https://github.com/dotcommander/defuddle/issues/1
```

### Flag reference (extractors)

| Flag | Type | Description |
|------|------|-------------|
| `--match` | string | Show which extractor matches the given URL |

## JSON output

`--json` (parse) and every successful line of `batch` emit a `defuddle.Result` object.

Always present:

`content`, `title`, `description`, `domain`, `favicon`, `image`, `language`, `parseTime`, `published`, `author`, `site`, `schemaOrgData`, `wordCount`

Present when applicable (omitted when empty):

`contentMarkdown`, `extractorType`, `variables`, `metaTags`, `debugInfo`

```bash
defuddle parse https://example.com --json | jq '{title, author, wordCount}'
```

## Exit codes & output streams

- Extracted content is written to **stdout**; status notices (e.g. `Output written to <file>`) and error messages go to **stderr**, so a piped stdout stays clean.
- The exit code classifies the failure so a calling script or agent can branch without parsing stderr. `0` is success; any error the CLI does not recognize stays `1` (unchanged from prior behavior). Recognized failure categories use codes `2`–`6`:

| Code | Category | Meaning | Example |
|------|----------|---------|---------|
| `0` | success | Command completed | — |
| `1` | error | Unclassified error (fallback) | JSON marshal failure, HTTP client build failure |
| `2` | validation | Bad flags, arguments, or input | Malformed `-H` header, unknown `--property`, non-HTML input, input over the 5 MiB cap |
| `3` | not_found | Source file / input file missing | `defuddle parse ./missing.html`, `batch --input missing.txt` |
| `4` | upstream | Fetch / HTTP / network failure | Non-2xx HTTP status, `304 Not Modified` |
| `5` | precondition | Operator action needed | `--render` with no Chrome/Chromium installed |
| `6` | cancelled | Context cancelled or timeout | `--timeout` / `--render-timeout` exceeded, interrupted |

- The classification is applied to the single top-level error via `errors.Is`; the error message is printed to stderr regardless of code. Example: `--render` on a machine without Chrome exits `5` with `chrome/chromium not found: install Google Chrome or Chromium, or set --chrome-path to an existing executable`.

## Version

```bash
defuddle --version
# defuddle version 0.7.3 (commit: abc1234, built: 2026-06-16)
```

The version, commit, and build date are injected at build time; a plain `go build` reports `dev`.

## Examples

### Extract article content as markdown

```bash
defuddle parse https://blog.example.com/post --markdown
```

### Extract a JavaScript-rendered page

```bash
defuddle parse --render --render-wait networkidle https://example.com/spa-article --markdown
```

### Get just the title

```bash
defuddle parse https://example.com --property title
```

### Batch extract with markdown, saving results

```bash
cat urls.txt | defuddle batch --markdown --continue-on-error > results.jsonl
```

### Debug extraction issues

```bash
defuddle parse https://example.com --debug --json 2>/dev/null | jq '.debugInfo'
```

### Parse behind authentication

```bash
defuddle parse https://example.com/private \
  -H "Authorization: Bearer mytoken" \
  -H "Cookie: session=abc123"
