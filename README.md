<p align="center">
<img src="https://raw.githubusercontent.com/kepano/defuddle/main/defuddle.png" width="120" alt="Defuddle">
</p>

<p align="center">
<a href="https://github.com/dotcommander/defuddle/releases"><img src="https://img.shields.io/github/v/release/dotcommander/defuddle" alt="Release"></a>
<a href="https://github.com/dotcommander/defuddle/actions"><img src="https://github.com/dotcommander/defuddle/workflows/Test/badge.svg" alt="Tests"></a>
<a href="https://goreportcard.com/report/github.com/dotcommander/defuddle"><img src="https://goreportcard.com/badge/github.com/dotcommander/defuddle" alt="Go Report Card"></a>
<a href="https://godoc.org/github.com/dotcommander/defuddle"><img src="https://godoc.org/github.com/dotcommander/defuddle?status.svg" alt="GoDoc"></a>
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

Require Defuddle Go using `go get`:

```bash
go get github.com/dotcommander/defuddle
```

> Requires Go 1.26 or higher.

## Quick Start

Extract the main content from any web page in just a few lines:

```go
d, err := defuddle.NewDefuddle(htmlString, nil)
if err != nil {
    log.Fatal(err)
}

result, err := d.Parse(context.Background())
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.Title)
fmt.Println(result.Content)
```

Or fetch and parse a URL directly:

```go
result, err := defuddle.ParseFromURL(ctx, "https://example.com/article", nil)
```

## Extracting Content

### From HTML

Pass raw HTML and receive structured content with metadata:

```go
d, err := defuddle.NewDefuddle(html, &defuddle.Options{
    URL: "https://example.com/article",
})
if err != nil {
    log.Fatal(err)
}

result, err := d.Parse(context.Background())

fmt.Printf("Title:       %s\n", result.Title)
fmt.Printf("Author:      %s\n", result.Author)
fmt.Printf("Published:   %s\n", result.Published)
fmt.Printf("Description: %s\n", result.Description)
fmt.Printf("Word Count:  %d\n", result.WordCount)
fmt.Printf("Language:    %s\n", result.Language)
```

### From a URL

`ParseFromURL` handles HTTP fetching, encoding detection, and parsing in one call:

```go
result, err := defuddle.ParseFromURL(ctx, "https://example.com/article", &defuddle.Options{
    Markdown: true,
})
```

### Markdown Output

Convert extracted content to Markdown for storage, indexing, or LLM consumption:

```go
result, err := d.Parse(ctx)

// When Markdown is enabled, Content is returned as Markdown
fmt.Println(result.Content)
```

To receive both HTML and Markdown in the same response:

```go
d, err := defuddle.NewDefuddle(html, &defuddle.Options{
    SeparateMarkdown: true,
})

result, err := d.Parse(ctx)

fmt.Println(result.Content)          // HTML
fmt.Println(*result.ContentMarkdown) // Markdown
```

## Site-Specific Extractors

Defuddle automatically detects popular platforms and applies specialized extraction logic. No configuration needed — if the URL matches, the right extractor activates.

| Platform | Content Type |
|----------|-------------|
| ChatGPT | Conversations with role-separated messages |
| Claude | Conversations with human/assistant turns |
| Gemini | Google AI conversations |
| Grok | xAI conversations |
| GitHub | Issues and pull requests with comments |
| Hacker News | Posts and threaded comment discussions |
| Reddit | Posts with comment trees |
| Substack | Newsletter articles |
| Twitter / X | Tweets and threads |
| YouTube | Video metadata and descriptions |

### Custom Extractors

Implement the `BaseExtractor` interface to add support for any site:

```go
type MyExtractor struct {
    doc          *goquery.Document
    url          string
    schemaOrgData any
}

func (e *MyExtractor) Name() string    { return "MyExtractor" }
func (e *MyExtractor) CanExtract() bool { return true }

func (e *MyExtractor) Extract() (*defuddle.ExtractedContent, error) {
    title := e.doc.Find("h1.article-title").Text()
    content, _ := e.doc.Find(".article-body").Html()
    return &defuddle.ExtractedContent{
        Title:       &title,
        ContentHTML: &content,
    }, nil
}
```

Register it before parsing:

```go
extractors.DefaultRegistry.Register(extractors.ExtractorMapping{
    Patterns:  []any{"mysite.com"},
    Extractor: func(doc, url, schema) { return &MyExtractor{doc, url, schema} },
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

    // Removal controls
    RemoveExactSelectors:   true, // Remove known clutter (ads, nav, social buttons)
    RemovePartialSelectors: true, // Remove probable clutter (class/id pattern matching)
    RemoveHiddenElements:   true, // Remove display:none and hidden elements
    RemoveContentPatterns:  true, // Remove boilerplate (breadcrumbs, related posts, etc.)
    RemoveImages:           false,// Strip all images from output

    // Element processing
    ProcessCode:      false, // Normalize code blocks with language detection
    ProcessImages:    false, // Optimize images (lazy-load resolution, srcset)
    ProcessHeadings:  false, // Clean heading hierarchy
    ProcessMath:      false, // Normalize MathJax/KaTeX formulas
    ProcessFootnotes: false, // Standardize footnote format
    ProcessRoles:     false, // Convert ARIA roles to semantic HTML

    // HTTP (for ParseFromURL)
    Client:         nil,   // Custom *requests.Client
    MaxConcurrency: 5,     // Parallel limit for ParseFromURLs
    Debug:          false, // Emit debug processing info
}
```

### Content Selector

Override automatic content detection with a CSS selector:

```go
d, err := defuddle.NewDefuddle(html, &defuddle.Options{
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
2. Site Detection      -- Match URL to specialized extractor
3. Shadow DOM          -- Flatten shadow roots and resolve React SSR
4. Selector Removal    -- Strip known clutter by CSS selector
5. Content Scoring     -- Score nodes and identify main content
6. Content Patterns    -- Remove boilerplate (breadcrumbs, related posts, newsletters)
7. Standardization     -- Normalize headings, footnotes, code blocks, images, math
8. Markdown            -- Convert to Markdown (if requested)
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
| `Published` | `string` | Publication date |
| `Language` | `string` | Content language (BCP 47) |
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

# As Markdown
defuddle parse https://example.com/article --markdown

# As JSON with all metadata
defuddle parse https://example.com/article --json

# Extract a single field
defuddle parse https://example.com/article --property title
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
| `--debug` | | Enable debug output |

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
