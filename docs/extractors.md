# Extractors

Defuddle includes site-specific extractors that understand the DOM structure of major platforms. When a URL matches a registered extractor, it runs instead of the general-purpose content detection algorithm.

Check the extractor selected for a URL:

```bash
defuddle extractors --match 'https://www.youtube.com/watch?v=dQw4w9WgXcQ'
```

## Supported Sites

### Conversation

| Extractor | Sites | What It Extracts |
|-----------|-------|------------------|
| ChatGPT | `chatgpt.com` | Conversation messages with role attribution |
| Claude | `claude.ai` | Conversation messages with role attribution |
| Grok | `grok.com`, `grok.x.ai`, `x.ai` | Conversation messages with role attribution |
| Gemini | `gemini.google.com` | Conversation messages with role attribution |

### News

| Extractor | Sites | What It Extracts |
|-----------|-------|------------------|
| Substack | `substack.com` | Newsletter articles with author metadata |
| Medium | `medium.com` | Articles with author and publication metadata |
| NYTimes | `nytimes.com` | News articles with author and section metadata |
| LWN | `lwn.net` | Articles and subscriber content |

### Social

| Extractor | Sites | What It Extracts |
|-----------|-------|------------------|
| X / Twitter (article) | `x.com`, `twitter.com` | Long-form articles (Draft.js format) |
| Twitter (legacy) | `x.com`, `twitter.com` | Tweets and threads |
| Bluesky | `bsky.app` | Posts and threads |
| Threads | `threads.com`, `threads.net` | Posts and threads |
| LinkedIn | `linkedin.com` | Posts and articles |
| X oEmbed | `publish.twitter.com`, `publish.x.com` | Embedded tweet markup |

### Tech

| Extractor | Sites | What It Extracts |
|-----------|-------|------------------|
| YouTube | `youtube.com`, `youtu.be` | Video metadata, caption links, and timed-text transcripts |
| Reddit | `reddit.com`, `old.reddit.com`, `new.reddit.com` | Post content, comments, subreddit context |
| Hacker News | `news.ycombinator.com` | Story content and comment threads |
| GitHub | `github.com` | Issues, pull requests, repository content |
| Wikipedia | `*.wikipedia.org` | Article body with section structure |
| C2 Wiki | `c2.com` | Wiki pages |
| LeetCode | `leetcode.com` | Problem statements and editorial content |

### Catchall (DOM-signature)

| Extractor | Sites | What It Extracts |
|-----------|-------|------------------|
| Discourse | Any Discourse instance | Forum topics and reply threads |
| Mastodon | Any Mastodon instance | Posts and threads |

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

The registry is organized across `registry_conversation.go`, `registry_news.go`, `registry_social.go`, `registry_tech.go`, and `registry_catchall.go`; catchall extractors register last so domain-specific matches win.

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

The ChatGPT, Claude, Grok, and Gemini extractors parse structured message exchanges and produce output with clear role attribution (user/assistant).

Conversation extractors return structured data:

```go
type ConversationMessage struct {
    Author    string
    Content   string
    Timestamp string
    Metadata  map[string]any
}
```

## YouTube Captions and Transcripts

For a YouTube watch page, Defuddle extracts the available video metadata and
adds a labeled link for each caption track present in
`ytInitialPlayerResponse`. Automatically generated tracks are labeled as such.
Only HTTPS `youtube.com/api/timedtext` links are emitted.

Defuddle does not follow those links automatically. Pass a returned timed-text
URL to `defuddle parse` (or `ParseFromURL`) to extract the transcript:

```bash
defuddle parse 'https://www.youtube.com/api/timedtext?...' --markdown
```

Transcript extraction collapses whitespace, removes adjacent duplicate cues,
and escapes cue text before producing HTML. Output is limited to 10,000 cues
and 1 MiB of normalized transcript text; truncated output includes a visible
marker and sets `ExtractedContent["truncated"]` to `true`.

## Fallback Behavior

A matching `ContentSelector` or all-false extraction bypass takes precedence over
site dispatch. Otherwise, if no extractor matches and confirms the DOM, Defuddle
uses Trafilatura v2.2.6 with native fallback. An extraction or rich-restoration
failure recovers from a processed marker-free body snapshot; acquisition and
cancellation errors still return errors.

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
