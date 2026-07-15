# When NOT to Use Defuddle

Defuddle is an article extractor for static HTML. Several classes of pages will produce empty, partial, or incorrect results. This page describes each case, what symptom you'll see, and what to do instead.

---

## JS-Rendered Pages

**Symptom:** `result.Content` is nearly empty or contains only skeleton HTML like `<div id="root"></div>`. `result.WordCount` is 0 or very low.

**Why:** Defuddle fetches raw HTML over HTTP. For sites that use client-side rendering (Next.js app router without SSR, create-react-app, Vite SPA), the server returns a shell document with no readable content. JavaScript runs in a browser, not in defuddle.

**Example:** A Next.js client-only page returns something like:

```html
<html>
  <body><div id="__next"></div></body>
  <script src="/_next/static/chunks/main.js"></script>
</html>
```

Defuddle extracts the `<div id="__next">` — which is empty.

**What to do instead:**

**CLI:** use the built-in `--render` (alias `--js`) flag, which renders the page in headless Chrome before extraction:

```bash
defuddle parse --render https://example.com/article
defuddle parse --render --render-wait networkidle https://example.com/article
```

This requires an existing Chrome/Chromium install (chromedp drives it over CDP; no browser is bundled). See the [CLI reference](cli.md#javascript-rendering-opt-in) for `--chrome-path`, `--render-timeout`, and `--render-user-agent`.

**Library:** the `defuddle` package itself does not execute JavaScript. Pre-render with a headless browser (e.g. [chromedp](https://github.com/chromedp/chromedp)) and pass the resulting HTML to `ParseFromString`:

```go
html := fetchWithChromedp(ctx, url) // your headless fetch
result, err := defuddle.ParseFromString(ctx, html, &defuddle.Options{URL: url})
```

---

## Paywalls

**Symptom:** `result.Content` contains the paywall or metered-article message instead of the article body. Title and metadata may be correct because they appear in `<head>`.

**Why:** Defuddle fetches what the server sends. If the server enforces a paywall in HTML (showing a truncated article + a subscribe prompt), that's what defuddle extracts.

**What to do instead:**

Pass an authenticated `*http.Client` with session cookies from a valid subscription:

```go
import "net/http"

jar := createCookieJar() // load session cookies from browser or login flow
client := &http.Client{Jar: jar}

result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    Client: client,
    Headers: http.Header{"User-Agent": []string{"Mozilla/5.0 ..."}},
})
```

Note: using another person's session cookies without authorization violates most terms of service.

---

## Login-Gated Content

**Symptom:** `ParseFromURL` returns an error like `fetch https://example.com: content-type "text/html": ...` with empty content, or the result contains a login form.

**Why:** The server responds with a 401, 302 redirect to a login page, or a login form in HTML. Defuddle does not follow auth redirects in any special way.

**What to do instead:**

- Same cookie-jar approach as paywalls above.
- Alternatively, use a browser session export tool to dump cookies and pass them as `Cookie: ...` headers via a custom client.

---

## PDFs and Binary Content

**Symptom:** `ParseFromURL` returns `defuddle.ErrNotHTML`.

**Why:** Defuddle checks the `Content-Type` response header. Any type that isn't `text/html`, `application/xhtml+xml`, `text/xml`, or `text/*` is rejected immediately with `ErrNotHTML`. This includes `application/pdf`, `image/*`, `application/octet-stream`, etc.

**What to do instead:**

Sniff the content type before calling defuddle, and route binary content to the appropriate handler:

```go
resp, err := http.Head(url)
if err == nil && !strings.Contains(resp.Header.Get("Content-Type"), "html") {
    // handle as PDF, image, etc.
    return
}
result, err := defuddle.ParseFromURL(ctx, url, nil)
```

---

## Size Limits

**Symptom:** `ParseFromURL` returns `defuddle.ErrTooLarge`.

**Why:** Defuddle enforces a hard cap of **5 MB** (`maxResponseSize`) on response bodies. Pages over this limit are rejected before parsing begins. This is intentional — defuddle is an article extractor, and 5 MB HTML is almost certainly not an article.

**What to do instead:**

Check `errors.Is(err, defuddle.ErrTooLarge)` and either skip the URL or fetch with a streaming client and truncate to a safe size before calling `ParseFromString`.

---

## CAPTCHA and Bot Detection

**Symptom:** `result.Content` contains a CAPTCHA challenge, a Cloudflare interstitial, or a "please enable JavaScript" message.

**Why:** Defuddle makes a plain HTTP request with a browser-like User-Agent. Aggressive bot-detection (Cloudflare JS challenge, DataDome, PerimeterX) detects the absence of browser-side JavaScript execution and serves a challenge page instead of the article.

**What to do instead:**

There is no reliable general solution. Options:

- A headless browser with stealth plugins can pass some bot-detection checks.
- Request a copy of the content via an official API if one exists (e.g. NYTimes, Guardian).
- Use a residential proxy service — though this is often against the site's terms of service.

---

## Heuristic Limits on Non-Article Pages

**Symptom:** Results are noisy or incomplete on forum threads, search results pages, category listings, or comment-heavy pages.

**Why:** Defuddle's content scoring is tuned for article-shaped pages: a dominant text block, headings, paragraphs. Pages where content is distributed across many equal-weight blocks (comment threads, listing pages) confuse the scorer, and the automatic retry cascade may not recover.

**What to do instead:**

- Check whether a site-specific extractor exists: `defuddle extractors --match https://example.com/thread/123`. Discourse, Reddit, and Hacker News have extractors that handle threaded pages correctly.
- Use `ContentSelector` to pin the content root to a known CSS selector for that site:

```go
result, err := defuddle.ParseFromURL(ctx, url, &defuddle.Options{
    ContentSelector: "div.thread-body",
})
```

---

## Workarounds Summary

| Problem | Workaround |
|---------|-----------|
| JS-rendered page | CLI: `defuddle parse --render`; Library: pre-render (e.g. chromedp) then `ParseFromString` |
| Paywall / login wall | Pass authenticated `*http.Client` with a cookie jar |
| PDF / binary | Check `Content-Type` before calling defuddle; route separately |
| Over 5 MB | Skip, or truncate before `ParseFromString` |
| CAPTCHA / bot-detection | Headless browser with stealth mode; official API if available |
| Non-article page | Use `ContentSelector`; check for a site-specific extractor first |
