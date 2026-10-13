package verify

import (
	"fmt"
	"strings"

	"github.com/inful/illsample/internal/model"
)

// rule is a single verification rule. applies reports whether the
// rule is meaningful for the given sheet; check performs the check
// and appends any findings to the result.
type rule struct {
	code     string
	severity Severity
	applies  func(*model.SampleSheet) bool
	check    func(*model.SampleSheet, *VerificationResult)
}

// allRules is the complete set of rules, in the order they are
// evaluated. The order is chosen so that "structural" issues (no
// data, no reads) are reported before per-row issues.
var allRules = []rule{
	// v1 rules
	v1RequireData,
	v1RequireSampleIDColumn,
	v1RequireSampleNameColumn,
	v1RequireIndexColumn,
	v1RequireSampleProjectColumn,
	v1NoEmptySampleID,
	v1NoDuplicateSampleID,
	v1ValidIndex,
	v1ConsistentIndexLength,
	v1ValidSampleName,
	v1WarnInconsistentPlateWell,
	v1WarnLongSampleName,

	// v2 rules
	v2RequireDataSection,
	v2RequireReadsSection,
	v2RequireSampleIDColumn,
	v2RequireSampleNameColumn,
	v2RequireSampleProjectColumn,
	v2RequireIndexColumn,
	v2RequireRead1Cycles,
	v2NoEmptySampleID,
	v2NoDuplicateSampleID,
	v2ValidIndex,
	v2ConsistentIndexLength,
	v2IndexMatchesReads,
	v2ValidSampleName,
	v2WarnNoAdapter,
	v2WarnInconsistentPlateWell,
	v2WarnInconsistentIndexIDs,
	v2WarnLongSampleName,
}

// ---- v1 rules -------------------------------------------------------------

var v1RequireData = rule{
	code:     "E_V1_NO_DATA",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if len(s.Data) == 0 {
			r.add(SeverityError, "E_V1_NO_DATA", "sheet has no data rows", Location{Row: -1})
		}
	},
}

var v1RequireSampleIDColumn = rule{
	code:     "E_V1_MISSING_SAMPLE_ID",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "Sample_ID") {
			r.add(SeverityError, "E_V1_MISSING_SAMPLE_ID",
				`required column "Sample_ID" is missing from the header`,
				Location{Section: "Data", Row: -1})
		}
	},
}

var v1RequireSampleNameColumn = rule{
	code:     "E_V1_MISSING_SAMPLE_NAME",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "Sample_Name") {
			r.add(SeverityError, "E_V1_MISSING_SAMPLE_NAME",
				`required column "Sample_Name" is missing from the header`,
				Location{Section: "Data", Row: -1})
		}
	},
}

var v1RequireIndexColumn = rule{
	code:     "E_V1_MISSING_INDEX",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		has1 := sheetsColumnHeaderContains(s, "index")
		has2 := sheetsColumnHeaderContains(s, "index2")
		if !has1 && !has2 {
			r.add(SeverityError, "E_V1_MISSING_INDEX",
				`at least one of "index" or "index2" must be present`,
				Location{Section: "Data", Row: -1})
		}
	},
}

var v1RequireSampleProjectColumn = rule{
	code:     "E_V1_MISSING_SAMPLE_PROJECT",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "Sample_Project") {
			r.add(SeverityError, "E_V1_MISSING_SAMPLE_PROJECT",
				`required column "Sample_Project" is missing from the header`,
				Location{Section: "Data", Row: -1})
		}
	},
}

var v1NoEmptySampleID = rule{
	code:     "E_V1_EMPTY_SAMPLE_ID",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		for i, sample := range s.Data {
			if strings.TrimSpace(sample.SampleID) == "" {
				r.add(SeverityError, "E_V1_EMPTY_SAMPLE_ID",
					"Sample_ID is empty",
					Location{Section: "Data", Row: i, Column: "Sample_ID"})
			}
		}
	},
}

var v1NoDuplicateSampleID = rule{
	code:     "E_V1_DUPLICATE_SAMPLE_ID",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		seen := make(map[string]int, len(s.Data))
		for i, sample := range s.Data {
			id := strings.TrimSpace(sample.SampleID)
			if id == "" {
				continue
			}
			if prev, ok := seen[id]; ok {
				r.add(SeverityError, "E_V1_DUPLICATE_SAMPLE_ID",
					fmt.Sprintf("Sample_ID %q is also used in row %d", id, prev),
					Location{Section: "Data", Row: i, Column: "Sample_ID"})
				continue
			}
			seen[id] = i
		}
	},
}

var v1ValidIndex = rule{
	code:     "E_V1_INVALID_INDEX",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		for i, sample := range s.Data {
			if sample.Index != "" && !isValidIndex(sample.Index) {
				r.add(SeverityError, "E_V1_INVALID_INDEX",
					fmt.Sprintf("index %q contains characters outside ACGTN", sample.Index),
					Location{Section: "Data", Row: i, Column: "index"})
			}
			if sample.Index2 != "" && !isValidIndex(sample.Index2) {
				r.add(SeverityError, "E_V1_INVALID_INDEX",
					fmt.Sprintf("index2 %q contains characters outside ACGTN", sample.Index2),
					Location{Section: "Data", Row: i, Column: "index2"})
			}
		}
	},
}

var v1ConsistentIndexLength = rule{
	code:     "E_V1_INDEX_LENGTH_MISMATCH",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
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
					r.add(SeverityError, "E_V1_INDEX_LENGTH_MISMATCH",
						fmt.Sprintf("%s length is %d but row %d has length %d", col, len(v), firstRow, firstLen),
						Location{Section: "Data", Row: i, Column: col})
				}
			}
		}
		checkColumn(func(s model.Sample) string { return s.Index }, "index")
		checkColumn(func(s model.Sample) string { return s.Index2 }, "index2")
	},
}

var v1ValidSampleName = rule{
	code:     "E_V1_INVALID_SAMPLE_NAME",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		for i, sample := range s.Data {
			if sample.SampleName == "" {
				continue
			}
			if hasInvalidNameChars(sample.SampleName) {
				r.add(SeverityError, "E_V1_INVALID_SAMPLE_NAME",
					fmt.Sprintf("Sample_Name %q contains illegal characters", sample.SampleName),
					Location{Section: "Data", Row: i, Column: "Sample_Name"})
			}
		}
	},
}

var v1WarnInconsistentPlateWell = rule{
	code:     "W_V1_INCONSISTENT_PLATE_WELL",
	severity: SeverityWarning,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
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
				r.add(SeverityWarning, "W_V1_INCONSISTENT_PLATE_WELL",
					"some rows have Sample_Plate/Sample_Well while others do not",
					Location{Section: "Data", Row: -1})
				return
			}
		}
	},
}

var v1WarnLongSampleName = rule{
	code:     "W_V1_LONG_SAMPLE_NAME",
	severity: SeverityWarning,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV1 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		for i, sample := range s.Data {
			if len(sample.SampleName) > 100 {
				r.add(SeverityWarning, "W_V1_LONG_SAMPLE_NAME",
					fmt.Sprintf("Sample_Name has %d characters; long names can cause filesystem issues", len(sample.SampleName)),
					Location{Section: "Data", Row: i, Column: "Sample_Name"})
			}
		}
	},
}

// ---- v2 rules -------------------------------------------------------------

var v2RequireDataSection = rule{
	code:     "E_V2_NO_DATA",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		// A v2 sheet that parses but produces no rows is empty.
		// The parser allows empty [Data]; we flag it as an error.
		if len(s.Data) == 0 {
			r.add(SeverityError, "E_V2_NO_DATA",
				"[Data] section has no rows",
				Location{Section: "Data", Row: -1})
		}
	},
}

var v2RequireReadsSection = rule{
	code:     "E_V2_NO_READS",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if s.Reads == nil {
			r.add(SeverityError, "E_V2_NO_READS",
				"[Reads] section is missing",
				Location{Section: "Reads", Row: -1})
		}
	},
}

var v2RequireSampleIDColumn = rule{
	code:     "E_V2_MISSING_SAMPLE_ID",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "Sample_ID") {
			r.add(SeverityError, "E_V2_MISSING_SAMPLE_ID",
				`required column "Sample_ID" is missing from the [Data] section`,
				Location{Section: "Data", Row: -1, Column: "Sample_ID"})
		}
	},
}

var v2RequireSampleNameColumn = rule{
	code:     "E_V2_MISSING_SAMPLE_NAME",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "Sample_Name") {
			r.add(SeverityError, "E_V2_MISSING_SAMPLE_NAME",
				`required column "Sample_Name" is missing from the [Data] section`,
				Location{Section: "Data", Row: -1, Column: "Sample_Name"})
		}
	},
}

var v2RequireSampleProjectColumn = rule{
	code:     "E_V2_MISSING_SAMPLE_PROJECT",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "Sample_Project") {
			r.add(SeverityError, "E_V2_MISSING_SAMPLE_PROJECT",
				`required column "Sample_Project" is missing from the [Data] section`,
				Location{Section: "Data", Row: -1, Column: "Sample_Project"})
		}
	},
}

var v2RequireIndexColumn = rule{
	code:     "E_V2_MISSING_INDEX",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if !sheetsColumnHeaderContains(s, "index") {
			r.add(SeverityError, "E_V2_MISSING_INDEX",
				`required column "index" is missing from the [Data] section`,
				Location{Section: "Data", Row: -1, Column: "index"})
		}
	},
}

var v2RequireRead1Cycles = rule{
	code:     "E_V2_READ1_MISSING",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if s.Reads == nil {
			// Already reported by v2RequireReadsSection.
			return
		}
		if s.Reads.Read1Cycles <= 0 {
			r.add(SeverityError, "E_V2_READ1_MISSING",
				"[Reads] Read1Cycles is required",
				Location{Section: "Reads", Row: -1, Column: "Read1Cycles"})
		}
	},
}

var v2NoEmptySampleID = rule{
	code:     "E_V2_EMPTY_SAMPLE_ID",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		for i, sample := range s.Data {
			if strings.TrimSpace(sample.SampleID) == "" {
				r.add(SeverityError, "E_V2_EMPTY_SAMPLE_ID",
					"Sample_ID is empty",
					Location{Section: "Data", Row: i, Column: "Sample_ID"})
			}
		}
	},
}

var v2NoDuplicateSampleID = rule{
	code:     "E_V2_DUPLICATE_SAMPLE_ID",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		seen := make(map[string]int, len(s.Data))
		for i, sample := range s.Data {
			id := strings.TrimSpace(sample.SampleID)
			if id == "" {
				continue
			}
			if prev, ok := seen[id]; ok {
				r.add(SeverityError, "E_V2_DUPLICATE_SAMPLE_ID",
					fmt.Sprintf("Sample_ID %q is also used in row %d", id, prev),
					Location{Section: "Data", Row: i, Column: "Sample_ID"})
				continue
			}
			seen[id] = i
		}
	},
}

var v2ValidIndex = rule{
	code:     "E_V2_INVALID_INDEX",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		for i, sample := range s.Data {
			if sample.Index != "" && !isValidIndex(sample.Index) {
				r.add(SeverityError, "E_V2_INVALID_INDEX",
					fmt.Sprintf("index %q contains characters outside ACGTN", sample.Index),
					Location{Section: "Data", Row: i, Column: "index"})
			}
			if sample.Index2 != "" && !isValidIndex(sample.Index2) {
				r.add(SeverityError, "E_V2_INVALID_INDEX",
					fmt.Sprintf("index2 %q contains characters outside ACGTN", sample.Index2),
					Location{Section: "Data", Row: i, Column: "index2"})
			}
		}
	},
}

var v2ConsistentIndexLength = rule{
	code:     "E_V2_INDEX_LENGTH_MISMATCH",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
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
					r.add(SeverityError, "E_V2_INDEX_LENGTH_MISMATCH",
						fmt.Sprintf("%s length is %d but row %d has length %d", col, len(v), firstRow, firstLen),
						Location{Section: "Data", Row: i, Column: col})
				}
			}
		}
		checkColumn(func(s model.Sample) string { return s.Index }, "index")
		checkColumn(func(s model.Sample) string { return s.Index2 }, "index2")
	},
}

var v2IndexMatchesReads = rule{
	code:     "E_V2_INDEX_VS_READS",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
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
					r.add(SeverityError, "E_V2_INDEX_VS_READS",
						fmt.Sprintf("%s length %d does not match [Reads] %s = %d", col, len(v), col, declared),
						Location{Section: "Data", Row: i, Column: col})
				}
			}
		}
		checkColumn(func(s model.Sample) string { return s.Index }, s.Reads.Index1Cycles, "index")
		checkColumn(func(s model.Sample) string { return s.Index2 }, s.Reads.Index2Cycles, "index2")
	},
}

var v2ValidSampleName = rule{
	code:     "E_V2_INVALID_SAMPLE_NAME",
	severity: SeverityError,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		for i, sample := range s.Data {
			if sample.SampleName == "" {
				continue
			}
			if hasInvalidNameChars(sample.SampleName) {
				r.add(SeverityError, "E_V2_INVALID_SAMPLE_NAME",
					fmt.Sprintf("Sample_Name %q contains illegal characters", sample.SampleName),
					Location{Section: "Data", Row: i, Column: "Sample_Name"})
			}
		}
	},
}

var v2WarnNoAdapter = rule{
	code:     "W_V2_NO_ADAPTER",
	severity: SeverityWarning,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		if s.Settings == nil {
			r.add(SeverityWarning, "W_V2_NO_ADAPTER",
				"[Settings] section is absent; consider specifying an Adapter sequence",
				Location{Section: "Settings", Row: -1})
			return
		}
		if s.Settings.Adapter == "" {
			r.add(SeverityWarning, "W_V2_NO_ADAPTER",
				"Adapter is empty in [Settings]; consider specifying one",
				Location{Section: "Settings", Row: -1, Column: "Adapter"})
		}
	},
}

var v2WarnInconsistentPlateWell = rule{
	code:     "W_V2_INCONSISTENT_PLATE_WELL",
	severity: SeverityWarning,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
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
				r.add(SeverityWarning, "W_V2_INCONSISTENT_PLATE_WELL",
					"some rows have Sample_Plate/Sample_Well while others do not",
					Location{Section: "Data", Row: -1})
				return
			}
		}
	},
}

var v2WarnInconsistentIndexIDs = rule{
	code:     "W_V2_INCONSISTENT_INDEX_IDS",
	severity: SeverityWarning,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		// If I5_Index_ID column exists but index2 is empty on at
		// least one row, warn (and vice versa).
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
			r.add(SeverityWarning, "W_V2_INCONSISTENT_INDEX_IDS",
				"some rows have I7_Index_ID while others do not",
				Location{Section: "Data", Row: -1, Column: "I7_Index_ID"})
		}
		if anyI5Filled && anyI5Empty {
			r.add(SeverityWarning, "W_V2_INCONSISTENT_INDEX_IDS",
				"some rows have I5_Index_ID/index2 while others do not",
				Location{Section: "Data", Row: -1, Column: "I5_Index_ID"})
		}
	},
}

var v2WarnLongSampleName = rule{
	code:     "W_V2_LONG_SAMPLE_NAME",
	severity: SeverityWarning,
	applies:  func(s *model.SampleSheet) bool { return s.Format == model.FormatV2 },
	check: func(s *model.SampleSheet, r *VerificationResult) {
		for i, sample := range s.Data {
			if len(sample.SampleName) > 100 {
				r.add(SeverityWarning, "W_V2_LONG_SAMPLE_NAME",
					fmt.Sprintf("Sample_Name has %d characters; long names can cause filesystem issues", len(sample.SampleName)),
					Location{Section: "Data", Row: i, Column: "Sample_Name"})
			}
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
