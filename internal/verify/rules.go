package verify

import (
	"fmt"
	"strings"

	"github.com/inful/illsample/internal/model"
)

// rule is a single verification rule.
//
// Adding a rule is a single struct literal in the allRules slice. The
// rule declares the format versions it applies to (formats), the
// severity of any findings it produces (severity), and the trailing
// segment of the public code (suffix). The full public code is built
// at run time from severity + sheet format + suffix, so a rule that
// applies to both v1 and v2 emits "E_V1_<suffix>" or "E_V2_<suffix>"
// as appropriate without duplicating the check function.
//
// check is invoked once per matching sheet, with the matching format
// and the pre-computed code string passed in. The closure may branch
// on format to choose format-specific message text or Location
// fields; the code is always the correct one for the format so it
// can be used verbatim.
type rule struct {
	suffix   string
	severity Severity
	formats  []model.Format
	check    func(format model.Format, sheet *model.SampleSheet, code string, r *VerificationResult)
}

// issueCode builds the public code for a finding.
//
// Examples:
//
//	issueCode(SeverityError,   model.FormatV1, "INVALID_INDEX") -> "E_V1_INVALID_INDEX"
//	issueCode(SeverityWarning, model.FormatV2, "LONG_SAMPLE_NAME") -> "W_V2_LONG_SAMPLE_NAME"
//
// Unknown severities or formats degrade to "?" segments. Both cases
// are unreachable in practice because allRules only declares rules
// against known severities and formats, but the helper degrades
// gracefully rather than panicking if a new severity or format is
// added without updating the helper.
func issueCode(sev Severity, format model.Format, suffix string) string {
	var sevChar string
	switch sev {
	case SeverityError:
		sevChar = "E"
	case SeverityWarning:
		sevChar = "W"
	}
	var ver int
	switch format {
	case model.FormatV1:
		ver = 1
	case model.FormatV2:
		ver = 2
	}
	return fmt.Sprintf("%s_V%d_%s", sevChar, ver, suffix)
}

// allRules is the complete set of rules, in the order they are
// evaluated. The order is chosen so that "structural" issues (no
// data, no reads, missing required columns) are reported before
// per-row issues, and warnings are reported after errors.
//
// Rules that apply to both v1 and v2 are listed first within each
// tier so the firing order is consistent across formats. v2-only
// rules are interleaved at the natural position (e.g. NO_READS right
// after NO_DATA, INDEX_VS_READS after INDEX_LENGTH_MISMATCH).
var allRules = []rule{
	// Structural: presence of required sections / columns.
	requireData,
	requireReadsSection, // v2 only
	requireSampleIDColumn,
	requireSampleNameColumn,
	requireSampleProjectColumn,
	requireIndexColumn,
	requireRead1Cycles, // v2 only

	// Per-row: row-level integrity.
	noEmptySampleID,
	noDuplicateSampleID,
	validIndex,
	consistentIndexLength,
	indexMatchesReads, // v2 only
	validSampleName,

	// Warnings.
	warnNoAdapter, // v2 only
	warnInconsistentPlateWell,
	warnInconsistentIndexIDs, // v2 only
	warnLongSampleName,
}

// ---- shared rules (both v1 and v2) ----------------------------------------

// requireData: the sheet must contain at least one data row. The
// message and Location differ slightly between v1 and v2 (v1 talks
// about "the sheet", v2 talks about "[Data]") to match the existing
// public messages.
var requireData = rule{
	suffix:   "NO_DATA",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		if len(s.Data) == 0 {
			switch format {
			case model.FormatV1:
				r.add(SeverityError, code, "sheet has no data rows",
					Location{Row: -1})
			case model.FormatV2:
				r.add(SeverityError, code, "[Data] section has no rows",
					Location{Section: "Data", Row: -1})
			}
		}
	},
}

var requireSampleIDColumn = rule{
	suffix:   "MISSING_SAMPLE_ID",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "Sample_ID") {
			switch format {
			case model.FormatV1:
				r.add(SeverityError, code,
					`required column "Sample_ID" is missing from the header`,
					Location{Section: "Data", Row: -1})
			case model.FormatV2:
				r.add(SeverityError, code,
					`required column "Sample_ID" is missing from the [Data] section`,
					Location{Section: "Data", Row: -1, Column: "Sample_ID"})
			}
		}
	},
}

var requireSampleNameColumn = rule{
	suffix:   "MISSING_SAMPLE_NAME",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "Sample_Name") {
			switch format {
			case model.FormatV1:
				r.add(SeverityError, code,
					`required column "Sample_Name" is missing from the header`,
					Location{Section: "Data", Row: -1})
			case model.FormatV2:
				r.add(SeverityError, code,
					`required column "Sample_Name" is missing from the [Data] section`,
					Location{Section: "Data", Row: -1, Column: "Sample_Name"})
			}
		}
	},
}

var requireSampleProjectColumn = rule{
	suffix:   "MISSING_SAMPLE_PROJECT",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "Sample_Project") {
			switch format {
			case model.FormatV1:
				r.add(SeverityError, code,
					`required column "Sample_Project" is missing from the header`,
					Location{Section: "Data", Row: -1})
			case model.FormatV2:
				r.add(SeverityError, code,
					`required column "Sample_Project" is missing from the [Data] section`,
					Location{Section: "Data", Row: -1, Column: "Sample_Project"})
			}
		}
	},
}

// requireIndexColumn: v1 allows index OR index2, v2 requires index.
// The semantics differ, so the rule branches on format.
var requireIndexColumn = rule{
	suffix:   "MISSING_INDEX",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		switch format {
		case model.FormatV1:
			has1 := sheetsColumnHeaderContains(s, "index")
			has2 := sheetsColumnHeaderContains(s, "index2")
			if !has1 && !has2 {
				r.add(SeverityError, code,
					`at least one of "index" or "index2" must be present`,
					Location{Section: "Data", Row: -1})
			}
		case model.FormatV2:
			if !sheetsColumnHeaderContains(s, "index") {
				r.add(SeverityError, code,
					`required column "index" is missing from the [Data] section`,
					Location{Section: "Data", Row: -1, Column: "index"})
			}
		}
	},
}

var noEmptySampleID = rule{
	suffix:   "EMPTY_SAMPLE_ID",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		for i, sample := range s.Data {
			if strings.TrimSpace(sample.SampleID) == "" {
				r.add(SeverityError, code, "Sample_ID is empty",
					Location{Section: "Data", Row: i, Column: "Sample_ID"})
			}
		}
	},
}

var noDuplicateSampleID = rule{
	suffix:   "DUPLICATE_SAMPLE_ID",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		seen := make(map[string]int, len(s.Data))
		for i, sample := range s.Data {
			id := strings.TrimSpace(sample.SampleID)
			if id == "" {
				continue
			}
			if prev, ok := seen[id]; ok {
				r.add(SeverityError, code,
					fmt.Sprintf("Sample_ID %q is also used in row %d", id, prev),
					Location{Section: "Data", Row: i, Column: "Sample_ID"})
				continue
			}
			seen[id] = i
		}
	},
}

var validIndex = rule{
	suffix:   "INVALID_INDEX",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		for i, sample := range s.Data {
			if sample.Index != "" && !isValidIndex(sample.Index) {
				r.add(SeverityError, code,
					fmt.Sprintf("index %q contains characters outside ACGTN", sample.Index),
					Location{Section: "Data", Row: i, Column: "index"})
			}
			if sample.Index2 != "" && !isValidIndex(sample.Index2) {
				r.add(SeverityError, code,
					fmt.Sprintf("index2 %q contains characters outside ACGTN", sample.Index2),
					Location{Section: "Data", Row: i, Column: "index2"})
			}
		}
	},
}

var consistentIndexLength = rule{
	suffix:   "INDEX_LENGTH_MISMATCH",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		checkColumn := func(get func(model.Sample) string, col string) {
			firstLen := -1
			firstRow := -1
			for i, sample := range s.Data {
				v := get(sample)
				if v == "" {
					continue
				}
				if firstLen == -1 {
					firstLen = len(v)
					firstRow = i
					continue
				}
				if len(v) != firstLen {
					r.add(SeverityError, code,
						fmt.Sprintf("%s length is %d but row %d has length %d", col, len(v), firstRow, firstLen),
						Location{Section: "Data", Row: i, Column: col})
				}
			}
		}
		checkColumn(func(s model.Sample) string { return s.Index }, "index")
		checkColumn(func(s model.Sample) string { return s.Index2 }, "index2")
	},
}

var validSampleName = rule{
	suffix:   "INVALID_SAMPLE_NAME",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		for i, sample := range s.Data {
			if sample.SampleName == "" {
				continue
			}
			if hasInvalidNameChars(sample.SampleName) {
				r.add(SeverityError, code,
					fmt.Sprintf("Sample_Name %q contains illegal characters", sample.SampleName),
					Location{Section: "Data", Row: i, Column: "Sample_Name"})
			}
		}
	},
}

var warnInconsistentPlateWell = rule{
	suffix:   "INCONSISTENT_PLATE_WELL",
	severity: SeverityWarning,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		if len(s.Data) < 2 {
			return
		}
		anyPlate := false
		anyMissing := false
		for _, sample := range s.Data {
			if sample.SamplePlate != "" || sample.SampleWell != "" {
				anyPlate = true
			} else {
				anyMissing = true
			}
			if anyPlate && anyMissing {
				r.add(SeverityWarning, code,
					"some rows have Sample_Plate/Sample_Well while others do not",
					Location{Section: "Data", Row: -1})
				return
			}
		}
	},
}

var warnLongSampleName = rule{
	suffix:   "LONG_SAMPLE_NAME",
	severity: SeverityWarning,
	formats:  []model.Format{model.FormatV1, model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		for i, sample := range s.Data {
			if len(sample.SampleName) > 100 {
				r.add(SeverityWarning, code,
					fmt.Sprintf("Sample_Name has %d characters; long names can cause filesystem issues", len(sample.SampleName)),
					Location{Section: "Data", Row: i, Column: "Sample_Name"})
			}
		}
	},
}

// ---- v2-only rules --------------------------------------------------------

var requireReadsSection = rule{
	suffix:   "NO_READS",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		if s.Reads == nil {
			r.add(SeverityError, code, "[Reads] section is missing",
				Location{Section: "Reads", Row: -1})
		}
	},
}

var requireRead1Cycles = rule{
	suffix:   "READ1_MISSING",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		if s.Reads == nil {
			// Already reported by requireReadsSection.
			return
		}
		if s.Reads.Read1Cycles <= 0 {
			r.add(SeverityError, code, "[Reads] Read1Cycles is required",
				Location{Section: "Reads", Row: -1, Column: "Read1Cycles"})
		}
	},
}

var indexMatchesReads = rule{
	suffix:   "INDEX_VS_READS",
	severity: SeverityError,
	formats:  []model.Format{model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		if s.Reads == nil {
			return
		}
		checkColumn := func(get func(model.Sample) string, declared int, col string) {
			if declared == 0 {
				return
			}
			for i, sample := range s.Data {
				v := get(sample)
				if v == "" {
					continue
				}
				if len(v) != declared {
					r.add(SeverityError, code,
						fmt.Sprintf("%s length %d does not match [Reads] %s = %d", col, len(v), col, declared),
						Location{Section: "Data", Row: i, Column: col})
				}
			}
		}
		checkColumn(func(s model.Sample) string { return s.Index }, s.Reads.Index1Cycles, "index")
		checkColumn(func(s model.Sample) string { return s.Index2 }, s.Reads.Index2Cycles, "index2")
	},
}

var warnNoAdapter = rule{
	suffix:   "NO_ADAPTER",
	severity: SeverityWarning,
	formats:  []model.Format{model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		if s.Settings == nil {
			r.add(SeverityWarning, code,
				"[Settings] section is absent; consider specifying an Adapter sequence",
				Location{Section: "Settings", Row: -1})
			return
		}
		if s.Settings.Adapter == "" {
			r.add(SeverityWarning, code,
				"Adapter is empty in [Settings]; consider specifying one",
				Location{Section: "Settings", Row: -1, Column: "Adapter"})
		}
	},
}

var warnInconsistentIndexIDs = rule{
	suffix:   "INCONSISTENT_INDEX_IDS",
	severity: SeverityWarning,
	formats:  []model.Format{model.FormatV2},
	check: func(format model.Format, s *model.SampleSheet, code string, r *VerificationResult) {
		hasI5Col := sheetsColumnHeaderContains(s, "I5_Index_ID")
		hasI7Col := sheetsColumnHeaderContains(s, "I7_Index_ID")
		if !hasI5Col || !hasI7Col {
			return
		}
		anyI7Empty := false
		anyI5Empty := false
		anyI7Filled := false
		anyI5Filled := false
		for _, sample := range s.Data {
			if sample.I7IndexID == "" {
				anyI7Empty = true
			} else {
				anyI7Filled = true
			}
			if sample.I5IndexID == "" || sample.Index2 == "" {
				anyI5Empty = true
			} else {
				anyI5Filled = true
			}
		}
		if anyI7Filled && anyI7Empty {
			r.add(SeverityWarning, code,
				"some rows have I7_Index_ID while others do not",
				Location{Section: "Data", Row: -1, Column: "I7_Index_ID"})
		}
		if anyI5Filled && anyI5Empty {
			r.add(SeverityWarning, code,
				"some rows have I5_Index_ID/index2 while others do not",
				Location{Section: "Data", Row: -1, Column: "I5_Index_ID"})
		}
	},
}

// ---- helpers --------------------------------------------------------------

// sheetsColumnHeaderContains reports whether the sheet's data header
// contains the given column name. The check is case-insensitive.
func sheetsColumnHeaderContains(s *model.SampleSheet, name string) bool {
	want := strings.ToLower(name)
	for _, h := range s.Header {
		if strings.ToLower(h) == want {
			return true
		}
	}
	return false
}
