# Recipes

Concrete workflows for common defuddle use cases.

---

## 1. Batch a URL List to JSONL

Read one URL per line from a file, output one JSON object per line. The `batch` command uses `ParseFromURLs` internally with configurable concurrency.

```bash
# From a file
defuddle batch --input urls.txt > articles.jsonl

# From stdin, with markdown, 10 parallel fetches
cat urls.txt | defuddle batch --markdown --concurrency 10 > articles.jsonl

# Continue on individual URL errors (default: stop on first error)
defuddle batch --input urls.txt --continue-on-error > articles.jsonl
```

Each output line is a complete JSON representation of `defuddle.Result`. The `url` field is not included in the JSON; maintain a mapping from your input list if you need it.

---

## 2. Pipe a Feed into Defuddle

Extract article text from a list of URLs produced by another tool:

```bash
# jq parses an RSS/Atom JSON feed and extracts URLs one per line
cat feed.json | jq -r '.items[].url' | defuddle batch --markdown > articles.jsonl
```

For RSS XML, use any tool that produces one URL per line (`rss2email`, `sfeed`, `xmllint`):

```bash
sfeed_plain < feed.xml | awk '{print $NF}' | defuddle batch > articles.jsonl
```

Pipe a single URL from a shell script:

```bash
echo "https://example.com/article" | defuddle batch
```

---

## 3. Build a Markdown Vault

Parse a list of URLs and write each article as a Markdown file, named by a URL slug. Useful for Obsidian vaults, static site inputs, or archiving.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "regexp"
    "strings"

    "github.com/dotcommander/defuddle"
)

// slugRe strips characters that are unsafe in filenames.
var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

func slug(title string) string {
    s := strings.ToLower(strings.TrimSpace(title))
    s = slugRe.ReplaceAllString(s, "-")
    s = strings.Trim(s, "-")
    if s == "" {
        s = "untitled"
    }
    return s
}

func main() {
    urls := []string{
        "https://example.com/article-one",
        "https://example.com/article-two",
    }

    results := defuddle.ParseFromURLs(context.Background(), urls, &defuddle.Options{
        SeparateMarkdown: true,
        MaxConcurrency:   5,
    })

    for _, r := range results {
        if r.Err != nil {
            log.Printf("skip %s: %v", r.URL, r.Err)
            continue
        }

        name := slug(r.Result.Title) + ".md"
        var content string
        if r.Result.ContentMarkdown != nil {
            content = *r.Result.ContentMarkdown
        } else {
            content = r.Result.Content
        }

        if err := os.WriteFile(name, []byte(content), 0644); err != nil {
            log.Printf("write %s: %v", name, err)
            continue
        }
        fmt.Printf("wrote %s (%d words)\n", name, r.Result.WordCount)
    }
}
```

Output: one `.md` file per article, named by slugified title.

---

## 4. RAG Ingest

Parse articles and feed the text into a vector store for retrieval-augmented generation. Chunk on paragraph boundaries.

```go
package main

import (
    "context"
    "fmt"
    "strings"

    "github.com/dotcommander/defuddle"
)

// Chunk splits text into paragraph-boundary chunks of roughly maxChars.
func Chunk(text string, maxChars int) []string {
    paragraphs := strings.Split(text, "\n\n")
    var chunks []string
    var current strings.Builder

    for _, p := range paragraphs {
        p = strings.TrimSpace(p)
        if p == "" {
            continue
        }
        if current.Len()+len(p) > maxChars && current.Len() > 0 {
            chunks = append(chunks, current.String())
            current.Reset()
        }
        if current.Len() > 0 {
            current.WriteString("\n\n")
        }
        current.WriteString(p)
    }
    if current.Len() > 0 {
        chunks = append(chunks, current.String())
    }
    return chunks
}

func ingestURL(ctx context.Context, url string, store VectorStore) error {
    result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{Markdown: true})
    if err != nil {
        return err
    }

    text := result.Content // Markdown when Markdown: true
    chunks := Chunk(text, 1500)

    for i, chunk := range chunks {
        doc := Document{
            ID:     fmt.Sprintf("%s#chunk%d", url, i),
            Text:   chunk,
            Title:  result.Title,
            Author: result.Author,
            URL:    url,
        }
        if err := store.Upsert(ctx, doc); err != nil {
            return err
        }
    }
    return nil
}
```

Replace `VectorStore` and `Document` with your store's types (pgvector, chromem-go, Weaviate, etc.).

---

## 5. Custom HTTP Client with Cookies

For login-gated content, build a `*requests.Client` with a cookie jar populated from a browser session export.

Dependencies: `github.com/kaptinlin/requests` (transitive via defuddle), `golang.org/x/net` (transitive via defuddle).

```go
package main

import (
    "context"
    "net/http"
    "net/http/cookiejar"
    "net/url"
    "time"

    "github.com/dotcommander/defuddle"
    "github.com/kaptinlin/requests"
    "golang.org/x/net/publicsuffix"
)

func parseWithCookies(ctx context.Context, targetURL string, cookies []*http.Cookie) (*defuddle.Result, error) {
    jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

    u, err := url.Parse(targetURL)
    if err != nil {
        return nil, err
    }
    jar.SetCookies(u, cookies)

    client := requests.New(
        requests.WithTimeout(30*time.Second),
        requests.WithCookieJar(jar),
        requests.WithUserAgent("Mozilla/5.0 (compatible; MyBot/1.0)"),
    )

    return defuddle.ParseFromURL(ctx, targetURL, &defuddle.Options{
        Client: client,
    })
}
```

Load `cookies` from a browser extension export (e.g. [cookies.txt format](https://curl.se/docs/http-cookies.html)) or a previous login flow.

---

## 6. Pipe HTML from a Headless Browser

For JS-rendered pages, fetch with a headless browser and pipe the rendered HTML into defuddle. This is the standard workaround for client-only SPAs.

**Shell pipeline (using Playwright CLI):**

```bash
# Render and pipe
npx playwright eval --browser chromium "
  const page = await browser.newPage();
  await page.goto('https://example.com/spa-page', {waitUntil: 'networkidle'});
  process.stdout.write(await page.content());
" | defuddle parse
```

**Go with chromedp:**

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/chromedp/chromedp"
    "github.com/dotcommander/defuddle"
)

func fetchRendered(rawURL string) (string, error) {
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()
    ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    var html string
    err := chromedp.Run(ctx,
        chromedp.Navigate(rawURL),
        chromedp.WaitReady("body"),
        chromedp.OuterHTML("html", &html),
    )
    return html, err
}

func main() {
    targetURL := "https://example.com/spa-page"

    html, err := fetchRendered(targetURL)
    if err != nil {
        log.Fatal(err)
    }

    result, err := defuddle.ParseFromString(context.Background(), html, &defuddle.Options{
        URL:      targetURL,
        Markdown: true,
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Title)
    fmt.Println(result.Content)
}
```

See [docs/limitations.md](limitations.md) for a full discussion of JS-rendered pages and other cases where defuddle needs help from a headless browser.
