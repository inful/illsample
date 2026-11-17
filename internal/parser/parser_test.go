package parser

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inful/illsample/internal/model"
)

// fixtureDir is the directory that contains the test data. It is the
// module's testdata directory, so that every package reads the same
// fixtures rather than a copy of them.
const fixtureDir = "../../testdata"

// readFile reads a file under the fixture directory.
func readFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestParseV1_SingleIndex(t *testing.T) {
	sheet, err := Parse(strings.NewReader(readFile(t, "v1_single_index.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV1 {
		t.Errorf("Format = %v, want v1", sheet.Format)
	}
	if got, want := len(sheet.Data), 4; got != want {
		t.Fatalf("len(Data) = %d, want %d", got, want)
	}
	first := sheet.Data[0]
	if first.SampleID != "Sample_01" {
		t.Errorf("Data[0].SampleID = %q, want Sample_01", first.SampleID)
	}
	if first.Index != "ATCGATCG" {
		t.Errorf("Data[0].Index = %q, want ATCGATCG", first.Index)
	}
	if first.Index2 != "" {
		t.Errorf("Data[0].Index2 = %q, want empty", first.Index2)
	}
	if first.SourceRow != 1 {
		t.Errorf("Data[0].SourceRow = %d, want 1", first.SourceRow)
	}
	if first.SampleProject != "Project_Alpha" {
		t.Errorf("Data[0].SampleProject = %q, want Project_Alpha", first.SampleProject)
	}
}

func TestParseV1_DualIndex(t *testing.T) {
	sheet, err := Parse(strings.NewReader(readFile(t, "v1_dual_index.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV1 {
		t.Errorf("Format = %v, want v1", sheet.Format)
	}
	if got, want := len(sheet.Data), 4; got != want {
		t.Fatalf("len(Data) = %d, want %d", got, want)
	}
	first := sheet.Data[0]
	if first.Index != "TAAGGCGA" {
		t.Errorf("Data[0].Index = %q, want TAAGGCGA", first.Index)
	}
	if first.Index2 != "AGAGTAGA" {
		t.Errorf("Data[0].Index2 = %q, want AGAGTAGA", first.Index2)
	}
}

func TestParseV1_ExtraColumns(t *testing.T) {
	sheet, err := Parse(strings.NewReader(readFile(t, "v1_extra_columns.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(sheet.Data))
	}
	first := sheet.Data[0]
	if first.Extra == nil {
		t.Fatal("Extra is nil, want non-nil")
	}
	if got := first.Extra["Custom_Field"]; got != "abc" {
		t.Errorf("Extra[Custom_Field] = %q, want abc", got)
	}
	if got := first.Extra["Notes"]; got != "note one" {
		t.Errorf("Extra[Notes] = %q, want note one", got)
	}
}

func TestParseV1_ColumnOrderIndependent(t *testing.T) {
	input := "index,Sample_Project,Sample_ID,Sample_Name\nATCG,P1,id1,name1\nGCTA,P2,id2,name2\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Data[0].Index != "ATCG" {
		t.Errorf("Index = %q, want ATCG", sheet.Data[0].Index)
	}
	if sheet.Data[0].SampleID != "id1" {
		t.Errorf("SampleID = %q, want id1", sheet.Data[0].SampleID)
	}
}

func TestParseV1_HeaderCaseInsensitive(t *testing.T) {
	input := "sampleid,SAMPLENAME,INDEX,sampleproject\nid1,name1,ATCG,P1\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Data[0].SampleID != "id1" {
		t.Errorf("SampleID = %q, want id1", sheet.Data[0].SampleID)
	}
	if sheet.Data[0].SampleName != "name1" {
		t.Errorf("SampleName = %q, want name1", sheet.Data[0].SampleName)
	}
}

func TestParseV1_BOM(t *testing.T) {
	input := "\uFEFFSample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Columns) == 0 || sheet.Columns[0] != "Sample_ID" {
		t.Errorf("Columns[0] = %q, want Sample_ID (no BOM)", sheet.Columns[0])
	}
}

func TestParseV1_CRLF(t *testing.T) {
	input := "Sample_ID,Sample_Name,index,Sample_Project\r\nid1,name1,ATCG,P1\r\nid2,name2,GCTA,P2\r\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(sheet.Data))
	}
}

func TestParseV2_Basic(t *testing.T) {
	sheet, err := Parse(strings.NewReader(readFile(t, "v2_single_index.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV2 {
		t.Errorf("Format = %v, want v2", sheet.Format)
	}
	if got, want := len(sheet.Data), 3; got != want {
		t.Fatalf("len(Data) = %d, want %d", got, want)
	}
	if sheet.Reads == nil {
		t.Fatal("Reads is nil")
	}
	if sheet.Reads.Read1Cycles != 151 {
		t.Errorf("Read1Cycles = %d, want 151", sheet.Reads.Read1Cycles)
	}
	if sheet.Reads.Index1Cycles != 8 {
		t.Errorf("Index1Cycles = %d, want 8", sheet.Reads.Index1Cycles)
	}
	if sheet.Settings == nil {
		t.Fatal("Settings is nil")
	}
	if sheet.Settings.Adapter == "" {
		t.Errorf("Adapter is empty")
	}
}

func TestParseV2_DualIndex(t *testing.T) {
	sheet, err := Parse(strings.NewReader(readFile(t, "v2_dual_index.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV2 {
		t.Errorf("Format = %v, want v2", sheet.Format)
	}
	if len(sheet.Data) != 4 {
		t.Fatalf("len(Data) = %d, want 4", len(sheet.Data))
	}
	first := sheet.Data[0]
	if first.Index != "TAAGGCGA" || first.Index2 != "AGAGTAGA" {
		t.Errorf("first indexes = %q/%q, want TAAGGCGA/AGAGTAGA", first.Index, first.Index2)
	}
	if len(sheet.Manifests) != 2 {
		t.Errorf("len(Manifests) = %d, want 2", len(sheet.Manifests))
	}
	if sheet.Manifests[0].Filename != "/path/to/manifest_one.csv" {
		t.Errorf("Manifests[0].Filename = %q, want /path/to/manifest_one.csv", sheet.Manifests[0].Filename)
	}
	if sheet.Manifests[1].Filename != "/path/to/manifest_two.csv" {
		t.Errorf("Manifests[1].Filename = %q, want /path/to/manifest_two.csv", sheet.Manifests[1].Filename)
	}
}

func TestParseV2_EmptyManifests(t *testing.T) {
	sheet, err := Parse(strings.NewReader(readFile(t, "v2_empty_manifests.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Manifests) != 0 {
		t.Errorf("len(Manifests) = %d, want 0", len(sheet.Manifests))
	}
}

func TestParseV2_ManifestsSingleAIsKept(t *testing.T) {
	// Regression test for WR-03: a [Manifests] section that contains
	// only a single "A" row must be kept verbatim. The old
	// implementation unconditionally dropped the first "A" row,
	// losing data.
	input := "[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n" +
		"[Manifests]\nA\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Manifests) != 1 {
		t.Errorf("len(Manifests) = %d, want 1", len(sheet.Manifests))
	}
	if len(sheet.Manifests) == 1 && sheet.Manifests[0].Key != "A" {
		t.Errorf("Manifests[0].Key = %q, want A", sheet.Manifests[0].Key)
	}
}

func TestParseV2_ManifestsAHeaderSkippedWhenFollowed(t *testing.T) {
	// When a leading "A" is followed by at least one more row, it
	// is a header and must be skipped.
	input := "[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n" +
		"[Manifests]\nA\n/path/to/manifest.csv\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Manifests) != 1 {
		t.Fatalf("len(Manifests) = %d, want 1", len(sheet.Manifests))
	}
	if sheet.Manifests[0].Filename != "/path/to/manifest.csv" {
		t.Errorf("Manifests[0].Filename = %q, want /path/to/manifest.csv", sheet.Manifests[0].Filename)
	}
}

func TestParseV2_HeaderCaseInsensitive(t *testing.T) {
	input := "[data]\nSample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n[READS]\nRead1Cycles\n151\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV2 {
		t.Errorf("Format = %v, want v2", sheet.Format)
	}
	if len(sheet.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(sheet.Data))
	}
	if sheet.Reads.Read1Cycles != 151 {
		t.Errorf("Read1Cycles = %d, want 151", sheet.Reads.Read1Cycles)
	}
}

func TestParseV2_ReadsWithoutValue(t *testing.T) {
	input := "[Reads]\nRead1Cycles\n151\nIndex1Cycles\n"
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("Parse: expected error for [Reads] key without value")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("error type = %T, want *ParseError", err)
	}
}

func TestParseV2_UnknownReadsKey(t *testing.T) {
	input := "[Reads]\nRead3Cycles\n10\n"
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("Parse: expected error for unknown [Reads] key")
	}
}

func TestParseV2_UnknownSectionPreserved(t *testing.T) {
	input := "[Data]\nSample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n[Custom]\nfoo,bar\none,two\n[Reads]\nRead1Cycles\n151\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Extra == nil {
		t.Fatal("Extra is nil, want non-nil")
	}
	sec, ok := sheet.Extra["Custom"]
	if !ok {
		t.Fatal("Extra[Custom] missing")
	}
	if len(sec.Rows) != 2 {
		t.Errorf("len(sec.Rows) = %d, want 2", len(sec.Rows))
	}
}

func TestParseV2_QuotedField(t *testing.T) {
	input := "[Data]\nSample_ID,Sample_Name,Sample_Plate,Sample_Well,I7_Index_ID,index,I5_Index_ID,index2,Sample_Project,Description\nid1,\"name, with comma\",plate,A1,UDI,ATCG,,,P1,note\n[Reads]\nRead1Cycles\n151\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Data[0].SampleName != "name, with comma" {
		t.Errorf("SampleName = %q, want %q", sheet.Data[0].SampleName, "name, with comma")
	}
}

func TestParseV2_CommentsAndBlankLines(t *testing.T) {
	input := "# generated by IEM\n\n[Data]\n# inline comment in Data\nSample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n\n[Reads]\n# another comment\nRead1Cycles\n151\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Data) != 1 {
		t.Errorf("len(Data) = %d, want 1", len(sheet.Data))
	}
	if sheet.Reads.Read1Cycles != 151 {
		t.Errorf("Read1Cycles = %d, want 151", sheet.Reads.Read1Cycles)
	}
}

func TestParseV2_SettingsVerticalMultipleKeys(t *testing.T) {
	// A single-column [Settings] section holds the key on one line and
	// its value on the next. With two keys the second value used to
	// land in the first key (the Read 2 adapter was written as the
	// Read 1 adapter) and the second key was dropped.
	input := "[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n" +
		"[Settings]\n" +
		"Adapter\n" +
		"AGATCGGAAGAGCACACGTCTGAACTCCAGTCA\n" +
		"AdapterRead2\n" +
		"AGATCGGAAGAGCGTCGTGTAGGGAAAGAGTGT\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Settings == nil {
		t.Fatal("Settings is nil")
	}
	if got, want := sheet.Settings.Adapter, "AGATCGGAAGAGCACACGTCTGAACTCCAGTCA"; got != want {
		t.Errorf("Adapter = %q, want %q", got, want)
	}
	if got, want := sheet.Settings.AdapterRead2, "AGATCGGAAGAGCGTCGTGTAGGGAAAGAGTGT"; got != want {
		t.Errorf("AdapterRead2 = %q, want %q", got, want)
	}
}

func TestParseV2_SettingsVerticalUnknownSecondKey(t *testing.T) {
	// The second key of a vertical section does not have to be one of
	// the adapter names; a CamelCase name still starts a new key.
	input := "[Data]\n" +
		"Sample_ID,index\n" +
		"id1,ATCG\n" +
		"[Reads]\nRead1Cycles\n151\n" +
		"[Settings]\n" +
		"SoftwareVersion\n" +
		"4.1.27\n" +
		"OverrideCycles\n" +
		"10\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := sheet.Settings.Extra["SoftwareVersion"], "4.1.27"; got != want {
		t.Errorf("Extra[SoftwareVersion] = %q, want %q", got, want)
	}
	if got, want := sheet.Settings.Extra["OverrideCycles"], "10"; got != want {
		t.Errorf("Extra[OverrideCycles] = %q, want %q", got, want)
	}
}

func TestParseV1_DuplicateAliasColumnsFirstWins(t *testing.T) {
	// A header with both "Sample_ID" and its alias "SampleID" feeds
	// the same field twice. The parser used to walk a map, so the
	// value came from a random column and the same sheet parsed
	// differently from one run to the next.
	input := "Sample_ID,SampleID,Sample_Name,index,Sample_Project\n" +
		"first,second,name1,ATCG,P1\n"
	for i := 0; i < 50; i++ {
		sheet, err := Parse(strings.NewReader(input))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if got := sheet.Data[0].SampleID; got != "first" {
			t.Fatalf("Data[0].SampleID = %q, want %q (first column wins)", got, "first")
		}
	}
}

func TestParseV2_DuplicateAliasColumnsFirstWins(t *testing.T) {
	input := "[Data]\n" +
		"Sample_ID,SampleID,index\n" +
		"first,second,ATCG\n" +
		"[Reads]\nRead1Cycles\n151\n"
	for i := 0; i < 50; i++ {
		sheet, err := Parse(strings.NewReader(input))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if got := sheet.Data[0].SampleID; got != "first" {
			t.Fatalf("Data[0].SampleID = %q, want %q (first column wins)", got, "first")
		}
	}
}

func TestParseV2_HeaderWhitespaceTrimmed(t *testing.T) {
	// A v2 header with surrounding whitespace must be trimmed, like
	// the v1 header is. Otherwise the writer looks the column up as
	// " Notes" in sample.Extra, where the key is "Notes", and the
	// column's values are dropped on round-trip.
	input := "[Data]\n" +
		"Sample_ID,index, Notes \n" +
		"id1,ATCG,keepme\n" +
		"[Reads]\nRead1Cycles\n151\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := sheet.Columns[2], "Notes"; got != want {
		t.Errorf("Header[2] = %q, want %q", got, want)
	}
	if got, want := sheet.Data[0].Extra["Notes"], "keepme"; got != want {
		t.Errorf("Extra[Notes] = %q, want %q", got, want)
	}
}

func TestParseErrorMessage(t *testing.T) {
	// The line number is part of the message a user sees; it must
	// appear only when there is one.
	tests := []struct {
		err  *ParseError
		want string
	}{
		{&ParseError{Line: 7, Msg: "invalid CSV"}, "line 7: invalid CSV"},
		{&ParseError{Msg: "empty input"}, "empty input"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

func TestParse_Empty(t *testing.T) {
	_, err := Parse(strings.NewReader(""))
	if err == nil {
		t.Fatal("Parse: expected error for empty input")
	}
}

func TestParse_OnlyWhitespace(t *testing.T) {
	_, err := Parse(strings.NewReader("   \n\n\t\n"))
	if err == nil {
		t.Fatal("Parse: expected error for whitespace-only input")
	}
}

func TestParseV1_I7IndexIDNoUnderscore(t *testing.T) {
	// Regression test for CR-01: a v1 sheet with the no-underscore
	// "I7IndexID" column must place the value in I7IndexID, not in
	// I5IndexID. Previously the setter was wired to the wrong field,
	// which would silently send i7 values to the i5 demultiplexing
	// slot.
	input := "Sample_ID,Sample_Name,I7IndexID,index,Sample_Project\n" +
		"id1,name1,UDI001,ATCG,P1\n" +
		"id2,name2,UDI002,GCTA,P2\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := sheet.Data[0].I7IndexID; got != "UDI001" {
		t.Errorf("Data[0].I7IndexID = %q, want UDI001", got)
	}
	if got := sheet.Data[0].I5IndexID; got != "" {
		t.Errorf("Data[0].I5IndexID = %q, want empty", got)
	}
	if got := sheet.Data[1].I7IndexID; got != "UDI002" {
		t.Errorf("Data[1].I7IndexID = %q, want UDI002", got)
	}
}

func TestParseV1_RejectsNonUTF8(t *testing.T) {
	// Regression test for HI-03: ParseV1 must reject non-UTF-8 input
	// with the same behaviour as the auto-detecting Parse. \xff\xfe
	// is not a valid UTF-8 start byte.
	input := "Sample_ID,Sample_Name,index,Sample_Project\nid1,name1,\xff\xfeATCG,P1\n"
	_, err := ParseV1(strings.NewReader(input))
	if err == nil {
		t.Fatal("ParseV1: expected error for non-UTF-8 input, got nil")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("error type = %T, want *ParseError", err)
	}
}

func TestParseV2_RejectsNonUTF8(t *testing.T) {
	input := "[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,\xff\xfeATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n"
	_, err := ParseV2(strings.NewReader(input))
	if err == nil {
		t.Fatal("ParseV2: expected error for non-UTF-8 input, got nil")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("error type = %T, want *ParseError", err)
	}
}

func TestParseV1_RaggedRow(t *testing.T) {
	input := "Sample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1,extra\nid2,name2,GCTA\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Data[0].SampleProject != "P1" {
		t.Errorf("Data[0].SampleProject = %q, want P1", sheet.Data[0].SampleProject)
	}
	// The "extra" column on row 0 is dropped because there is no header
	// for it; that's acceptable for a ragged row.
	if sheet.Data[1].SampleID != "id2" {
		t.Errorf("Data[1].SampleID = %q, want id2", sheet.Data[1].SampleID)
	}
}

func TestParseV2_PreservesSettingsExtra(t *testing.T) {
	input := "[Settings]\nAdapter,CustomKey\nAGATCG,customvalue\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Settings.Adapter != "AGATCG" {
		t.Errorf("Adapter = %q, want AGATCG", sheet.Settings.Adapter)
	}
	if sheet.Settings.Extra["CustomKey"] != "customvalue" {
		t.Errorf("Extra[CustomKey] = %q, want customvalue", sheet.Settings.Extra["CustomKey"])
	}
}

func TestParseV1_HeaderWhitespaceTrimmed(t *testing.T) {
	// Regression test for WR-01: a v1 header with surrounding
	// whitespace must be trimmed so the writer can find the typed
	// field on round-trip.
	input := "Sample_ID, Sample_Name ,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Data[0].SampleName != "name1" {
		t.Errorf("SampleName = %q, want name1", sheet.Data[0].SampleName)
	}
	// Header should be trimmed.
	for i, h := range sheet.Columns {
		if h != "" && (h != strings.TrimSpace(h)) {
			t.Errorf("Header[%d] = %q, want trimmed", i, h)
		}
	}
}

func TestParseV2_SlashCommentInPreamble(t *testing.T) {
	// Regression test for WR-02: a v2 sheet that starts with `//`
	// comments must be detected as v2, not misclassified as v1.
	input := "// generated by some upstream tool\n" +
		"# another comment style\n" +
		"[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV2 {
		t.Errorf("Format = %v, want v2", sheet.Format)
	}
	if len(sheet.Data) != 1 {
		t.Errorf("len(Data) = %d, want 1", len(sheet.Data))
	}
	if sheet.Data[0].SampleID != "id1" {
		t.Errorf("SampleID = %q, want id1", sheet.Data[0].SampleID)
	}
}

func TestParseV2_IlluminaBCLConvertDialect(t *testing.T) {
	// Illumina's v2 dialect (the official SampleSheet_v2_template)
	// names the sections [BCLConvert_Data]/[BCLConvert_Settings] and
	// writes [Reads] as "Key,Value" pairs on one line.
	input := "[Header]\n" +
		"FileFormatVersion,2\n" +
		"RunName,MyRun\n" +
		"[Reads]\n" +
		"Read1Cycles,151\n" +
		"Index1Cycles,8\n" +
		"[BCLConvert_Settings]\n" +
		"AdapterRead1,AGATCG\n" +
		"SoftwareVersion,4.1.27\n" +
		"[BCLConvert_Data]\n" +
		"Sample_ID,index,index2\n" +
		"id1,ATCG,TTAA\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV2 {
		t.Errorf("Format = %v, want v2", sheet.Format)
	}
	if len(sheet.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(sheet.Data))
	}
	if sheet.Data[0].SampleID != "id1" || sheet.Data[0].Index != "ATCG" || sheet.Data[0].Index2 != "TTAA" {
		t.Errorf("Data[0] = %+v, want id1/ATCG/TTAA", sheet.Data[0])
	}
	if sheet.Reads == nil {
		t.Fatal("Reads is nil")
	}
	if sheet.Reads.Read1Cycles != 151 || sheet.Reads.Index1Cycles != 8 {
		t.Errorf("Reads = %+v, want Read1Cycles=151 Index1Cycles=8", sheet.Reads)
	}
	if sheet.Settings == nil {
		t.Fatal("Settings is nil")
	}
	if got := sheet.Settings.Extra["AdapterRead1"]; got != "AGATCG" {
		t.Errorf("Extra[AdapterRead1] = %q, want AGATCG", got)
	}
	if got := sheet.Settings.Extra["SoftwareVersion"]; got != "4.1.27" {
		t.Errorf("Extra[SoftwareVersion] = %q, want 4.1.27", got)
	}
	// The [Header] section is read into the typed field.
	if got := sheet.Header["FileFormatVersion"]; got != "2" {
		t.Errorf("Header[FileFormatVersion] = %q, want 2", got)
	}
}

func TestParseV2_TextPreamble(t *testing.T) {
	// A v2 sheet may carry a short text preamble (the Illumina
	// template does); it must not stop the v2 detection.
	input := "Illumina SampleSheet, exported 2026-11-10\n" +
		"[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV2 {
		t.Errorf("Format = %v, want v2", sheet.Format)
	}
	if len(sheet.Data) != 1 {
		t.Errorf("len(Data) = %d, want 1", len(sheet.Data))
	}
	if sheet.Data[0].SampleID != "id1" {
		t.Errorf("SampleID = %q, want id1", sheet.Data[0].SampleID)
	}
}

func TestParseV1_NotDetectedAsV2(t *testing.T) {
	// A v1 header row is not a preamble: it must stop the scan so
	// the sheet stays v1 and keeps its rows.
	input := "Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Format != model.FormatV1 {
		t.Errorf("Format = %v, want v1", sheet.Format)
	}
	if len(sheet.Data) != 1 {
		t.Errorf("len(Data) = %d, want 1", len(sheet.Data))
	}
}

func TestParseV2_SettingsCaseInsensitive(t *testing.T) {
	// Regression test for WR-04: a [Settings] section with
	// lowercase keys must still populate the typed fields.
	input := "[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n" +
		"[Settings]\n" +
		"adapter,adapterread2\n" +
		"AGATCG,AGATCG-R2\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Settings == nil {
		t.Fatal("Settings is nil")
	}
	if sheet.Settings.Adapter != "AGATCG" {
		t.Errorf("Adapter = %q, want AGATCG", sheet.Settings.Adapter)
	}
	if sheet.Settings.AdapterRead2 != "AGATCG-R2" {
		t.Errorf("AdapterRead2 = %q, want AGATCG-R2", sheet.Settings.AdapterRead2)
	}
}

func TestByteOffsetToLine(t *testing.T) {
	// byteOffsetToLine is the path between an encoding/csv error
	// string ("... on line N") and a 0-based line offset into the
	// raw slice passed to readCSV. The parser runs with
	// r.LazyQuotes = true and r.FieldsPerRecord = -1, which
	// suppresses almost every encoding/csv error in production;
	// this test exercises the helper directly to keep its
	// branch coverage honest.
	raw := []string{"a,b", "c,d", "e,f"}
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"error contains line 2", fmt.Errorf("invalid CSV on line 2"), 1},
		{"error contains line 3", fmt.Errorf("foo on line 3 bar"), 2},
		{"error without line marker", fmt.Errorf("unexpected EOF"), 0},
		{"line number out of range", fmt.Errorf("on line 99"), 0},
		{"empty error", fmt.Errorf(""), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := byteOffsetToLine(raw, tt.err); got != tt.want {
				t.Errorf("byteOffsetToLine = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseV2_EmptySettings(t *testing.T) {
	input := "[Data]\nSample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n[Reads]\nRead1Cycles\n151\n[Settings]\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// An empty [Settings] section is a no-op; we do not allocate a
	// Settings struct so the writer can skip emitting it.
	if sheet.Settings != nil {
		t.Errorf("Settings = %+v, want nil for empty section", sheet.Settings)
	}
}

func TestParseV2_MultipleSettingsRows(t *testing.T) {
	// Regression test for HI-02: a [Settings] section with multiple
	// data rows must not silently drop rows 2+. Later non-empty
	// values win.
	input := "[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n" +
		"[Settings]\n" +
		"Adapter,AdapterRead2,CustomKey\n" +
		"first-adapter,first-read2,first-custom\n" +
		"second-adapter,second-read2,second-custom\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Settings == nil {
		t.Fatal("Settings is nil")
	}
	if got := sheet.Settings.Adapter; got != "second-adapter" {
		t.Errorf("Adapter = %q, want second-adapter", got)
	}
	if got := sheet.Settings.AdapterRead2; got != "second-read2" {
		t.Errorf("AdapterRead2 = %q, want second-read2", got)
	}
	if got := sheet.Settings.Extra["CustomKey"]; got != "second-custom" {
		t.Errorf("Extra[CustomKey] = %q, want second-custom", got)
	}
}

func TestParseV2_SettingsLastNonEmptyWins(t *testing.T) {
	// An empty cell in a later row must not overwrite a non-empty
	// value in an earlier row.
	input := "[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,name1,ATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n" +
		"[Settings]\n" +
		"Adapter\n" +
		"first\n" +
		"\n" +
		"third\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := sheet.Settings.Adapter; got != "third" {
		t.Errorf("Adapter = %q, want third (empty row must not clobber)", got)
	}
}

func TestUnescapeCSVSafe(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		// Trigger chars: leading ' is stripped when the next char is a danger.
		{"'=2+2", "=2+2"},
		{"'+1+1", "+1+1"},
		{"'-2+2", "-2+2"},
		{"'@SUM(A1:A10)", "@SUM(A1:A10)"},
		{"'\tleading-tab", "\tleading-tab"},
		{"'\rleading-cr", "\rleading-cr"},
		// Legitimate leading quote survives.
		{"'twas the night", "'twas the night"},
		{"'normal", "'normal"},
		// Too short to be an escape; left alone.
		{"'", "'"},
		{"", ""},
		// No leading quote; left alone.
		{"normal", "normal"},
		{"=2+2", "=2+2"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := unescapeCSVSafe(tt.in); got != tt.want {
				t.Errorf("unescapeCSVSafe(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseV1_StripsCSVInjectionEscape(t *testing.T) {
	// A v1 sheet whose Sample_Name starts with a formula trigger
	// (and is therefore escaped to "'=…" by the writer) must round
	// trip back to the original unescaped value.
	input := "Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,'=2+2,ATCG,P1\n" +
		"id2,'@SUM,ATCG,P1\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := sheet.Data[0].SampleName; got != "=2+2" {
		t.Errorf("Data[0].SampleName = %q, want %q (leading ' should be stripped)", got, "=2+2")
	}
	if got := sheet.Data[1].SampleName; got != "@SUM" {
		t.Errorf("Data[1].SampleName = %q, want %q (leading ' should be stripped)", got, "@SUM")
	}
}

func TestParseV2_StripsCSVInjectionEscape(t *testing.T) {
	input := "[Data]\n" +
		"Sample_ID,Sample_Name,index,Sample_Project\n" +
		"id1,'=2+2,ATCG,P1\n" +
		"[Reads]\nRead1Cycles\n151\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := sheet.Data[0].SampleName; got != "=2+2" {
		t.Errorf("Data[0].SampleName = %q, want %q (leading ' should be stripped)", got, "=2+2")
	}
}

func TestParse_InputTooLarge(t *testing.T) {
	// Shrink the cap so the test does not need to allocate 2 MB.
	orig := maxInputSize
	maxInputSize = 16
	defer func() { maxInputSize = orig }()

	// 17-byte input: one byte over the cap.
	input := "Sample_ID,Name\nid1,longname2"
	if len(input) <= maxInputSize {
		t.Fatalf("test setup: input is %d bytes, must exceed cap %d", len(input), maxInputSize)
	}
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("Parse: expected error for too-large input, got nil")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("error type = %T, want *ParseError", err)
	}
	if !strings.Contains(pe.Msg, "too large") {
		t.Errorf("error Msg = %q, want substring \"too large\"", pe.Msg)
	}
}

func TestParse_InputAtCap(t *testing.T) {
	// Input exactly at the cap is accepted (the limit is
	// "more than maxInputSize").
	orig := maxInputSize
	maxInputSize = 16
	defer func() { maxInputSize = orig }()

	// 16-byte input: exactly at the cap. Pad "x" to reach the size.
	input := "Sample_ID,Name\n" + strings.Repeat("x", 16-len("Sample_ID,Name\n"))
	if len(input) != maxInputSize {
		t.Fatalf("test setup: input is %d bytes, must equal cap %d", len(input), maxInputSize)
	}
	if _, err := Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
}

// FuzzParse exercises the parser with arbitrary input. The property
// checked is that Parse never panics, regardless of input. A
// successful parse must produce a sheet in a known format. Round-trip
// properties are covered by the deterministic table-driven tests, not
// by the fuzzer.
//
// Use `go test -fuzz=FuzzParse -fuzztime=10s` to run it.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		"a,b,c\n1,2,3\n",
		"Sample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n",
		"[Data]\nSample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n[Reads]\nRead1Cycles\n151\n",
		"\"name with, comma\",value\n",
		"\uFEFFSample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n",
		"# comment\n[Data]\nSample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n",
		"[Data]\n\n[Reads]\nRead1Cycles\n151\n",
		strings.Repeat("x", 4096),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		// The parser must never panic.
		sheet, err := Parse(strings.NewReader(input))
		if err != nil {
			return // errors are fine; we just check no panic
		}
		// If it parsed, it must be in a known format.
		if sheet.Format != model.FormatV1 && sheet.Format != model.FormatV2 {
			t.Errorf("unknown format %v", sheet.Format)
		}
	})
}

// TestParse_NilReader checks that the entry points report a nil
// reader as an error instead of panicking. Write reports a nil sheet
// and Verify reports E_NIL_SHEET, so Parse should not be the one
// that crashes.
func TestParse_NilReader(t *testing.T) {
	for name, fn := range map[string]func(io.Reader) (*model.SampleSheet, error){
		"Parse":   Parse,
		"ParseV1": ParseV1,
		"ParseV2": ParseV2,
	} {
		sheet, err := fn(nil)
		if err == nil {
			t.Errorf("%s(nil) = %v, want an error", name, sheet)
		}
		if sheet != nil {
			t.Errorf("%s(nil) returned a sheet %v, want nil", name, sheet)
		}
	}
}

// TestParseV2_SettingsSingleNameValuedRecord checks that a settings
// section with one record whose value looks like a name keeps its
// value. Illumina's own template writes "SoftwareVersion,x.y.z" and
// "x.y.z" is a lower-case word, so the record looks like a table
// header.
func TestParseV2_SettingsSingleNameValuedRecord(t *testing.T) {
	input := "[Header]\nFileFormatVersion,2\n" +
		"[Data]\nSample_ID,index\nid1,ATCG\n" +
		"[Settings]\nSoftwareVersion,x.y.z\n"
	sheet, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Settings == nil {
		t.Fatal("Settings is nil")
	}
	if got := sheet.Settings.Extra["SoftwareVersion"]; got != "x.y.z" {
		t.Errorf("Settings.Extra[SoftwareVersion] = %q, want x.y.z (settings: %+v)", got, sheet.Settings)
	}
}
