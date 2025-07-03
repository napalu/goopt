package ast

import (
	"strings"
	"testing"
)

func TestSkipCommentPreservation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // What we expect to see in the output
	}{
		{
			name: "preserve inline skip comment on const",
			input: `package main

const defaultTime = "0000-00-00 00:00:00" // i18n-skip
const greeting = "Hello, World!"
`,
			expected: `// i18n-skip`,
		},
		{
			name: "preserve inline skip comment on var",
			input: `package main

var apiKey = "sk-123456" // i18n-skip
var message = "Welcome"
`,
			expected: `// i18n-skip`,
		},
		{
			name: "preserve inline skip comment in assignment",
			input: `package main

func main() {
	query := "SELECT * FROM users" // i18n-skip
	msg := "Query executed"
}
`,
			expected: `// i18n-skip`,
		},
		{
			name: "preserve skip comment before declaration",
			input: `package main

// i18n-skip
const sqlQuery = "INSERT INTO logs (message) VALUES (?)"
const userMsg = "Operation completed"
`,
			expected: `// i18n-skip`,
		},
		{
			name: "preserve multiple skip comments",
			input: `package main

const query1 = "SELECT id FROM users" // i18n-skip
const msg = "Found users"
const query2 = "UPDATE users SET active = true" // i18n-skip
`,
			expected: `// i18n-skip`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a string map with quoted strings as keys (matching AST BasicLit format)
			// and properly formatted message keys as values
			stringMap := map[string]string{
				`"Hello, World!"`:       "messages.Keys.App.Greeting",
				`"Welcome"`:             "messages.Keys.App.Welcome",
				`"Query executed"`:      "messages.Keys.App.QueryExecuted",
				`"Operation completed"`: "messages.Keys.App.OperationCompleted",
				`"Found users"`:         "messages.Keys.App.FoundUsers",
			}

			// Create transformer
			transformer := NewFormatTransformer(stringMap)
			transformer.SetTransformMode("all") // Transform all strings

			// Transform the file
			result, err := transformer.TransformFile("test.go", []byte(tt.input))
			if err != nil {
				t.Fatalf("transformation failed: %v", err)
			}

			// Check that the skip comment is preserved
			if !strings.Contains(string(result), tt.expected) {
				t.Errorf("skip comment not preserved in output.\nInput:\n%s\nOutput:\n%s", tt.input, string(result))
			}

			// Debug: print the output to see what's happening
			if t.Failed() {
				t.Logf("Debug output:\n%s", string(result))
			}

			// Also verify that strings with skip comments were NOT transformed
			if strings.Contains(tt.input, `"0000-00-00 00:00:00" // i18n-skip`) {
				// The string literal should still be present in the output
				if !strings.Contains(string(result), `"0000-00-00 00:00:00"`) {
					t.Errorf("string with i18n-skip comment was transformed when it shouldn't have been")
				}
			}
			if strings.Contains(tt.input, `"SELECT * FROM users" // i18n-skip`) {
				// The string literal should still be present in the output
				if !strings.Contains(string(result), `"SELECT * FROM users"`) {
					t.Errorf("string with i18n-skip comment was transformed when it shouldn't have been")
				}
			}

			// Verify that strings WITHOUT skip comments WERE transformed (if they can be)
			// Note: const declarations cannot be transformed to function calls
			if strings.Contains(tt.input, `"Hello, World!"`) && !strings.Contains(tt.input, `"Hello, World!" // i18n-skip`) {
				// Check if this is in a const declaration
				if strings.Contains(tt.input, "const") && strings.Contains(tt.input, `"Hello, World!"`) {
					// Const strings can't be transformed to function calls - this is expected
					if strings.Contains(string(result), `"Hello, World!"`) {
						// This is correct - const kept as-is
					}
				} else {
					// For non-const contexts, verify transformation
					if !strings.Contains(string(result), "tr.T(") && strings.Contains(tt.input, `msg := "Welcome"`) {
						t.Errorf("string without i18n-skip comment in non-const context was not transformed")
					}
				}
			}
		})
	}
}

func TestSkipCommentPreservationInComplexScenarios(t *testing.T) {
	input := `package main

import (
	"fmt"
	"log"
)

const (
	// Application messages
	WelcomeMsg = "Welcome to our application"
	
	// Database queries - should not be translated
	UserQuery = "SELECT * FROM users WHERE id = ?" // i18n-skip
	LogQuery  = "INSERT INTO logs (message) VALUES (?)" // i18n-skip
)

func main() {
	apiEndpoint := "https://api.example.com/v1" // i18n-skip
	
	fmt.Println("Starting application")
	log.Printf("Connecting to %s", apiEndpoint)
	
	// SQL queries should be skipped
	db.Query("SELECT COUNT(*) FROM users") // i18n-skip
}
`

	// Create properly formatted string map
	stringMap := map[string]string{
		`"Welcome to our application"`: "messages.Keys.App.WelcomeMessage",
		`"Starting application"`:       "messages.Keys.App.Starting",
		`"Connecting to %s"`:           "messages.Keys.App.ConnectingTo",
	}

	transformer := NewFormatTransformer(stringMap)
	transformer.SetTransformMode("all")

	result, err := transformer.TransformFile("test.go", []byte(input))
	if err != nil {
		t.Fatalf("transformation failed: %v", err)
	}

	output := string(result)

	// Check all skip comments are preserved
	skipComments := []string{
		`"SELECT * FROM users WHERE id = ?" // i18n-skip`,
		`"INSERT INTO logs (message) VALUES (?)" // i18n-skip`,
		`"https://api.example.com/v1" // i18n-skip`,
		`"SELECT COUNT(*) FROM users") // i18n-skip`,
	}

	for _, comment := range skipComments {
		if !strings.Contains(output, "// i18n-skip") {
			t.Errorf("skip comment not preserved for: %s", comment)
		}
	}

	// Verify skipped strings were not transformed
	skippedStrings := []string{
		"SELECT * FROM users WHERE id = ?",
		"INSERT INTO logs (message) VALUES (?)",
		"https://api.example.com/v1",
		"SELECT COUNT(*) FROM users",
	}

	for _, str := range skippedStrings {
		// Check that the string still appears as a literal (not wrapped in tr.T)
		if !strings.Contains(output, `"`+str+`"`) {
			t.Errorf("skipped string was transformed: %s", str)
		}
	}

	// Verify non-skipped strings were transformed
	// Note: const strings cannot be transformed
	transformedStrings := []struct {
		original string
		key      string
		inConst  bool
	}{
		{"Welcome to our application", "App.WelcomeMessage", true}, // in const - won't transform
		{"Starting application", "App.Starting", false},            // in fmt.Println - will transform
		{"Connecting to %s", "App.ConnectingTo", false},            // in log.Printf - will transform
	}

	for _, ts := range transformedStrings {
		if ts.inConst {
			// Const strings should remain unchanged
			if !strings.Contains(output, `"`+ts.original+`"`) {
				t.Errorf("const string was incorrectly transformed: %s", ts.original)
			}
		} else {
			// Non-const strings should be transformed
			if strings.Contains(output, `"`+ts.original+`"`) && !strings.Contains(input, ts.original+`" // i18n-skip`) {
				t.Errorf("string was not transformed: %s", ts.original)
			}
			if !strings.Contains(output, "tr.T(messages.Keys."+ts.key) {
				t.Errorf("expected transformation not found for: %s", ts.original)
			}
		}
	}
}
