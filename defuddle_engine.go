package defuddle

import (
	"fmt"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dotcommander/defuddle/internal/metadata"
	"github.com/dotcommander/defuddle/internal/urlutil"
	trafilatura "github.com/markusmobius/go-trafilatura/v2"
	"golang.org/x/net/html"
)

type extractionEngine func(*html.Node, trafilatura.Options) (*trafilatura.ExtractResult, error)

// Only the upstream invocation is protected: defects in preparation or safety
// processing must remain visible to callers rather than silently become recovery.
func callExtraction(engine extractionEngine, doc *goquery.Document, o *Options) (result *trafilatura.ExtractResult, reason string) {
	if engine == nil {
		engine = trafilatura.ExtractDocument
	}
	pageURL := urlutil.EffectiveBaseURL(o.URL, urlutil.ExtractBaseHref(doc))
	return invokeExtraction(engine, doc.Get(0), trafilatura.Options{Config: trafilatura.DefaultConfig(), OriginalURL: pageURL, ExcludeComments: true, EnableFallback: true, IncludeLinks: true, IncludeImages: !o.RemoveImages})
}

func invokeExtraction(engine extractionEngine, node *html.Node, options trafilatura.Options) (result *trafilatura.ExtractResult, reason string) {
	defer func() {
		if p := recover(); p != nil {
			result = nil
			reason = fmt.Sprintf("upstream panic: %v", p)
		}
	}()
	result, err := engine(node, options)
	if err != nil {
		return nil, "upstream error: " + err.Error()
	}
	if result == nil || result.ContentNode == nil {
		return nil, "nil upstream result"
	}
	return result, ""
}

func fillEngineMetadata(m *metadata.Metadata, r *trafilatura.ExtractResult) {
	fill := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}
	fill(&m.Title, r.Metadata.Title)
	fill(&m.Author, r.Metadata.Author)
	fill(&m.Description, r.Metadata.Description)
	fill(&m.Site, r.Metadata.Sitename)
	fill(&m.Image, r.Metadata.Image)
	fill(&m.Language, r.Metadata.Language)
	if m.Published == "" && !r.Metadata.Date.IsZero() {
		m.Published = r.Metadata.Date.Format(time.RFC3339)
	}
}
