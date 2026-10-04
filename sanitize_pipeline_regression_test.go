package defuddle

import (
	"context"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sanitizerPipelineFragment = `<p>Article content remains readable after sanitization.</p><math><msub><mi>x</mi><mn>1</mn></msub><msup><mi>y</mi><mn>2</mn></msup><script>unsafe-math-script</script></math><svg><circle cx="2" cy="3" r="1"></circle><script>unsafe-svg-script</script></svg>`

func TestSanitizerPipeline_GenericAndExtractorMathScripts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, url, selector, extractor string }{
		{"generic", "https://generic-fixture.invalid/article", "article", ""},
		{"extractor", "https://en.wikipedia.org/wiki/Fixture", "", "wikipedia"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseFromString(context.Background(), `<html><body><article id="mw-content-text">`+sanitizerPipelineFragment+"</article></body></html>", &Options{URL: tc.url, ContentSelector: tc.selector})
			require.NoError(t, err)
			if tc.extractor != "" {
				require.NotNil(t, result.ExtractorType)
				assert.Equal(t, tc.extractor, *result.ExtractorType)
			} else {
				assert.Nil(t, result.ExtractorType)
			}
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(result.Content))
			require.NoError(t, err)
			assert.Empty(t, doc.Find("script").Nodes)
			assert.NotContains(t, result.Content, "unsafe-math-script")
			assert.NotContains(t, result.Content, "unsafe-svg-script")
			assert.Contains(t, result.Content, "Article content remains readable")
			assert.Equal(t, 1, doc.Find("msub").Length())
			assert.Equal(t, 1, doc.Find("msup").Length())
			assert.Equal(t, 1, doc.Find("svg circle").Length())
		})
	}
}
