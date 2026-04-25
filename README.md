<p align="center">
<a href="https://github.com/dotcommander/defuddle/releases"><img src="https://img.shields.io/github/v/release/dotcommander/defuddle" alt="Release"></a>
<a href="https://github.com/dotcommander/defuddle/actions"><img src="https://github.com/dotcommander/defuddle/workflows/Test/badge.svg" alt="Tests"></a>
<a href="https://goreportcard.com/report/github.com/dotcommander/defuddle"><img src="https://goreportcard.com/badge/github.com/dotcommander/defuddle" alt="Go Report Card"></a>
<a href="https://pkg.go.dev/github.com/dotcommander/defuddle"><img src="https://pkg.go.dev/badge/github.com/dotcommander/defuddle.svg" alt="Go Reference"></a>
</p>

## Introduction

Defuddle Go is a port of the [Defuddle](https://github.com/kepano/defuddle) TypeScript library. It extracts clean, readable content from any web page — stripping away navigation, ads, sidebars, and other clutter so you're left with just the article.

Available as both a **Go library** and a drop-in **CLI tool** compatible with the original [Defuddle CLI](https://github.com/kepano/defuddle-cli).

## Installation

### CLI

Download a pre-built binary from the [releases page](https://github.com/dotcommander/defuddle/releases), or install with Go:

```bash
go install github.com/dotcommander/defuddle/cmd/defuddle@latest
```

### Library

```bash
go get github.com/dotcommander/defuddle
```

> Requires Go 1.26 or higher.

## Quick Start

### CLI

```bash
defuddle parse https://example.com/article
```

Add `--markdown` or `--json` for different output formats:

```bash
defuddle parse https://example.com/article --markdown
defuddle parse https://example.com/article --json
```

### Library — fetch and parse a URL

```go
import (
    "context"
    "fmt"
    "github.com/dotcommander/defuddle"
)

result, err := defuddle.ParseFromURL(context.Background(), "https://example.com/article", nil)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.Title)
fmt.Println(result.Content) // clean HTML
```

### Library — parse HTML you already have

```go
result, err := defuddle.ParseFromString(ctx, htmlString, &defuddle.Options{
    URL: "https://example.com/article", // enables relative URL resolution
})
```

### Lower-level API

When you need to reuse the parsed document or configure options before parsing, use the two-step form:

```go
d, err := defuddle.NewDefuddle(htmlString, &defuddle.Options{
    URL:      "https://example.com/article",
    Markdown: true,
})
if err != nil {
    log.Fatal(err)
}

result, err := d.Parse(ctx)

fmt.Printf("Title:       %s\n", result.Title)
fmt.Printf("Author:      %s\n", result.Author)
fmt.Printf("Published:   %s\n", result.Published)
fmt.Printf("Language:    %s\n", result.Language)
fmt.Printf("Word Count:  %d\n", result.WordCount)
fmt.Printf("Content:     %s\n", result.Content) // Markdown when Markdown: true
```

## Extracting Content

### From a URL

`ParseFromURL` handles HTTP fetching, encoding detection, and parsing in one call:

```go
result, err := defuddle.ParseFromURL(ctx, "https://example.com/article", &defuddle.Options{
    Markdown: true,
})
```

### Multiple URLs (concurrent)

```go
urls := []string{
    "https://example.com/article-1",
    "https://example.com/article-2",
}

results := defuddle.ParseFromURLs(ctx, urls, &defuddle.Options{
    MaxConcurrency: 10,
    Markdown:       true,
})

for _, r := range results {
    if r.Err != nil {
        log.Printf("failed %s: %v", r.URL, r.Err)
        continue
    }
    fmt.Printf("%s (%d words)\n", r.Result.Title, r.Result.WordCount)
}
```

### Markdown Output

Set `Markdown: true` to receive the extracted content as Markdown:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{Markdown: true})
fmt.Println(result.Content) // Markdown
```

To receive both HTML and Markdown in the same result:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{SeparateMarkdown: true})
fmt.Println(result.Content)          // HTML
fmt.Println(*result.ContentMarkdown) // Markdown
```

## Site-Specific Extractors

Defuddle automatically detects popular platforms and applies specialized extraction logic. No configuration needed — if the URL matches, the right extractor activates.

**Conversation**

| Platform | Domains | Content Type |
|----------|---------|-------------|
| ChatGPT | `chatgpt.com` | Conversations with role-separated messages |
| Claude | `claude.ai` | Conversations with human/assistant turns |
| Grok | `grok.com`, `grok.x.ai`, `x.ai` | xAI conversations |
| Gemini | `gemini.google.com` | Google AI conversations |

**News**

| Platform | Domains | Content Type |
|----------|---------|-------------|
| Substack | `substack.com` | Newsletter articles |
| Medium | `medium.com` | Articles with publication metadata |
| NYTimes | `nytimes.com` | News articles |
| LWN | `lwn.net` | Linux Weekly News articles |

**Social**

| Platform | Domains | Content Type |
|----------|---------|-------------|
| X / Twitter (article) | `x.com`, `twitter.com` | Long-form articles (Draft.js) |
| Twitter (legacy) | `x.com`, `twitter.com` | Tweets and threads |
| Bluesky | `bsky.app` | Posts and threads |
| Threads | `threads.com`, `threads.net` | Posts and threads |
| LinkedIn | `linkedin.com` | Posts and articles |
| X oEmbed | `publish.twitter.com`, `publish.x.com` | Embedded tweet markup |

**Tech**

| Platform | Domains | Content Type |
|----------|---------|-------------|
| YouTube | `youtube.com`, `youtu.be` | Video metadata and descriptions |
| Reddit | `reddit.com`, `old.reddit.com`, `new.reddit.com` | Posts with comment trees |
| Hacker News | `news.ycombinator.com` | Posts and threaded comment discussions |
| GitHub | `github.com` | Issues and pull requests with comments |
| Wikipedia | `*.wikipedia.org` | Article body with section structure |
| C2 Wiki | `c2.com` | Wiki pages |
| LeetCode | `leetcode.com` | Problem statements |

**Catchall (DOM-signature — matches any host)**

| Platform | Content Type |
|----------|-------------|
| Discourse | Forum topics and reply threads |
| Mastodon | Posts and threads |

23 extractors total: 4 conversation, 4 news, 6 social, 7 tech, 2 catchall.

### Custom Extractors

Implement the `BaseExtractor` interface to add support for any site.

Three things to know before you write one:

1. Registration order matters — the first matching extractor wins.
2. `CanExtract()` runs before fallback content scoring. Return `false` to fall through to the generic pipeline.
3. Setting `Variables["title"]` and `Variables["author"]` overrides the values in `Result.Title` / `Result.Author`.

```go
type RecipeExtractor struct {
    *extractors.ExtractorBase
}

func NewRecipeExtractor(doc *goquery.Document, url string, schema any) extractors.BaseExtractor {
    return &RecipeExtractor{ExtractorBase: extractors.NewExtractorBase(doc, url, schema)}
}

func (e *RecipeExtractor) Name() string { return "RecipeExtractor" }

// CanExtract returns true only when the page has a recipe card — not every page on the host.
func (e *RecipeExtractor) CanExtract() bool {
    return e.GetDocument().Find("article.recipe-card").Length() > 0
}

func (e *RecipeExtractor) Extract() *extractors.ExtractorResult {
    doc := e.GetDocument()

    // ContentHTML is what becomes Result.Content.
    content, _ := doc.Find("article.recipe-card").Html()

    title := strings.TrimSpace(doc.Find("h1.recipe-title").Text())
    author := strings.TrimSpace(doc.Find(".recipe-author").Text())

    return &extractors.ExtractorResult{
        ContentHTML: content,
        Variables: map[string]string{
            "title":  title,
            "author": author,
            "site":   "Recipe Site",
        },
    }
}
```

Register it before parsing — typically in `init()` or application startup:

```go
extractors.Register(extractors.ExtractorMapping{
    Patterns:  []any{"recipes.example.com"},
    Extractor: NewRecipeExtractor,
})
```

## Configuration

### Options

All options have sensible defaults. Pass `nil` for zero-config extraction.

```go
opts := &defuddle.Options{
    // Output
    Markdown:         false, // Return content as Markdown
    SeparateMarkdown: false, // Return both HTML and Markdown

    // Content selection
    ContentSelector:  "",    // CSS selector override for main content
    URL:              "",    // Source URL (used for link resolution and domain detection)

    // Removal controls — pointer bools default to true when nil.
    // Use defuddle.PtrBool(false) to explicitly disable.
    RemoveExactSelectors:   nil, // Remove known clutter (ads, nav, social buttons)
    RemovePartialSelectors: nil, // Remove probable clutter (class/id pattern matching)
    RemoveHiddenElements:   nil, // Remove display:none and hidden elements
    RemoveContentPatterns:  nil, // Remove boilerplate (breadcrumbs, related posts, etc.)
    RemoveLowScoring:       nil, // Remove low-scoring non-content blocks
    RemoveImages:           false, // Strip all images from output

    // Element processing
    ProcessCode:      false, // Normalize code blocks with language detection
    ProcessImages:    false, // Optimize images (lazy-load resolution, srcset)
    ProcessHeadings:  false, // Clean heading hierarchy
    ProcessMath:      false, // Normalize MathJax/KaTeX formulas
    ProcessFootnotes: false, // Standardize footnote format
    ProcessRoles:     false, // Convert ARIA roles to semantic HTML

    // HTTP (for ParseFromURL / ParseFromURLs)
    Client:         nil, // Custom *requests.Client; default uses 30s timeout
    MaxConcurrency: 5,   // Parallel limit for ParseFromURLs
    Debug:          false,
}
```

### Content Selector

Override automatic content detection with a CSS selector:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    ContentSelector: "article.post-body",
})
```

## The Extraction Pipeline

Defuddle processes content through a multi-stage pipeline:

```
HTML Input
 |
 v
1. Schema.org         -- Extract JSON-LD structured data
2. Site Detection     -- Match URL to specialized extractor
3. Shadow DOM         -- Flatten shadow roots and resolve React SSR
4. Selector Removal   -- Strip known clutter by CSS selector
5. Content Scoring    -- Score nodes and identify main content
6. Content Patterns   -- Remove boilerplate (breadcrumbs, related posts, newsletters)
7. Standardization    -- Normalize headings, footnotes, code blocks, images, math
8. Markdown           -- Convert to Markdown (if requested)
 |
 v
Result
```

The pipeline includes an automatic retry cascade: if initial extraction yields fewer than 50 words, Defuddle progressively relaxes removal filters to recover content from heavily-decorated pages.

## The Result Object

| Field | Type | Description |
|-------|------|-------------|
| `Title` | `string` | Article title |
| `Author` | `string` | Article author |
| `Description` | `string` | Article description or summary |
| `Domain` | `string` | Website domain |
| `Favicon` | `string` | Website favicon URL |
| `Image` | `string` | Main article image URL |
| `Language` | `string` | BCP 47 language tag (e.g. `en`, `pt-BR`) |
| `Published` | `string` | Publication date |
| `Site` | `string` | Website name |
| `Content` | `string` | Cleaned HTML (or Markdown if enabled) |
| `ContentMarkdown` | `*string` | Markdown version (with `SeparateMarkdown`) |
| `WordCount` | `int` | Word count of extracted content |
| `ParseTime` | `int64` | Parse duration in milliseconds |
| `SchemaOrgData` | `any` | Schema.org structured data |
| `Variables` | `map[string]string` | Extractor-specific variables |
| `MetaTags` | `[]MetaTag` | Document meta tags |
| `ExtractorType` | `*string` | Which extractor was used |
| `DebugInfo` | `*debug.Info` | Debug processing steps (with `Debug`) |

## CLI Usage

The `defuddle` command provides a fast interface for content extraction, fully compatible with the original [TypeScript CLI](https://github.com/kepano/defuddle-cli).

### Extracting Content

```bash
# From a URL
defuddle parse https://example.com/article

# From a local file
defuddle parse article.html

# From stdin (pipe HTML in)
curl -s https://example.com/article | defuddle parse

# As Markdown
defuddle parse https://example.com/article --markdown

# As JSON with all metadata
defuddle parse https://example.com/article --json

# Extract a single field
defuddle parse https://example.com/article --property title
```

### Batch Processing

Read one URL per line, output one JSON object per line (JSONL):

```bash
defuddle batch < urls.txt > articles.jsonl

# From a file, with markdown, 10 parallel fetches
defuddle batch --input urls.txt --markdown --concurrency 10 > articles.jsonl
```

### Saving Output

```bash
defuddle parse https://example.com/article --markdown --output article.md
```

### Authentication and Proxies

```bash
# Custom headers
defuddle parse https://example.com --header "Authorization: Bearer token123"

# Through a proxy
defuddle parse https://example.com --proxy http://localhost:8080

# Custom timeout
defuddle parse https://slow-site.com --timeout 120s
```

### All CLI Options

| Option | Short | Description |
|--------|-------|-------------|
| `--output` | `-o` | Output file path (default: stdout) |
| `--markdown` | `-m` | Convert content to Markdown |
| `--json` | `-j` | Output as JSON with metadata |
| `--property` | `-p` | Extract a specific property |
| `--header` | `-H` | Custom header (repeatable) |
| `--proxy` | | Proxy URL |
| `--user-agent` | | Custom user agent |
| `--timeout` | | Request timeout (default: 30s) |
| `--content-selector` | | CSS selector for content root |
| `--no-clutter-removal` | | Disable all clutter removal heuristics |
| `--remove-images` | | Strip images from output |
| `--debug` | | Enable debug output |

## Limitations

Defuddle works best on static, article-style HTML. Several categories of pages will produce poor or empty results:

**JS-rendered pages.** If a site uses client-side rendering (React, Vue, Svelte without SSR), defuddle receives the shell HTML before JavaScript runs — usually near-empty. Pre-render with a headless browser and pipe the resulting HTML in: `playwright ... | defuddle parse -`.

**Paywalled and login-gated content.** Defuddle fetches exactly what an unauthenticated request returns. For login-gated content, pass an authenticated `*requests.Client` with session cookies. For hard paywalls, you get the paywall HTML.

**PDFs and binary content.** Any response whose `Content-Type` is not HTML, XML, or text returns `ErrNotHTML`. Sniff the content type before calling defuddle.

**Large responses.** Responses over 5 MB return `ErrTooLarge`. This is intentional — defuddle is an article extractor, not a bulk downloader.

**CAPTCHA and bot-detection pages.** Defuddle returns whatever HTML the server sent. It does not solve CAPTCHAs or bypass bot-detection.

**Non-article pages.** Content scoring is heuristic. Forum threads, comment sections, and listing pages without a site-specific extractor may return partial or noisy results.

See [docs/limitations.md](docs/limitations.md) for detailed workarounds.

## Examples

The [`examples/`](./examples/) directory contains ready-to-run programs:

```bash
go run ./examples/basic              # Simple extraction
go run ./examples/markdown           # HTML to Markdown
go run ./examples/advanced           # Full option usage
go run ./examples/extractors         # Site-specific extraction
go run ./examples/custom_extractor   # Building a custom extractor
```

## Testing

```bash
# Run all tests
go test ./...

# With race detection
go test -race ./...

# Benchmarks
go test -bench=. -benchmem ./...
```

## Credits

- [Defuddle](https://github.com/kepano/defuddle) by Steph Ango ([@kepano](https://github.com/kepano)) — the original TypeScript library
- [Defuddle CLI](https://github.com/kepano/defuddle-cli) by Steph Ango — the original CLI tool
- Inspired by Mozilla's [Readability](https://github.com/mozilla/readability) algorithm

## License

Defuddle Go is open-sourced software licensed under the [MIT license](LICENSE).
