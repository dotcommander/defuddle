package extractors

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// LinkedInExtractor handles LinkedIn post and feed-update content extraction.
//
// Only publicly-accessible feed posts are handled — pages behind the login
// wall return no post article, so CanExtract() gates on article presence.
//
// TypeScript original:
//
//	export class LinkedInExtractor extends BaseExtractor {
//	  private postArticle: Element | null;
//	  constructor(...) {
//	    this.postArticle = document.querySelector('[role="article"].feed-shared-update-v2');
//	  }
//	  canExtract(): boolean { return !!this.postArticle; }
//	}
type LinkedInExtractor struct {
	*ExtractorBase
	postArticle *goquery.Selection
}

// NewLinkedInExtractor creates a new LinkedIn extractor and locates the post article.
func NewLinkedInExtractor(document *goquery.Document, url string, schemaOrgData any) *LinkedInExtractor {
	e := &LinkedInExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}
	if a := document.Find(`[role="article"].feed-shared-update-v2`).First(); a.Length() > 0 {
		e.postArticle = a
	}
	return e
}

// Name returns the extractor identifier.
func (e *LinkedInExtractor) Name() string { return "LinkedInExtractor" }

// CanExtract returns true when a LinkedIn feed-update article is present.
// Login-walled pages have no such article and return false.
func (e *LinkedInExtractor) CanExtract() bool {
	return e.postArticle != nil && e.postArticle.Length() > 0
}

// Extract returns the structured content for a LinkedIn post page.
func (e *LinkedInExtractor) Extract() *ExtractorResult {
	postContent := e.getPostContent()
	comments := e.extractComments()
	contentHTML := buildContentHtml("linkedin", postContent, comments)

	author := e.getAuthorName()
	description := e.createDescription()
	title := TruncateTitle(fmt.Sprintf("%s on LinkedIn", author), 100)
	if author == "" {
		title = "LinkedIn Post"
	}

	postURN, _ := e.postArticle.Attr("data-urn")

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"postUrn": postURN,
		},
		Variables: map[string]string{
			"title":       title,
			"author":      author,
			"site":        "LinkedIn",
			"description": description,
		},
	}
}

// getPostContent builds the HTML for the main post: text + images + video +
// quoted/reposted content.
func (e *LinkedInExtractor) getPostContent() string {
	if e.postArticle == nil {
		return ""
	}

	quotedWrapper := e.postArticle.Find(".feed-shared-update-v2__update-content-wrapper").First()
	textEl := e.postArticle.Find(
		".update-components-text.update-components-update-v2__commentary",
	).First()

	text := ""
	// Only take the top-level commentary, not one nested inside the quoted post.
	if textEl.Length() > 0 && (quotedWrapper.Length() == 0 || !linkedinContains(quotedWrapper, textEl)) {
		text = e.cleanTextContent(textEl)
	}

	images := e.extractImages()
	video := e.extractVideo()
	quotedPost := e.extractQuotedPost(quotedWrapper)

	var parts []string
	if text != "" {
		parts = append(parts, text)
	}
	if images != "" {
		parts = append(parts, images)
	}
	if video != "" {
		parts = append(parts, video)
	}
	if quotedPost != "" {
		parts = append(parts, quotedPost)
	}
	return strings.Join(parts, "\n")
}

// getAuthorName returns the post author's display name without badges/icons.
func (e *LinkedInExtractor) getAuthorName() string {
	if e.postArticle == nil {
		return ""
	}
	nameEl := e.postArticle.Find(".update-components-actor__title").First()
	return e.getVisibleText(nameEl,
		".text-view-model__verified-icon, .update-components-actor__supplementary-actor-info")
}

// createDescription produces a ≤140-char plain-text description from the
// top-level commentary text, excluding any quoted post body.
func (e *LinkedInExtractor) createDescription() string {
	if e.postArticle == nil {
		return ""
	}
	quotedWrapper := e.postArticle.Find(".feed-shared-update-v2__update-content-wrapper").First()
	textEl := e.postArticle.Find(
		".update-components-text.update-components-update-v2__commentary",
	).First()
	if textEl.Length() == 0 || (quotedWrapper.Length() > 0 && linkedinContains(quotedWrapper, textEl)) {
		return ""
	}
	text := e.getVisibleText(textEl)
	runes := []rune(text)
	if len(runes) > 140 {
		runes = runes[:140]
	}
	return whitespaceRe.ReplaceAllString(string(runes), " ")
}
