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
				if !reflect.DeepEqual(sheet.Data[i], round.Data[i]) {
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
		Format: model.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
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
		Data: []model.Sample{
			{SampleID: "id1", SampleName: "name1", Index: "ATCGATCG", SampleProject: "P1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteV2(&buf, sheet); err != nil {
		t.Fatalf("WriteV2: %v", err)
	}
	out := buf.String()
	// [Data] must come before [Reads] which must come before [Settings].
	dataIdx := strings.Index(out, "[Data]")
	readsIdx := strings.Index(out, "[Reads]")
	settingsIdx := strings.Index(out, "[Settings]")
	if dataIdx < 0 || readsIdx < 0 || settingsIdx < 0 {
		t.Fatalf("output missing section headers:\n%s", out)
	}
	if dataIdx >= readsIdx || readsIdx >= settingsIdx {
		t.Errorf("section order: data=%d reads=%d settings=%d (want data < reads < settings)\n%s",
			dataIdx, readsIdx, settingsIdx, out)
	}
}

func TestWriteV1_HeaderHasAliasAndDataHasCanonical(t *testing.T) {
	// Header uses v2 canonical names; data rows reference v1 aliases
	// in Extra. The writer should not duplicate the alias as a
	// separate column.
	sheet := &model.SampleSheet{
		Format: model.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
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
				Format: model.FormatV2,
				Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description"},
				Reads:  &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
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
		Format: model.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
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

func TestWriteV2_PreservesExtraColumns(t *testing.T) {
	// Regression test for HI-01: a v2 sheet carrying a custom
	// column must round-trip the column through the writer.
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
		Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description", "CustomColumn"},
		Reads:  &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
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
		Format: model.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "I7IndexID", "index", "Sample_Project"},
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
		Format: model.FormatV2,
		Header: []string{"Sample_ID", "Sample_Name", "Sample_Plate", "Sample_Well", "I7_Index_ID", "index", "I5_Index_ID", "index2", "Sample_Project", "Description", "Assay"},
		Reads:  &model.Reads{Read1Cycles: 151, Index1Cycles: 8},
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

func TestWriteV1_AppendsUnknownExtraColumns(t *testing.T) {
	// A column that is not in the header AND not an alias of a
	// header column should be appended at the end of the header.
	sheet := &model.SampleSheet{
		Format: model.FormatV1,
		Header: []string{"Sample_ID", "Sample_Name", "index", "Sample_Project"},
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
