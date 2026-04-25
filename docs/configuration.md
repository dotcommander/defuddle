# Configuration

## Options

Pass `*Options` to any parse function to control extraction behavior. All fields are optional -- `nil` uses sensible defaults.

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    Markdown: true,
    Debug:    true,
})
```

## Output Options

### Markdown

```go
// Include markdown alongside HTML
opts := &defuddle.Options{Markdown: true}
result, _ := defuddle.ParseFromURL(ctx, url, opts)
fmt.Println(*result.ContentMarkdown)
```

### SeparateMarkdown

```go
// Convert original HTML to markdown independently from cleaned content
opts := &defuddle.Options{SeparateMarkdown: true}
```

### URL

Set the base URL for resolving relative links when parsing HTML strings:

```go
d, _ := defuddle.NewDefuddle(html, &defuddle.Options{
    URL: "https://example.com/article",
})
```

## Clutter Removal

Each removal stage runs independently. Disable specific stages to keep more content:

```go
opts := &defuddle.Options{
    RemoveExactSelectors:   defuddle.PtrBool(false), // keep ads, social buttons
    RemovePartialSelectors: defuddle.PtrBool(false), // keep class/id pattern matches
    RemoveHiddenElements:   defuddle.PtrBool(false), // keep display:none elements
    RemoveLowScoring:       defuddle.PtrBool(false), // keep low-scoring blocks
    RemoveContentPatterns:  defuddle.PtrBool(false), // keep boilerplate text
}
```

All removal flags default to `true` when `nil`. Use `defuddle.PtrBool(v)` to set explicit values.

### Stages

| Stage | Default | What It Removes |
|-------|---------|-----------------|
| ExactSelectors | on | Ads, social widgets, share buttons via exact CSS selectors |
| PartialSelectors | on | Elements matching ad/clutter patterns in class and id attributes |
| HiddenElements | on | Elements with `display:none`, `visibility:hidden`, or Tailwind hidden classes |
| LowScoring | on | Blocks that score below the content threshold (sidebars, footers, related articles) |
| ContentPatterns | on | Boilerplate text (read time, breadcrumbs, article cards) |

### Remove All Clutter Removal

```go
// Equivalent to --no-clutter-removal in the CLI
opts := &defuddle.Options{
    RemoveExactSelectors:   defuddle.PtrBool(false),
    RemovePartialSelectors: defuddle.PtrBool(false),
    RemoveHiddenElements:   defuddle.PtrBool(false),
    RemoveLowScoring:       defuddle.PtrBool(false),
    RemoveContentPatterns:  defuddle.PtrBool(false),
}
```

## Content Selection

### ContentSelector

Force a specific element as the content root, bypassing auto-detection:

```go
opts := &defuddle.Options{
    ContentSelector: "article.post-body",
}
```

### RemoveImages

Strip all images from the extracted content:

```go
opts := &defuddle.Options{RemoveImages: true}
```

## HTTP Options

### Custom Client

```go
import "github.com/kaptinlin/requests"

client := requests.New(
    requests.WithUserAgent("MyBot/1.0"),
    requests.WithTimeout(60 * time.Second),
)
opts := &defuddle.Options{Client: client}
```

### MaxConcurrency

Controls parallel URL fetching in `ParseFromURLs`:

```go
opts := &defuddle.Options{MaxConcurrency: 10} // default: 5
```

## Element Processing

Enable specialized processors for specific content types. Each processor has its own options struct:

### Code Blocks

```go
opts := &defuddle.Options{
    ProcessCode: true,
    CodeOptions: &elements.CodeBlockProcessingOptions{
        DetectLanguage: true, // detect language from class names
        FormatCode:     true, // normalize whitespace
    },
}
```

### Images

```go
opts := &defuddle.Options{
    ProcessImages: true,
    ImageOptions: &elements.ImageProcessingOptions{
        RemoveSmallImages: true,
        MinImageWidth:     50,  // pixels
        MinImageHeight:    50,
    },
}
```

### Math

```go
opts := &defuddle.Options{
    ProcessMath: true,
    MathOptions: &elements.MathProcessingOptions{
        ExtractMathML:   true,
        ExtractLaTeX:    true,
        CleanupScripts:  true,
        PreserveDisplay: true,
    },
}
```

### Footnotes

```go
opts := &defuddle.Options{
    ProcessFootnotes: true,
    FootnoteOptions: &elements.FootnoteProcessingOptions{
        DetectFootnotes:      true,
        LinkFootnotes:        true,
        NumberFootnotes:      true,
        ImproveAccessibility: true,
        GenerateSection:      true,
        SectionTitle:         "Footnotes",
        SectionLocation:      "end", // "end", "after-content", or "custom"
    },
}
```

### Headings

```go
opts := &defuddle.Options{
    ProcessHeadings: true,
}
```

### ARIA Roles

```go
opts := &defuddle.Options{
    ProcessRoles: true,
    RoleOptions: &elements.RoleProcessingOptions{
        ConvertParagraphs: true, // role="paragraph" -> <p>
        ConvertLists:      true, // role="list" -> <ul>/<ol>
        ConvertButtons:    true, // role="button" -> <button>
        ConvertLinks:      true, // role="link" -> <a>
    },
}
```

## Debug Mode

```go
opts := &defuddle.Options{Debug: true}
result, _ := defuddle.ParseFromURL(ctx, url, opts)

info := result.DebugInfo
fmt.Printf("Removed %d elements\n", info.Statistics.RemovedElementCount)
for _, step := range info.ProcessingSteps {
    fmt.Printf("  %s\n", step)
}
```

Debug output includes:

- **RemovedElements** -- each element removed, with reason and selector
- **ProcessingSteps** -- ordered list of pipeline steps executed
- **Timings** -- nanosecond-precision timing for each stage
- **Statistics** -- element counts, word count, image count, link count
- **ExtractorUsed** -- name of the site extractor, if any

## Defaults Summary

| Option | Default |
|--------|---------|
| Markdown | false |
| SeparateMarkdown | false |
| Debug | false |
| RemoveImages | false |
| All removal flags | true (when nil) |
| All process flags | false |
| MaxConcurrency | 5 |
| HTTP timeout | 30s (library and CLI) |
| Max response size | 5 MB |
