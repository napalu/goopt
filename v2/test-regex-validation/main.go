package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/types"
	"github.com/napalu/goopt/v2/validation"
)

// Local copy of ParseValidatorSpecs to test the logic
func parseValidatorSpecs(input string) []string {
	if input == "" {
		return nil
	}

	var result []string
	var current strings.Builder
	var escaped bool

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if escaped {
			switch ch {
			case ',', ':', '\\':
				current.WriteByte(ch) // Write the literal character
			default:
				current.WriteByte('\\') // Preserve unknown escapes
				current.WriteByte(ch)
			}
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		if ch == ',' {
			// End of current validator spec
			spec := strings.TrimSpace(current.String())
			if spec != "" {
				result = append(result, spec)
			}
			current.Reset()
			continue
		}

		current.WriteByte(ch)
	}

	// Handle the last validator
	spec := strings.TrimSpace(current.String())
	if spec != "" {
		result = append(result, spec)
	}

	return result
}

func main() {
	// Test 1: ParseValidatorSpecs with escaped commas
	fmt.Println("=== Test 1: ParseValidatorSpecs ===")
	spec := `regex:^[A-Z]{2\,4}-[0-9]{3\,5}$`
	fmt.Printf("Input spec: %s\n", spec)

	specs := parseValidatorSpecs(spec)
	fmt.Printf("Parsed specs: %v\n", specs)
	for i, s := range specs {
		fmt.Printf("  [%d]: %q\n", i, s)
	}

	// Test 2: Create validators from parsed specs
	fmt.Println("\n=== Test 2: Create Validators ===")
	validators, err := validation.ParseValidators(specs)
	if err != nil {
		fmt.Printf("Error creating validators: %v\n", err)
	} else {
		fmt.Printf("Successfully created %d validator(s)\n", len(validators))

		// Test the validator
		testValues := []string{
			"AB-123",     // valid
			"ABCD-12345", // valid
			"A-123",      // invalid (too short prefix)
			"ABCDE-123",  // invalid (too long prefix)
			"AB-12",      // invalid (too short suffix)
			"AB-123456",  // invalid (too long suffix)
		}

		fmt.Println("\nTesting validator:")
		for _, val := range testValues {
			err := validators[0](val)
			if err != nil {
				fmt.Printf("  %q -> Error: %v\n", val, err)
			} else {
				fmt.Printf("  %q -> Valid\n", val)
			}
		}
	}

	// Test 3: Direct regex compilation
	fmt.Println("\n=== Test 3: Direct Regex Compilation ===")
	if len(specs) > 0 && len(specs[0]) > 6 && specs[0][:6] == "regex:" {
		pattern := specs[0][6:]
		fmt.Printf("Extracted pattern: %q\n", pattern)

		re, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Printf("Regex compilation error: %v\n", err)
		} else {
			fmt.Println("Regex compiled successfully!")
			fmt.Printf("Pattern matches: %v\n", re.MatchString("AB-123"))
		}
	}

	// Test 4: Using goopt with validators
	fmt.Println("\n=== Test 4: Using goopt with validators ===")

	var productCode string
	parser := goopt.NewParser()

	// Add flag with validator
	err = parser.BindFlag(&productCode, "product-code", goopt.NewArg(
		goopt.WithType(types.Single),
		goopt.WithShortFlag("p"),
		goopt.WithDescription("Product code (format: XX-NNN)"),
		goopt.WithValidator(validation.Regex("^[A-Z]{2,4}-[0-9]{3,5}$", "Invalid product code format")),
	))

	if err != nil {
		fmt.Printf("Error binding flag: %v\n", err)
		os.Exit(1)
	}

	// Parse command line arguments
	success := parser.Parse(os.Args[1:])
	if !success {
		fmt.Printf("Parse returned false (likely validation error or help requested)\n")
	}

	if productCode != "" {
		fmt.Printf("\nParsed product code: %q\n", productCode)
	}

	// Test 5: Test without escaping (this should fail)
	fmt.Println("\n=== Test 5: Without Escaping (should fail) ===")
	badSpec := `regex:^[A-Z]{2,4}-[0-9]{3,5}$`
	fmt.Printf("Input spec (no escaping): %s\n", badSpec)

	badSpecs := parseValidatorSpecs(badSpec)
	fmt.Printf("Parsed specs: %v\n", badSpecs)
	fmt.Printf("Number of specs: %d\n", len(badSpecs))

	if len(badSpecs) > 1 {
		fmt.Println("ERROR: Comma in regex pattern was incorrectly treated as separator!")
		fmt.Printf("  First spec: %q\n", badSpecs[0])
		fmt.Printf("  Second spec: %q\n", badSpecs[1])
	}
}
