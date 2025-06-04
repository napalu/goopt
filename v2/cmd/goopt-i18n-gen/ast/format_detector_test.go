package ast

import (
	"go/ast"
	"go/parser"
	"testing"
)

func parseCallExpr(code string) (*ast.CallExpr, error) {
	expr, err := parser.ParseExpr(code)
	if err != nil {
		return nil, err
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, err
	}
	return call, nil
}

func TestFormatDetectorCustomPatterns(t *testing.T) {
	tests := []struct {
		name           string
		pattern        string
		argIndex       int
		code           string
		shouldDetect   bool
		expectedFormat string
	}{
		{
			name:           "MsgAll with format at index 1",
			pattern:        `.*\.MsgAll$`,
			argIndex:       1,
			code:           `s.Log.MsgAll(fields, "User %s disabled", username)`,
			shouldDetect:   true,
			expectedFormat: "User %s disabled",
		},
		{
			name:           "Custom logger with format at index 0",
			pattern:        `.*\.Logf$`,
			argIndex:       0,
			code:           `logger.Logf("Error: %v", err)`,
			shouldDetect:   true,
			expectedFormat: "Error: %v",
		},
		{
			name:           "Pattern doesn't match",
			pattern:        `.*\.Printf$`,
			argIndex:       0,
			code:           `logger.Log("Simple message")`,
			shouldDetect:   false,
			expectedFormat: "",
		},
		{
			name:           "Match but no format specifiers",
			pattern:        `.*\.Info$`,
			argIndex:       0,
			code:           `logger.Info("Simple message without format")`,
			shouldDetect:   true,
			expectedFormat: "Simple message without format",
		},
		{
			name:           "Complex pattern for multiple methods",
			pattern:        `.*\.(Infof|Warnf|Errorf)$`,
			argIndex:       0,
			code:           `log.Warnf("Warning: %d items failed", count)`,
			shouldDetect:   true,
			expectedFormat: "Warning: %d items failed",
		},
		{
			name:           "Index out of bounds",
			pattern:        `.*\.BadFunc$`,
			argIndex:       5,
			code:           `obj.BadFunc("only one arg")`,
			shouldDetect:   false,
			expectedFormat: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewFormatDetector()
			err := detector.RegisterCustomFormatPattern(tt.pattern, tt.argIndex)
			if err != nil {
				t.Fatalf("Failed to register pattern: %v", err)
			}

			call, err := parseCallExpr(tt.code)
			if err != nil {
				t.Fatalf("Failed to parse expression: %v", err)
			}

			info := detector.DetectFormatCall(call)
			if tt.shouldDetect {
				if info == nil {
					t.Error("Expected format call to be detected, but it wasn't")
					return
				}
				if info.FormatString != tt.expectedFormat {
					t.Errorf("Expected format string %q, got %q", tt.expectedFormat, info.FormatString)
				}
				if info.FormatStringIndex != tt.argIndex {
					t.Errorf("Expected format string index %d, got %d", tt.argIndex, info.FormatStringIndex)
				}
			} else {
				if info != nil {
					t.Errorf("Expected no detection, but got: %+v", info)
				}
			}
		})
	}
}

func TestFormatDetectorInvalidRegex(t *testing.T) {
	detector := NewFormatDetector()
	
	// Test invalid regex
	err := detector.RegisterCustomFormatPattern("[invalid", 0)
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}

func TestFormatDetectorMultiplePatterns(t *testing.T) {
	detector := NewFormatDetector()
	
	// Register multiple patterns
	patterns := []struct {
		pattern  string
		argIndex int
	}{
		{`.*\.MsgAll$`, 1},
		{`.*\.Logf$`, 0},
		{`.*\.(Infof|Warnf|Errorf)$`, 0},
	}
	
	for _, p := range patterns {
		err := detector.RegisterCustomFormatPattern(p.pattern, p.argIndex)
		if err != nil {
			t.Fatalf("Failed to register pattern %s: %v", p.pattern, err)
		}
	}
	
	// Test each pattern
	testCases := []struct {
		code           string
		expectedIndex  int
		expectedFormat string
	}{
		{`s.Log.MsgAll(nil, "Format %s", arg)`, 1, "Format %s"},
		{`logger.Logf("Debug: %v", val)`, 0, "Debug: %v"},
		{`log.Errorf("Error: %w", err)`, 0, "Error: %w"},
	}
	
	for _, tc := range testCases {
		call, err := parseCallExpr(tc.code)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", tc.code, err)
		}
		
		info := detector.DetectFormatCall(call)
		if info == nil {
			t.Errorf("Expected detection for %s", tc.code)
			continue
		}
		
		if info.FormatStringIndex != tc.expectedIndex {
			t.Errorf("For %s: expected index %d, got %d", tc.code, tc.expectedIndex, info.FormatStringIndex)
		}
		if info.FormatString != tc.expectedFormat {
			t.Errorf("For %s: expected format %q, got %q", tc.code, tc.expectedFormat, info.FormatString)
		}
	}
}