package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inful/illsample/internal/model"
	"github.com/inful/illsample/internal/parser"
)

// readFixture reads a file under the package's testdata directory.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
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

func TestVerify_PromoteWarningsToErrors(t *testing.T) {
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
		Reads:  &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
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
		Format: model.FormatV2,
		Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
		Reads:  &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
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
		Format: model.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "index", "Sample_Project"},
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
		Format: model.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: long, Index: "ATCG", SampleProject: "P1"},
		},
	}
	r := Verify(sheet)
	if !r.HasWarnings() {
		t.Errorf("expected W_V1_LONG_SAMPLE_NAME warning, got none")
	}
}

func TestVerify_V2IndexIDsInconsistent(t *testing.T) {
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
		Reads:  &model.Reads{Read1Cycles: 151, Index1Cycles: 8, Index2Cycles: 8},
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
