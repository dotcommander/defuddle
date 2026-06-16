// Package main: property accessors for `defuddle parse --property`.
//
// propertyExtractors is the canonical map of valid --property values to their
// display name and Result accessor. knownProperties exposes the sorted display
// list for error messages; getProperty performs case-insensitive lookup.
package main

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/dotcommander/defuddle"
)

// propertyAccessor pairs a property's display name (for error messages) with
// its Result accessor. One entry per --property value — the single source of
// truth for the valid property set.
type propertyAccessor struct {
	display string
	get     func(*defuddle.Result) string
}

// propertyExtractors maps lowercase property names to their accessor.
// The keys are the canonical list of valid --property values.
var propertyExtractors = map[string]propertyAccessor{
	"content":     {"content", func(r *defuddle.Result) string { return r.Content }},
	"title":       {"title", func(r *defuddle.Result) string { return r.Title }},
	"description": {"description", func(r *defuddle.Result) string { return r.Description }},
	"domain":      {"domain", func(r *defuddle.Result) string { return r.Domain }},
	"favicon":     {"favicon", func(r *defuddle.Result) string { return r.Favicon }},
	"image":       {"image", func(r *defuddle.Result) string { return r.Image }},
	"author":      {"author", func(r *defuddle.Result) string { return r.Author }},
	"site":        {"site", func(r *defuddle.Result) string { return r.Site }},
	"published":   {"published", func(r *defuddle.Result) string { return r.Published }},
	"wordcount":   {"wordCount", func(r *defuddle.Result) string { return strconv.Itoa(r.WordCount) }},
	"parsetime":   {"parseTime", func(r *defuddle.Result) string { return strconv.FormatInt(r.ParseTime, 10) }},
	"metatags": {"metaTags", func(r *defuddle.Result) string {
		if r.MetaTags == nil {
			return ""
		}
		b, err := json.Marshal(r.MetaTags)
		if err != nil {
			return ""
		}
		return string(b)
	}},
	"schemaorgdata": {"schemaOrgData", func(r *defuddle.Result) string {
		if r.SchemaOrgData == nil {
			return "null"
		}
		b, err := json.Marshal(r.SchemaOrgData)
		if err != nil {
			return ""
		}
		return string(b)
	}},
	"extractortype": {"extractorType", func(r *defuddle.Result) string {
		if r.ExtractorType != nil {
			return *r.ExtractorType
		}
		return ""
	}},
	"contentmarkdown": {"contentMarkdown", func(r *defuddle.Result) string {
		if r.ContentMarkdown != nil {
			return *r.ContentMarkdown
		}
		return ""
	}},
}

// knownProperties is the sorted display list for error messages.
var knownProperties = func() []string {
	keys := make([]string, 0, len(propertyExtractors))
	for _, p := range propertyExtractors {
		keys = append(keys, p.display)
	}
	slices.Sort(keys)
	return keys
}()

func getProperty(result *defuddle.Result, property string) (string, bool) {
	// Convert to lowercase for case-insensitive matching (matching TypeScript behavior)
	p, ok := propertyExtractors[strings.ToLower(property)]
	if !ok {
		return "", false
	}
	return p.get(result), true
}
