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
// Include Markdown alongside cleaned HTML (same conversion as Markdown)
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

The five removal fields are deprecated compatibility controls. Their effective
values default to `true` when nil. Individual combinations no longer change
extraction: Trafilatura owns generic content selection, with comments excluded,
native fallback enabled, and links and tables retained.

Set **all five** to `false` to bypass extraction and process the body (or a
matching `ContentSelector`). Formatting, URL resolution, safety processing, and
`RemoveImages` still apply. This preserves `--no-clutter-removal`:

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

Process the first matching subtree before site-specific or generic extraction.
A selector miss continues normal extraction:

```go
opts := &defuddle.Options{
    ContentSelector: "article.post-body",
}
```

### RemoveImages

Strip images on selector, bypass, site-extractor, generic, and recovery paths:

```go
opts := &defuddle.Options{RemoveImages: true}
```

## HTTP Options

### Custom Client

```go
import (
    "net/http"
    "time"
)

client := &http.Client{Timeout: 60 * time.Second}
opts := &defuddle.Options{
    Client: client,
    Headers: http.Header{"User-Agent": []string{"MyBot/1.0"}},
}
```

### MaxConcurrency

Controls parallel URL fetching in `ParseFromURLs`:

```go
opts := &defuddle.Options{MaxConcurrency: 10} // default: 5
```

## Element Processing

All six `Process*` flags default to `false`. Enable normalization for specific
content types via the public boolean flags. Basic preservation of supported safe
code, math, and local footnotes remains active when their normalization is off. The per-processor option structs (`CodeOptions`, `ImageOptions`, `MathOptions`, `FootnoteOptions`, `HeadingOptions`, `RoleOptions`) reference types in an internal package and are not part of the public external API — external modules cannot import them. The boolean flags below enable each processor with sensible defaults and are the supported external surface.

### Code Blocks

```go
opts := &defuddle.Options{
    ProcessCode: true, // detect language and normalize code block whitespace
}
```

### Images

```go
opts := &defuddle.Options{
    ProcessImages: true, // normalize image markup
}
```

### Math

```go
opts := &defuddle.Options{
    ProcessMath: true, // extract MathML/LaTeX and clean up math scripts
}
```

### Footnotes

```go
opts := &defuddle.Options{
    ProcessFootnotes: true, // detect, link, number, and group footnotes into a section
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
    ProcessRoles: true, // convert role="paragraph"/"list"/"button"/"link" to native elements
}
```

### Custom processor settings

Each `Process*` flag enables its stage. The corresponding `CodeOptions`,
`ImageOptions`, `HeadingOptions`, `MathOptions`, `FootnoteOptions`, or
`RoleOptions` pointer configures that stage; a pointer alone does not enable it.
Nil pointers retain the existing default processing path. Non-nil role and
footnote options use the configurable processors within the selected content
root, including footnote section generation there.

Custom options expose the behavior already supported by those processors.
Image lazy-loading, responsive, alt-text, optimization, and maximum-size fields
remain outside this change; forwarding them does not add implementations.
Footnote section locations retain the existing `end`, `after-content`, and
`custom` behavior rather than adding new placement modes.

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

- **RemovedElements** -- retained compatibility data; generic extraction no longer
  supplies the old per-element clutter-removal trace
- **ProcessingSteps** -- ordered pipeline steps, including `trafilatura`,
  `extraction_bypass`, or `extraction_recovery` and its reason
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
