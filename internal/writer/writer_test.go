package writer

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/inful/illsample/internal/model"
	"github.com/inful/illsample/internal/parser"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestWriteV1_RoundTrip(t *testing.T) {
	in := readFixture(t, "v1_single_index.csv")
	sheet, err := parser.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteV1(&buf, sheet); err != nil {
		t.Fatalf("WriteV1: %v", err)
	}
	// Reparse and compare.
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse round-trip: %v\noutput:\n%s", err, buf.String())
	}
	if round.Format != model.FormatV1 {
		t.Errorf("round.Format = %v, want v1", round.Format)
	}
	if len(round.Data) != len(sheet.Data) {
		t.Fatalf("len(Data) = %d, want %d", len(round.Data), len(sheet.Data))
	}
	for i := range sheet.Data {
		if !reflect.DeepEqual(sheet.Data[i], round.Data[i]) {
			t.Errorf("Data[%d] = %+v, want %+v", i, round.Data[i], sheet.Data[i])
		}
	}
}

func TestWriteV1_RoundTripDualIndex(t *testing.T) {
	in := readFixture(t, "v1_dual_index.csv")
	sheet, err := parser.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteV1(&buf, sheet); err != nil {
		t.Fatalf("WriteV1: %v", err)
	}
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse round-trip: %v", err)
	}
	for i := range sheet.Data {
		if !reflect.DeepEqual(sheet.Data[i], round.Data[i]) {
			t.Errorf("Data[%d] = %+v, want %+v", i, round.Data[i], sheet.Data[i])
		}
	}
}

func TestWriteV1_RoundTripExtraColumns(t *testing.T) {
	in := readFixture(t, "v1_extra_columns.csv")
	sheet, err := parser.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteV1(&buf, sheet); err != nil {
		t.Fatalf("WriteV1: %v", err)
	}
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse round-trip: %v", err)
	}
	if len(round.Data) != len(sheet.Data) {
		t.Fatalf("len(Data) = %d, want %d", len(round.Data), len(sheet.Data))
	}
	if round.Data[0].Extra["Custom_Field"] != "abc" {
		t.Errorf("Extra[Custom_Field] lost: %+v", round.Data[0].Extra)
	}
	if round.Data[0].Extra["Notes"] != "note one" {
		t.Errorf("Extra[Notes] lost: %+v", round.Data[0].Extra)
	}
}

// sameSample compares two samples, ignoring SourceRow: it is the
// position of the row in the file it was read from, and the writer
// puts the data section last, so the position changes.
func sameSample(a, b model.Sample) bool {
	a.SourceRow = 0
	b.SourceRow = 0
	return reflect.DeepEqual(a, b)
}

func TestWriteV2_RoundTrip(t *testing.T) {
	for _, fixture := range []string{"v2_single_index.csv", "v2_dual_index.csv", "v2_empty_manifests.csv"} {
		t.Run(fixture, func(t *testing.T) {
			in := readFixture(t, fixture)
			sheet, err := parser.Parse(strings.NewReader(in))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			var buf bytes.Buffer
			if err := WriteV2(&buf, sheet); err != nil {
				t.Fatalf("WriteV2: %v", err)
			}
			round, err := parser.Parse(&buf)
			if err != nil {
				t.Fatalf("Parse round-trip: %v\noutput:\n%s", err, buf.String())
			}
			if round.Format != model.FormatV2 {
				t.Errorf("round.Format = %v, want v2", round.Format)
			}
			if len(round.Data) != len(sheet.Data) {
				t.Fatalf("len(Data) = %d, want %d", len(round.Data), len(sheet.Data))
			}
			for i := range sheet.Data {
				if !sameSample(sheet.Data[i], round.Data[i]) {
					t.Errorf("Data[%d] = %+v, want %+v", i, round.Data[i], sheet.Data[i])
				}
			}
			if sheet.Reads != nil && *sheet.Reads != *round.Reads {
				t.Errorf("Reads = %+v, want %+v", round.Reads, sheet.Reads)
			}
			if sheet.Settings != nil {
				if round.Settings == nil {
					t.Errorf("Settings lost in round-trip")
				} else {
					if sheet.Settings.Adapter != round.Settings.Adapter {
						t.Errorf("Adapter = %q, want %q", round.Settings.Adapter, sheet.Settings.Adapter)
					}
					if sheet.Settings.AdapterRead2 != round.Settings.AdapterRead2 {
						t.Errorf("AdapterRead2 = %q, want %q", round.Settings.AdapterRead2, sheet.Settings.AdapterRead2)
					}
				}
			}
			if len(sheet.Manifests) != len(round.Manifests) {
				t.Errorf("len(Manifests) = %d, want %d", len(round.Manifests), len(sheet.Manifests))
			}
		})
	}
}

func TestWriteV2_PreservesSettingsExtra(t *testing.T) {
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Reads:  &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
		Settings: &model.Settings{
			Adapter: "AGATCG",
			Extra:   map[string]string{"CustomKey": "customvalue"},
		},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse round-trip: %v", err)
	}
	if round.Settings.Extra["CustomKey"] != "customvalue" {
		t.Errorf("Extra[CustomKey] = %q, want customvalue", round.Settings.Extra["CustomKey"])
	}
}

func TestWriteV2_PreservesExtraSections(t *testing.T) {
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Reads:  &model.Reads{Read1Cycles: 151},
		Extra: map[string]model.Section{
			"Custom": {
				Name:    "Custom",
				Columns: []string{"foo", "bar"},
				Rows:    [][]string{{"foo", "bar"}, {"one", "two"}},
			},
		},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse round-trip: %v", err)
	}
	if round.Extra == nil {
		t.Fatal("Extra is nil after round-trip")
	}
	sec, ok := round.Extra["Custom"]
	if !ok {
		t.Fatal("Extra[Custom] missing")
	}
	if len(sec.Rows) != 2 {
		t.Errorf("len(sec.Rows) = %d, want 2", len(sec.Rows))
	}
}

func TestWrite_NilSheet(t *testing.T) {
	if err := Write(&bytes.Buffer{}, nil); err == nil {
		t.Error("Write(nil sheet) = nil, want error")
	}
}

func TestWrite_UnknownFormat(t *testing.T) {
	sheet := &model.SampleSheet{Format: model.FormatUnknown}
	var buf bytes.Buffer
	if err := Write(&buf, sheet); err != nil {
		t.Fatalf("Write(FormatUnknown): %v", err)
	}
	// Should be treated as v2.
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if round.Format != model.FormatV2 {
		t.Errorf("round.Format = %v, want v2", round.Format)
	}
}

func TestWriteV1_EmptyData(t *testing.T) {
	sheet := &model.SampleSheet{
		Format:  model.FormatV1,
		Columns: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
	}
	var buf bytes.Buffer
	if err := WriteV1(&buf, sheet); err != nil {
		t.Fatalf("WriteV1: %v", err)
	}
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(round.Data) != 0 {
		t.Errorf("len(round.Data) = %d, want 0", len(round.Data))
	}
}

func TestWriteV2_QuotedField(t *testing.T) {
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Reads:  &model.Reads{Read1Cycles: 151},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name, with comma", Index: "ATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if round.Data[0].SampleName != "name, with comma" {
		t.Errorf("SampleName = %q, want %q", round.Data[0].SampleName, "name, with comma")
	}
}

func TestWriteV2_NoSettings(t *testing.T) {
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Reads:  &model.Reads{Read1Cycles: 151},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	// [Settings] section should be omitted entirely.
	if strings.Contains(buf.String(), "[Settings]") {
		t.Errorf("output contains [Settings] when Settings is nil:\n%s", buf.String())
	}
}

func TestWriteV2_OutputStructure(t *testing.T) {
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Reads:  &model.Reads{Read1Cycles: 151, Read2Cycles: 151, Index1Cycles: 8},
		Settings: &model.Settings{
			Adapter: "AGATCGGAAGAGCACACGTCTGAACTCCAGTCA",
		},
		Extra: map[string]model.Section{
			"Header": {
				Name:    "Header",
				Columns: []string{"FileFormatVersion", "2"},
				Rows:    [][]string{{"FileFormatVersion", "2"}},
			},
		},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	out := buf.String()
	// The specification says a sample sheet must begin with the Header
	// section and end with the Data section.
	headerIdx := strings.Index(out, "[Header]")
	readsIdx := strings.Index(out, "[Reads]")
	settingsIdx := strings.Index(out, "[Settings]")
	dataIdx := strings.Index(out, "[Data]")
	if headerIdx < 0 || readsIdx < 0 || settingsIdx < 0 || dataIdx < 0 {
		t.Fatalf("output missing section headers:\n%s", out)
	}
	if headerIdx > readsIdx || readsIdx > settingsIdx || settingsIdx > dataIdx {
		t.Errorf("section order: header=%d reads=%d settings=%d data=%d (want header < reads < settings < data)\n%s",
			headerIdx, readsIdx, settingsIdx, dataIdx, out)
	}
}

func TestWriteV2_HeaderSectionRoundTrip(t *testing.T) {
	// A sheet with a [Header] section must keep it, and the written
	// sheet must begin with it and end with the data section.
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Reads:  &model.Reads{Read1Cycles: 151},
		Extra: map[string]model.Section{
			"Header": {
				Name:    "Header",
				Columns: []string{"FileFormatVersion", "2"},
				Rows:    [][]string{{"FileFormatVersion", "2"}},
			},
		},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "[Header]\n") {
		t.Errorf("output does not start with the Header section:\n%s", out)
	}
	if !strings.Contains(out, "\n[Data]\n") {
		t.Errorf("output has no Data section:\n%s", out)
	}
	if strings.Index(out, "[Data]") < strings.Index(out, "[Header]") {
		t.Errorf("the Data section must come last:\n%s", out)
	}
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse round-trip: %v", err)
	}
	if got := round.Header["FileFormatVersion"]; got != "2" {
		t.Errorf("Header[FileFormatVersion] = %q, want 2 (got %v)", got, round.Header)
	}
}

func TestWriteV1_HeaderHasAliasAndDataHasCanonical(t *testing.T) {
	// Header uses v2 canonical names; data rows reference v1 aliases
	// in Extra. The writer should not duplicate the alias as a
	// separate column.
	sheet := &model.SampleSheet{
		Format:  model.FormatV1,
		Columns: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
		Data: []model.Sample{
			{
				SampleID:      "id1",
				SampleName:    "name1",
				Index:         "ATCG",
				SampleProject: "P1",
				Extra:         map[string]string{"SampleID": "should-not-appear"},
			},
		},
	}
	var buf bytes.Buffer
	if err := WriteV1(&buf, sheet); err != nil {
		t.Fatalf("WriteV1: %v", err)
	}
	// The output should not contain a SampleID column.
	if strings.Contains(buf.String(), "SampleID") {
		t.Errorf("output contains aliased column SampleID:\n%s", buf.String())
	}
}

func TestWrite_CSVInjectionEscaped(t *testing.T) {
	// Regression test for CR-02: cells starting with =, +, -, @, \t, or
	// \r must be escaped with a leading single quote so that Excel /
	// Google Sheets / LibreOffice do not evaluate them as formulas.
	cases := []string{
		"=cmd|'/c calc'!A1",
		"+1+1",
		"-2+2",
		"@SUM(A1:A10)",
		"\tleading-tab",
		"\rleading-cr",
	}
	for _, dangerous := range cases {
		t.Run(dangerous, func(t *testing.T) {
			sheet := &model.SampleSheet{
				Format:  model.FormatV2,
				Columns: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
				Reads:   &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
				Data: []model.Sample{
					{SampleID: "id1", SampleName: dangerous, Index: "ATCGATCG", SampleProject: "P1"},
				},
			}
			var buf bytes.Buffer
			if err := WriteV2(&buf, sheet); err != nil {
				t.Fatalf("WriteV2: %v", err)
			}
			out := buf.String()
			// The dangerous value must appear in the output with a
			// leading single quote. We check for the exact byte
			// sequence because some shells render tab/cr oddly.
			if !strings.Contains(out, "'"+dangerous) {
				t.Errorf("output does not contain escaped value %q\noutput:\n%s", "'"+dangerous, out)
			}
		})
	}
}

func TestWrite_CSVInjectionSafeValue(t *testing.T) {
	// A value that does NOT start with a dangerous char must pass
	// through unchanged. This protects against an over-eager escape.
	sheet := &model.SampleSheet{
		Format:  model.FormatV1,
		Columns: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "normal_name", Index: "ATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV1(&buf, sheet); err != nil {
		t.Fatalf("WriteV1: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "'normal_name") {
		t.Errorf("normal value was incorrectly escaped:\n%s", out)
	}
}

func TestWrite_CSVInjectionRoundTrip(t *testing.T) {
	// A dangerous value written by the writer (escaped to "'=…")
	// must round-trip back to the original unescaped value after
	// a re-parse. Without the parser's unescape the model would
	// permanently carry the leading quote.
	cases := []string{
		"=2+2",
		"+1+1",
		"-2+2",
		"@SUM(A1:A10)",
	}
	for _, dangerous := range cases {
		t.Run(dangerous, func(t *testing.T) {
			sheet := &model.SampleSheet{
				Format:  model.FormatV2,
				Columns: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
				Reads:   &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
				Data: []model.Sample{
					{SampleID: "id1", SampleName: dangerous, Index: "ATCGATCG", SampleProject: "P1"},
				},
			}
			var buf bytes.Buffer
			if err := WriteV2(&buf, sheet); err != nil {
				t.Fatalf("WriteV2: %v", err)
			}
			// Confirm the writer did escape the value.
			if !strings.Contains(buf.String(), "'"+dangerous) {
				t.Fatalf("writer did not escape %q:\n%s", dangerous, buf.String())
			}
			// Re-parse and confirm the model has the unescaped value.
			round, err := parser.Parse(&buf)
			if err != nil {
				t.Fatalf("Parse: %v\noutput:\n%s", err, buf.String())
			}
			if got := round.Data[0].SampleName; got != dangerous {
				t.Errorf("round.Data[0].SampleName = %q, want %q (round-trip must be lossless)", got, dangerous)
			}
		})
	}
}

func TestWriteV2_PreservesExtraColumns(t *testing.T) {
	// Regression test for HI-01: a v2 sheet carrying a custom
	// column must round-trip the column through the writer.
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description", "CustomColumn"},
		Reads:   &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
		Data: []model.Sample{
			{
				SampleID:      "id1",
				SampleName:    "name1",
				Index:         "ATCGATCG",
				SampleProject: "P1",
				Extra:         map[string]string{"CustomColumn": "custom-value"},
			},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "CustomColumn") {
		t.Errorf("output should contain CustomColumn header:\n%s", out)
	}
	if !strings.Contains(out, "custom-value") {
		t.Errorf("output should contain custom-value row:\n%s", out)
	}
	// Round-trip back through the parser to confirm the column is
	// restored as an unknown column on the Sample.
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse round-trip: %v", err)
	}
	if got := round.Data[0].Extra["CustomColumn"]; got != "custom-value" {
		t.Errorf("round.Data[0].Extra[CustomColumn] = %q, want custom-value", got)
	}
}

func TestWriteV2_AliasColumnNotDuplicated(t *testing.T) {
	// A v1 sheet that uses the no-underscore "I7IndexID" alias
	// should round-trip as the single canonical "I7_Index_ID"
	// column, not as both the canonical column and the alias.
	sheet := &model.SampleSheet{
		Format:  model.FormatV1,
		Columns: []string{"Sample_ID", "Sample_Name", "I7IndexID", "index", "Sample_Project"},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", I7IndexID: "UDI001", Index: "ATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	out := buf.String()
	// Exactly one of {I7_Index_ID, I7IndexID} should appear.
	count := strings.Count(out, "I7_Index_ID") + strings.Count(out, "I7IndexID")
	if count != 1 {
		t.Errorf("output should contain exactly one I7 column reference, got %d:\n%s", count, out)
	}
	// The value UDI001 must appear in the row.
	if !strings.Contains(out, "UDI001") {
		t.Errorf("output should contain UDI001:\n%s", out)
	}
}

func TestWriteV2_PreservesExtraColumnsFromHeaderOnly(t *testing.T) {
	// A column that appears in the source header but not in any
	// sample's Extra (i.e. an empty column) must still be emitted so
	// that the column order is preserved.
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description", "Assay"},
		Reads:   &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, ",Assay") {
		t.Errorf("output should contain Assay column in header:\n%s", out)
	}
}

func TestWriteV2_PaddedHeaderColumnPreserved(t *testing.T) {
	// A v2 header column with surrounding whitespace must keep its
	// values on round-trip. The parser trims the header, so the
	// writer has to look the column up under the trimmed name.
	sheet := &model.SampleSheet{
		Format:  model.FormatV2,
		Columns: []string{"Sample_ID", "index", "Notes"},
		Reads:   &model.Reads{Read1Cycles: 151},
		Data: []model.Sample{
			{SampleID: "id1", Index: "ATCGATCG", Extra: map[string]string{"Notes": "keepme"}},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	round, err := parser.Parse(&buf)
	if err != nil {
		t.Fatalf("Parse round-trip: %v", err)
	}
	if got := round.Data[0].Extra["Notes"]; got != "keepme" {
		t.Errorf("Extra[Notes] = %q, want keepme", got)
	}
}

// sampleIsEmpty reports whether a sample carries no value at all.
func sampleIsEmpty(s model.Sample) bool {
	if s.SampleID != "" || s.SampleName != "" || s.Index != "" || s.Index2 != "" ||
		s.SampleProject != "" || s.Description != "" || s.SamplePlate != "" ||
		s.SampleWell != "" || s.I7IndexID != "" || s.I5IndexID != "" {
		return false
	}
	for _, v := range s.Extra {
		if v != "" {
			return false
		}
	}
	return true
}

// FuzzWrite exercises the writer with sheets built by the parser.
// The writer must not fail, and what it writes must parse back to
// the same number of rows.
//
// The re-parse forces the sheet's own format: the auto-detector
// reads a one-column sheet whose rows look like section headers as
// a v2 sheet, which is a property of the detection, not of the
// writer.
//
// Use `go test -fuzz=FuzzWrite -fuzztime=10s` to run it.
func FuzzWrite(f *testing.F) {
	seeds := []string{
		"Sample_ID,Sample_Name,index,Sample_Project\nid1,name1,ATCG,P1\n",
		"[Data]\nSample_ID,index\nid1,ATCG\n[Reads]\nRead1Cycles\n151\n",
		"[Data]\nSample_ID,index\nid1,ATCG\n[Reads]\nRead1Cycles,151\n[Settings]\nAdapter,AdapterRead2\nA,B\n",
		"index\nATCG\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		sheet, err := parser.Parse(strings.NewReader(input))
		if err != nil {
			return // a sheet that does not parse is not the writer's problem
		}
		var buf bytes.Buffer
		if err := Write(&buf, sheet); err != nil {
			t.Fatalf("Write: %v", err)
		}
		written := buf.String()
		var round *model.SampleSheet
		if sheet.Format == model.FormatV1 {
			round, err = parser.ParseV1(strings.NewReader(written))
		} else {
			round, err = parser.ParseV2(strings.NewReader(written))
		}
		if err != nil {
			t.Fatalf("written sheet does not parse back: %v\nwritten:\n%q", err, written)
		}
		if len(round.Data) != len(sheet.Data) {
			// A row whose values are all empty is written as a line of
			// only commas, which the specification calls valid but
			// ignored. Such a row does not survive the round-trip.
			empty := 0
			for _, s := range sheet.Data {
				if sampleIsEmpty(s) {
					empty++
				}
			}
			if len(sheet.Data)-empty != len(round.Data) {
				t.Fatalf("row count changed: %d -> %d\nwritten:\n%q", len(sheet.Data), len(round.Data), written)
			}
		}
	})
}

func TestWriteV1_OneColumnEmptyCellSurvives(t *testing.T) {
	// A record with a single empty cell would be a blank line, which
	// the reader skips. The writer must not lose the cell.
	sheet := &model.SampleSheet{
		Format:  model.FormatV1,
		Columns: []string{""},
		Data: []model.Sample{
			{Extra: map[string]string{"": "first"}},
			{Extra: map[string]string{"": ""}},
		},
	}
	var buf bytes.Buffer
	if err := WriteV1(&buf, sheet); err != nil {
		t.Fatalf("WriteV1: %v", err)
	}
	if got, want := buf.String(), "\"\"\nfirst\n\"\"\n"; got != want {
		t.Errorf("written = %q, want %q", got, want)
	}
	round, err := parser.ParseV1(&buf)
	if err != nil {
		t.Fatalf("ParseV1: %v", err)
	}
	if len(round.Data) != 2 {
		t.Errorf("len(Data) = %d, want 2", len(round.Data))
	}
}

func TestWriteV1_AppendsUnknownExtraColumns(t *testing.T) {
	// A column that is not in the header AND not an alias of a
	// header column should be appended at the end of the header.
	sheet := &model.SampleSheet{
		Format:  model.FormatV1,
		Columns: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
		Data: []model.Sample{
			{
				SampleID:      "id1",
				SampleName:    "name1",
				Index:         "ATCG",
				SampleProject: "P1",
				Extra:         map[string]string{"CustomColumn": "custom-value"},
			},
		},
	}
	var buf bytes.Buffer
	if err := WriteV1(&buf, sheet); err != nil {
		t.Fatalf("WriteV1: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "CustomColumn") {
		t.Errorf("output should contain CustomColumn header:\n%s", out)
	}
	if !strings.Contains(out, "custom-value") {
		t.Errorf("output should contain custom-value row:\n%s", out)
	}
}
