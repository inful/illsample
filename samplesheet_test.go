package illsample_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inful/illsample"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestParse_Write_Verify_V1(t *testing.T) {
	in := readFixture(t, "v1_single_index.csv")
	sheet, err := illsample.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != illsample.FormatV1 {
		t.Errorf("Format = %v, want v1", sheet.Format)
	}
	var buf bytes.Buffer
	if err := illsample.Write(&buf, sheet); err != nil {
		t.Fatalf("Write: %v", err)
	}
	round, err := illsample.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse round-trip: %v", err)
	}
	if round.Format != illsample.FormatV1 {
		t.Errorf("round.Format = %v, want v1", round.Format)
	}
	result := illsample.Verify(sheet)
	if result.HasErrors() {
		t.Errorf("expected no errors, got %d: %+v", len(result.Errors()), result.Errors())
	}
}

func TestParse_Write_Verify_V2(t *testing.T) {
	for _, fixture := range []string{"v2_single_index.csv", "v2_dual_index.csv"} {
		t.Run(fixture, func(t *testing.T) {
			in := readFixture(t, fixture)
			sheet, err := illsample.Parse(strings.NewReader(in))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if sheet.Format != illsample.FormatV2 {
				t.Errorf("Format = %v, want v2", sheet.Format)
			}
			var buf bytes.Buffer
			if err := illsample.Write(&buf, sheet); err != nil {
				t.Fatalf("Write: %v", err)
			}
			round, err := illsample.Parse(&buf)
			if err != nil {
				t.Fatalf("Parse round-trip: %v", err)
			}
			result := illsample.Verify(round)
			if result.HasErrors() {
				t.Errorf("expected no errors, got %d: %+v", len(result.Errors()), result.Errors())
			}
		})
	}
}

func TestParse_InvalidV1DetectedByVerify(t *testing.T) {
	in := readFixture(t, "invalid_v1_duplicate_ids.csv")
	sheet, err := illsample.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	result := illsample.Verify(sheet)
	if !result.HasErrors() {
		t.Fatal("expected errors, got none")
	}
	found := false
	for _, iss := range result.Errors() {
		if iss.Code == "E_V1_DUPLICATE_SAMPLE_ID" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected E_V1_DUPLICATE_SAMPLE_ID, got: %+v", result.Errors())
	}
}

func TestParse_InvalidV2DetectedByVerify(t *testing.T) {
	in := readFixture(t, "invalid_v2_no_reads.csv")
	sheet, err := illsample.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	result := illsample.Verify(sheet)
	if !result.HasErrors() {
		t.Fatal("expected errors, got none")
	}
	found := false
	for _, iss := range result.Errors() {
		if iss.Code == "E_V2_NO_READS" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected E_V2_NO_READS, got: %+v", result.Errors())
	}
}

func TestParse_Empty(t *testing.T) {
	_, err := illsample.Parse(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestWriteV1Explicit(t *testing.T) {
	sheet := &illsample.SampleSheet{
		Format: illsample.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
		Data: []illsample.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := illsample.WriteV1(&buf, sheet); err != nil {
		t.Fatalf("WriteV1: %v", err)
	}
	round, err := illsample.ParseV1(&buf)
	if err != nil {
		t.Fatalf("ParseV1: %v", err)
	}
	if round.Data[0].SampleID != "id1" {
		t.Errorf("SampleID = %q, want id1", round.Data[0].SampleID)
	}
}

func TestWriteV2Explicit(t *testing.T) {
	sheet := &illsample.SampleSheet{
		Format: illsample.FormatV2,
		Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
		Reads:  &illsample.Reads{Read1Cycles: 151, Index1Cycles: 8},
		Data: []illsample.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := illsample.WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	round, err := illsample.ParseV2(&buf)
	if err != nil {
		t.Fatalf("ParseV2: %v", err)
	}
	if round.Reads.Read1Cycles != 151 {
		t.Errorf("Read1Cycles = %d, want 151", round.Reads.Read1Cycles)
	}
}

func TestVerifyWithOptions_PromoteWarnings(t *testing.T) {
	sheet := &illsample.SampleSheet{
		Format: illsample.FormatV2,
		Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
		Reads:  &illsample.Reads{Read1Cycles: 151, Index1Cycles: 8},
		Data: []illsample.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG", SampleProject: "P1"},
		},
	}
	defaultResult := illsample.Verify(sheet)
	if defaultResult.HasErrors() {
		t.Fatalf("default Verify has errors: %+v", defaultResult.Errors())
	}
	strict := illsample.VerifyWithOptions(sheet, illsample.VerifyOptions{PromoteWarningsToErrors: true})
	if !strict.HasErrors() {
		t.Error("PromoteWarningsToErrors: expected errors, got none")
	}
}

func TestFormatString(t *testing.T) {
	if got := illsample.FormatV1.String(); got != "v1" {
		t.Errorf("FormatV1.String() = %q, want v1", got)
	}
	if got := illsample.FormatV2.String(); got != "v2" {
		t.Errorf("FormatV2.String() = %q, want v2", got)
	}
	if got := illsample.FormatUnknown.String(); got != "unknown" {
		t.Errorf("FormatUnknown.String() = %q, want unknown", got)
	}
}
