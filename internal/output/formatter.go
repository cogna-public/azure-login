// Package output provides output formatting functionality for azure-login commands.
//
// This package supports multiple output formats (JSON, TSV, table) and JMESPath
// queries for filtering and transforming command output, compatible with Azure CLI
// output conventions.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/jmespath/go-jmespath"
)

// Print outputs data in the specified format
func Print(data any, format string, query string) error {
	// Apply JMESPath query if provided
	if query != "" {
		result, err := jmespath.Search(query, data)
		if err != nil {
			return fmt.Errorf("invalid query: %w", err)
		}
		data = result
	}

	// Output in requested format
	switch strings.ToLower(format) {
	case "json":
		return printJSON(data)
	case "tsv":
		return printTSV(data)
	case "table":
		return printTable(data)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func printJSON(data any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

func printTSV(data any) error {
	// For simple types, just print the value
	switch v := data.(type) {
	case string:
		fmt.Println(v)
	case int, int64, float64, bool:
		fmt.Println(v)
	case nil:
		// Print nothing for nil
	default:
		// For complex types, try to print first field or convert to string
		val := reflect.ValueOf(data)
		if val.Kind() == reflect.Map {
			// For single-value maps with simple values, print just the value
			if val.Len() == 1 {
				for _, key := range val.MapKeys() {
					mapValue := val.MapIndex(key).Interface()
					// Check if the value is simple (not a map, slice, or struct)
					valueKind := reflect.ValueOf(mapValue).Kind()
					if valueKind != reflect.Map && valueKind != reflect.Slice && valueKind != reflect.Struct {
						fmt.Println(mapValue)
						return nil
					}
					// If value is complex, fall through to JSON encoding
				}
			}
		}
		// Fallback to JSON encoding for complex structures
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to convert to TSV: %w", err)
		}
		fmt.Println(string(jsonData))
	}
	return nil
}

func printTable(data any) error {
	// For now, table format is the same as JSON
	// This can be enhanced later with proper table formatting
	return printJSON(data)
}
