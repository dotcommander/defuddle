package extractors

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// threadsJSONDepthLimit caps recursion when walking Threads React hydration JSON.
// Threads GraphQL responses can be deeply nested; 35 matches the upstream constant.
const threadsJSONDepthLimit = 35

// ThreadsPost holds the parsed data for a single post extracted from a pagelet.
type ThreadsPost struct {
	username  string
	date      string
	permalink string
	content   string
	el        *goquery.Selection
}

// ThreadsExtractor handles content extraction from threads.com and threads.net.
// Threads serves content via two paths: interactive React (pagelet divs) and
// server-rendered HTML (div[role="region"] + embedded JSON hydration).
type ThreadsExtractor struct {
	*ExtractorBase
	pagelets        []*goquery.Selection
	regionContainer *goquery.Selection
}

// NewThreadsExtractor creates a new Threads extractor and caches DOM anchors.
func NewThreadsExtractor(document *goquery.Document, url string, schemaOrgData any) *ThreadsExtractor {
	e := &ThreadsExtractor{
		ExtractorBase: NewExtractorBase(document, url, schemaOrgData),
	}

	// Pagelet path: each pagelet wraps a post or reply group.
	document.Find(`[data-pagelet^="threads_post_page_"]`).Each(func(_ int, s *goquery.Selection) {
		if s.Find(`a[href^="/@"], time[datetime]`).Length() > 0 {
			e.pagelets = append(e.pagelets, s)
		}
	})

	// Region path: server-rendered HTML fallback — a single region div with /@user links.
	if len(e.pagelets) == 0 {
		region := document.Find(`div[role="region"]`).First()
		if region.Length() > 0 && region.Find(`a[href^="/@"]`).Length() > 0 {
			e.regionContainer = region
		}
	}

	return e
}

// Name returns the extractor identifier.
func (e *ThreadsExtractor) Name() string { return "ThreadsExtractor" }

// CanExtract returns true when either extraction path found suitable anchors.
func (e *ThreadsExtractor) CanExtract() bool {
	return len(e.pagelets) > 0 || e.regionContainer != nil
}

// Extract dispatches to the appropriate extraction path.
func (e *ThreadsExtractor) Extract() *ExtractorResult {
	if len(e.pagelets) == 0 && e.regionContainer != nil {
		return e.extractFromRegion(e.regionContainer)
	}
	return e.extractFromPagelets()
}

// extractFromPagelets handles the React interactive DOM path.
func (e *ThreadsExtractor) extractFromPagelets() *ExtractorResult {
	mainAuthor := e.getUsername(e.pagelets[0])

	// Consecutive single-post pagelets by the main author form the thread.
	// The first pagelet by a different author (or multi-post pagelet) starts replies.
	var threadPosts []ThreadsPost
	var replyPosts [][]ThreadsPost
	threadEnded := false

	for _, pagelet := range e.pagelets {
		posts := e.getPostsFromPagelet(pagelet)
		if len(posts) == 0 {
			continue
		}
		if !threadEnded && posts[0].username == mainAuthor && len(posts) == 1 {
			threadPosts = append(threadPosts, posts[0])
		} else {
			threadEnded = true
			replyPosts = append(replyPosts, posts)
		}
	}

	var postParts []string
	for _, p := range threadPosts {
		postParts = append(postParts, p.content)
	}
	postContent := strings.Join(postParts, "\n<hr>\n")
	comments := e.extractComments(replyPosts)
	contentHTML := buildContentHtml("threads", postContent, comments)

	author := "@" + mainAuthor
	published := ""
	if len(threadPosts) > 0 {
		published = threadPosts[0].date
	}

	var firstEl *goquery.Selection
	if len(threadPosts) > 0 {
		firstEl = threadPosts[0].el
	}
	description := e.createDescription(firstEl)
	title := TruncateTitle(fmt.Sprintf("%s on Threads", author), 100)

	vars := map[string]string{
		"title":  title,
		"author": author,
		"site":   "Threads",
	}
	if description != "" {
		vars["description"] = description
	}
	if published != "" {
		vars["published"] = published
	}

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"postAuthor": mainAuthor,
		},
		Variables: vars,
	}
}

// extractFromRegion handles server-rendered HTML — content in div[role="region"]
// with post text in DOM and replies extracted from embedded React JSON blobs.
func (e *ThreadsExtractor) extractFromRegion(region *goquery.Selection) *ExtractorResult {
	mainAuthor := e.getUsername(region)
	if mainAuthor == "" {
		return &ExtractorResult{Content: "", ContentHTML: ""}
	}

	author := "@" + mainAuthor
	postContent := e.extractPostContent(region)
	comments := e.extractCommentsFromJSON(mainAuthor)
	contentHTML := buildContentHtml("threads", postContent, comments)

	date := e.getDate(region)
	description := e.createDescription(region)
	title := TruncateTitle(fmt.Sprintf("%s on Threads", author), 100)

	vars := map[string]string{
		"title":  title,
		"author": author,
		"site":   "Threads",
	}
	if description != "" {
		vars["description"] = description
	}
	if date != "" {
		vars["published"] = date
	}

	return &ExtractorResult{
		Content:     contentHTML,
		ContentHTML: contentHTML,
		ExtractedContent: map[string]any{
			"postAuthor": mainAuthor,
		},
		Variables: vars,
	}
}

// extractComments builds the comment HTML from reply pagelet post groups.
// Single-post groups → depth 0 (top-level reply).
// Multi-post groups → linear chain (depth 0, 1, 2, …).
func (e *ThreadsExtractor) extractComments(replyPosts [][]ThreadsPost) string {
	var comments []CommentData
	for _, posts := range replyPosts {
		for i, p := range posts {
			depth := i
			if len(posts) == 1 {
				depth = 0
			}
			comments = append(comments, CommentData{
				Depth:   depth,
				Author:  "@" + p.username,
				Date:    p.date,
				URL:     p.permalink,
				Content: p.content,
			})
		}
	}
	if len(comments) == 0 {
		return ""
	}
	return renderCommentThread(comments)
}

// extractCommentsFromJSON parses React hydration scripts for reply data.
// Scripts must contain at least 2 occurrences of "text_fragments" to be
// considered relevant (excludes small config blobs).
func (e *ThreadsExtractor) extractCommentsFromJSON(mainAuthor string) string {
	type jsonPost struct {
		username string
		text     string
	}

	var allPosts []jsonPost
	seen := make(map[string]bool)

	e.GetDocument().Find(`script[type="application/json"]`).Each(func(_ int, s *goquery.Selection) {
		raw := s.Text()
		if strings.Count(raw, `"text_fragments"`) < 2 {
			return
		}
		if !strings.Contains(raw, `"username"`) {
			return
		}

		var data any
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			return
		}

		for _, p := range findPostsInJSON(data, 0) {
			key := p.username + ":" + threadsDedupeKey(p.text)
			if seen[key] {
				continue
			}
			seen[key] = true
			allPosts = append(allPosts, jsonPost{username: p.username, text: p.text})
		}
	})

	if len(allPosts) < 2 {
		return ""
	}

	// Skip the first entry by the main author (it's the post itself, not a reply).
	var comments []CommentData
	skippedMain := false
	for _, p := range allPosts {
		if !skippedMain && p.username == mainAuthor {
			skippedMain = true
			continue
		}
		comments = append(comments, CommentData{
			Depth:   0,
			Author:  "@" + p.username,
			Content: fmt.Sprintf("<p>%s</p>", html.EscapeString(p.text)),
		})
	}

	if len(comments) == 0 {
		return ""
	}
	return renderCommentThread(comments)
}
