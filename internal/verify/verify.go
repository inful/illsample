// Package verify runs validation rules against a model.SampleSheet.
//
// Two severities of issue are reported: Errors (findings that will
// prevent bcl2fastq or DRAGEN from succeeding) and Warnings
// (best-practice violations that are likely to cause problems
// downstream but may be intentional). The default Verify function
// reports both; callers that want to gate on warnings can promote
// them with VerifyOptions.PromoteWarningsToErrors.
package verify

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/inful/illsample/internal/model"
)

// Severity classifies an Issue. A sheet with any Error-severity
// issues is not safe to feed to a sequencer.
type Severity int

const (
	// SeverityError means the sheet will not run. The sequencer
	// (bcl2fastq, DRAGEN, BaseSpace) will reject it.
	SeverityError Severity = iota
	// SeverityWarning means the sheet will probably run but the
	// finding should be reviewed.
	SeverityWarning
)

// String returns the canonical lower-case name of the severity.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "unknown"
	}
}

// Issue is a single finding produced by the verifier.
type Issue struct {
	Severity Severity
	Code     string
	Message  string
	Location Location
}

// Location identifies where in the sheet an issue applies.
//
// The zero value (no Section, no Row, no Column) refers to the
// sheet as a whole. Set Section to refer to a particular section.
// Set Row to a 0-based index to refer to a single row; the row
// index is only displayed when Section is also set.
type Location struct {
	Section string
	Row     int
	Column  string
}

// String returns a human-readable location string like
// "Data[2].Sample_ID" or "Reads" or "Sheet".
func (l Location) String() string {
	var b strings.Builder
	switch l.Section {
	case "":
		b.WriteString("Sheet")
	case "Data", "Reads", "Settings", "Manifests":
		b.WriteString(l.Section)
	default:
		b.WriteString(l.Section)
	}
	if l.Section != "" && l.Row >= 0 {
		fmt.Fprintf(&b, "[%d]", l.Row)
	}
	if l.Column != "" {
		b.WriteString(".")
		b.WriteString(l.Column)
	}
	return b.String()
}

// VerificationResult aggregates the issues produced by Verify.
type VerificationResult struct {
	// Issues is the full list of findings, in the order they were
	// produced. The order is deterministic so test assertions on the
	// slice are stable.
	Issues []Issue
	// promoteWarnings, when true, causes Warnings to be reported
	// alongside Errors in the Errors() slice and HasErrors() to
	// return true if any warnings exist. The Severity field of each
	// Issue is preserved.
	promoteWarnings bool
}

// Errors returns the Error-severity issues, and, when warnings have
// been promoted, the Warning-severity issues as well.
func (r *VerificationResult) Errors() []Issue {
	var out []Issue
	for _, iss := range r.Issues {
		if iss.Severity == SeverityError {
			out = append(out, iss)
			continue
		}
		if r.promoteWarnings && iss.Severity == SeverityWarning {
			out = append(out, iss)
		}
	}
	return out
}

// Warnings returns only the Warning-severity issues.
func (r *VerificationResult) Warnings() []Issue {
	var out []Issue
	for _, iss := range r.Issues {
		if iss.Severity == SeverityWarning {
			out = append(out, iss)
		}
	}
	return out
}

// HasErrors reports whether any Error-severity issues are present
// (or, with promotion, any Warning-severity issues).
func (r *VerificationResult) HasErrors() bool {
	for _, iss := range r.Issues {
		if iss.Severity == SeverityError {
			return true
		}
		if r.promoteWarnings && iss.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// HasWarnings reports whether any Warning-severity issues are present.
func (r *VerificationResult) HasWarnings() bool {
	for _, iss := range r.Issues {
		if iss.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// OK reports whether the result has no issues of any kind. A nil
// VerificationResult is treated as OK.
func (r *VerificationResult) OK() bool {
	return r == nil || len(r.Issues) == 0
}

// VerifyOptions tunes the verifier's strictness.
//
// The stuttering name (verify.VerifyOptions) is intentional: when
// re-exported from the top-level package as illsample.VerifyOptions
// it does not stutter, and the same applies to its sibling types.
//
//nolint:revive // stutter is intentional; the public alias is illsample.VerifyOptions
type VerifyOptions struct {
	// PromoteWarningsToErrors causes warning-severity findings to be
	// reported in the Errors() slice (and to flip HasErrors() to
	// true). The Severity field of each Issue is preserved.
	PromoteWarningsToErrors bool
	// SuppressWarnings drops warning-severity findings from the
	// result entirely. Takes precedence over PromoteWarningsToErrors
	// when both are set.
	SuppressWarnings bool
}

// Verify runs the default verification against sheet.
func Verify(sheet *model.SampleSheet) *VerificationResult {
	return VerifyWithOptions(sheet, VerifyOptions{})
}

// VerifyWithOptions runs verification with the given options. The
// rules applied are the same as Verify; only the presentation of
// the result changes.
//
//nolint:revive // stutter is intentional; the public alias is illsample.VerifyWithOptions
func VerifyWithOptions(sheet *model.SampleSheet, opts VerifyOptions) *VerificationResult {
	if sheet == nil {
		return &VerificationResult{Issues: []Issue{{
			Severity: SeverityError,
			Code:     "E_NIL_SHEET",
			Message:  "sheet is nil",
		}}}
	}
	r := &VerificationResult{promoteWarnings: opts.PromoteWarningsToErrors}
	for _, rule := range allRules {
		if !rule.applies(sheet) {
			continue
		}
		rule.check(sheet, r)
	}
	if opts.SuppressWarnings {
		r.Issues = filterOut(r.Issues, func(i Issue) bool { return i.Severity == SeverityWarning })
	}
	return r
}

// filterOut returns a new slice with elements for which keep returns
// false removed. The order of retained elements is preserved.
func filterOut(in []Issue, drop func(Issue) bool) []Issue {
	out := make([]Issue, 0, len(in))
	for _, i := range in {
		if !drop(i) {
			out = append(out, i)
		}
	}
	return out
}

// add is a convenience for rule implementations to append an issue.
func (r *VerificationResult) add(severity Severity, code, message string, loc Location) {
	r.Issues = append(r.Issues, Issue{
		Severity: severity,
		Code:     code,
		Message:  message,
		Location: loc,
	})
}

// isValidIndex reports whether s consists only of A/C/G/T/N (and is
// non-empty). Illumina indices are DNA sequences; some kits include
// N as a wildcard. Lower-case is also accepted because IEM sometimes
// emits lower-case indices.
func isValidIndex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch unicode.ToUpper(r) {
		case 'A', 'C', 'G', 'T', 'N':
			// ok
		default:
			return false
		}
	}
	return true
}

// hasInvalidNameChars reports whether s contains any character that
// bcl2fastq will refuse. The actual restriction is undocumented but
// empirically includes whitespace, '/', '\\', and characters outside
// printable ASCII.
func hasInvalidNameChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return true
		}
		if r == ' ' || r == '\t' {
			return true
		}
	}
	return false
}
