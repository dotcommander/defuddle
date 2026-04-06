# CLI Reference

## parse

Extract content from a URL, file, or stdin.

```bash
defuddle parse https://example.com/article
```

Parse a local HTML file:

```bash
defuddle parse ./page.html
```

Parse from stdin:

```bash
curl -s https://example.com | defuddle parse -
```

### Output Flags

```bash
# JSON output with full metadata
defuddle parse https://example.com --json

# Markdown content
defuddle parse https://example.com --markdown

# Extract a single property
defuddle parse https://example.com --property title
defuddle parse https://example.com --property author
defuddle parse https://example.com --property wordCount

# Write to file
defuddle parse https://example.com --output article.html
defuddle parse https://example.com --markdown --output article.md
```

Available `--property` values: `title`, `description`, `domain`, `favicon`, `image`, `author`, `site`, `published`, `wordCount`, `parseTime`, `metaTags`, `schemaOrgData`, `extractorType`, `contentMarkdown`.

### HTTP Flags

```bash
# Custom timeout
defuddle parse https://example.com --timeout 60s

# Custom user agent
defuddle parse https://example.com --user-agent "MyBot/1.0"

# Custom headers
defuddle parse https://example.com -H "Authorization: Bearer token123"
defuddle parse https://example.com -H "Cookie: session=abc" -H "Accept-Language: en"

# Route through a proxy
defuddle parse https://example.com --proxy http://localhost:8080
defuddle parse https://example.com --proxy socks5://localhost:1080
```

### Content Control Flags

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

### Flag Reference

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--json` | `-j` | bool | false | Output as JSON with metadata |
| `--markdown` | `-m` | bool | false | Convert content to markdown |
| `--property` | `-p` | string | | Extract a single metadata property |
| `--output` | `-o` | string | stdout | Write output to file |
| `--user-agent` | | string | Mozilla | Custom user agent string |
| `--header` | `-H` | string[] | | Custom HTTP headers (`Key: Value`) |
| `--timeout` | | duration | 30s | HTTP request timeout |
| `--proxy` | | string | | Proxy URL |
| `--debug` | | bool | false | Enable debug output |
| `--remove-images` | | bool | false | Strip images from content |
| `--content-selector` | | string | | CSS selector for content root |
| `--no-clutter-removal` | | bool | false | Disable all clutter removal |

## batch

Parse multiple URLs concurrently. Reads one URL per line from stdin or a file. Outputs JSONL (one JSON object per line).

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

# Save results
defuddle batch --input urls.txt > results.jsonl
```

### Flag Reference

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--input` | `-i` | string | stdin | Input file with URLs (one per line) |
| `--concurrency` | `-c` | int | 5 | Max concurrent requests |
| `--markdown` | `-m` | bool | false | Include markdown in output |
| `--continue-on-error` | | bool | false | Continue processing on individual URL failures |

## extractors

List all registered site-specific extractors.

```bash
defuddle extractors
```

Check which extractor matches a URL:

```bash
defuddle extractors --match https://github.com/dotcommander/defuddle/issues/1
```

### Flag Reference

| Flag | Type | Description |
|------|------|-------------|
| `--match` | string | Check which extractor matches the given URL |

## Examples

### Extract article content as markdown

```bash
defuddle parse https://blog.example.com/post --markdown
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
```

### Force content selector for tricky pages

```bash
defuddle parse https://example.com --content-selector "div.article-content"
```
