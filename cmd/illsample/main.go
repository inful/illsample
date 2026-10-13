// Command illsample parses, validates, and converts Illumina sample
// sheets.
//
// Usage:
//
//	illsample validate <file>          Parse and verify; exit 0 on success, 1 on errors, 2 on usage errors.
//	illsample convert <file>           Convert to the other format; default output is stdout.
//	illsample convert <file> -o OUT    Write to OUT instead of stdout.
//	illsample convert <file> -f v1|v2  Force output format (default: the other one).
//	illsample info <file>              Print a summary of the sheet.
//	illsample version                  Print the version and exit.
//
// Run "illsample help" for the full help text.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/inful/illsample"
)

// version is set via -ldflags at build time.
var version = "dev"

const usage = `illsample — parse, validate, and convert Illumina sample sheets

Usage:
  illsample <command> [flags] <file>

Commands:
  validate     Parse a sample sheet and run verification rules.
  convert      Re-emit a sample sheet in v1 or v2 format.
  info         Print a summary of a sample sheet.
  version      Print the version and exit.
  help         Show this help.

Common flags:
  -h, --help  Show help for a subcommand.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "validate":
		os.Exit(runValidate(args))
	case "convert":
		os.Exit(runConvert(args))
	case "info":
		os.Exit(runInfo(args))
	case "version", "-version", "--version":
		fmt.Println("illsample", version)
		os.Exit(0)
	case "help", "-h", "--help", "-help":
		fmt.Print(usage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "illsample: unknown command %q\n\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

// runValidate implements the "validate" subcommand.
func runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	strict := fs.Bool("strict", false, "treat warnings as errors")
	noWarn := fs.Bool("no-warn", false, "suppress warnings from the output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "validate: expected exactly one file argument")
		return 2
	}
	sheet, err := loadSheet(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate: %v\n", err)
		return 1
	}
	opts := illsample.VerifyOptions{
		PromoteWarningsToErrors: *strict,
		SuppressWarnings:        *noWarn,
	}
	result := illsample.VerifyWithOptions(sheet, opts)
	printResult(os.Stdout, result, *strict, *noWarn)
	if result.HasErrors() {
		return 1
	}
	return 0
}

// runConvert implements the "convert" subcommand.
func runConvert(args []string) int {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	out := fs.String("o", "", "output file (default: stdout)")
	format := fs.String("f", "", "output format: v1 or v2 (default: the other one)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "convert: expected exactly one file argument")
		return 2
	}
	sheet, err := loadSheet(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "convert: %v\n", err)
		return 1
	}
	switch strings.ToLower(*format) {
	case "":
		// Default: flip.
		if sheet.Format == illsample.FormatV1 {
			sheet.Format = illsample.FormatV2
		} else {
			sheet.Format = illsample.FormatV1
		}
	case "v1":
		sheet.Format = illsample.FormatV1
	case "v2":
		sheet.Format = illsample.FormatV2
	default:
		fmt.Fprintf(os.Stderr, "convert: invalid format %q (want v1 or v2)\n", *format)
		return 2
	}
	var w io.Writer = os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "convert: %v\n", err)
			return 1
		}
		defer func() { _ = f.Close() }()
		w = f
	}
	if err := illsample.Write(w, sheet); err != nil {
		fmt.Fprintf(os.Stderr, "convert: %v\n", err)
		return 1
	}
	return 0
}

// runInfo implements the "info" subcommand.
func runInfo(args []string) int {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "info: expected exactly one file argument")
		return 2
	}
	sheet, err := loadSheet(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "info: %v\n", err)
		return 1
	}
	fmt.Printf("Format:        %s\n", sheet.Format)
	fmt.Printf("Rows:          %d\n", len(sheet.Data))
	if sheet.Reads != nil {
		fmt.Printf("Read1Cycles:   %d\n", sheet.Reads.Read1Cycles)
		fmt.Printf("Read2Cycles:   %d\n", sheet.Reads.Read2Cycles)
		fmt.Printf("Index1Cycles:  %d\n", sheet.Reads.Index1Cycles)
		fmt.Printf("Index2Cycles:  %d\n", sheet.Reads.Index2Cycles)
	}
	if sheet.Settings != nil {
		fmt.Printf("Adapter:       %s\n", sheet.Settings.Adapter)
		fmt.Printf("AdapterRead2:  %s\n", sheet.Settings.AdapterRead2)
	}
	if len(sheet.Manifests) > 0 {
		fmt.Printf("Manifests:     %d\n", len(sheet.Manifests))
	}
	if len(sheet.Extra) > 0 {
		fmt.Printf("Extra sections: %s\n", strings.Join(extraNames(sheet.Extra), ", "))
	}
	// Print a short sample list.
	limit := 3
	if len(sheet.Data) < limit {
		limit = len(sheet.Data)
	}
	if limit > 0 {
		fmt.Println("First samples:")
		for i := 0; i < limit; i++ {
			s := sheet.Data[i]
			fmt.Printf("  %s [%s] index=%s\n", s.SampleID, s.SampleName, s.Index)
		}
		if len(sheet.Data) > limit {
			fmt.Printf("  ... %d more\n", len(sheet.Data)-limit)
		}
	}
	return 0
}

// extraNames returns the names of unknown sections, sorted.
func extraNames(extra map[string]illsample.Section) []string {
	names := make([]string, 0, len(extra))
	for name := range extra {
		names = append(names, name)
	}
	// We could sort, but Go map iteration order is already
	// randomised, which is fine for human consumption.
	return names
}

// loadSheet parses a file from disk. It returns a user-friendly error
// on failure.
func loadSheet(path string) (*illsample.SampleSheet, error) {
	if path == "-" {
		return illsample.Parse(os.Stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sheet, err := illsample.Parse(f)
	if err != nil {
		var pe *illsample.ParseError
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), pe)
		}
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return sheet, nil
}

// printResult formats a verification result for the terminal.
// Errors are printed to stderr-like prefix; warnings get a different
// prefix. The colour-free format is used because we cannot reliably
// detect a TTY from inside a flag-parsed subcommand on every OS.
func printResult(w io.Writer, result *illsample.VerificationResult, strict, noWarn bool) {
	if result == nil || result.OK() {
		_, _ = fmt.Fprintln(w, "OK: no issues found")
		return
	}
	if !noWarn {
		for _, iss := range result.Warnings() {
			_, _ = fmt.Fprintf(w, "warning %s: %s [%s]\n", iss.Code, iss.Message, iss.Location)
		}
	}
	for _, iss := range result.Errors() {
		_, _ = fmt.Fprintf(w, "error %s: %s [%s]\n", iss.Code, iss.Message, iss.Location)
	}
	counts := struct{ errors, warnings int }{}
	counts.errors = len(result.Errors())
	counts.warnings = len(result.Warnings())
	_, _ = fmt.Fprintf(w, "summary: %d error(s), %d warning(s) (strict=%v)\n", counts.errors, counts.warnings, strict)
}
