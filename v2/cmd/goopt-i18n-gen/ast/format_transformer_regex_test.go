package ast

import (
	"testing"
)

func TestUserFacingRegex(t *testing.T) {
	tests := []struct {
		name        string
		regex       string
		funcName    string
		shouldMatch bool
	}{
		{
			name:        "match MsgAll method",
			regex:       `.*\.MsgAll$`,
			funcName:    "s.Log.MsgAll",
			shouldMatch: true,
		},
		{
			name:        "match any Msg* method",
			regex:       `.*\.Msg.*`,
			funcName:    "logger.MsgDebug",
			shouldMatch: true,
		},
		{
			name:        "match custom logger methods",
			regex:       `.*\.(Log|Print|Display|Show|Render|Msg).*`,
			funcName:    "s.Log.MsgAll",
			shouldMatch: true,
		},
		{
			name:        "match Display function",
			regex:       `.*\.(Log|Print|Display|Show|Render|Msg).*`,
			funcName:    "ui.DisplayError",
			shouldMatch: true,
		},
		{
			name:        "not match non-user-facing",
			regex:       `.*\.(Log|Print|Display|Show|Render|Msg).*`,
			funcName:    "db.Query",
			shouldMatch: false,
		},
		{
			name:        "complex regex for multiple patterns",
			regex:       `(.*\.(Log|Print|Display|Show|Render|Msg).*)|(.*\.(Info|Error|Warn|Debug)$)`,
			funcName:    "logger.Info",
			shouldMatch: true,
		},
		{
			name:        "match nested method calls",
			regex:       `.*\.Log\.MsgAll$`,
			funcName:    "s.Log.MsgAll",
			shouldMatch: true,
		},
		{
			name:        "match chained calls",
			regex:       `.*MsgAll$`,
			funcName:    "s.Log.MsgAll",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal string map
			stringMap := map[string]string{
				`"test message"`: "messages.Keys.TestMessage",
			}

			transformer := NewFormatTransformer(stringMap)
			err := transformer.SetUserFacingRegexes([]string{tt.regex})
			if err != nil {
				t.Fatalf("Failed to set regex: %v", err)
			}

			result := transformer.isUserFacingFunction(tt.funcName)
			if result != tt.shouldMatch {
				t.Errorf("isUserFacingFunction(%q) = %v, want %v", tt.funcName, result, tt.shouldMatch)
			}
		})
	}
}

func TestUserFacingRegexInvalidPattern(t *testing.T) {
	transformer := NewFormatTransformer(map[string]string{})
	
	// Test invalid regex pattern
	err := transformer.SetUserFacingRegexes([]string{"[invalid"})
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
	
	// Test empty pattern (should be ignored)
	err = transformer.SetUserFacingRegexes([]string{""})
	if err != nil {
		t.Errorf("Unexpected error for empty pattern: %v", err)
	}
	if len(transformer.userFacingRegexes) != 0 {
		t.Error("Expected userFacingRegexes to be empty after setting empty pattern")
	}
}

func TestUserFacingRegexIntegration(t *testing.T) {
	// Test that regex works alongside built-in patterns
	stringMap := map[string]string{
		`"test message"`: "messages.Keys.TestMessage",
	}
	
	transformer := NewFormatTransformer(stringMap)
	err := transformer.SetUserFacingRegexes([]string{`.*\.MsgAll$`})
	if err != nil {
		t.Fatalf("Failed to set regex: %v", err)
	}
	
	// Should match via regex
	if !transformer.isUserFacingFunction("s.Log.MsgAll") {
		t.Error("Expected s.Log.MsgAll to match regex")
	}
	
	// Should still match built-in patterns
	if !transformer.isUserFacingFunction("fmt.Println") {
		t.Error("Expected fmt.Println to match built-in pattern")
	}
	
	// Should not match neither
	if transformer.isUserFacingFunction("db.Query") {
		t.Error("Expected db.Query to not match")
	}
}