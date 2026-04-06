package defuddle

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkParse benchmarks the main Parse operation
func BenchmarkParse(b *testing.B) {
	html := `<html>
		<head>
			<title>Test Article</title>
			<meta name="description" content="This is a test article">
		</head>
		<body>
			<article>
				<h1>Main Article Title</h1>
				<p>This is the first paragraph with some content.</p>
				<p>This is the second paragraph with more content.</p>
				<p>This is the third paragraph with even more content.</p>
			</article>
		</body>
	</html>`

	defuddle, err := NewDefuddle(html, nil)
	if err != nil {
		b.Fatalf("Failed to create Defuddle instance: %v", err)
	}

	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		_, err := defuddle.Parse(ctx)
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

// BenchmarkParseFromString benchmarks parsing from string
func BenchmarkParseFromString(b *testing.B) {
	html := `<html>
		<head>
			<title>Test Article</title>
			<meta name="description" content="This is a test article">
		</head>
		<body>
			<article>
				<h1>Main Article Title</h1>
				<p>This is the first paragraph with some content.</p>
				<p>This is the second paragraph with more content.</p>
				<p>This is the third paragraph with even more content.</p>
			</article>
		</body>
	</html>`

	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		_, err := ParseFromString(ctx, html, nil)
		if err != nil {
			b.Fatalf("ParseFromString failed: %v", err)
		}
	}
}

// BenchmarkParseWithMarkdown benchmarks parsing with markdown conversion
func BenchmarkParseWithMarkdown(b *testing.B) {
	html := `<html>
		<head>
			<title>Test Article</title>
		</head>
		<body>
			<article>
				<h1>Main Article Title</h1>
				<p>This is the first paragraph with some content.</p>
				<p>This is the second paragraph with more content.</p>
			</article>
		</body>
	</html>`

	options := &Options{
		Markdown: true,
	}

	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		_, err := ParseFromString(ctx, html, options)
		if err != nil {
			b.Fatalf("ParseFromString with markdown failed: %v", err)
		}
	}
}

// BenchmarkNewDefuddle benchmarks Defuddle instance creation
func BenchmarkNewDefuddle(b *testing.B) {
	html := `<html>
		<head><title>Test</title></head>
		<body>
			<article>
				<h1>Title</h1>
				<p>Content paragraph.</p>
			</article>
		</body>
	</html>`

	b.ResetTimer()

	for b.Loop() {
		_, err := NewDefuddle(html, nil)
		if err != nil {
			b.Fatalf("NewDefuddle failed: %v", err)
		}
	}
}

// BenchmarkCountWords benchmarks word counting
func BenchmarkCountWords(b *testing.B) {
	html := `<html><body><p>This is a test paragraph with multiple words to count.</p></body></html>`

	defuddle, err := NewDefuddle(html, nil)
	if err != nil {
		b.Fatalf("Failed to create Defuddle instance: %v", err)
	}

	content := "<p>This is a test paragraph with multiple words to count. " +
		"It has several sentences and should provide a good benchmark for word counting.</p>"

	b.ResetTimer()

	for b.Loop() {
		_ = defuddle.countWords(content)
	}
}

// BenchmarkExtractSchemaOrgData benchmarks schema.org data extraction
func BenchmarkExtractSchemaOrgData(b *testing.B) {
	html := `<html>
		<head>
			<script type="application/ld+json">
			{
				"@context": "https://schema.org",
				"@type": "Article",
				"headline": "Test Article",
				"author": {
					"@type": "Person",
					"name": "John Doe"
				}
			}
			</script>
		</head>
		<body><p>Content</p></body>
	</html>`

	defuddle, err := NewDefuddle(html, nil)
	if err != nil {
		b.Fatalf("Failed to create Defuddle instance: %v", err)
	}

	b.ResetTimer()

	for b.Loop() {
		_ = defuddle.extractSchemaOrgData()
	}
}

// BenchmarkFindMainContent benchmarks main content detection
func BenchmarkFindMainContent(b *testing.B) {
	html := `<html>
		<body>
			<header>Header content</header>
			<nav>Navigation</nav>
			<article>
				<h1>Main Article</h1>
				<p>This is the main content of the article.</p>
				<p>More content here.</p>
			</article>
			<aside>Sidebar</aside>
			<footer>Footer</footer>
		</body>
	</html>`

	defuddle, err := NewDefuddle(html, nil)
	if err != nil {
		b.Fatalf("Failed to create Defuddle instance: %v", err)
	}

	b.ResetTimer()

	for b.Loop() {
		_ = defuddle.findMainContent(defuddle.doc)
	}
}

// BenchmarkRemoveBySelector benchmarks selector-based removal
func BenchmarkRemoveBySelector(b *testing.B) {
	html := `<html>
		<body>
			<div class="advertisement">Ad</div>
			<div class="content">Content</div>
			<div class="sidebar">Sidebar</div>
			<div class="footer">Footer</div>
		</body>
	</html>`

	b.ResetTimer()

	for b.Loop() {
		defuddle, err := NewDefuddle(html, nil)
		if err != nil {
			b.Fatalf("Failed to create Defuddle instance: %v", err)
		}
		defuddle.removeBySelector(defuddle.doc, true, true, nil)
	}
}

// loadFixture reads a real-world HTML fixture from the reference test directory.
// Returns empty string if fixture is not available (skips benchmark).
func loadFixture(b *testing.B, name string) string {
	b.Helper()
	path := filepath.Join(".reference", "defuddle", "tests", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		b.Skipf("fixture %s not available: %v", name, err)
	}
	return string(data)
}

// BenchmarkRealWorld_Blog benchmarks a real blog post (~19KB)
func BenchmarkRealWorld_Blog(b *testing.B) {
	html := loadFixture(b, "stephango.com-buy-wisely.html")
	ctx := context.Background()
	b.SetBytes(int64(len(html)))
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := ParseFromString(ctx, html, nil)
		if err != nil {
			b.Fatalf("parse failed: %v", err)
		}
	}
}

// BenchmarkRealWorld_BlogMarkdown benchmarks a real blog post with markdown conversion
func BenchmarkRealWorld_BlogMarkdown(b *testing.B) {
	html := loadFixture(b, "stephango.com-buy-wisely.html")
	ctx := context.Background()
	opts := &Options{Markdown: true}
	b.SetBytes(int64(len(html)))
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := ParseFromString(ctx, html, opts)
		if err != nil {
			b.Fatalf("parse failed: %v", err)
		}
	}
}

// BenchmarkRealWorld_GitHubIssue benchmarks a GitHub issue page (~295KB)
func BenchmarkRealWorld_GitHubIssue(b *testing.B) {
	html := loadFixture(b, "github.com-issue-56.html")
	ctx := context.Background()
	b.SetBytes(int64(len(html)))
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := ParseFromString(ctx, html, nil)
		if err != nil {
			b.Fatalf("parse failed: %v", err)
		}
	}
}

// BenchmarkRealWorld_LargeArticle benchmarks a large article page (~664KB)
func BenchmarkRealWorld_LargeArticle(b *testing.B) {
	html := loadFixture(b, "x.com-article-2026-02-13.html")
	ctx := context.Background()
	b.SetBytes(int64(len(html)))
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := ParseFromString(ctx, html, nil)
		if err != nil {
			b.Fatalf("parse failed: %v", err)
		}
	}
}

// BenchmarkRealWorld_TechBlog benchmarks a technical blog with code blocks (~5KB)
func BenchmarkRealWorld_TechBlog(b *testing.B) {
	html := loadFixture(b, "rockthejvm.com!articles!kotlin-101-type-classes.html")
	ctx := context.Background()
	b.SetBytes(int64(len(html)))
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := ParseFromString(ctx, html, nil)
		if err != nil {
			b.Fatalf("parse failed: %v", err)
		}
	}
}

// BenchmarkRealWorld_LessWrong benchmarks a LessWrong article with footnotes (~247KB)
func BenchmarkRealWorld_LessWrong(b *testing.B) {
	html := loadFixture(b, "lesswrong.com!s!N7nDePaNabJdnbXeE!p!vJFdjigzmcXMhNTsx.html")
	ctx := context.Background()
	b.SetBytes(int64(len(html)))
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := ParseFromString(ctx, html, nil)
		if err != nil {
			b.Fatalf("parse failed: %v", err)
		}
	}
}

// BenchmarkCollectMetaTags benchmarks meta tag collection
func BenchmarkCollectMetaTags(b *testing.B) {
	html := `<html>
		<head>
			<meta name="description" content="Test description">
			<meta name="author" content="John Doe">
			<meta property="og:title" content="Test Title">
			<meta property="og:description" content="OG Description">
			<meta property="og:image" content="https://example.com/image.jpg">
		</head>
		<body><p>Content</p></body>
	</html>`

	defuddle, err := NewDefuddle(html, nil)
	if err != nil {
		b.Fatalf("Failed to create Defuddle instance: %v", err)
	}

	b.ResetTimer()

	for b.Loop() {
		_ = defuddle.collectMetaTags()
	}
}
