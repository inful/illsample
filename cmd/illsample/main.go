// Command illsample parses, validates, and converts Illumina sample
// sheets.
//
// Usage:
//
//	illsample validate <file>          Parse and verify; exit 0 on success, 1 on errors, 2 on usage errors.
//	illsample new [-f v1|v2] [-o OUT] Write a sample sheet to fill in.
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
	"sort"
	"strings"

	"github.com/inful/illsample"
)

// version, commit, and date are set via -ldflags at build time
// (see .goreleaser.yml). The defaults let `go run` and `go install`
// work without the build tags; `go install @v0.1.0` and the
// release binaries will have real values injected.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `illsample — parse, validate, and convert Illumina sample sheets

Usage:
  illsample <command> [flags] <file>

Commands:
  validate     Parse a sample sheet and run verification rules.
  new          Write a sample sheet to fill in.
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
	case "new":
		os.Exit(runNew(args))
	case "convert":
		os.Exit(runConvert(args))
	case "info":
		os.Exit(runInfo(args))
	case "version", "-version", "--version":
		fmt.Printf("illsample %s\n", version)
		if commit != "none" {
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		}
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

// newFlagSet returns the flag set of a subcommand. The flag package
// prints parse errors itself, so the output is muted here;
// parseFlags decides where the usage text goes.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "Usage: illsample %s [flags] <file>\n\nFlags:\n", name)
		fs.PrintDefaults()
	}
	return fs
}

// parseFlags parses a subcommand's flags. It returns ok=false when
// the command must stop, with the exit code to use: 2 for a usage
// error, 0 for a help request (-h), which is not an error.
func parseFlags(fs *flag.FlagSet, args []string) (exitCode int, ok bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(os.Stdout)
			fs.Usage()
			return 0, false
		}
		fs.SetOutput(os.Stderr)
		fmt.Fprintf(os.Stderr, "usage error: %v\n", err)
		fs.Usage()
		return 2, false
	}
	return 0, true
}

// runValidate implements the "validate" subcommand.
//
// Output goes to stderr for diagnostics (errors, warnings) and to
// stdout for the summary line ("OK: no issues found" or the
// counts). This means `illsample validate good.csv | sed ...` does
// not mix error messages into the parsed stream.
func runValidate(args []string) int {
	fs := newFlagSet("validate")
	strict := fs.Bool("strict", false, "treat warnings as errors")
	noWarn := fs.Bool("no-warn", false, "suppress warnings from the output")
	format := fs.String("f", "", "input format: v1 or v2 (default: auto-detect)")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "validate: expected exactly one file argument")
		return 2
	}
	switch strings.ToLower(*format) {
	case "", "v1", "v2":
	default:
		fmt.Fprintf(os.Stderr, "validate: invalid format %q (want v1 or v2)\n", *format)
		return 2
	}
	sheet, err := loadSheet(fs.Arg(0), *format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate: %v\n", err)
		return 1
	}
	opts := illsample.VerifyOptions{
		PromoteWarningsToErrors: *strict,
		SuppressWarnings:        *noWarn,
	}
	result := illsample.VerifyWithOptions(sheet, opts)
	printResult(os.Stdout, os.Stderr, result, *strict, *noWarn)
	if result.HasErrors() {
		return 1
	}
	return 0
}

// runConvert implements the "convert" subcommand.
func runConvert(args []string) int {
	fs := newFlagSet("convert")
	out := fs.String("o", "", "output file (default: stdout)")
	format := fs.String("f", "", "output format: v1 or v2 (default: the other one)")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "convert: expected exactly one file argument")
		return 2
	}
	sheet, err := loadSheet(fs.Arg(0), "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "convert: %v\n", err)
		return 1
	}
	switch strings.ToLower(*format) {
	case "":
		// Default: flip v1 <-> v2. A programmatically-constructed
		// sheet (FormatUnknown) is ambiguous: the user must be
		// explicit about which side of the flip they want.
		switch sheet.Format {
		case illsample.FormatV1:
			sheet.Format = illsample.FormatV2
		case illsample.FormatV2:
			sheet.Format = illsample.FormatV1
		default:
			fmt.Fprintln(os.Stderr, "convert: sheet format is unknown; specify -f v1 or -f v2")
			return 2
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

// runNew implements the "new" subcommand: it writes a sample sheet
// to fill in.
func runNew(args []string) int {
	fs := newFlagSet("new")
	out := fs.String("o", "", "output file (default: stdout)")
	format := fs.String("f", "v2", "format: v1 or v2 (default: v2)")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "new: expected no file argument")
		return 2
	}
	var sheet *illsample.SampleSheet
	switch strings.ToLower(*format) {
	case "v1":
		sheet = newTemplateV1()
	case "v2":
		sheet = newTemplateV2()
	default:
		fmt.Fprintf(os.Stderr, "new: invalid format %q (want v1 or v2)\n", *format)
		return 2
	}
	var w io.Writer = os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "new: %v\n", err)
			return 1
		}
		defer func() { _ = f.Close() }()
		w = f
	}
	if err := illsample.Write(w, sheet); err != nil {
		fmt.Fprintf(os.Stderr, "new: %v\n", err)
		return 1
	}
	return 0
}

// newTemplateV2 returns a v2 sample sheet to fill in: the sections
// Illumina's own template carries, and one example sample.
func newTemplateV2() *illsample.SampleSheet {
	return &illsample.SampleSheet{
		Format: illsample.FormatV2,
		Header: map[string]string{
			"FileFormatVersion":  "2",
			"RunName":            "MyRun",
			"InstrumentPlatform": "NovaSeq",
			"InstrumentType":     "NovaSeq",
		},
		Reads: &illsample.Reads{
			Read1Cycles:  151,
			Read2Cycles:  151,
			Index1Cycles: 8,
			Index2Cycles: 8,
		},
		Data: []illsample.Sample{
			{
				SampleID:      "sample_01",
				SampleName:    "sample_one",
				SamplePlate:   "plate1",
				SampleWell:    "A1",
				Index:         "ATCGATCG",
				SampleProject: "Project_Alpha",
			},
		},
	}
}

// newTemplateV1 returns a v1 sample sheet to fill in: the columns
// bcl2fastq reads, and one example sample.
//
// The sheet records no columns, so the writer gives it the v1 default
// header: the template and the writer agree on one definition.
func newTemplateV1() *illsample.SampleSheet {
	return &illsample.SampleSheet{
		Format: illsample.FormatV1,
		Data: []illsample.Sample{
			{
				SampleID:      "sample_01",
				SampleName:    "sample_one",
				Index:         "ATCGATCG",
				SampleProject: "Project_Alpha",
			},
		},
	}
}

// runInfo implements the "info" subcommand.
func runInfo(args []string) int {
	fs := newFlagSet("info")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "info: expected exactly one file argument")
		return 2
	}
	sheet, err := loadSheet(fs.Arg(0), "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "info: %v\n", err)
		return 1
	}
	fmt.Printf("Format:        %s\n", sheet.Format)
	fmt.Printf("Rows:          %d\n", len(sheet.Data))
	// The [Header] section holds the run metadata; it is the first
	// thing a user wants to know about a sheet. The labels are as
	// wide as the longest of them, so the values line up.
	names := headerNames(sheet.Header)
	width := 0
	for _, k := range names {
		width = max(width, len(k)+1)
	}
	for _, k := range names {
		fmt.Printf("%-*s %s\n", width, k+":", sheet.Header[k])
	}
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

// headerNames returns the names of the [Header] records, sorted
// alphabetically. Sorting gives stable output across runs, which
// matters when the user is comparing two invocations of `illsample
// info` or copy/pasting into a bug report.
func headerNames(header map[string]string) []string {
	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// extraNames returns the names of unknown sections, sorted
// alphabetically. Sorting gives stable output across runs, which
// matters when the user is comparing two invocations of `illsample
// info` or copy/pasting into a bug report.
func extraNames(extra map[string]illsample.Section) []string {
	names := make([]string, 0, len(extra))
	for name := range extra {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// loadSheet parses a file from disk. The format argument forces a
// specific layout ("v1" or "v2"); an empty argument auto-detects it.
// The returned error is wrapped with the file's basename (or "<stdin>"
// for the `-` path) so the user can tell which file failed when
// they're looping over many.
func loadSheet(path, format string) (*illsample.SampleSheet, error) {
	parse := illsample.Parse
	switch strings.ToLower(format) {
	case "v1":
		parse = illsample.ParseV1
	case "v2":
		parse = illsample.ParseV2
	}
	if path == "-" {
		sheet, err := parse(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("<stdin>: %w", err)
		}
		return sheet, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	sheet, err := parse(f)
	if err != nil {
		var pe *illsample.ParseError
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("%s: %w", path, pe)
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return sheet, nil
}

// printResult formats a verification result for the terminal. Errors
// and warnings are written to errOut (typically stderr) so a
// downstream pipe only sees the summary on stdout.
//
// Each issue is printed once. Under -strict the warnings are also
// counted as errors, so they are not repeated in the error list; the
// summary line says so.
func printResult(out io.Writer, errOut io.Writer, result *illsample.VerificationResult, strict, noWarn bool) {
	if result == nil || result.OK() {
		_, _ = fmt.Fprintln(out, "OK: no issues found")
		return
	}
	if !noWarn {
		for _, iss := range result.Warnings() {
			_, _ = fmt.Fprintf(errOut, "warning %s: %s [%s]\n", iss.Code, iss.Message, iss.Location)
		}
	}
	for _, iss := range result.Errors() {
		if iss.Severity == illsample.SeverityWarning {
			// Already printed above; -strict only changes the count.
			continue
		}
		_, _ = fmt.Fprintf(errOut, "error %s: %s [%s]\n", iss.Code, iss.Message, iss.Location)
	}
	counts := struct{ errors, warnings int }{}
	counts.errors = len(result.Errors())
	if !strict {
		counts.warnings = len(result.Warnings())
	}
	_, _ = fmt.Fprintf(out, "summary: %d error(s), %d warning(s) (strict=%v)\n", counts.errors, counts.warnings, strict)
}
