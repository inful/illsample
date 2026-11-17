package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inful/illsample/internal/model"
	"github.com/inful/illsample/internal/parser"
)

// readFixture reads a file under the module's testdata directory. Every
// package reads the same fixtures rather than a copy of them.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

// codes returns the set of issue codes present in r.
func codes(r *VerificationResult) map[string]int {
	out := make(map[string]int)
	for _, iss := range r.Issues {
		out[iss.Code]++
	}
	return out
}

func TestVerify_NilSheet(t *testing.T) {
	r := Verify(nil)
	if !r.HasErrors() {
		t.Fatal("nil sheet should produce an error")
	}
	if got := r.Issues[0].Code; got != "E_NIL_SHEET" {
		t.Errorf("code = %q, want E_NIL_SHEET", got)
	}
}

func TestVerify_ValidV1(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "v1_single_index.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	if r.HasErrors() {
		t.Errorf("expected no errors, got %d: %+v", len(r.Errors()), r.Errors())
	}
}

func TestVerify_ValidV2(t *testing.T) {
	for _, fixture := range []string{"v2_single_index.csv", "v2_dual_index.csv", "v2_empty_manifests.csv"} {
		t.Run(fixture, func(t *testing.T) {
			sheet, err := parser.Parse(strings.NewReader(readFixture(t, fixture)))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			r := Verify(sheet)
			if r.HasErrors() {
				t.Errorf("expected no errors, got %d: %+v", len(r.Errors()), r.Errors())
			}
		})
	}
}

func TestVerify_InvalidV1NoData(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "invalid_v1_no_data.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V1_NO_DATA"] == 0 {
		t.Errorf("expected E_V1_NO_DATA, got codes: %v", c)
	}
}

func TestVerify_InvalidV1DuplicateIDs(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "invalid_v1_duplicate_ids.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V1_DUPLICATE_SAMPLE_ID"] == 0 {
		t.Errorf("expected E_V1_DUPLICATE_SAMPLE_ID, got codes: %v", c)
	}
}

func TestVerify_InvalidV1BadIndex(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "invalid_v1_bad_index.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V1_INVALID_INDEX"] == 0 {
		t.Errorf("expected E_V1_INVALID_INDEX, got codes: %v", c)
	}
}

func TestVerify_InvalidV1NoIndexColumn(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "invalid_v1_no_index_column.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V1_MISSING_INDEX"] == 0 {
		t.Errorf("expected E_V1_MISSING_INDEX, got codes: %v", c)
	}
}

func TestVerify_InvalidV1IndexLengthMismatch(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "invalid_v1_index_length_mismatch.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V1_INDEX_LENGTH_MISMATCH"] == 0 {
		t.Errorf("expected E_V1_INDEX_LENGTH_MISMATCH, got codes: %v", c)
	}
}

func TestVerify_InvalidV2NoReads(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "invalid_v2_no_reads.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V2_NO_READS"] == 0 {
		t.Errorf("expected E_V2_NO_READS, got codes: %v", c)
	}
}

func TestVerify_InvalidV2IndexLengthMismatch(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "invalid_v2_index_length_mismatch.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V2_INDEX_LENGTH_MISMATCH"] == 0 {
		t.Errorf("expected E_V2_INDEX_LENGTH_MISMATCH, got codes: %v", c)
	}
}

func TestVerify_InvalidV2ReadsMismatch(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "invalid_v2_reads_mismatch.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V2_INDEX_VS_READS"] == 0 {
		t.Errorf("expected E_V2_INDEX_VS_READS, got codes: %v", c)
	}
}

func TestVerify_InvalidV2NoSampleID(t *testing.T) {
	sheet, err := parser.Parse(strings.NewReader(readFixture(t, "invalid_v2_no_sample_id.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V2_MISSING_SAMPLE_ID"] == 0 {
		t.Errorf("expected E_V2_MISSING_SAMPLE_ID, got codes: %v", c)
	}
}

func TestVerify_V2OptionalColumnsAreWarnings(t *testing.T) {
	// Illumina's v2 spec requires only Sample_ID and index, so the
	// optional Sample_Name/Sample_Project columns must not fail the
	// sheet.
	sheet, err := parser.Parse(strings.NewReader(
		"[Header]\nFileFormatVersion,2\n[Data]\nSample_ID,index\nid1,ATCGATCG\n[Reads]\nRead1Cycles,151\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	if r.HasErrors() {
		t.Errorf("expected no errors, got %d: %+v", len(r.Errors()), r.Errors())
	}
	c := codes(r)
	if c["W_V2_MISSING_SAMPLE_NAME"] == 0 {
		t.Errorf("expected W_V2_MISSING_SAMPLE_NAME, got codes: %v", c)
	}
	if c["W_V2_MISSING_SAMPLE_PROJECT"] == 0 {
		t.Errorf("expected W_V2_MISSING_SAMPLE_PROJECT, got codes: %v", c)
	}
}

func TestVerify_V2AdapterRead1CountsAsAdapter(t *testing.T) {
	// Illumina names the Read 1 adapter "AdapterRead1"; a sheet that
	// uses it must not be warned about a missing adapter.
	sheet, err := parser.Parse(strings.NewReader(
		"[Header]\nFileFormatVersion,2\n" +
			"[Data]\nSample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCGATCG,P1\n" +
			"[Reads]\nRead1Cycles,151\n" +
			"[BCLConvert_Settings]\nAdapterRead1,AGATCG\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := Verify(sheet)
	if r.HasErrors() {
		t.Errorf("expected no errors, got %d: %+v", len(r.Errors()), r.Errors())
	}
	if c := codes(r); c["W_V2_NO_ADAPTER"] != 0 {
		t.Errorf("unexpected W_V2_NO_ADAPTER, got codes: %v", c)
	}
}

func TestVerify_PromoteWarningsToErrors(t *testing.T) {
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Header:  map[string]string{"FileFormatVersion": "2"},
		Columns: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
		Reads:   &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
		// No Settings -> triggers W_V2_NO_ADAPTER
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG", SampleProject: "P1"},
		},
	}
	defaultResult := Verify(sheet)
	if defaultResult.HasErrors() {
		t.Fatalf("default Verify has errors, expected none: %+v", defaultResult.Errors())
	}
	if !defaultResult.HasWarnings() {
		t.Fatal("expected W_V2_NO_ADAPTER warning")
	}
	strict := VerifyWithOptions(sheet, VerifyOptions{PromoteWarningsToErrors: true})
	if !strict.HasErrors() {
		t.Error("PromoteWarningsToErrors: expected errors, got none")
	}
	if !strict.HasWarnings() {
		t.Error("PromoteWarningsToErrors: expected warnings still present")
	}
}

func TestVerify_SuppressWarnings(t *testing.T) {
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
		Reads:   &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG", SampleProject: "P1"},
		},
	}
	r := VerifyWithOptions(sheet, VerifyOptions{SuppressWarnings: true})
	if r.HasWarnings() {
		t.Errorf("SuppressWarnings: warnings present: %+v", r.Warnings())
	}
}

func TestVerify_LocationString(t *testing.T) {
	tests := []struct {
		loc  Location
		want string
	}{
		{Location{}, "Sheet"},
		{Location{Section: "Data", Row: -1}, "Data"},
		{Location{Section: "Data", Row: 2}, "Data[2]"},
		{Location{Section: "Data", Row: 2, Column: "Sample_ID"}, "Data[2].Sample_ID"},
		{Location{Section: "Reads", Row: -1, Column: "Read1Cycles"}, "Reads.Read1Cycles"},
		{Location{Section: "Custom", Row: -1}, "Custom"},
	}
	for _, tt := range tests {
		if got := tt.loc.String(); got != tt.want {
			t.Errorf("Location%+v.String() = %q, want %q", tt.loc, got, tt.want)
		}
	}
}

func TestVerify_SeverityString(t *testing.T) {
	if got := SeverityError.String(); got != "error" {
		t.Errorf("SeverityError.String() = %q, want error", got)
	}
	if got := SeverityWarning.String(); got != "warning" {
		t.Errorf("SeverityWarning.String() = %q, want warning", got)
	}
	if got := Severity(99).String(); got != "unknown" {
		t.Errorf("Severity(99).String() = %q, want unknown", got)
	}
}

func TestVerify_EmptyResultIsOK(t *testing.T) {
	var r *VerificationResult
	if !r.OK() {
		t.Error("nil result should be OK")
	}
	r2 := &VerificationResult{}
	if !r2.OK() {
		t.Error("empty result should be OK")
	}
}

func TestVerify_InconsistentPlateWell(t *testing.T) {
	sheet := &model.SampleSheet{
		Format:  model.FormatV1,
		Columns: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "index", "Sample_Project"},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "n1", SamplePlate: "p1", SampleWell: "A1", Index: "ATCG", SampleProject: "P1"},
			{SampleID: "id2", SampleName: "n2", Index: "GCTA", SampleProject: "P1"},
		},
	}
	r := Verify(sheet)
	if !r.HasWarnings() {
		t.Errorf("expected W_V1_INCONSISTENT_PLATE_WELL warning, got none")
	}
}

func TestVerify_LongSampleName(t *testing.T) {
	long := strings.Repeat("x", 200)
	sheet := &model.SampleSheet{
		Format:  model.FormatV1,
		Columns: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: long, Index: "ATCG", SampleProject: "P1"},
		},
	}
	r := Verify(sheet)
	if !r.HasWarnings() {
		t.Errorf("expected W_V1_LONG_SAMPLE_NAME warning, got none")
	}
}

func TestVerify_SampleIDCharacters(t *testing.T) {
	// The specification allows letters, digits, dashes and underscores
	// in a Sample_ID, and nothing else.
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "index"},
		Reads:   &model.Reads{Read1Cycles: 151},
		Data: []model.Sample{
			{SampleID: "id-1_ok", Index: "ATCGATCG"},
			{SampleID: "id 2", Index: "GCTAGCTA"},
		},
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V2_INVALID_SAMPLE_ID"] != 1 {
		t.Errorf("expected 1 E_V2_INVALID_SAMPLE_ID, got codes: %v", c)
	}
}

func TestVerify_SampleIDTooLong(t *testing.T) {
	// The specification limits the Sample_ID to 100 characters.
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "index"},
		Reads:   &model.Reads{Read1Cycles: 151},
		Data: []model.Sample{
			{SampleID: strings.Repeat("x", 101), Index: "ATCGATCG"},
			{SampleID: strings.Repeat("x", 100), Index: "GCTAGCTA"},
		},
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V2_LONG_SAMPLE_ID"] != 1 {
		t.Errorf("expected 1 E_V2_LONG_SAMPLE_ID, got codes: %v", c)
	}
}

func TestVerify_MiSeqColumnSpellings(t *testing.T) {
	// The MiSeq guide writes the identifier column "SampleID". The
	// verifier must accept it, like the parser does.
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Header:  map[string]string{"FileFormatVersion": "2"},
		Columns: []string{"SampleID", "Sample_Name", "Index", "Index2"},
		Reads:   &model.Reads{Read1Cycles: 151},
		Data: []model.Sample{
			{SampleID: "A10001", SampleName: "Sample_A", Index: "ATTACTCG", Index2: "TATAGCCT"},
		},
	}
	r := Verify(sheet)
	c := codes(r)
	if c["E_V2_MISSING_SAMPLE_ID"] != 0 {
		t.Errorf("unexpected E_V2_MISSING_SAMPLE_ID, got codes: %v", c)
	}
	if r.HasErrors() {
		t.Errorf("expected no errors, got %d: %+v", len(r.Errors()), r.Errors())
	}
}

// TestReadmeRulesTable checks that the README's table of rules is a
// table: every row starts with a pipe, and every code the rules can
// produce is listed in it.
func TestReadmeRulesTable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	readme := string(data)

	// Every code a rule can produce must be in the table.
	for _, rule := range allRules {
		for _, format := range rule.formats {
			code := issueCode(rule.severity, format, rule.suffix)
			if !strings.Contains(readme, "`"+code+"`") {
				t.Errorf("the README does not list %q", code)
			}
		}
	}
	if !strings.Contains(readme, "`E_NIL_SHEET`") {
		t.Error("the README does not list E_NIL_SHEET")
	}

	// Every row of the table must be a table row: a line that starts
	// with something else ends the table and leaves the rest of the
	// rules unrendered.
	inTable := false
	for i, line := range strings.Split(readme, "\n") {
		switch {
		case strings.HasPrefix(line, "| Code |"):
			inTable = true
		case inTable && strings.HasPrefix(line, "|"):
			// a table row
		case inTable && strings.TrimSpace(line) == "":
			inTable = false
		case inTable:
			t.Errorf("README line %d breaks the rules table: %q", i+1, line)
		}
	}
}

// TestVerify_SheetBuiltInCode checks that a sheet built in code, with
// no declared columns, is not told that the columns it plainly has
// are missing.
func TestVerify_SheetBuiltInCode(t *testing.T) {
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Header: map[string]string{"FileFormatVersion": "2"},
		Reads:  &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
		Data: []model.Sample{
			{SampleID: "sample_01", SampleName: "sample_one", Index: "ATCGATCG", SampleProject: "RNAseq"},
		},
	}
	r := Verify(sheet)
	c := codes(r)
	for _, code := range []string{
		"E_V2_MISSING_SAMPLE_ID",
		"E_V2_MISSING_INDEX",
		"W_V2_MISSING_SAMPLE_NAME",
		"W_V2_MISSING_SAMPLE_PROJECT",
	} {
		if c[code] != 0 {
			t.Errorf("a sheet built in code must not report %s, got codes: %v", code, c)
		}
	}
	if len(r.Errors()) != 0 {
		t.Errorf("errors = %v, want none", r.Errors())
	}
}

// TestVerify_MissingHeaderSection checks that a v2 sheet without the
// [Header] section is an error: Illumina's specification says that
// the Header and Reads sections are present in every v2 sample sheet.
func TestVerify_MissingHeaderSection(t *testing.T) {
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
		Reads:   &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG", SampleProject: "P1"},
		},
	}
	r := Verify(sheet)
	if got := codes(r)["E_V2_NO_HEADER"]; got != 1 {
		t.Errorf("E_V2_NO_HEADER = %d, want 1 (codes: %v)", got, codes(r))
	}

	// A sheet that carries the section is not reported.
	sheet.Header = map[string]string{"FileFormatVersion": "2", "RunName": "MyRun"}
	r = Verify(sheet)
	if got := codes(r)["E_V2_NO_HEADER"]; got != 0 {
		t.Errorf("E_V2_NO_HEADER = %d, want 0 (codes: %v)", got, codes(r))
	}
}

func TestVerify_DuplicateColumn(t *testing.T) {
	// The section requirements say that the column headers must all be
	// unique; a repeated name is ignored, so the first one wins.
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "index", "Index"},
		Reads:   &model.Reads{Read1Cycles: 151},
		Data: []model.Sample{
			{SampleID: "id1", Index: "ATCGATCG"},
		},
	}
	r := Verify(sheet)
	c := codes(r)
	if c["W_V2_DUPLICATE_COLUMN"] != 1 {
		t.Errorf("expected 1 W_V2_DUPLICATE_COLUMN, got codes: %v", c)
	}
}

func TestVerify_V2IndexIDsInconsistent(t *testing.T) {
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
		Reads:   &model.Reads{Read1Cycles: 151, Index1Cycles: 8, Index2Cycles: 8},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "n1", I7IndexID: "UDI1", Index: "ATCGATCG", I5IndexID: "UDI2", Index2: "GCTAGCTA", SampleProject: "P1"},
			{SampleID: "id2", SampleName: "n2", I7IndexID: "UDI3", Index: "GCTAGCTA", SampleProject: "P1"},
		},
	}
	r := Verify(sheet)
	c := codes(r)
	if c["W_V2_INCONSISTENT_INDEX_IDS"] == 0 {
		t.Errorf("expected W_V2_INCONSISTENT_INDEX_IDS, got codes: %v", c)
	}
}

func TestVerify_V2IndexIDsInconsistentWithoutI5Column(t *testing.T) {
	// A sheet that carries only I7_Index_ID must still be warned when
	// some rows fill it and others do not.
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "Sample_Name", "I7_Index_ID", "index", "Sample_Project"},
		Reads:   &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "n1", I7IndexID: "UDI1", Index: "ATCGATCG", SampleProject: "P1"},
			{SampleID: "id2", SampleName: "n2", Index: "GCTAGCTA", SampleProject: "P1"},
		},
	}
	r := Verify(sheet)
	c := codes(r)
	if c["W_V2_INCONSISTENT_INDEX_IDS"] == 0 {
		t.Errorf("expected W_V2_INCONSISTENT_INDEX_IDS, got codes: %v", c)
	}
}

func TestIsValidIndex(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"ATCG", true},
		{"atcg", true},
		{"ATCGN", true},
		{"ATCX", false},
		{"ATC-", false},
		{"AT CG", false},
	}
	for _, tt := range tests {
		if got := isValidIndex(tt.in); got != tt.want {
			t.Errorf("isValidIndex(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestHasInvalidNameChars(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"sample_one", false},
		{"sample one", true},
		{"sample\tone", true},
		{"sample/one", false}, // forward slash is not flagged by our heuristic
		{"sample\none", true},
		{"", false},
	}
	for _, tt := range tests {
		if got := hasInvalidNameChars(tt.in); got != tt.want {
			t.Errorf("hasInvalidNameChars(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestResultAccessors(t *testing.T) {
	r := &VerificationResult{
		Issues: []Issue{
			{Severity: SeverityError, Code: "E1", Message: "err1"},
			{Severity: SeverityError, Code: "E2", Message: "err2"},
			{Severity: SeverityWarning, Code: "W1", Message: "warn1"},
		},
	}
	if got := len(r.Errors()); got != 2 {
		t.Errorf("len(Errors) = %d, want 2", got)
	}
	if got := len(r.Warnings()); got != 1 {
		t.Errorf("len(Warnings) = %d, want 1", got)
	}
	if !r.HasErrors() {
		t.Error("HasErrors() = false, want true")
	}
	if !r.HasWarnings() {
		t.Error("HasWarnings() = false, want true")
	}
	if r.OK() {
		t.Error("OK() = true, want false")
	}
}

func TestResultAccessors_OnlyWarnings(t *testing.T) {
	r := &VerificationResult{
		Issues: []Issue{
			{Severity: SeverityWarning, Code: "W1", Message: "warn1"},
		},
	}
	if r.HasErrors() {
		t.Error("HasErrors() = true, want false")
	}
	if !r.HasWarnings() {
		t.Error("HasWarnings() = false, want true")
	}
}

func TestResultAccessors_PromoteWarnings(t *testing.T) {
	r := &VerificationResult{
		Issues: []Issue{
			{Severity: SeverityWarning, Code: "W1", Message: "warn1"},
		},
		promoteWarnings: true,
	}
	if !r.HasErrors() {
		t.Error("HasErrors() with promote = false, want true")
	}
	errs := r.Errors()
	if len(errs) != 1 || errs[0].Code != "W1" {
		t.Errorf("Errors() = %+v, want to contain W1", errs)
	}
}
