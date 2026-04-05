package extractors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const githubBaseHTML = `<html>
<head>
	<meta name="expected-hostname" content="github.com">
	<meta name="github-keyboard-shortcuts" content="">
	<title>Bug report · owner/repo</title>
</head>
<body>
	<div data-testid="issue-metadata-sticky">metadata</div>
	<div data-testid="issue-title">Bug report</div>
	<div data-testid="issue-viewer-issue-container">
		<a data-testid="issue-body-header-author" href="/testuser">testuser</a>
		<relative-time datetime="2024-03-15T10:00:00Z"></relative-time>
		<div data-testid="issue-body-viewer">
			<div class="markdown-body">
				<p>This is the issue body with some details.</p>
				<pre><code>fmt.Println("hello")</code></pre>
			</div>
		</div>
	</div>
</body>
</html>`

func TestGitHubExtractor_CanExtract_True(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, githubBaseHTML)
	ext := NewGitHubExtractor(doc, "https://github.com/owner/repo/issues/42", nil)
	assert.True(t, ext.CanExtract())
}

func TestGitHubExtractor_CanExtract_MissingGitHubIndicator(t *testing.T) {
	t.Parallel()
	html := `<html><head><title>Not GitHub</title></head>
	<body><div data-testid="issue-metadata-sticky">x</div><div data-testid="issue-title">y</div></body></html>`
	doc := newTestDoc(t, html)
	ext := NewGitHubExtractor(doc, "https://github.com/o/r/issues/1", nil)
	assert.False(t, ext.CanExtract())
}

func TestGitHubExtractor_CanExtract_MissingPageIndicator(t *testing.T) {
	t.Parallel()
	html := `<html><head>
	<meta name="expected-hostname" content="github.com">
	<meta name="github-keyboard-shortcuts" content="">
	</head><body><p>No issue indicators</p></body></html>`
	doc := newTestDoc(t, html)
	ext := NewGitHubExtractor(doc, "https://github.com/o/r/issues/1", nil)
	assert.False(t, ext.CanExtract())
}

func TestGitHubExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewGitHubExtractor(doc, "", nil)
	assert.Equal(t, "GitHubExtractor", ext.Name())
}

func TestGitHubExtractor_Extract(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, githubBaseHTML)
	ext := NewGitHubExtractor(doc, "https://github.com/owner/repo/issues/42", nil)
	result := ext.Extract()

	require.NotNil(t, result)
	assert.Contains(t, result.ContentHTML, "This is the issue body")
	// goquery HTML-encodes quotes inside attributes/text, so check for the encoded form.
	assert.Contains(t, result.ContentHTML, "fmt.Println")
	assert.Equal(t, "issue", result.ExtractedContent["type"])
	assert.Equal(t, "42", result.ExtractedContent["issueNumber"])
	assert.Equal(t, "owner", result.ExtractedContent["owner"])
	assert.Equal(t, "repo", result.ExtractedContent["repository"])
}

func TestGitHubExtractor_ExtractRepoInfo_FromURL(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><head><title>Some Title</title></head><body></body></html>")
	ext := NewGitHubExtractor(doc, "https://github.com/myorg/myrepo/issues/1", nil)
	info := ext.extractRepoInfo()
	assert.Equal(t, "myorg", info["owner"])
	assert.Equal(t, "myrepo", info["repo"])
}

func TestGitHubExtractor_ExtractRepoInfo_FallbackToTitle(t *testing.T) {
	t.Parallel()
	doc := newTestDoc(t, "<html><head><title>Issue · orgname/reponame</title></head><body></body></html>")
	ext := NewGitHubExtractor(doc, "https://not-a-github-url.com/page", nil)
	info := ext.extractRepoInfo()
	assert.Equal(t, "orgname", info["owner"])
	assert.Equal(t, "reponame", info["repo"])
}

func TestGitHubExtractor_ExtractIssueNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"standard issue", "https://github.com/o/r/issues/123", "123"},
		{"no issue number", "https://github.com/o/r/pulls", ""},
		{"issue with path after", "https://github.com/o/r/issues/456/comments", "456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := newTestDoc(t, "<html><body></body></html>")
			ext := NewGitHubExtractor(doc, tt.url, nil)
			assert.Equal(t, tt.want, ext.extractIssueNumber())
		})
	}
}

func TestGitHubExtractor_CreateDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		maxLen  int
	}{
		{"empty", "", 0},
		{"short content", "<p>Short text</p>", 140},
		{"truncates long content", "<p>" + longText(200) + "</p>", 140},
	}

	doc := newTestDoc(t, "<html><body></body></html>")
	ext := NewGitHubExtractor(doc, "https://github.com/o/r/issues/1", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			desc := ext.createDescription(tt.content)
			if tt.content == "" {
				assert.Empty(t, desc)
			} else {
				assert.LessOrEqual(t, len(desc), tt.maxLen)
				assert.NotEmpty(t, desc)
			}
		})
	}
}

func longText(n int) string {
	word := "word "
	result := ""
	for len(result) < n {
		result += word
	}
	return result
}
