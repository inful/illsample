package illsample_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inful/illsample"
)

// readIlluminaSheet reads one of the sample sheets that Illumina
// publishes. They live in testdata/illumina; see the README there for
// where they come from.
func readIlluminaSheet(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "illumina", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// TestParse_IlluminaSampleSheets parses the sample sheets that
// Illumina publishes: the v2 template, the NovaSeq 6000 example and
// the 5-base DNA prep templates. They are the reference for the
// formats the parser has to read, so every one of them is checked.
func TestParse_IlluminaSampleSheets(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "illumina", "*.csv"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no Illumina sample sheets found")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			sheet, err := illsample.Parse(strings.NewReader(readIlluminaSheet(t, filepath.Base(file))))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if sheet.Format != illsample.FormatV2 {
				t.Errorf("Format = %v, want v2", sheet.Format)
			}
			if sheet.Reads == nil || sheet.Reads.Read1Cycles <= 0 {
				t.Errorf("Reads = %+v, want a read 1 cycle count", sheet.Reads)
			}
			if len(sheet.Data) == 0 {
				t.Fatal("no samples")
			}
			for i, s := range sheet.Data {
				if s.SampleID == "" {
					t.Errorf("Data[%d].SampleID is empty", i)
				}
				if s.Index == "" && s.Index2 == "" {
					t.Errorf("Data[%d] has no index", i)
				}
			}
			// The sheet must validate: only the optional columns of
			// the v2 layout may be missing.
			result := illsample.Verify(sheet)
			if result.HasErrors() {
				t.Errorf("expected no errors, got %d: %+v", len(result.Errors()), result.Errors())
			}

			// Writing the sheet and reading it back must not lose a
			// sample or change one.
			var buf bytes.Buffer
			if err := illsample.Write(&buf, sheet); err != nil {
				t.Fatalf("Write: %v", err)
			}
			round, err := illsample.Parse(&buf)
			if err != nil {
				t.Fatalf("Parse round-trip: %v", err)
			}
			if len(round.Data) != len(sheet.Data) {
				t.Fatalf("row count changed: %d -> %d", len(sheet.Data), len(round.Data))
			}
			for i := range sheet.Data {
				if sheet.Data[i].SampleID != round.Data[i].SampleID ||
					sheet.Data[i].Index != round.Data[i].Index ||
					sheet.Data[i].Index2 != round.Data[i].Index2 {
					t.Errorf("Data[%d] changed: %+v -> %+v", i, sheet.Data[i], round.Data[i])
				}
			}
		})
	}
}

// TestParse_IlluminaV2Template checks the details of the v2 template:
// the padded labels and records, the instruction lines and the
// application sections.
func TestParse_IlluminaV2Template(t *testing.T) {
	sheet, err := illsample.Parse(strings.NewReader(readIlluminaSheet(t, "SampleSheet_v2_template.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Reads == nil {
		t.Fatal("Reads is nil")
	}
	if sheet.Reads.Read1Cycles != 125 || sheet.Reads.Read2Cycles != 125 ||
		sheet.Reads.Index1Cycles != 8 || sheet.Reads.Index2Cycles != 8 {
		t.Errorf("Reads = %+v, want 125/125/8/8", sheet.Reads)
	}
	if len(sheet.Data) != 4 {
		t.Fatalf("len(Data) = %d, want 4", len(sheet.Data))
	}
	if got, want := sheet.Data[0].SampleID, "S01-TOO-12plex-P1-rep1"; got != want {
		t.Errorf("Data[0].SampleID = %q, want %q", got, want)
	}
	if got, want := sheet.Data[0].Index, "ATCCACTG"; got != want {
		t.Errorf("Data[0].Index = %q, want %q", got, want)
	}
	// The Lane column is not a column the model knows; it is kept.
	if got, want := sheet.Data[0].Extra["Lane"], "1"; got != want {
		t.Errorf("Extra[Lane] = %q, want %q", got, want)
	}
	// The application settings section holds a key-value pair.
	if got, want := sheet.Settings.Extra["SoftwareVersion"], "x.y.z"; got != want {
		t.Errorf("Extra[SoftwareVersion] = %q, want %q", got, want)
	}
}

// TestParse_Illumina5BaseTemplate checks the details of the 5-base DNA
// prep template: the "Index"/"Index2" spelling, the Lane column and
// the settings the model does not know.
func TestParse_Illumina5BaseTemplate(t *testing.T) {
	name := "SampleSheet_TEMPLATE_NovaSeq6000_Illumina-5-base-DNA-Prep-Illumina-Unique-Dual-Indexes-LT.csv"
	sheet, err := illsample.Parse(strings.NewReader(readIlluminaSheet(t, name)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Reads == nil || sheet.Reads.Read1Cycles != 151 || sheet.Reads.Index1Cycles != 10 {
		t.Errorf("Reads = %+v, want 151/151/10/10", sheet.Reads)
	}
	if len(sheet.Data) != 48 {
		t.Fatalf("len(Data) = %d, want 48", len(sheet.Data))
	}
	first := sheet.Data[0]
	if got, want := first.SampleID, "Sample1-IndexSetLT-A01"; got != want {
		t.Errorf("Data[0].SampleID = %q, want %q", got, want)
	}
	if got, want := first.Index, "AGATAGTAGC"; got != want {
		t.Errorf("Data[0].Index = %q, want %q", got, want)
	}
	if got, want := first.Index2, "GGCACGCCAT"; got != want {
		t.Errorf("Data[0].Index2 = %q, want %q", got, want)
	}
	if got, want := first.Extra["Lane"], "1"; got != want {
		t.Errorf("Extra[Lane] = %q, want %q", got, want)
	}
	if got, want := sheet.Settings.Extra["TrimUMI"], "1"; got != want {
		t.Errorf("Extra[TrimUMI] = %q, want %q", got, want)
	}
	if got, want := sheet.Settings.Extra["OverrideCycles"], "U7N1Y143;I10;I10;U7N1Y143"; got != want {
		t.Errorf("Extra[OverrideCycles] = %q, want %q", got, want)
	}
}

// TestParse_IlluminaNovaSeq6000Example checks the NovaSeq 6000 example
// sheet, which has an unknown settings section and the "Index" spelling.
func TestParse_IlluminaNovaSeq6000Example(t *testing.T) {
	sheet, err := illsample.Parse(strings.NewReader(readIlluminaSheet(t, "SampleSheet_NovaSeq6000_Example.csv")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sheet.Reads == nil || sheet.Reads.Read1Cycles != 151 || sheet.Reads.Index2Cycles != 10 {
		t.Errorf("Reads = %+v, want 151/151/10/10", sheet.Reads)
	}
	if len(sheet.Data) != 24 {
		t.Fatalf("len(Data) = %d, want 24", len(sheet.Data))
	}
	if got, want := sheet.Settings.Extra["AdapterRead1"], "CTGTCTCTTATACACATCT"; got != want {
		t.Errorf("Extra[AdapterRead1] = %q, want %q", got, want)
	}
	if _, ok := sheet.Extra["Sequencing_Settings"]; !ok {
		t.Errorf("Extra[Sequencing_Settings] missing, got %v", sheet.Extra)
	}
}
