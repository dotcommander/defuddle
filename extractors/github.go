package extractors

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns for GitHub extraction.
var (
	githubUserRe      = regexp.MustCompile(`github\.com/([^/?#]+)`)
	githubRepoRe      = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)`)
	githubTitleRepoRe = regexp.MustCompile(`([^/\s]+)/([^/\s]+)`)
	githubIssueRe     = regexp.MustCompile(`/(?:issues|pull)/(\d+)`)
)

// GitHubExtractor handles GitHub content extraction
// TypeScript original code:
//
//	export class GitHubExtractor extends BaseExtractor {
//		canExtract(): boolean {
//			const githubIndicators = [
//				'meta[name="expected-hostname"][content="github.com"]',
//				'meta[name="octolytics-url"]',
//				'meta[name="github-keyboard-shortcuts"]',
//				'.js-header-wrapper',
//				'#js-repo-pjax-container',
//			];
//
//			const githubPageIndicators = {
//				issue: [
//					'[data-testid="issue-metadata-sticky"]',
//					'[data-testid="issue-title"]',
//				],
//			}
//
//			return githubIndicators.some(selector => this.document.querySelector(selector) !== null)
//				&& Object.values(githubPageIndicators).some(selectors => selectors.some(selector => this.document.querySelector(selector) !== null));
//		}
//
//		extract(): ExtractorResult {
//			return this.extractIssue();
//		}
type GitHubExtractor struct {
	*ExtractorBase
}

// NewGitHubExtractor creates a new GitHub extractor
func NewGitHubExtractor(document *goquery.Document, url string, schemaOrgData any) *GitHubExtractor {
	extractor := &GitHubExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}

	slog.Debug("GitHub extractor initialized", "url", url)
	return extractor
}

// CanExtract checks if the extractor can extract content
// TypeScript original code:
//
//	canExtract(): boolean {
//		const githubIndicators = [
//			'meta[name="expected-hostname"][content="github.com"]',
//			'meta[name="octolytics-url"]',
//			'meta[name="github-keyboard-shortcuts"]',
//			'.js-header-wrapper',
//			'#js-repo-pjax-container',
//		];
//
//		const githubPageIndicators = {
//			issue: [
//				'[data-testid="issue-metadata-sticky"]',
//				'[data-testid="issue-title"]',
//			],
//		}
//
//		return githubIndicators.some(selector => this.document.querySelector(selector) !== null)
//			&& Object.values(githubPageIndicators).some(selectors => selectors.some(selector => this.document.querySelector(selector) !== null));
//	}
func (g *GitHubExtractor) CanExtract() bool {
	githubIndicators := []string{
		`meta[name="expected-hostname"][content="github.com"]`,
		`meta[name="octolytics-url"]`,
		`meta[name="github-keyboard-shortcuts"]`,
		`.js-header-wrapper`,
		`#js-repo-pjax-container`,
	}

	githubPageIndicators := []string{
		`[data-testid="issue-metadata-sticky"]`,
		`[data-testid="issue-title"]`,
	}

	// Check for GitHub indicators
	hasGitHubIndicator := false
	for _, selector := range githubIndicators {
		if g.document.Find(selector).Length() > 0 {
			hasGitHubIndicator = true
			break
		}
	}

	// Check for page-specific indicators
	hasPageIndicator := false
	for _, selector := range githubPageIndicators {
		if g.document.Find(selector).Length() > 0 {
			hasPageIndicator = true
			break
		}
	}

	canExtract := hasGitHubIndicator && hasPageIndicator
	slog.Debug("GitHub extractor can extract check", "canExtract", canExtract, "url", g.url)
	return canExtract
}

// Name returns the name of the extractor
func (g *GitHubExtractor) Name() string {
	return "GitHubExtractor"
}

// Extract extracts the GitHub content
// TypeScript original code:
//
//	extract(): ExtractorResult {
//		return this.extractIssue();
//	}
func (g *GitHubExtractor) Extract() *ExtractorResult {
	slog.Debug("GitHub extractor starting extraction", "url", g.url)
	return g.extractIssue()
}

// extractIssue extracts GitHub issue content with comprehensive structure
// TypeScript original code: Full implementation with issue body, comments, and metadata
func (g *GitHubExtractor) extractIssue() *ExtractorResult {
	slog.Debug("GitHub extractor extracting issue")

	repoInfo := g.extractRepoInfo()
	issueNumber := g.extractIssueNumber()

	var content strings.Builder

	// Extract the main issue body first
	issueContainer := g.document.Find(`[data-testid="issue-viewer-issue-container"]`).First()
	if issueContainer.Length() > 0 {
		issueAuthor := g.extractAuthor(issueContainer, []string{
			`a[data-testid="issue-body-header-author"]`,
			`.IssueBodyHeaderAuthor-module__authorLoginLink--_S7aT`,
			`.ActivityHeader-module__AuthorLink--iofTU`,
			`a[href*="/users/"][data-hovercard-url*="/users/"]`,
			`a[aria-label*="profile"]`,
		})

		issueTimestamp := relativeTimeDatetime(issueContainer)

		issueBodyElement := issueContainer.Find(`[data-testid="issue-body-viewer"] .markdown-body`).First()
		if issueBodyElement.Length() > 0 {
			bodyContent := g.cleanBodyContent(issueBodyElement)

			// Add the main issue
			fmt.Fprintf(&content, `<div class="issue-author"><strong>%s</strong>`, issueAuthor)
			if d := formatGitHubDate(issueTimestamp); d != "" {
				fmt.Fprintf(&content, ` opened this issue on %s`, d)
			}
			content.WriteString("</div>\n\n")
			fmt.Fprintf(&content, `<div class="issue-body">%s</div>\n\n`, bodyContent)
		}
	}

	// Extract comments
	g.appendIssueComments(&content)

	contentHTML := content.String()
	description := g.createDescription(contentHTML)
	title := g.document.Find("title").Text()

	slog.Debug("GitHub issue extraction completed",
		"title", title,
		"issueNumber", issueNumber,
		"repo", fmt.Sprintf("%s/%s", repoInfo["owner"], repoInfo["repo"]),
		"contentLength", len(contentHTML))

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"type":        "issue",
			"issueNumber": issueNumber,
			"repository":  repoInfo["repo"],
			"owner":       repoInfo["owner"],
		},
		Variables: map[string]string{
			"title":       title,
			"author":      "",
			"site":        fmt.Sprintf("GitHub - %s/%s", repoInfo["owner"], repoInfo["repo"]),
			"description": description,
		},
	}
}

// appendIssueComments writes each unique issue comment (author, date, body) to content.
func (g *GitHubExtractor) appendIssueComments(content *strings.Builder) {
	commentElements := g.document.Find(`[data-wrapper-timeline-id]`)
	processedComments := make(map[string]bool)

	commentElements.Each(func(_ int, commentElement *goquery.Selection) {
		commentContainer := commentElement.Find(".react-issue-comment").First()
		if commentContainer.Length() == 0 {
			return
		}

		commentID, exists := commentElement.Attr("data-wrapper-timeline-id")
		if !exists || commentID == "" || processedComments[commentID] {
			return
		}
		processedComments[commentID] = true

		author := g.extractAuthor(commentContainer, []string{
			`.ActivityHeader-module__AuthorLink--iofTU`,
			`a[data-testid="avatar-link"]`,
			`a[href^="/"][data-hovercard-url*="/users/"]`,
		})

		timestamp := relativeTimeDatetime(commentContainer)

		bodyElement := commentContainer.Find(".markdown-body").First()
		if bodyElement.Length() > 0 {
			bodyContent := g.cleanBodyContent(bodyElement)
			if bodyContent != "" {
				content.WriteString(`<div class="comment">\n`)
				fmt.Fprintf(content, `<div class="comment-header"><strong>%s</strong>`, author)
				if d := formatGitHubDate(timestamp); d != "" {
					fmt.Fprintf(content, ` commented on %s`, d)
				}
				content.WriteString(`</div>\n`)
				fmt.Fprintf(content, `<div class="comment-body">%s</div>\n`, bodyContent)
				content.WriteString(`</div>\n\n`)
			}
		}
	})
}
