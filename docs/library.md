# Library Usage

## Parse a URL

```go
import "github.com/dotcommander/defuddle"

result, err := defuddle.ParseFromURL(ctx, "https://example.com/article", nil)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.Title)
fmt.Println(result.Content) // clean HTML
```

## Parse an HTML String

```go
result, err := defuddle.ParseFromString(ctx, htmlString, nil)
```

Or use the two-step API for more control:

```go
d, err := defuddle.NewDefuddle(htmlString, &defuddle.Options{
    URL: "https://example.com/article", // used for relative URL resolution
})
if err != nil {
    log.Fatal(err)
}
result, err := d.Parse(ctx)
```

## Parse Multiple URLs

`ParseFromURLs` fetches and parses concurrently with a configurable concurrency limit:

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

## Get Markdown Output

Set `Markdown: true` to receive a markdown conversion alongside the HTML:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    Markdown: true,
})
if result.ContentMarkdown != nil {
    fmt.Println(*result.ContentMarkdown)
}
```

Set `SeparateMarkdown: true` to convert the original HTML to markdown independently from the cleaned content.

## Custom HTTP Client

Pass your own `requests.Client` to control timeouts, headers, proxies, and retry behavior:

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

When no client is provided, Defuddle creates one with a Mozilla user agent and a 10-second timeout.

## Result Structure

```go
type Result struct {
    Metadata                    // embedded: Title, Description, Author, etc.
    Content         string      // cleaned HTML
    ContentMarkdown *string     // markdown (when Markdown option is true)
    ExtractorType   *string     // name of site extractor used, if any
    Variables       map[string]string // extractor-specific metadata
    MetaTags        []MetaTag   // all collected meta tags
    DebugInfo       *debug.Info // debug data (when Debug option is true)
}

type Metadata struct {
    Title         string
    Description   string
    Domain        string
    Favicon       string
    Image         string
    Language      string
    ParseTime     int64   // milliseconds
    Published     string
    Author        string
    Site          string
    SchemaOrgData any     // parsed JSON-LD
    WordCount     int
}
```

## Content Control

### Disable Clutter Removal

Each removal stage can be toggled independently. Use `PtrBool()` to set explicit boolean values:

```go
// Disable all clutter removal
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    RemoveExactSelectors:   defuddle.PtrBool(false),
    RemovePartialSelectors: defuddle.PtrBool(false),
    RemoveHiddenElements:   defuddle.PtrBool(false),
    RemoveLowScoring:       defuddle.PtrBool(false),
    RemoveContentPatterns:  defuddle.PtrBool(false),
})
```

All removal flags default to `true` when `nil`.

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

Enable debug mode to inspect the extraction pipeline:

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

Defuddle defines sentinel errors you can check with `errors.Is()`:

```go
result, err := defuddle.ParseFromURL(ctx, url, nil)
if err != nil {
    switch {
    case errors.Is(err, defuddle.ErrNotHTML):
        // URL returned non-HTML content (PDF, image, etc.)
    case errors.Is(err, defuddle.ErrTooLarge):
        // Response exceeded 5MB limit
    case errors.Is(err, defuddle.ErrTimeout):
        // Request timed out
    case errors.Is(err, defuddle.ErrNoContent):
        // Extraction found no meaningful content
    default:
        // Network error, invalid URL, etc.
    }
}
```

## Extraction Pipeline

Understanding the pipeline helps when tuning options:

1. **Schema.org extraction** -- JSON-LD structured data is parsed first
2. **Site extractor check** -- if a registered extractor matches the URL, it runs and returns
3. **Entry-point detection** -- looks for `<article>`, `<main>`, `[role="main"]`, and common content selectors
4. **Content scoring** -- scores every block element by word density and structure
5. **Clutter removal** -- strips ads, navigation, hidden elements, and boilerplate in stages
6. **Standardization** -- normalizes heading levels, flattens wrappers, cleans whitespace
7. **Markdown conversion** -- converts to markdown if requested
8. **Retry logic** -- if content is too short (< 200 words), retries with progressively relaxed removal
