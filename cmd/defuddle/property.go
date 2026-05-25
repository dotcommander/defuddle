// Package main: property accessors for `defuddle parse --property`.
//
// propertyExtractors is the canonical map of valid --property values to
// Result accessors. knownProperties exposes the sorted key list for error
// messages; getProperty performs case-insensitive lookup.
package main

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/dotcommander/defuddle"
)

// propertyExtractors maps lowercase property names to their Result accessor.
// The keys also serve as the canonical list of valid --property values.
var propertyExtractors = map[string]func(*defuddle.Result) string{
	"content":     func(r *defuddle.Result) string { return r.Content },
	"title":       func(r *defuddle.Result) string { return r.Title },
	"description": func(r *defuddle.Result) string { return r.Description },
	"domain":      func(r *defuddle.Result) string { return r.Domain },
	"favicon":     func(r *defuddle.Result) string { return r.Favicon },
	"image":       func(r *defuddle.Result) string { return r.Image },
	"author":      func(r *defuddle.Result) string { return r.Author },
	"site":        func(r *defuddle.Result) string { return r.Site },
	"published":   func(r *defuddle.Result) string { return r.Published },
	"wordcount":   func(r *defuddle.Result) string { return strconv.Itoa(r.WordCount) },
	"parsetime":   func(r *defuddle.Result) string { return strconv.FormatInt(r.ParseTime, 10) },
	"metatags": func(r *defuddle.Result) string {
		if r.MetaTags == nil {
			return ""
		}
		b, err := json.Marshal(r.MetaTags)
		if err != nil {
			return ""
		}
		return string(b)
	},
	"schemaorgdata": func(r *defuddle.Result) string {
		if r.SchemaOrgData == nil {
			return "null"
		}
		b, err := json.Marshal(r.SchemaOrgData)
		if err != nil {
			return ""
		}
		return string(b)
	},
	"extractortype": func(r *defuddle.Result) string {
		if r.ExtractorType != nil {
			return *r.ExtractorType
		}
		return ""
	},
	"contentmarkdown": func(r *defuddle.Result) string {
		if r.ContentMarkdown != nil {
			return *r.ContentMarkdown
		}
		return ""
	},
}

var propertyDisplayNames = map[string]string{
	"content":         "content",
	"title":           "title",
	"description":     "description",
	"domain":          "domain",
	"favicon":         "favicon",
	"image":           "image",
	"author":          "author",
	"site":            "site",
	"published":       "published",
	"wordcount":       "wordCount",
	"parsetime":       "parseTime",
	"metatags":        "metaTags",
	"schemaorgdata":   "schemaOrgData",
	"extractortype":   "extractorType",
	"contentmarkdown": "contentMarkdown",
}

// knownProperties is the sorted display list for error messages.
var knownProperties = func() []string {
	keys := make([]string, 0, len(propertyExtractors))
	for k := range propertyExtractors {
		keys = append(keys, propertyDisplayNames[k])
	}
	slices.Sort(keys)
	return keys
}()

func getProperty(result *defuddle.Result, property string) (string, bool) {
	// Convert to lowercase for case-insensitive matching (matching TypeScript behavior)
	fn, ok := propertyExtractors[strings.ToLower(property)]
	if !ok {
		return "", false
	}
	return fn(result), true
}
