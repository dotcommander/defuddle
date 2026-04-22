package extractors

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// linkedinDoubleBrRe matches two or more consecutive <br> tags (with optional
// whitespace between them), used to split post text into paragraphs.
var linkedinDoubleBrRe = regexp.MustCompile(`(?:<br\s*/?>)\s*(?:<br\s*/?>)+`)

// linkedinSingleBrRe matches a single <br> tag, replaced with a space when
// collapsing intra-paragraph line breaks.
var linkedinSingleBrRe = regexp.MustCompile(`<br\s*/?>`)

// linkedinHTMLCommentRe strips HTML comments injected by LinkedIn's renderer.
var linkedinHTMLCommentRe = regexp.MustCompile(`<!--.*?-->`)

// linkedinRelDateRe extracts the leading relative-date token (e.g. "1w") from
// LinkedIn's sub-description text (which may also contain " • 🌐").
var linkedinRelDateRe = regexp.MustCompile(`^(\d+\w+)`)

// linkedinContains reports whether ancestor contains descendant by node identity.
// Used to test whether the commentary element is nested inside the quoted post
// wrapper — if so, the top-level text extraction should be skipped.
func linkedinContains(ancestor, descendant *goquery.Selection) bool {
	if ancestor.Length() == 0 || descendant.Length() == 0 {
		return false
	}
	aNode := ancestor.Get(0)
	dNode := descendant.Get(0)
	for cur := dNode.Parent; cur != nil; cur = cur.Parent {
		if cur == aNode {
			return true
		}
	}
	return false
}

// getVisibleText returns the plain text of el after removing .visually-hidden
// nodes and any additional CSS selectors passed in alsoRemove.
func (e *LinkedInExtractor) getVisibleText(el *goquery.Selection, alsoRemove ...string) string {
	if el.Length() == 0 {
		return ""
	}
	clone := el.Clone()
	selector := ".visually-hidden"
	if len(alsoRemove) > 0 && alsoRemove[0] != "" {
		selector += ", " + alsoRemove[0]
	}
	clone.Find(selector).Remove()
	return strings.TrimSpace(clone.Text())
}

// cleanTextContent converts a LinkedIn commentary element into clean paragraph
// HTML. Screen-reader duplicates, see-more toggles, and link cruft are stripped;
// double-<br> sequences are converted to paragraph boundaries.
func (e *LinkedInExtractor) cleanTextContent(el *goquery.Selection) string {
	if el.Length() == 0 {
		return ""
	}
	clone := el.Clone()

	// Strip screen-reader-only spans and see-more toggles.
	clone.Find(".visually-hidden, .feed-shared-inline-show-more-text__see-more-less-toggle").Remove()

	// Simplify links: keep href+text, drop all LinkedIn tracking attributes.
	clone.Find("a").Each(func(_ int, link *goquery.Selection) {
		href, _ := link.Attr("href")
		text := strings.TrimSpace(link.Text())
		if href != "" && text != "" {
			for _, node := range link.Nodes {
				kept := node.Attr[:0]
				for _, a := range node.Attr {
					if a.Key == "href" {
						kept = append(kept, a)
					}
				}
				node.Attr = kept
			}
			link.SetText(text)
		} else {
			link.ReplaceWithHtml(html.EscapeString(link.Text()))
		}
	})

	rawHTML, _ := clone.Html()
	rawHTML = linkedinHTMLCommentRe.ReplaceAllString(rawHTML, "")

	var result []string
	for _, p := range linkedinDoubleBrRe.Split(rawHTML, -1) {
		p = linkedinSingleBrRe.ReplaceAllString(p, " ")
		p = whitespaceRe.ReplaceAllString(strings.TrimSpace(p), " ")
		if p != "" {
			result = append(result, "<p>"+p+"</p>")
		}
	}
	return strings.Join(result, "\n")
}

// extractQuotedPost renders a reposted/quoted LinkedIn post as a blockquote card.
func (e *LinkedInExtractor) extractQuotedPost(wrapper *goquery.Selection) string {
	if wrapper == nil || wrapper.Length() == 0 {
		return ""
	}

	actorTitle := wrapper.Find(".update-components-actor__title").First()
	authorName := e.getVisibleText(actorTitle,
		".update-components-actor__supplementary-actor-info, .text-view-model__verified-icon")

	date := linkedinQuotedDate(wrapper)

	textEl := wrapper.Find(".update-components-text.update-components-update-v2__commentary").First()
	content := e.cleanTextContent(textEl)

	postURL := linkedinAbsURL(wrapper.Find("a.update-components-mini-update-v2__link-to-details-page").First())

	return buildQuotedPost(authorName, content, date, postURL)
}

// linkedinQuotedDate extracts the relative-date string (e.g. "1w") from the
// quoted post's sub-description element.
func linkedinQuotedDate(wrapper *goquery.Selection) string {
	subDesc := wrapper.Find(".update-components-actor__sub-description").First()
	if subDesc.Length() == 0 {
		return ""
	}
	visible := subDesc.Find(`[aria-hidden="true"]`).First()
	src := subDesc
	if visible.Length() > 0 {
		src = visible
	}
	raw := strings.TrimSpace(src.Text())
	if m := linkedinRelDateRe.FindStringSubmatch(raw); len(m) > 1 {
		return m[1]
	}
	return ""
}

// linkedinAbsURL returns the absolute URL from a link element, stripping
// query parameters. Relative paths are prefixed with the LinkedIn origin.
func linkedinAbsURL(link *goquery.Selection) string {
	if link.Length() == 0 {
		return ""
	}
	href, _ := link.Attr("href")
	if href == "" {
		return ""
	}
	path := strings.SplitN(href, "?", 2)[0]
	if strings.HasPrefix(path, "http") {
		return path
	}
	return "https://www.linkedin.com" + path
}

// extractImages returns <img> tags for post images, skipping profile/avatar photos.
func (e *LinkedInExtractor) extractImages() string {
	if e.postArticle == nil {
		return ""
	}
	var images []string
	e.postArticle.Find(".update-components-image img, .feed-shared-image img").Each(func(_ int, img *goquery.Selection) {
		src, _ := img.Attr("src")
		alt, _ := img.Attr("alt")
		if src == "" || strings.Contains(src, "profile-displayphoto") || strings.Contains(src, "avm-avatar") {
			return
		}
		images = append(images, fmt.Sprintf(`<img src="%s" alt="%s" />`,
			html.EscapeString(src), html.EscapeString(alt)))
	})
	return strings.Join(images, "\n")
}

// extractVideo returns a poster-image thumbnail for LinkedIn native video posts.
func (e *LinkedInExtractor) extractVideo() string {
	if e.postArticle == nil {
		return ""
	}
	video := e.postArticle.Find(".update-components-linkedin-video video[poster]").First()
	if video.Length() == 0 {
		return ""
	}
	poster, _ := video.Attr("poster")
	if poster == "" {
		return ""
	}
	return fmt.Sprintf(`<img src="%s" alt="Video thumbnail" />`, html.EscapeString(poster))
}
