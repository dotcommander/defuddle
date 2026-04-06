# Getting Started

## Installation

Install the CLI:

```bash
go install github.com/dotcommander/defuddle/cmd/defuddle@latest
```

Or add the library to your project:

```bash
go get github.com/dotcommander/defuddle
```

## Quick Start

### CLI

Extract the main content from any web page:

```bash
defuddle parse https://example.com/article
```

Convert to markdown:

```bash
defuddle parse https://example.com/article --markdown
```

Get structured JSON output with metadata:

```bash
defuddle parse https://example.com/article --json
```

### Library

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/dotcommander/defuddle"
)

func main() {
    result, err := defuddle.ParseFromURL(
        context.Background(),
        "https://example.com/article",
        nil,
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Title)
    fmt.Println(result.Content)
}
```

## What You Get Back

Every parse returns a `Result` containing:

- **Content** -- clean HTML with ads, navigation, and clutter removed
- **Title, Author, Published** -- extracted from meta tags, Schema.org, and page structure
- **Domain, Favicon, Image** -- site identity and social sharing image
- **WordCount** -- CJK-aware word count of the extracted content
- **SchemaOrgData** -- parsed JSON-LD structured data, when present
- **ParseTime** -- extraction time in milliseconds

## Core Concepts

### Automatic Content Detection

Defuddle scores every block element in the page by word count, structure, and proximity to headings. The highest-scoring content block becomes your extracted content. Ads, sidebars, navigation, and boilerplate are stripped away automatically.

### Site-Specific Extractors

For major platforms -- YouTube, Reddit, GitHub, ChatGPT, Claude, and others -- Defuddle uses purpose-built extractors that understand each site's DOM structure. When a URL matches a known platform, the site extractor runs instead of the general-purpose algorithm.

List all supported extractors:

```bash
defuddle extractors
```

Check which extractor matches a URL:

```bash
defuddle extractors --match https://www.youtube.com/watch?v=dQw4w9WgXcQ
```

### Markdown Conversion

Request markdown output to get clean, readable text suitable for LLMs, note-taking, or further processing:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    Markdown: true,
})
fmt.Println(*result.ContentMarkdown)
```

```bash
defuddle parse https://example.com --markdown
```

### Batch Processing

Parse multiple URLs concurrently:

```bash
echo -e "https://example.com/a\nhttps://example.com/b" | defuddle batch
```

```go
results := defuddle.ParseFromURLs(ctx, urls, &defuddle.Options{
    MaxConcurrency: 10,
})
```

## Next Steps

- [CLI Reference](cli.md) -- all commands, flags, and usage patterns
- [Library Usage](library.md) -- Go API with examples
- [Extractors](extractors.md) -- supported platforms and custom extraction
- [Configuration](configuration.md) -- all options, defaults, and tuning
