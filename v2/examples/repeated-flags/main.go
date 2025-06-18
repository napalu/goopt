package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/goopt/v2/types"
)

func main() {
	// Example 1: Using repeated flags with bound variables
	var (
		includes []string
		excludes []string
		tags     []string
		verbose  bool
	)

	p := goopt.NewParser()

	// Bind slice variables with Chained type - supports both patterns
	if err := p.BindFlag(&includes, "include", goopt.NewArg(
		goopt.WithType(types.Chained),
		goopt.WithShortFlag("i"),
		goopt.WithDescription("Include paths (can be repeated)"),
	)); err != nil {
		log.Fatal(err)
	}

	if err := p.BindFlag(&excludes, "exclude", goopt.NewArg(
		goopt.WithType(types.Chained),
		goopt.WithShortFlag("e"),
		goopt.WithDescription("Exclude patterns (can be repeated)"),
	)); err != nil {
		log.Fatal(err)
	}

	if err := p.BindFlag(&tags, "tag", goopt.NewArg(
		goopt.WithType(types.Chained),
		goopt.WithShortFlag("t"),
		goopt.WithDescription("Tags to apply (can be repeated or comma-separated)"),
	)); err != nil {
		log.Fatal(err)
	}

	if err := p.BindFlag(&verbose, "verbose", goopt.NewArg(
		goopt.WithType(types.Standalone),
		goopt.WithShortFlag("v"),
		goopt.WithDescription("Enable verbose output"),
	)); err != nil {
		log.Fatal(err)
	}

	// Parse command line arguments
	if !p.Parse(os.Args) {
		fmt.Fprintf(os.Stderr, "Error parsing arguments:\n")
		for _, err := range p.GetErrors() {
			fmt.Fprintf(os.Stderr, "  - %v\n", err)
		}
		fmt.Fprintf(os.Stderr, "\nUsage:\n")
		p.PrintUsage(os.Stderr)
		os.Exit(1)
	}

	// Show results
	fmt.Println("Repeated Flags Example")
	fmt.Println(strings.Repeat("=", 40))

	if verbose {
		fmt.Println("Verbose mode: ON")
	}

	if len(includes) > 0 {
		fmt.Printf("\nIncluded paths (%d):\n", len(includes))
		for i, inc := range includes {
			fmt.Printf("  %d. %s\n", i+1, inc)
		}
	}

	if len(excludes) > 0 {
		fmt.Printf("\nExcluded patterns (%d):\n", len(excludes))
		for i, exc := range excludes {
			fmt.Printf("  %d. %s\n", i+1, exc)
		}
	}

	if len(tags) > 0 {
		fmt.Printf("\nTags (%d):\n", len(tags))
		for i, tag := range tags {
			fmt.Printf("  %d. %s\n", i+1, tag)
		}
	}

	fmt.Println("\nExample usage:")
	fmt.Println("  # Traditional comma-separated approach:")
	fmt.Println("  ./repeated-flags --include \"src,test,docs\" --tag \"dev,prod\"")
	fmt.Println()
	fmt.Println("  # New repeated flag approach:")
	fmt.Println("  ./repeated-flags -i src -i test -i docs -t dev -t prod")
	fmt.Println()
	fmt.Println("  # Mix both approaches:")
	fmt.Println("  ./repeated-flags --include src,test --include docs --tag dev --tag prod,staging")
}
