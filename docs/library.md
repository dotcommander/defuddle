# Library Usage

## Parse a URL

The simplest call: fetch, decode, and extract in one step.

```go
import (
    "context"
    "fmt"
    "log"
    "github.com/dotcommander/defuddle"
)

result, err := defuddle.ParseFromURL(context.Background(), "https://example.com/article", nil)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.Title)
fmt.Println(result.Content) // clean HTML
```

## Parse an HTML String

When you already have HTML (browser automation, local files, test fixtures):

```go
result, err := defuddle.ParseFromString(ctx, htmlString, &defuddle.Options{
    URL: "https://example.com/article", // enables relative URL resolution
})
if err != nil {
    log.Fatal(err)
}
```

### Two-step form

Use `NewDefuddle` + `Parse` when you need more control or want to inspect the document before parsing:

```go
d, err := defuddle.NewDefuddle(htmlString, &defuddle.Options{
    URL:      "https://example.com/article",
    Markdown: true,
})
if err != nil {
    log.Fatal(err)
}
result, err := d.Parse(ctx)
```

## Parse Multiple URLs

`ParseFromURLs` fetches and parses concurrently. It never returns an error — per-URL failures are stored in `URLResult.Err`.

```go
urls := []string{
    "https://example.com/article-1",
    "https://example.com/article-2",
    "https://example.com/article-3",
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

Default concurrency is 5. Set `MaxConcurrency` in `Options` to override.

## Get Markdown Output

Set `Markdown: true` to receive the cleaned content as Markdown (replaces HTML in `Content`):

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    Markdown: true,
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.Content) // Markdown string
```

Set `SeparateMarkdown: true` to get both formats simultaneously:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    SeparateMarkdown: true,
})
fmt.Println(result.Content)          // HTML
fmt.Println(*result.ContentMarkdown) // Markdown (check for nil first)
```

## Custom HTTP Client

Pass your own `*requests.Client` to control timeouts, headers, proxies, and retry behavior:

```go
import "github.com/kaptinlin/requests"

client := requests.New(
    requests.WithUserAgent("MyApp/1.0"),
    requests.WithTimeout(60 * time.Second),
    requests.WithProxy("http://proxy.example.com:8080"),
)

result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    Client: client,
})
```

Default HTTP client timeout: 30 seconds (both library and CLI). Override by passing `Options.Client` with a custom `*requests.Client`.

## Result Structure

```go
type Result struct {
    Metadata                         // embedded struct (see below)
    Content         string           // cleaned HTML (or Markdown when Markdown: true)
    ContentMarkdown *string          // Markdown version (when SeparateMarkdown: true)
    ExtractorType   *string          // name of site extractor used, if any
    Variables       map[string]string // extractor-specific metadata
    MetaTags        []MetaTag        // all collected meta tags
    DebugInfo       *debug.Info      // debug data (when Debug: true)
}

type Metadata struct {
    Title         string  // article title
    Description   string  // summary or meta description
    Domain        string  // hostname without www
    Favicon       string  // absolute favicon URL
    Image         string  // main article image URL
    Language      string  // BCP 47 language tag (e.g. "en", "pt-BR")
    ParseTime     int64   // parse duration in milliseconds
    Published     string  // publication date string
    Author        string  // author name(s), comma-separated
    Site          string  // site/publisher name
    SchemaOrgData any     // parsed JSON-LD from the page
    WordCount     int     // word count of extracted content
}
```

`Language` is populated from `<html lang>`, `content-language` meta, `og:locale`, or Schema.org `inLanguage`, in that priority order. The value is normalized to BCP 47 format (e.g. `en_US` → `en-US`).

## Content Control

### Disable Clutter Removal

Each removal stage can be toggled independently. All default to `true`. Use `PtrBool(false)` to disable:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    RemoveExactSelectors:   defuddle.PtrBool(false), // keep known clutter elements
    RemovePartialSelectors: defuddle.PtrBool(false), // keep pattern-matched elements
    RemoveHiddenElements:   defuddle.PtrBool(false), // keep hidden elements
    RemoveLowScoring:       defuddle.PtrBool(false), // keep low-scoring blocks
    RemoveContentPatterns:  defuddle.PtrBool(false), // keep boilerplate
})
```

### Force a Content Root

Bypass auto-detection by specifying a CSS selector:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    ContentSelector: "article.post-body",
})
```

### Remove Images

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    RemoveImages: true,
})
```

## Element Processing

Enable specialized processing for specific element types:

```go
import "github.com/dotcommander/defuddle/internal/elements"

result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    ProcessCode:      true,
    ProcessMath:      true,
    ProcessFootnotes: true,
    ProcessImages:    true,
    CodeOptions: &elements.CodeBlockProcessingOptions{
        DetectLanguage: true,
        FormatCode:     true,
    },
    MathOptions: &elements.MathProcessingOptions{
        ExtractMathML:   true,
        ExtractLaTeX:    true,
        CleanupScripts:  true,
        PreserveDisplay: true,
    },
})
```

## Debug Mode

Enable debug mode to inspect the extraction pipeline step by step:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    Debug: true,
})

info := result.DebugInfo
fmt.Printf("Extractor: %s\n", info.ExtractorUsed)
fmt.Printf("Elements: %d original, %d final, %d removed\n",
    info.Statistics.OriginalElementCount,
    info.Statistics.FinalElementCount,
    info.Statistics.RemovedElementCount,
)
for step, ns := range info.Timings {
    fmt.Printf("  %s: %dms\n", step, ns/1_000_000)
}
```

## Error Handling

Defuddle defines sentinel errors for structured error handling. All are wrapped with `fmt.Errorf("%w")` so use `errors.Is` rather than direct comparison.

| Sentinel | Trigger | How to handle |
|----------|---------|---------------|
| `ErrNotHTML` | `Content-Type` is not HTML, XML, or text | Skip or route to a different handler |
| `ErrTooLarge` | Response body exceeds 5 MB | Reject or stream the URL separately |
| `ErrTimeout` | Context cancelled or deadline exceeded | Retry with longer timeout or skip |

`ErrTimeout` is wrapped: `fmt.Errorf("fetch %s: %w", url, ErrTimeout)`. Unwrap with `errors.Is(err, defuddle.ErrTimeout)`.

```go
result, err := defuddle.ParseFromURL(ctx, url, nil)
if err != nil {
    switch {
    case errors.Is(err, defuddle.ErrNotHTML):
        // URL returned non-HTML content (PDF, image, binary)
    case errors.Is(err, defuddle.ErrTooLarge):
        // Response exceeded 5 MB
    case errors.Is(err, defuddle.ErrTimeout):
        // Request timed out or context cancelled
    default:
        // Network error, DNS failure, invalid URL, etc.
    }
}
```

## Extraction Pipeline

Understanding the pipeline helps when tuning options:

1. **Schema.org extraction** — JSON-LD structured data is parsed first
2. **Site extractor check** — if a registered extractor matches the URL, it runs and returns early
3. **Entry-point detection** — looks for `<article>`, `<main>`, `[role="main"]`, and common content selectors
4. **Content scoring** — scores every block element by word density and structure
5. **Clutter removal** — strips ads, navigation, hidden elements, and boilerplate in stages
6. **Standardization** — normalizes heading levels, flattens wrappers, cleans whitespace
7. **Markdown conversion** — converts to Markdown if requested
8. **Retry logic** — if content is under 200 words, retries with progressively relaxed removal filters
