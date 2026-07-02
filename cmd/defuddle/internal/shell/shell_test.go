package shell

import "testing"

const staticArticleHTML = `<!doctype html><html><head><title>Real Article</title></head>
<body><article>
<h1>A Genuine Article Headline That Is Reasonably Descriptive</h1>
<p>This is the first paragraph of a real article. It contains enough prose that the visible-text heuristic comfortably exceeds the two hundred character threshold on its own.</p>
<p>A second paragraph continues the discussion with additional sentences, more than doubling the amount of human-readable content present in semantic tags.</p>
<p>A third paragraph closes things out so the document clearly reads as an article rather than an empty JavaScript shell awaiting hydration.</p>
</article></body></html>`

const spaShellHTML = `<!doctype html><html><head><title>My App</title></head>
<body>
<div id="root"></div>
<noscript>You need to enable JavaScript to run this app.</noscript>
<script src="/static/js/main.9f2a4c.chunk.js"></script>
<script src="/static/js/2.b3c1d0.chunk.js"></script>
</body></html>`

// jsLoadingEdgeHTML has >=200 visible chars but the content is itself a
// JavaScript-required notice inside a shell root: must NOT be treated as static.
const jsLoadingEdgeHTML = `<!doctype html><html><head><title>Loading</title></head>
<body>
<div id="root">
<main><p>You need to enable JavaScript to run this app. This placeholder message is deliberately padded well beyond two hundred characters so the raw visible-text length passes the static threshold even though the page is really a client-rendered shell.</p></main>
</div>
<script src="/static/js/bundle.4471.js"></script>
</body></html>`

const ambiguousHTML = `<!doctype html><html><head><title>List</title></head>
<body><ul><li>alpha</li><li>beta</li><li>gamma</li></ul></body></html>`

func TestClassify_StaticArticle(t *testing.T) {
	t.Parallel()
	if got := Classify(staticArticleHTML); got != Static {
		t.Fatalf("static article: got %q, want %q", got, Static)
	}
}

func TestClassify_SPAShellEscalates(t *testing.T) {
	t.Parallel()
	if got := Classify(spaShellHTML); got != LikelyShell {
		t.Fatalf("spa shell: got %q, want %q", got, LikelyShell)
	}
}

func TestClassify_JSLoadingOver200NotStatic(t *testing.T) {
	t.Parallel()
	got := Classify(jsLoadingEdgeHTML)
	if got == Static {
		t.Fatalf("js-loading edge: got Static, want non-static (likely_shell)")
	}
	if got != LikelyShell {
		t.Fatalf("js-loading edge: got %q, want %q", got, LikelyShell)
	}
}

func TestClassify_AmbiguousDoesNotEscalate(t *testing.T) {
	t.Parallel()
	if got := Classify(ambiguousHTML); got == LikelyShell {
		t.Fatalf("ambiguous page escalated to LikelyShell; want static/ambiguous")
	}
}

func TestClassify_EmptyInputDoesNotEscalate(t *testing.T) {
	t.Parallel()
	if got := Classify(""); got == LikelyShell {
		t.Fatalf("empty input escalated; want non-shell")
	}
}
