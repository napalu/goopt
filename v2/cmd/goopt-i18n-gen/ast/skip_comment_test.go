package ast

import (
	"testing"
)

func TestI18nSkipComments(t *testing.T) {
	tests := []struct {
		name             string
		code             string
		expectedExtracted []string
		expectedSkipped   []string
	}{
		{
			name: "inline skip comment",
			code: `package main
func test() {
	msg := "This should be extracted"
	query := "SELECT * FROM users" // i18n-skip
	fmt.Println(msg)
}`,
			expectedExtracted: []string{"This should be extracted"},
			expectedSkipped:   []string{"SELECT * FROM users"},
		},
		{
			name: "comment before single line",
			code: `package main
func test() {
	// i18n-skip
	apiKey := "sk-1234567890abcdef"
	msg := "API initialized"
	fmt.Println(msg)
}`,
			expectedExtracted: []string{"API initialized"},
			expectedSkipped:   []string{"sk-1234567890abcdef"},
		},
		{
			name: "comment before multi-line string",
			code: `package main
func test() {
	// i18n-skip
	query := ` + "`" + `
		SELECT u.id, u.name, u.email
		FROM users u
		WHERE u.status = 'active'
	` + "`" + `
	msg := "Query executed"
}`,
			expectedExtracted: []string{"Query executed"},
			expectedSkipped:   []string{"\n\t\tSELECT u.id, u.name, u.email\n\t\tFROM users u\n\t\tWHERE u.status = 'active'\n\t"},
		},
		{
			name: "skip entire function call",
			code: `package main
func test() {
	// i18n-skip
	log.Printf("Debug: User ID = %d, Name = %s", id, name)
	fmt.Println("User processed")
}`,
			expectedExtracted: []string{"User processed"},
			expectedSkipped:   []string{"Debug: User ID = %d, Name = %s"},
		},
		{
			name: "inline comment on multi-line string",
			code: `package main
func test() {
	query := ` + "`" + `SELECT * FROM users` + "`" + ` // i18n-skip
	msg := "Done"
}`,
			expectedExtracted: []string{"Done"},
			expectedSkipped:   []string{"SELECT * FROM users"},
		},
		{
			name: "mixed skip and extract",
			code: `package main
func test() {
	// i18n-skip
	regex := "^[a-zA-Z0-9]+$"
	
	errMsg := "Invalid input format"
	
	// i18n-skip
	template := "{{.Name}} - {{.Date}}"
	
	successMsg := "Operation completed successfully"
}`,
			expectedExtracted: []string{"Invalid input format", "Operation completed successfully"},
			expectedSkipped:   []string{"^[a-zA-Z0-9]+$", "{{.Name}} - {{.Date}}"},
		},
		{
			name: "skip in slog context",
			code: `package main
func test() {
	// i18n-skip
	slog.Info("Debug: Raw SQL", "query", "SELECT * FROM users")
	slog.Info("User logged in", "user", username)
}`,
			expectedExtracted: []string{"User logged in", "user"},
			expectedSkipped:   []string{"Debug: Raw SQL", "query", "SELECT * FROM users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create extractor
			extractor, err := NewStringExtractor(nil, "", "", 0)
			if err != nil {
				t.Fatalf("Failed to create extractor: %v", err)
			}

			// Extract from code
			err = extractor.ExtractFromString("test.go", tt.code)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			extracted := extractor.GetExtractedStrings()

			// Check expected extracted strings
			for _, expected := range tt.expectedExtracted {
				if _, found := extracted[expected]; !found {
					t.Errorf("Expected to extract %q but it was not found", expected)
				}
			}

			// Check expected skipped strings
			for _, skipped := range tt.expectedSkipped {
				if _, found := extracted[skipped]; found {
					t.Errorf("Expected to skip %q but it was extracted", skipped)
				}
			}

			// Log what was extracted for debugging
			if t.Failed() {
				t.Logf("Extracted %d strings:", len(extracted))
				for str := range extracted {
					t.Logf("  - %q", str)
				}
			}
		})
	}
}

func TestI18nSkipEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		code             string
		expectedExtracted []string
		expectedSkipped   []string
	}{
		{
			name: "skip comment not immediately before",
			code: `package main
func test() {
	// i18n-skip
	
	// This line has a gap
	msg := "Should this be skipped?"
}`,
			expectedExtracted: []string{"Should this be skipped?"}, // Gap breaks the association
			expectedSkipped:   []string{},
		},
		{
			name: "case sensitivity",
			code: `package main
func test() {
	// I18N-SKIP
	msg1 := "Case insensitive?"
	// i18n-Skip
	msg2 := "Mixed case?"
}`,
			expectedExtracted: []string{}, // Should handle case variations
			expectedSkipped:   []string{"Case insensitive?", "Mixed case?"},
		},
		{
			name: "skip in block comment",
			code: `package main
func test() {
	/* i18n-skip */
	msg := "Block comment skip"
}`,
			expectedExtracted: []string{},
			expectedSkipped:   []string{"Block comment skip"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create extractor
			extractor, err := NewStringExtractor(nil, "", "", 0)
			if err != nil {
				t.Fatalf("Failed to create extractor: %v", err)
			}

			// Extract from code
			err = extractor.ExtractFromString("test.go", tt.code)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			extracted := extractor.GetExtractedStrings()

			// Check expected extracted strings
			for _, expected := range tt.expectedExtracted {
				if _, found := extracted[expected]; !found {
					t.Errorf("Expected to extract %q but it was not found", expected)
				}
			}

			// Check expected skipped strings
			for _, skipped := range tt.expectedSkipped {
				if _, found := extracted[skipped]; found {
					t.Errorf("Expected to skip %q but it was extracted", skipped)
				}
			}
		})
	}
}
func TestI18nSkipComplexPatterns(t *testing.T) {
	tests := []struct {
		name             string
		code             string
		expectedExtracted []string
		expectedSkipped   []string
	}{
		{
			name: "skip with very long multiline function call",
			code: `package main
func test() {
	// i18n-skip
	db.Exec(` + "`" + `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)` + "`" + `)
	log.Println("Table created successfully")
}`,
			expectedExtracted: []string{"Table created successfully"},
			expectedSkipped:   []string{"\n\t\tCREATE TABLE IF NOT EXISTS users (\n\t\t\tid INTEGER PRIMARY KEY,\n\t\t\tname TEXT NOT NULL,\n\t\t\temail TEXT UNIQUE NOT NULL,\n\t\t\tcreated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,\n\t\t\tupdated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP\n\t\t)"},
		},
		{
			name: "multiple skip comments in same function",
			code: `package main
func test() {
	// i18n-skip
	query1 := "SELECT * FROM users"
	
	result1 := "Users fetched"
	
	// i18n-skip
	query2 := ` + "`" + `DELETE FROM logs WHERE created_at < ?` + "`" + `
	
	result2 := "Logs cleaned"
}`,
			expectedExtracted: []string{"Users fetched", "Logs cleaned"},
			expectedSkipped:   []string{"SELECT * FROM users", "DELETE FROM logs WHERE created_at < ?"},
		},
		{
			name: "skip in nested function calls",
			code: `package main
func test() {
	// i18n-skip
	result := db.Query(fmt.Sprintf("SELECT * FROM %s WHERE id = %d", tableName, id))
	
	if result != nil {
		log.Println("Query successful")
	}
}`,
			expectedExtracted: []string{"Query successful"},
			expectedSkipped:   []string{"SELECT * FROM %s WHERE id = %d"},
		},
		{
			name: "concatenation spanning many lines",
			code: `package main
func test() {
	// i18n-skip
	complexQuery := "SELECT u.id, u.name, u.email, " +
		"p.title, p.content, p.created_at " +
		"FROM users u " +
		"JOIN posts p ON u.id = p.user_id " +
		"WHERE u.active = true " +
		"AND p.published = true " +
		"ORDER BY p.created_at DESC"
	
	fmt.Println("Executing complex query")
}`,
			expectedExtracted: []string{"Executing complex query"},
			expectedSkipped:   []string{
				"SELECT u.id, u.name, u.email, ",
				"p.title, p.content, p.created_at ",
				"FROM users u ",
				"JOIN posts p ON u.id = p.user_id ",
				"WHERE u.active = true ",
				"AND p.published = true ",
				"ORDER BY p.created_at DESC",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create extractor
			extractor, err := NewStringExtractor(nil, "", "", 0)
			if err != nil {
				t.Fatalf("Failed to create extractor: %v", err)
			}

			// Extract from code
			err = extractor.ExtractFromString("test.go", tt.code)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			extracted := extractor.GetExtractedStrings()

			// Check expected extracted strings
			for _, expected := range tt.expectedExtracted {
				if _, found := extracted[expected]; !found {
					t.Errorf("Expected to extract %q but it was not found", expected)
				}
			}

			// Check expected skipped strings
			for _, skipped := range tt.expectedSkipped {
				if _, found := extracted[skipped]; found {
					t.Errorf("Expected to skip %q but it was extracted", skipped)
				}
			}

			// Log what was extracted for debugging
			if t.Failed() {
				t.Logf("Extracted %d strings:", len(extracted))
				for str := range extracted {
					t.Logf("  - %q", str)
				}
			}
		})
	}
}

func TestI18nSkipMultilineInFunctionCall(t *testing.T) {
	tests := []struct {
		name             string
		code             string
		expectedExtracted []string
		expectedSkipped   []string
	}{
		{
			name: "skip comment before function with string concatenation",
			code: `package main
func test() {
	// i18n-skip
	db.Query("SELECT id, name, email " +
		"FROM users " +
		"WHERE active = true", userID, status)
	msg := "Query executed"
}`,
			expectedExtracted: []string{"Query executed"},
			expectedSkipped:   []string{"SELECT id, name, email ", "FROM users ", "WHERE active = true"},
		},
		{
			name: "skip comment before function with backtick multiline",
			code: `package main
func test() {
	// i18n-skip
	db.Query(` + "`" + `SELECT id, name, email
		FROM users
		WHERE active = true` + "`" + `, userID, status)
	msg := "Done"
}`,
			expectedExtracted: []string{"Done"},
			expectedSkipped:   []string{"SELECT id, name, email\n\t\tFROM users\n\t\tWHERE active = true"},
		},
		{
			name: "skip comment affects entire function call with concatenation",
			code: `package main
func test() {
	// i18n-skip
	logger.Printf("User %s performed action %s " +
		"at time %s " +
		"with result %s", 
		username, action, time, result)
	fmt.Println("Action logged")
}`,
			expectedExtracted: []string{"Action logged"},
			expectedSkipped:   []string{"User %s performed action %s ", "at time %s ", "with result %s"},
		},
		{
			name: "inline skip on multiline function call",
			code: `package main
func test() {
	db.Query("SELECT * FROM users WHERE id = ?", id) // i18n-skip
	db.Query(` + "`" + `SELECT name, email
		FROM users
		WHERE active = true` + "`" + `, status) // i18n-skip
	msg := "Queries complete"
}`,
			expectedExtracted: []string{"Queries complete"},
			expectedSkipped:   []string{"SELECT * FROM users WHERE id = ?", "SELECT name, email\n\t\tFROM users\n\t\tWHERE active = true"},
		},
		{
			name: "skip does not affect subsequent lines",
			code: `package main
func test() {
	// i18n-skip
	db.Query(` + "`" + `
		SELECT * FROM users
	` + "`" + `)
	// This should be extracted
	msg1 := "First message"
	msg2 := "Second message"
}`,
			expectedExtracted: []string{"First message", "Second message"},
			expectedSkipped:   []string{"\n\t\tSELECT * FROM users\n\t"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create extractor
			extractor, err := NewStringExtractor(nil, "", "", 0)
			if err != nil {
				t.Fatalf("Failed to create extractor: %v", err)
			}

			// Extract from code
			err = extractor.ExtractFromString("test.go", tt.code)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			extracted := extractor.GetExtractedStrings()

			// Check expected extracted strings
			for _, expected := range tt.expectedExtracted {
				if _, found := extracted[expected]; !found {
					t.Errorf("Expected to extract %q but it was not found", expected)
				}
			}

			// Check expected skipped strings
			for _, skipped := range tt.expectedSkipped {
				if _, found := extracted[skipped]; found {
					t.Errorf("Expected to skip %q but it was extracted", skipped)
				}
			}

			// Log what was extracted for debugging
			if t.Failed() {
				t.Logf("Extracted %d strings:", len(extracted))
				for str := range extracted {
					t.Logf("  - %q", str)
				}
			}
		})
	}
}