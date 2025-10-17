package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureOutput(f func()) string {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrint_JSON(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		format   string
		query    string
		expected string
	}{
		{
			name: "Simple object",
			data: map[string]any{
				"name":  "test",
				"value": 123,
			},
			format:   "json",
			query:    "",
			expected: `"name": "test"`,
		},
		{
			name:     "Simple string",
			data:     "test-string",
			format:   "json",
			query:    "",
			expected: `"test-string"`,
		},
		{
			name:     "Number",
			data:     42,
			format:   "json",
			query:    "",
			expected: `42`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				err := Print(tt.data, tt.format, tt.query)
				if err != nil {
					t.Errorf("Print failed: %v", err)
				}
			})

			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expected, output)
			}
		})
	}
}

func TestPrint_TSV(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		expected string
	}{
		{
			name:     "Simple string",
			data:     "test-value",
			expected: "test-value",
		},
		{
			name:     "Number",
			data:     42,
			expected: "42",
		},
		{
			name:     "Boolean",
			data:     true,
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				err := Print(tt.data, "tsv", "")
				if err != nil {
					t.Errorf("Print failed: %v", err)
				}
			})

			output = strings.TrimSpace(output)
			if output != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, output)
			}
		})
	}
}

func TestPrint_JMESPathQuery(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		query    string
		format   string
		expected string
	}{
		{
			name: "Extract single field",
			data: map[string]any{
				"accessToken": "token-12345",
				"expiresOn":   "2024-10-16",
			},
			query:    "accessToken",
			format:   "tsv",
			expected: "token-12345",
		},
		{
			name: "Extract nested field",
			data: map[string]any{
				"user": map[string]any{
					"name": "test-user",
					"id":   123,
				},
			},
			query:    "user.name",
			format:   "tsv",
			expected: "test-user",
		},
		{
			name: "Extract from array",
			data: map[string]any{
				"items": []any{"a", "b", "c"},
			},
			query:    "items[0]",
			format:   "tsv",
			expected: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				err := Print(tt.data, tt.format, tt.query)
				if err != nil {
					t.Errorf("Print failed: %v", err)
				}
			})

			output = strings.TrimSpace(output)
			if output != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, output)
			}
		})
	}
}

func TestPrint_InvalidQuery(t *testing.T) {
	data := map[string]any{
		"name": "test",
	}

	err := Print(data, "json", "invalid[query")

	if err == nil {
		t.Error("Expected error for invalid JMESPath query, got none")
	}
}

func TestPrint_UnsupportedFormat(t *testing.T) {
	data := map[string]any{
		"name": "test",
	}

	err := Print(data, "unsupported-format", "")
	if err == nil {
		t.Error("Expected error for unsupported format, got none")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Errorf("Expected 'unsupported output format' error, got: %v", err)
	}
}

func TestPrint_TableFormat(t *testing.T) {
	// Table format currently falls back to JSON
	data := map[string]any{
		"name": "test",
	}

	output := captureOutput(func() {
		err := Print(data, "table", "")
		if err != nil {
			t.Errorf("Print failed: %v", err)
		}
	})

	// Should output JSON (as table is not yet implemented)
	if !strings.Contains(output, `"name"`) {
		t.Error("Table format should output JSON for now")
	}
}

func TestPrint_NilValue(t *testing.T) {
	output := captureOutput(func() {
		err := Print(nil, "tsv", "")
		if err != nil {
			t.Errorf("Print failed: %v", err)
		}
	})

	// TSV format should handle nil gracefully (print nothing)
	output = strings.TrimSpace(output)
	if output != "" {
		t.Errorf("Expected empty output for nil, got: %s", output)
	}
}

func TestPrint_ComplexObject(t *testing.T) {
	data := map[string]any{
		"environmentName": "AzureCloud",
		"id":              "subscription-123",
		"user": map[string]string{
			"name": "client-456",
			"type": "servicePrincipal",
		},
	}

	output := captureOutput(func() {
		err := Print(data, "json", "")
		if err != nil {
			t.Errorf("Print failed: %v", err)
		}
	})

	// Verify all fields are present in output
	if !strings.Contains(output, "AzureCloud") {
		t.Error("Expected environmentName in output")
	}
	if !strings.Contains(output, "subscription-123") {
		t.Error("Expected id in output")
	}
	if !strings.Contains(output, "servicePrincipal") {
		t.Error("Expected user.type in output")
	}
}

func TestPrint_QueryNonExistentField(t *testing.T) {
	data := map[string]any{
		"name": "test",
	}

	output := captureOutput(func() {
		err := Print(data, "tsv", "nonexistent")
		if err != nil {
			t.Errorf("Print failed: %v", err)
		}
	})

	// Query for non-existent field should return nothing
	output = strings.TrimSpace(output)
	if output != "" {
		t.Errorf("Expected empty output for non-existent field, got: %s", output)
	}
}

func TestPrintJSON_WithIndentation(t *testing.T) {
	data := map[string]any{
		"a": 1,
		"b": 2,
	}

	output := captureOutput(func() {
		err := printJSON(data)
		if err != nil {
			t.Errorf("printJSON failed: %v", err)
		}
	})

	// Should have indentation
	if !strings.Contains(output, "  ") {
		t.Error("Expected indented JSON output")
	}
}

func TestPrintTSV_ComplexType(t *testing.T) {
	// Complex types should fallback to JSON string
	data := map[string]any{
		"nested": map[string]any{
			"value": 123,
		},
	}

	output := captureOutput(func() {
		err := printTSV(data)
		if err != nil {
			t.Errorf("printTSV failed: %v", err)
		}
	})

	// Should contain JSON representation
	output = strings.TrimSpace(output)
	if !strings.Contains(output, "nested") || !strings.Contains(output, "value") {
		t.Error("Expected JSON fallback for complex type in TSV")
	}
}

func TestPrint_EmptyString(t *testing.T) {
	output := captureOutput(func() {
		err := Print("", "tsv", "")
		if err != nil {
			t.Errorf("Print failed: %v", err)
		}
	})

	output = strings.TrimSpace(output)
	if output != "" {
		t.Errorf("Expected empty output, got: %s", output)
	}
}

func TestPrint_ArrayOfStrings(t *testing.T) {
	data := []string{"item1", "item2", "item3"}

	output := captureOutput(func() {
		err := Print(data, "json", "")
		if err != nil {
			t.Errorf("Print failed: %v", err)
		}
	})

	if !strings.Contains(output, "item1") ||
		!strings.Contains(output, "item2") ||
		!strings.Contains(output, "item3") {
		t.Error("Expected all array items in output")
	}
}
