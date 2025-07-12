package ast

import (
	"strings"
	"testing"
)

func TestSkipCommentIntegration(t *testing.T) {
	// Real-world example with mixed content
	input := `package main

import (
	"database/sql"
	"fmt"
	"log"
)

const (
	// Database settings - not user facing
	DBHost = "localhost" // i18n-skip
	DBPort = "5432"      // i18n-skip
	
	// User messages
	AppTitle = "My Application"
	Version  = "1.0.0"
)

var (
	// API configuration
	apiKey    = "sk-prod-123456789" // i18n-skip
	apiSecret = "secret-key-xyz"     // i18n-skip
	
	// Messages
	welcomeMsg = "Welcome to our service!"
	errorMsg   = "An error occurred"
)

func main() {
	// SQL queries should not be translated
	query := "SELECT id, name FROM users WHERE active = true" // i18n-skip
	
	fmt.Println("Starting application...")
	log.Printf("Connecting to database at %s:%s", DBHost, DBPort)
	
	if err := db.Query(query); err != nil {
		log.Fatal("Database query failed")
	}
	
	// Configuration strings
	endpoint := "https://api.example.com/v2/data" // i18n-skip
	fmt.Printf("API endpoint: %s", endpoint)
}`

	// Create string map for translatable strings
	stringMap := map[string]string{
		`"My Application"`:                  "app.extracted.title",
		`"1.0.0"`:                           "app.extracted.version",
		`"Welcome to our service!"`:         "app.extracted.welcome",
		`"An error occurred"`:               "app.extracted.error",
		`"Starting application..."`:         "app.extracted.starting",
		`"Connecting to database at %s:%s"`: "app.extracted.connecting_db",
		`"Database query failed"`:           "app.extracted.db_query_failed",
		`"API endpoint: %s"`:                "app.extracted.api_endpoint",
	}

	transformer := NewFormatTransformer(stringMap)
	transformer.SetTransformMode("all")

	result, err := transformer.TransformFile("test.go", []byte(input))
	if err != nil {
		t.Fatalf("transformation failed: %v", err)
	}

	output := string(result)

	// Verify all skip comments are preserved
	skipCount := strings.Count(input, "// i18n-skip")
	outputSkipCount := strings.Count(output, "// i18n-skip")
	if skipCount != outputSkipCount {
		t.Errorf("Skip comments not preserved: expected %d, got %d", skipCount, outputSkipCount)
	}

	// Verify skipped strings remain unchanged
	skippedStrings := []string{
		`"localhost"`,
		`"5432"`,
		`"sk-prod-123456789"`,
		`"secret-key-xyz"`,
		`"SELECT id, name FROM users WHERE active = true"`,
		`"https://api.example.com/v2/data"`,
	}

	for _, str := range skippedStrings {
		if !strings.Contains(output, str) {
			t.Errorf("Skipped string %s was transformed", str)
		}
	}

	// Verify non-skipped strings were transformed (except in const context)
	transformedInVarOrFunc := []string{
		"Welcome to our service!",
		"An error occurred",
		"Starting application...",
		"Database query failed",
	}

	for _, str := range transformedInVarOrFunc {
		if strings.Contains(output, `"`+str+`"`) {
			t.Errorf("String %q was not transformed", str)
		}
	}

	// Verify const strings remain as literals (can't use function calls in const)
	constStrings := []string{
		"My Application",
		"1.0.0",
	}

	for _, str := range constStrings {
		if !strings.Contains(output, `"`+str+`"`) {
			t.Errorf("Const string %q was incorrectly transformed", str)
		}
	}

	// Verify imports were added
	if !strings.Contains(output, `"messages"`) || !strings.Contains(output, `"github.com/napalu/goopt/v2/i18n"`) {
		t.Error("Required imports were not added")
	}

	// Verify tr variable was declared
	if !strings.Contains(output, "var tr i18n.Translator") {
		t.Error("Translator variable was not declared")
	}
}
