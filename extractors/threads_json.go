package extractors

import "strings"

// threadsJSONPost is the minimal representation extracted from React hydration JSON.
type threadsJSONPost struct {
	username string
	text     string
}

// findPostsInJSON walks arbitrary JSON (unmarshalled as any) up to threadsJSONDepthLimit
// levels deep and collects {username, text} pairs from Threads GraphQL payloads.
//
// The Threads hydration blobs use a consistent structure:
//
//	"thread_items": [ { "post": { "user": { "username": "..." }, "caption": { "text": "..." } } } ]
//
// Walking the whole tree means we tolerate layout changes without explicit path coupling.
func findPostsInJSON(data any, depth int) []threadsJSONPost {
	if depth > threadsJSONDepthLimit {
		return nil
	}

	switch v := data.(type) {
	case map[string]any:
		return walkJSONObject(v, depth)
	case []any:
		return walkJSONArray(v, depth)
	}
	return nil
}

// walkJSONObject extracts posts from a JSON object node. When the node looks like
// a Threads "post" object (has both "user.username" and "caption.text"), it emits
// one post. Otherwise it recurses into all values.
func walkJSONObject(obj map[string]any, depth int) []threadsJSONPost {
	// Check whether this node is itself a post object.
	if post, ok := extractPostNode(obj); ok {
		return []threadsJSONPost{post}
	}

	var results []threadsJSONPost
	for _, val := range obj {
		results = append(results, findPostsInJSON(val, depth+1)...)
	}
	return results
}

// walkJSONArray recurses into each element of a JSON array.
func walkJSONArray(arr []any, depth int) []threadsJSONPost {
	var results []threadsJSONPost
	for _, item := range arr {
		results = append(results, findPostsInJSON(item, depth+1)...)
	}
	return results
}

// extractPostNode checks whether obj is a Threads post node (has "user" → "username"
// and "caption" → "text") and returns the extracted post if so.
func extractPostNode(obj map[string]any) (threadsJSONPost, bool) {
	userObj, ok := obj["user"].(map[string]any)
	if !ok {
		return threadsJSONPost{}, false
	}
	username, _ := userObj["username"].(string)
	if username == "" {
		return threadsJSONPost{}, false
	}

	captionObj, ok := obj["caption"].(map[string]any)
	if !ok {
		return threadsJSONPost{}, false
	}
	text, _ := captionObj["text"].(string)

	return threadsJSONPost{username: username, text: text}, true
}

// threadsDedupeKey normalises post text for deduplication: collapse whitespace and
// truncate to 80 runes so near-identical posts from different script blobs collapse.
func threadsDedupeKey(text string) string {
	normalised := strings.Join(strings.Fields(text), " ")
	runes := []rune(normalised)
	if len(runes) > 80 {
		return string(runes[:80])
	}
	return normalised
}
