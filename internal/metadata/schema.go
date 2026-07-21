package metadata

import (
	"strconv"
	"strings"
)

// getSchemaProperty extracts property from schema.org data
// JavaScript original code:
//
//	private static getSchemaProperty(schemaOrgData: any, property: string, defaultValue: string = ''): string {
//	  if (!schemaOrgData) return defaultValue;
//
//	  const searchSchema = (data: any, props: string[], fullPath: string, isExactMatch: boolean = true): string[] => {
//	    if (typeof data === 'string') {
//	      return props.length === 0 ? [data] : [];
//	    }
//
//	    if (!data || typeof data !== 'object') {
//	      return [];
//	    }
//
//	    if (Array.isArray(data)) {
//	      const currentProp = props[0];
//	      if (/^\\[\\d+\\]$/.test(currentProp)) {
//	        const index = parseInt(currentProp.slice(1, -1));
//	        if (data[index]) {
//	          return searchSchema(data[index], props.slice(1), fullPath, isExactMatch);
//	        }
//	        return [];
//	      }
//
//	      if (props.length === 0 && data.every(item => typeof item === 'string' || typeof item === 'number')) {
//	        return data.map(String);
//	      }
//
//	      return data.flatMap(item => searchSchema(item, props, fullPath, isExactMatch));
//	    }
//
//	    const [currentProp, ...remainingProps] = props;
//
//	    if (!currentProp) {
//	      if (typeof data === 'string') return [data];
//	      if (typeof data === 'object' && data.name) {
//	        return [data.name];
//	      }
//	      return [];
//	    }
//
//	    if (data.hasOwnProperty(currentProp)) {
//	      return searchSchema(data[currentProp], remainingProps,
//	        fullPath ? `${fullPath}.${currentProp}` : currentProp, true);
//	    }
//
//	    if (!isExactMatch) {
//	      const nestedResults: string[] = [];
//	      for (const key in data) {
//	        if (typeof data[key] === 'object') {
//	          const results = searchSchema(data[key], props,
//	            fullPath ? `${fullPath}.${key}` : key, false);
//	          nestedResults.push(...results);
//	        }
//	      }
//	      if (nestedResults.length > 0) {
//	        return nestedResults;
//	      }
//	    }
//
//	    return [];
//	  };
//
//	  try {
//	    let results = searchSchema(schemaOrgData, property.split('.'), '', true);
//	    if (results.length === 0) {
//	      results = searchSchema(schemaOrgData, property.split('.'), '', false);
//	    }
//	    const result = results.length > 0 ? results.filter(Boolean).join(', ') : defaultValue;
//	    return result;
//	  } catch (error) {
//	    console.error(`Error in getSchemaProperty for ${property}:`, error);
//	    return defaultValue;
//	  }
//	}
func getSchemaProperty(schemaOrgData any, property string) string {
	if schemaOrgData == nil {
		return ""
	}

	props := strings.Split(property, ".")
	results := searchSchema(schemaOrgData, props, true)
	if len(results) == 0 {
		results = searchSchema(schemaOrgData, props, false)
	}

	var filteredResults []string
	for _, result := range results {
		if result != "" {
			filteredResults = append(filteredResults, result)
		}
	}

	return strings.Join(filteredResults, ", ")
}

// searchSchema recursively walks schema.org data (strings, arrays, objects)
// following a dot-separated property path. When isExactMatch is false it also
// descends into nested objects and arrays to find the property anywhere.
func searchSchema(data any, props []string, isExactMatch bool) []string {
	// Handle string data
	if str, ok := data.(string); ok {
		if len(props) == 0 {
			return []string{str}
		}
		return []string{}
	}

	// Handle non-object data
	if data == nil {
		return []string{}
	}

	// Handle arrays
	if arr, ok := data.([]any); ok {
		if len(props) > 0 {
			currentProp := props[0]
			// Handle array index notation like [0]
			if arrayIndexRe.MatchString(currentProp) {
				indexStr := currentProp[1 : len(currentProp)-1]
				if index, err := strconv.Atoi(indexStr); err == nil && index < len(arr) {
					return searchSchema(arr[index], props[1:], isExactMatch)
				}
				return []string{}
			}
		}

		// If no props left and all items are strings/numbers, return them
		if len(props) == 0 {
			var results []string
			for _, item := range arr {
				if str, ok := item.(string); ok {
					results = append(results, str)
				} else if num, ok := item.(float64); ok {
					results = append(results, strconv.FormatFloat(num, 'f', -1, 64))
				}
			}
			if len(results) == len(arr) {
				return results
			}
		}

		// Search in all array items
		var allResults []string
		for _, item := range arr {
			results := searchSchema(item, props, isExactMatch)
			allResults = append(allResults, results...)
		}
		return allResults
	}

	// Handle maps/objects
	if obj, ok := data.(map[string]any); ok {
		if len(props) == 0 {
			if str, ok := obj["name"].(string); ok {
				return []string{str}
			}
			if str, ok := data.(string); ok {
				return []string{str}
			}
			return []string{}
		}

		currentProp := props[0]
		remainingProps := props[1:]

		// Check if property exists
		if value, exists := obj[currentProp]; exists {
			return searchSchema(value, remainingProps, true)
		}

		// If not exact match, search nested objects and arrays
		if !isExactMatch {
			var nestedResults []string
			for _, value := range obj {
				switch value.(type) {
				case map[string]any, []any:
					results := searchSchema(value, props, false)
					nestedResults = append(nestedResults, results...)
				}
			}
			return nestedResults
		}
	}

	return []string{}
}

// removeDuplicates removes duplicate strings from slice while preserving order
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
