package extractors

import (
	"fmt"
	"html"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// extractComments collects top-level comments and their direct replies,
// returning a rendered comment-thread HTML string.
func (e *LinkedInExtractor) extractComments() string {
	if e.postArticle == nil {
		return ""
	}
	var commentData []CommentData

	e.postArticle.Find(`article.comments-comment-entity:not(.comments-comment-entity--reply)`).Each(
		func(_ int, comment *goquery.Selection) {
			if data := e.extractCommentData(comment, 0); data != nil {
				commentData = append(commentData, *data)
			}
			comment.Find(".comments-replies-list article.comments-comment-entity--reply").Each(
				func(_ int, reply *goquery.Selection) {
					if data := e.extractCommentData(reply, 1); data != nil {
						commentData = append(commentData, *data)
					}
				},
			)
		},
	)

	if len(commentData) == 0 {
		return ""
	}
	return renderCommentThread(commentData)
}

// extractCommentData extracts a single comment's metadata and content.
// Returns nil when no author is found (deleted/hidden comment).
func (e *LinkedInExtractor) extractCommentData(comment *goquery.Selection, depth int) *CommentData {
	author := strings.TrimSpace(
		comment.Find(".comments-comment-meta__description-title").First().Text(),
	)
	if author == "" {
		return nil
	}

	textEl := comment.Find(".comments-comment-entity__content .update-components-text").First()
	content := e.cleanTextContent(textEl)

	date := strings.TrimSpace(comment.Find("time.comments-comment-meta__data").First().Text())

	profileURL := ""
	if link := comment.Find("a.comments-comment-meta__description-container").First(); link.Length() > 0 {
		profileURL = linkedinAbsURL(link)
	}

	reactions := strings.TrimSpace(
		comment.Find(".comments-comment-social-bar__reactions-count--cr span.v-align-middle").First().Text(),
	)

	cd := CommentData{
		Author:  html.EscapeString(author),
		Date:    date,
		Content: content,
		Depth:   depth,
	}
	if profileURL != "" {
		cd.URL = html.EscapeString(profileURL)
		cd.LinkText = html.EscapeString(author)
	}
	if reactions != "" {
		cd.Extra = fmt.Sprintf(` <span class="comment-score">%s</span>`,
			html.EscapeString(reactions+" reactions"))
	}
	return &cd
}
