package defuddle

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-json-experiment/json"
	"github.com/piprate/json-gold/ld"
)

// Pre-compiled regex patterns for JSON-LD content cleaning.
var (
	htmlCommentRe   = regexp.MustCompile(`<!--[\s\S]*?-->`)
	jsCommentRe     = regexp.MustCompile(`/\*[\s\S]*?\*/|^\s*//.*$`)
	cdataRe         = regexp.MustCompile(`^\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*$`)
	commentMarkerRe = regexp.MustCompile(`^\s*(\*/|/\*)\s*|\s*(\*/|/\*)\s*$`)
)

// extractSchemaOrgData extracts and processes schema.org structured data using JSON-LD processor
// JavaScript original code:
//
//	private _extractSchemaOrgData(document: Document) {
//	  const schemaItems = [];
//	  const scripts = document.querySelectorAll('script[type="application/ld+json"]');
//
//	  scripts.forEach(script => {
//	    try {
//	      const jsonData = JSON.parse(script.textContent);
//	      if (jsonData['@graph']) {
//	        schemaItems.push(...jsonData['@graph']);
//	      } else {
//	        schemaItems.push(jsonData);
//	      }
//	    } catch (e) {
//	      console.warn('Failed to parse schema.org data:', e);
//	    }
//	  });
//
//	  return schemaItems;
//	}
func (d *Defuddle) extractSchemaOrgData() any {
	processor := ld.NewJsonLdProcessor()
	options := ld.NewJsonLdOptions("")
	options.ProcessingMode = ld.JsonLd_1_1

	var allSchemaItems []any

	if d.debugger.IsEnabled() {
		d.debugger.StartTimer("schema_extraction")
	}

	d.doc.Find(`script[type="application/ld+json"]`).Each(func(i int, script *goquery.Selection) {
		jsonContent := strings.TrimSpace(script.Text())
		if jsonContent == "" {
			return
		}

		// Clean and validate JSON-LD content
		cleanedContent := d.cleanJSONLDContent(jsonContent)
		if cleanedContent == "" {
			if d.debug {
				slog.Debug("Empty JSON-LD content after cleaning", "index", i)
			}
			return
		}

		// Parse and process JSON-LD using json-gold
		processedData, err := d.processSchemaOrgData(processor, options, cleanedContent)
		if err != nil {
			if d.debug {
				slog.Debug("Failed to process schema.org JSON-LD",
					"error", err,
					"index", i,
					"content_preview", cleanedContent[:min(len(cleanedContent), 100)])
			}
			return
		}

		// Extract items from processed data
		items := d.extractSchemaItems(processedData)
		allSchemaItems = append(allSchemaItems, items...)
	})

	if d.debugger.IsEnabled() {
		d.debugger.EndTimer("schema_extraction")
		d.debugger.AddProcessingStep("schema_org_extraction",
			fmt.Sprintf("Extracted %d schema.org items", len(allSchemaItems)),
			len(allSchemaItems), "")
	}

	if d.debug {
		slog.Debug("Schema.org data extraction completed",
			"total_items", len(allSchemaItems),
			"unique_types", d.countSchemaTypes(allSchemaItems))
	}

	return allSchemaItems
}

// cleanJSONLDContent cleans and normalizes JSON-LD content
// JavaScript original code:
//
//	// Remove comments, CDATA, and other non-JSON content
func (d *Defuddle) cleanJSONLDContent(content string) string {
	// Remove HTML comments
	content = htmlCommentRe.ReplaceAllString(content, "")

	// Remove JavaScript-style comments
	content = jsCommentRe.ReplaceAllString(content, "")

	// Handle CDATA sections
	if matches := cdataRe.FindStringSubmatch(content); len(matches) > 1 {
		content = matches[1]
	}

	// Remove comment markers that might be left
	content = commentMarkerRe.ReplaceAllString(content, "")

	// Remove leading/trailing whitespace
	content = strings.TrimSpace(content)

	// Basic JSON validation - check if it starts and ends correctly
	isValidJSON := (strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}")) ||
		(strings.HasPrefix(content, "[") && strings.HasSuffix(content, "]"))

	if content != "" && !isValidJSON {
		if d.debug {
			slog.Debug("Invalid JSON-LD format detected", "content_preview", content[:min(len(content), 50)])
		}
		return ""
	}

	return content
}

// processSchemaOrgData processes JSON-LD data using json-gold processor
// JavaScript original code:
//
//	// Standard JSON-LD processing with context expansion and validation
func (d *Defuddle) processSchemaOrgData(processor *ld.JsonLdProcessor, options *ld.JsonLdOptions, jsonContent string) (any, error) {
	// Parse raw JSON first
	var rawData any
	if err := json.Unmarshal([]byte(jsonContent), &rawData); err != nil {
		return nil, fmt.Errorf("invalid JSON syntax: %w", err)
	}

	// Expand JSON-LD to resolve contexts and normalize structure
	expanded, err := processor.Expand(rawData, options)
	if err != nil {
		return nil, fmt.Errorf("JSON-LD expansion failed: %w", err)
	}

	// If expansion succeeded, try to compact with schema.org context for cleaner output
	if len(expanded) > 0 {
		schemaContext := map[string]any{
			"@context": "https://schema.org/",
		}

		compacted, err := processor.Compact(expanded, schemaContext, options)
		if err != nil {
			// If compaction fails, use expanded data
			if d.debug {
				slog.Debug("Schema.org compaction failed, using expanded data", "error", err)
			}
			return expanded, nil
		}

		return compacted, nil
	}

	return expanded, nil
}

// extractSchemaItems extracts individual schema items from processed JSON-LD data
// JavaScript original code:
//
//	// Handle both single items and @graph arrays
func (d *Defuddle) extractSchemaItems(data any) []any {
	var items []any

	switch typedData := data.(type) {
	case map[string]any:
		// Check for @graph property (common in schema.org JSON-LD)
		if graph, exists := typedData["@graph"]; exists {
			if graphArray, ok := graph.([]any); ok {
				items = append(items, graphArray...)
			} else {
				items = append(items, graph)
			}
		} else {
			// Single item
			items = append(items, typedData)
		}

	case []any:
		// Array of items (from JSON-LD expansion)
		items = append(items, typedData...)

	default:
		// Single item of unknown type
		items = append(items, data)
	}

	// Filter and validate schema items
	var validItems []any
	for _, item := range items {
		if d.isValidSchemaItem(item) {
			validItems = append(validItems, item)
		}
	}

	return validItems
}

// isValidSchemaItem validates if an item is a valid schema.org item
// JavaScript original code:
//
//	// Check for @type or other schema.org indicators
func (d *Defuddle) isValidSchemaItem(item any) bool {
	itemMap, ok := item.(map[string]any)
	if !ok {
		return false
	}

	// Check for @type or type property (required for schema.org items)
	var itemType any
	var exists bool
	if itemType, exists = itemMap["@type"]; !exists {
		itemType, exists = itemMap["type"]
	}

	if exists {
		switch typedValue := itemType.(type) {
		case string:
			return typedValue != ""
		case []any:
			return len(typedValue) > 0
		}
	}

	// Check for schema.org URL in @id
	if itemID, exists := itemMap["@id"]; exists {
		if idStr, ok := itemID.(string); ok {
			return strings.Contains(idStr, "schema.org") ||
				strings.Contains(idStr, "http") // Any URL-like identifier
		}
	}

	// Check if it has common schema.org properties
	commonProps := []string{"name", "description", "url", "image", "author", "publisher"}
	propCount := 0
	for _, prop := range commonProps {
		if _, exists := itemMap[prop]; exists {
			propCount++
		}
	}

	// Consider valid if it has multiple common properties
	return propCount >= 2
}

// countSchemaTypes counts unique schema types for debugging
// JavaScript original code:
//
//	// Helper for debugging and logging
func (d *Defuddle) countSchemaTypes(items []any) int {
	typeSet := make(map[string]bool)

	for _, item := range items {
		if itemMap, ok := item.(map[string]any); ok {
			// Check both @type and type (after JSON-LD processing)
			var itemType any
			var exists bool
			if itemType, exists = itemMap["@type"]; !exists {
				itemType, exists = itemMap["type"]
			}

			if exists {
				switch typedValue := itemType.(type) {
				case string:
					typeSet[typedValue] = true
				case []any:
					for _, t := range typedValue {
						if typeStr, ok := t.(string); ok {
							typeSet[typeStr] = true
						}
					}
				}
			}
		}
	}

	return len(typeSet)
}
