// Package illsample provides a parser, writer, and verifier for
// Illumina sample sheets.
//
// Two layouts are supported:
//
//   - v1 (legacy): a single CSV with a header row. The column order
//     does not matter; columns are matched by name.
//
//   - v2 (IEM): a sequence of sections, each introduced by a [Name]
//     header. [Data] is a CSV section, [Reads] is a vertical
//     key/value section, and [Settings] is a 2-column CSV section.
//
// The high-level Parse function auto-detects the layout; use
// ParseV1 or ParseV2 when the format is known. Write picks the
// serializer from the sheet's Format field. Verify runs a battery
// of rules and returns a structured result with errors and warnings.
//
// Typical use:
//
//	sheet, err := illsample.Parse(r)
//	if err != nil { ... }
//	result := illsample.Verify(sheet)
//	if result.HasErrors() { ... }
//	if err := illsample.Verify(w, sheet); err != nil { ... }
package illsample

import (
	"io"

	"github.com/inful/illsample/internal/model"
	"github.com/inful/illsample/internal/parser"
	"github.com/inful/illsample/internal/verify"
	"github.com/inful/illsample/internal/writer"
)

// Re-exported types from the internal packages. The aliases mean
// users can refer to, e.g., illsample.SampleSheet without needing
// to import any internal sub-package, while the underlying types
// remain the same.

type (
	// Format identifies the on-disk layout of a sample sheet.
	Format = model.Format
	// SampleSheet is the in-memory representation of an Illumina
	// sample sheet. See model.SampleSheet for full documentation.
	SampleSheet = model.SampleSheet
	// Sample is a single row of the [Data] section.
	Sample = model.Sample
	// Section is a generic v2 section that the model does not
	// capture explicitly (preserved verbatim on round-trip).
	Section = model.Section
	// Reads is the [Reads] section of a v2 sheet.
	Reads = model.Reads
	// Settings is the [Settings] section of a v2 sheet.
	Settings = model.Settings
	// Manifest is one row of the [Manifests] section of a v2 sheet.
	Manifest = model.Manifest
	// Severity classifies an Issue.
	Severity = verify.Severity
	// Issue is a single finding produced by Verify.
	Issue = verify.Issue
	// Location identifies where in the sheet an Issue applies.
	Location = verify.Location
	// VerificationResult aggregates the issues produced by Verify.
	VerificationResult = verify.VerificationResult
	// VerifyOptions tunes the verifier's strictness.
	VerifyOptions = verify.VerifyOptions
	// ParseError is returned by Parse, ParseV1, and ParseV2 when the
	// input cannot be parsed.
	ParseError = parser.ParseError
)

const (
	// FormatUnknown is the zero value. The writer treats it as v2.
	FormatUnknown = model.FormatUnknown
	// FormatV1 is the legacy single-section CSV layout.
	FormatV1 = model.FormatV1
	// FormatV2 is the IEM sectioned layout ([Data], [Reads], [Settings]).
	FormatV2 = model.FormatV2

	// SeverityError means the sheet will not run.
	SeverityError = verify.SeverityError
	// SeverityWarning means the sheet will probably run but the
	// finding should be reviewed.
	SeverityWarning = verify.SeverityWarning
)

// Parse reads a sample sheet from r and returns the parsed model.
// The on-disk layout is auto-detected from the first non-empty
// line. Use ParseV1 or ParseV2 to force a specific format.
func Parse(r io.Reader) (*SampleSheet, error) {
	return parser.Parse(r)
}

// ParseV1 parses a v1 (legacy single-section CSV) sample sheet.
func ParseV1(r io.Reader) (*SampleSheet, error) {
	return parser.ParseV1(r)
}

// ParseV2 parses a v2 (IEM sectioned) sample sheet.
func ParseV2(r io.Reader) (*SampleSheet, error) {
	return parser.ParseV2(r)
}

// Write serialises sheet to w. The output format is chosen from
// sheet.Format. FormatUnknown is treated as v2.
func Write(w io.Writer, sheet *SampleSheet) error {
	return writer.Write(w, sheet)
}

// WriteV1 writes sheet in v1 (legacy single-CSV) format.
func WriteV1(w io.Writer, sheet *SampleSheet) error {
	return writer.WriteV1(w, sheet)
}

// WriteV2 writes sheet in v2 (IEM sectioned) format.
func WriteV2(w io.Writer, sheet *SampleSheet) error {
	return writer.WriteV2(w, sheet)
}

// Verify runs the default verification against sheet. Both errors
// and warnings are reported. Use VerifyWithOptions to suppress
// warnings or to promote them to errors.
func Verify(sheet *SampleSheet) *VerificationResult {
	return verify.Verify(sheet)
}

// VerifyWithOptions runs verification with the given options.
func VerifyWithOptions(sheet *SampleSheet, opts VerifyOptions) *VerificationResult {
	return verify.VerifyWithOptions(sheet, opts)
}
