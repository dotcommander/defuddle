package extractors

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// nytPreloadRe extracts the window.__preloadedData JSON assignment from an
// inline <script> element. The NYT payload ends with an optional semicolon.
var nytPreloadRe = regexp.MustCompile(`window\.__preloadedData\s*=\s*(\{[\s\S]+\})\s*;?\s*$`)

// nytUndefinedRe replaces bare JS `undefined` values (after colons) with null
// so the payload becomes valid JSON.
var nytUndefinedRe = regexp.MustCompile(`(?m)(:\s*)undefined([,\}\]])`)

// NytimesExtractor handles New York Times article content extraction by parsing
// the window.__preloadedData JSON blob embedded in the page's inline scripts.
//
// TypeScript original:
//
//	export class NytimesExtractor extends BaseExtractor {
//	  private preloadedData: NytArticle | null = null;
//	  private contentSelector: string | null = null;
//	  constructor(...) { this.preloadedData = this.extractPreloadData(); ... }
//	  canExtract(): boolean { return this.contentSelector !== null; }
//	}
type NytimesExtractor struct {
	*ExtractorBase
	preloadedData *nytArticle
	renderedHTML  string // rendered block HTML; non-empty iff extraction succeeded
}

// NewNytimesExtractor creates a new NYTimes extractor, parsing the preloaded
// JSON and rendering the article body blocks to HTML.
func NewNytimesExtractor(document *goquery.Document, url string, schemaOrgData any) *NytimesExtractor {
	e := &NytimesExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
	e.preloadedData = e.extractPreloadData(document)
	if e.preloadedData != nil {
		body := e.preloadedData.SprinkledBody
		if body == nil {
			body = e.preloadedData.Body
		}
		if body != nil && len(body.Content) > 0 {
			e.renderedHTML = e.renderBlocks(body.Content)
		}
	}
	return e
}

// Name returns the extractor identifier.
func (e *NytimesExtractor) Name() string { return "NytimesExtractor" }

// CanExtract returns true when preloaded JSON with article body blocks was found.
func (e *NytimesExtractor) CanExtract() bool { return e.renderedHTML != "" }

// Extract returns structured metadata and the rendered article body.
func (e *NytimesExtractor) Extract() *ExtractorResult {
	article := e.preloadedData

	title := ""
	if article.Headline != nil {
		title = article.Headline.Default
	}

	var authorParts []string
	if len(article.Bylines) > 0 {
		for _, c := range article.Bylines[0].Creators {
			if c.DisplayName != "" {
				authorParts = append(authorParts, c.DisplayName)
			}
		}
	}

	return &ExtractorResult{
		Content:     e.renderedHTML,
		ContentHTML: e.renderedHTML,
		Variables: map[string]string{
			"title":       title,
			"author":      strings.Join(authorParts, ", "),
			"published":   article.FirstPublished,
			"description": article.Summary,
		},
	}
}

// extractPreloadData locates the inline script containing window.__preloadedData
// and unmarshals the article sub-object.
func (e *NytimesExtractor) extractPreloadData(doc *goquery.Document) *nytArticle {
	var found *nytArticle
	doc.Find("script:not([src])").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := s.Text()
		if !strings.Contains(text, "window.__preloadedData") {
			return true
		}
		m := nytPreloadRe.FindStringSubmatch(text)
		if m == nil {
			return true
		}
		// Replace JS `undefined` so json.Unmarshal accepts the payload.
		raw := nytUndefinedRe.ReplaceAllString(m[1], "${1}null${2}")
		var data nytPreloadedData
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			return true
		}
		if data.InitialData != nil && data.InitialData.Data != nil {
			found = data.InitialData.Data.Article
		}
		return false // stop at first match
	})
	return found
}

// renderBlocks converts the NYT block sequence to an HTML string.
// Mirrors the upstream TS renderBlocks() method.
func (e *NytimesExtractor) renderBlocks(blocks []json.RawMessage) string {
	var parts []string
	for _, raw := range blocks {
		var base nytBlock
		if err := json.Unmarshal(raw, &base); err != nil {
			continue
		}
		switch base.Typename {
		case "ParagraphBlock":
			var b nytParagraphBlock
			if err := json.Unmarshal(raw, &b); err == nil {
				parts = append(parts, fmt.Sprintf("<p>%s</p>", e.renderInlines(b.Content)))
			}
		case "Heading2Block":
			var b nytHeadingBlock
			if err := json.Unmarshal(raw, &b); err == nil {
				parts = append(parts, fmt.Sprintf("<h2>%s</h2>", e.renderInlines(b.Content)))
			}
		case "Heading3Block":
			var b nytHeadingBlock
			if err := json.Unmarshal(raw, &b); err == nil {
				parts = append(parts, fmt.Sprintf("<h3>%s</h3>", e.renderInlines(b.Content)))
			}
		case "Heading4Block":
			var b nytHeadingBlock
			if err := json.Unmarshal(raw, &b); err == nil {
				parts = append(parts, fmt.Sprintf("<h4>%s</h4>", e.renderInlines(b.Content)))
			}
		case "ImageBlock":
			if imgHTML := e.renderImageBlock(raw); imgHTML != "" {
				parts = append(parts, imgHTML)
			}
		case "HeaderBasicBlock", "Dropzone":
			// Structural chrome — skip.
		default:
			// Unknown blocks with inline content fall back to <p>.
			var b nytParagraphBlock
			if err := json.Unmarshal(raw, &b); err == nil && len(b.Content) > 0 {
				parts = append(parts, fmt.Sprintf("<p>%s</p>", e.renderInlines(b.Content)))
			}
		}
	}
	return strings.Join(parts, "\n")
}

// renderImageBlock unmarshals an ImageBlock and emits <figure> or <img> HTML.
func (e *NytimesExtractor) renderImageBlock(raw json.RawMessage) string {
	var b nytImageBlock
	if err := json.Unmarshal(raw, &b); err != nil || b.Media == nil {
		return ""
	}
	src := e.getBestImageURL(b.Media)
	if src == "" {
		return ""
	}
	altText := b.Media.AltText
	if altText == "" && b.Media.Caption != nil {
		altText = b.Media.Caption.Text
	}

	var caption, credit string
	if b.Media.Caption != nil {
		caption = b.Media.Caption.Text
	}
	credit = b.Media.Credit

	var figParts []string
	if caption != "" {
		figParts = append(figParts, caption)
	}
	if credit != "" {
		figParts = append(figParts, credit)
	}

	srcAttr := html.EscapeString(src)
	altAttr := html.EscapeString(altText)
	if len(figParts) > 0 {
		return fmt.Sprintf(`<figure><img src="%s" alt="%s"><figcaption>%s</figcaption></figure>`,
			srcAttr, altAttr, html.EscapeString(strings.Join(figParts, " ")))
	}
	return fmt.Sprintf(`<img src="%s" alt="%s">`, srcAttr, altAttr)
}

// getBestImageURL selects the highest-quality rendition URL from the media crops.
// Preference: superJumbo → jumbo → articleLarge → first available.
func (e *NytimesExtractor) getBestImageURL(media *nytMedia) string {
	for _, name := range []string{"superJumbo", "jumbo", "articleLarge"} {
		for _, crop := range media.Crops {
			for _, r := range crop.Renditions {
				if r.Name == name && r.URL != "" {
					return r.URL
				}
			}
		}
	}
	for _, crop := range media.Crops {
		if len(crop.Renditions) > 0 && crop.Renditions[0].URL != "" {
			return crop.Renditions[0].URL
		}
	}
	return ""
}

// renderInlines converts a slice of inline nodes to an HTML fragment.
// Applies BoldFormat, ItalicFormat, and LinkFormat decorations.
func (e *NytimesExtractor) renderInlines(inlines []nytInline) string {
	var b strings.Builder
	for _, inline := range inlines {
		text := html.EscapeString(inline.Text)
		for _, f := range inline.Formats {
			switch f.Typename {
			case "BoldFormat":
				text = "<strong>" + text + "</strong>"
			case "ItalicFormat":
				text = "<em>" + text + "</em>"
			case "LinkFormat":
				if f.URL != "" {
					text = `<a href="` + html.EscapeString(f.URL) + `">` + text + `</a>`
				}
			}
		}
		b.WriteString(text)
	}
	return b.String()
}
