# Extractors

Defuddle includes site-specific extractors that understand the DOM structure of major platforms. When a URL matches a registered extractor, it runs instead of the general-purpose content detection algorithm.

## Supported Sites

| Extractor | Sites | What It Extracts |
|-----------|-------|------------------|
| YouTube | `youtube.com`, `youtu.be` | Video title, description, channel, transcript |
| Reddit | `reddit.com` | Post content, comments, subreddit context |
| GitHub | `github.com` | Issues, pull requests, repository content |
| Hacker News | `news.ycombinator.com` | Story content and comment threads |
| Substack | `*.substack.com` | Newsletter articles with author metadata |
| X/Twitter | `x.com`, `twitter.com` | Tweets, threads, and articles |
| ChatGPT | `chatgpt.com` | Conversation messages with role attribution |
| Claude | `claude.ai` | Conversation messages with role attribution |
| Grok | `grok.com`, `x.ai` | Conversation messages with role attribution |
| Gemini | `gemini.google.com` | Conversation messages with role attribution |

## Listing Extractors

```bash
defuddle extractors
```

Check which extractor matches a specific URL:

```bash
defuddle extractors --match https://github.com/dotcommander/defuddle/issues/1
```

## How Extractors Work

Each extractor implements the `BaseExtractor` interface:

```go
type BaseExtractor interface {
    CanExtract() bool    // returns true if this extractor handles the page
    Extract() *ExtractorResult
    Name() string
}
```

Extractors are registered with URL patterns (domain strings or regex) and checked in priority order. The first extractor where `CanExtract()` returns `true` handles the page.

### Extractor Result

```go
type ExtractorResult struct {
    Content          string            // plain text content
    ContentHTML      string            // HTML content
    ExtractedContent map[string]any    // raw extracted data
    Variables        map[string]string // metadata (title, author, etc.)
}
```

When an extractor runs, its `Variables` map populates the `Result.Variables` field, and its name appears in `Result.ExtractorType`.

## Conversation Extractors

The ChatGPT, Claude, Grok, and Gemini extractors are conversation extractors. They parse structured message exchanges and produce output with clear role attribution (user/assistant).

Conversation extractors return structured data:

```go
type ConversationMessage struct {
    Author    string
    Content   string
    Timestamp string
    Metadata  map[string]any
}
```

## Fallback Behavior

If no extractor matches the URL, or if a matching extractor's `CanExtract()` returns `false`, Defuddle falls back to its general-purpose content scoring algorithm. This means every URL produces output -- extractors enhance quality for known sites but are never required.

## Registry

Extractors register themselves at init time. The registry is global and thread-safe:

```go
// Check the registry programmatically
extractor := extractors.FindExtractor(doc, url, schemaOrgData)
if extractor != nil {
    result := extractor.Extract()
}
```

The registry caches extractor lookups by domain for performance.
