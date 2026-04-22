package extractors

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLeetCodeDoc(t *testing.T, body string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	require.NoError(t, err)
	return doc
}

const leetcodeBasePage = `<html><head>
<meta property="og:title" content="Two Sum - LeetCode" />
</head><body>
<div data-track-load="description_content"><p>Given an array of integers.</p></div>
</body></html>`

const leetcodeNoContainer = `<html><head>
<meta property="og:title" content="LeetCode" />
</head><body><p>Login page</p></body></html>`

func TestLeetCodeExtractor_CanExtract_True(t *testing.T) {
	t.Parallel()
	doc := newLeetCodeDoc(t, leetcodeBasePage)
	e := NewLeetCodeExtractor(doc, "https://leetcode.com/problems/two-sum/", nil)
	assert.True(t, e.CanExtract())
}

func TestLeetCodeExtractor_CanExtract_False(t *testing.T) {
	t.Parallel()
	doc := newLeetCodeDoc(t, leetcodeNoContainer)
	e := NewLeetCodeExtractor(doc, "https://leetcode.com/", nil)
	assert.False(t, e.CanExtract())
}

func TestLeetCodeExtractor_Name(t *testing.T) {
	t.Parallel()
	doc := newLeetCodeDoc(t, leetcodeBasePage)
	e := NewLeetCodeExtractor(doc, "https://leetcode.com/problems/two-sum/", nil)
	assert.Equal(t, "LeetCodeExtractor", e.Name())
}

func TestLeetCodeExtractor_Extract_StripsSuffix(t *testing.T) {
	t.Parallel()
	doc := newLeetCodeDoc(t, leetcodeBasePage)
	e := NewLeetCodeExtractor(doc, "https://leetcode.com/problems/two-sum/", nil)
	result := e.Extract()

	assert.Equal(t, "Two Sum", result.Variables["title"])
	assert.Equal(t, "LeetCode", result.Variables["site"])
}

func TestLeetCodeExtractor_Extract_ContentFromContainer(t *testing.T) {
	t.Parallel()
	doc := newLeetCodeDoc(t, leetcodeBasePage)
	e := NewLeetCodeExtractor(doc, "https://leetcode.com/problems/two-sum/", nil)
	result := e.Extract()

	assert.Contains(t, result.ContentHTML, "Given an array of integers")
	assert.Equal(t, result.Content, result.ContentHTML)
}

func TestLeetCodeExtractor_Extract_EmptyTitleFallback(t *testing.T) {
	t.Parallel()
	const page = `<html><head>
<meta property="og:title" content="LeetCode" />
</head><body>
<div data-track-load="description_content"><p>Problem text</p></div>
</body></html>`
	doc := newLeetCodeDoc(t, page)
	e := NewLeetCodeExtractor(doc, "https://leetcode.com/problems/test/", nil)
	result := e.Extract()
	// Suffix strip leaves "LeetCode" but falls back to original og:title when blank after strip.
	assert.Equal(t, "LeetCode", result.Variables["title"])
}

func TestLeetCodeExtractor_Extract_DashVariants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ogTitle string
		want    string
	}{
		{"Two Sum - LeetCode", "Two Sum"},
		{"Two Sum – LeetCode", "Two Sum"},
		{"Two Sum — LeetCode", "Two Sum"},
		{"Two Sum  -  LeetCode", "Two Sum"},
	}
	for _, tc := range cases {
		t.Run(tc.ogTitle, func(t *testing.T) {
			t.Parallel()
			page := `<html><head><meta property="og:title" content="` + tc.ogTitle + `" /></head><body>` +
				`<div data-track-load="description_content"><p>x</p></div></body></html>`
			doc := newLeetCodeDoc(t, page)
			e := NewLeetCodeExtractor(doc, "https://leetcode.com/problems/two-sum/", nil)
			result := e.Extract()
			assert.Equal(t, tc.want, result.Variables["title"])
		})
	}
}
