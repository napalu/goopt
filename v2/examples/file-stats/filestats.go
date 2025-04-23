package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/napalu/goopt/v2"
)

// Config defines the command-line options using goopt struct tags.
type Config struct {
	// --- Positional Arguments ---
	// Use the `pos` tag to define positional arguments. Order matters.
	InputFile string `goopt:"pos:0;required:true;desc:Path to the input file"`
	// Default value "-" signifies stdout. We'll handle this in our logic.
	OutputFile string `goopt:"pos:1;short:o;default:-;desc:Path for the output results (use '-' for stdout)"`

	// --- Operation Flags ---
	// Boolean flags (type:standalone is inferred for bool)
	Lines bool `goopt:"short:l;desc:Count the number of lines"`
	Words bool `goopt:"short:w;desc:Count the number of words"`
	Chars bool `goopt:"short:c;desc:Count the number of characters"`
	All   bool `goopt:"short:a;desc:Count lines, words, and characters (overrides -l, -w, -c)"`

	// --- Other Flags ---
	Verbose bool `goopt:"short:v;desc:Enable verbose output during processing"`
	// Help flag is handled implicitly by checking parser.Parse result
	Help bool `goopt:"short:h;desc:Show this help message"`
	// or by adding a specific --help flag if custom behavior is needed.
}

func printStdErr(format string, a ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, a...)
}

func main() {
	cfg := &Config{}

	// 1. Initialize Parser from Struct
	// This reads the struct tags and configures the parser.
	parser, err := goopt.NewParserFromStruct(cfg)
	if err != nil {
		// This error usually indicates a problem with the struct tags themselves
		printStdErr("Error initializing parser: %v\n", err)
		os.Exit(1)
	}

	// 2. Parse Command-Line Arguments
	// Pass os.Args directly. goopt handles skipping the program name.
	success := parser.Parse(os.Args)
	// we check if help was requested, irrespective of whether an error was reported and when set print usage and exit.
	if cfg.Help {
		parser.PrintUsageWithGroups(os.Stdout)
		os.Exit(0)
	}

	if !success {
		// Parsing failed, likely due to missing required args or invalid format
		printStdErr("Error: Invalid command-line arguments.")
		for _, parseErr := range parser.GetErrors() {
			printStdErr(" - %s\n", parseErr) // Uses i18n error messages
		}

		printStdErr("\n")
		parser.PrintUsageWithGroups(os.Stdout) // Show detailed usage
		os.Exit(1)
	}

	processFile(cfg)
}

func processFile(cfg *Config) {
	if cfg.Verbose {
		printStdErr("Processing file: %s\n", cfg.InputFile)
	}

	// Determine output writer
	var err error
	var outputWriter io.Writer = os.Stdout
	var outFile *os.File // Keep track to close it later if it's a file
	if cfg.OutputFile != "-" {
		outFile, err = os.Create(cfg.OutputFile)
		if err != nil {
			printStdErr("Error creating output file '%s': %v\n", cfg.OutputFile, err)
			os.Exit(1)
		}
		defer outFile.Close()
		outputWriter = outFile
		if cfg.Verbose {
			printStdErr("Writing output to: %s\n", cfg.OutputFile)
		}
	} else {
		if cfg.Verbose {
			printStdErr("Writing output to stdout\n")
		}
	}

	inputFile, err := os.Open(cfg.InputFile)
	if err != nil {
		printStdErr("Error opening input file '%s': %v\n", cfg.InputFile, err)
		os.Exit(1)
	}
	defer inputFile.Close()

	contentBytes, err := io.ReadAll(inputFile)
	if err != nil {
		printStdErr("Error reading input file '%s': %v\n", cfg.InputFile, err)
		os.Exit(1)
	}
	content := string(contentBytes)

	// --- Calculate Stats ---
	lineCount := 0
	wordCount := 0
	charCount := 0

	// Determine which stats to calculate
	calcLines := cfg.Lines || cfg.All
	calcWords := cfg.Words || cfg.All
	calcChars := cfg.Chars || cfg.All

	// If no specific flags or --all are given, default to calculating all
	if !calcLines && !calcWords && !calcChars {
		if cfg.Verbose {
			_, _ = fmt.Fprintln(os.Stderr, "No specific stats requested, calculating all.")
		}
		calcLines, calcWords, calcChars = true, true, true
	}

	// Perform calculations
	if calcLines || calcWords { // Scanner needed for lines/words
		scanner := bufio.NewScanner(strings.NewReader(content))
		for scanner.Scan() {
			lineCount++
			if calcWords {
				wordCount += len(strings.Fields(scanner.Text()))
			}
		}
		if err := scanner.Err(); err != nil {
			printStdErr("Error scanning input file: %v\n", err)
			os.Exit(1)
		}
	}

	if calcChars {
		charCount = utf8.RuneCountInString(content)
	}

	// --- Output Results ---
	if calcLines {
		_, _ = fmt.Fprintf(outputWriter, "Lines: %d\n", lineCount)
	}
	if calcWords {
		_, _ = fmt.Fprintf(outputWriter, "Words: %d\n", wordCount)
	}
	if calcChars {
		_, _ = fmt.Fprintf(outputWriter, "Characters: %d\n", charCount)
	}

	if cfg.Verbose {
		_, _ = fmt.Fprintln(os.Stderr, "Processing complete.")
	}
}
