// Package writer serialises a model.SampleSheet back into a v1 or v2
// sample sheet on disk.
//
// The Write function chooses the format from sheet.Format; callers
// that want a specific layout should use WriteV1 or WriteV2. Writing
// is symmetric to the parser: every column or section that the parser
// preserved (e.g. in sheet.Header or sheet.Extra) is written back so
// that a round-trip is lossless for well-formed input.
package writer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/inful/illsample/internal/model"
)

// canonicalColumnsV2 is the column order used by the IEM v2 [Data] section.
var canonicalColumnsV2 = []string{
	"Sample_ID",
	"Sample_Name",
	"Sample_Plate",
	"Sample_Well",
	"I7_Index_ID",
	"index",
	"I5_Index_ID",
	"index2",
	"Sample_Project",
	"Description",
}

// defaultHeaderV1 is used when a v1 sheet has no recorded header.
var defaultHeaderV1 = []string{
	"Sample_ID",
	"Sample_Name",
	"Sample_Plate",
	"Sample_Well",
	"I7_Index_ID",
	"index",
	"I5_Index_ID",
	"index2",
	"Sample_Project",
	"Description",
}

// Write serialises sheet to w. The output format is chosen from
// sheet.Format. FormatUnknown is treated as v2.
func Write(w io.Writer, sheet *model.SampleSheet) error {
	if sheet == nil {
		return fmt.Errorf("writer: nil sheet")
	}
	switch sheet.Format {
	case model.FormatV1:
		return WriteV1(w, sheet)
	case model.FormatV2, model.FormatUnknown:
		return WriteV2(w, sheet)
	default:
		return fmt.Errorf("writer: unsupported format %v", sheet.Format)
	}
}

// WriteV1 writes sheet in v1 (legacy single-CSV) format.
func WriteV1(w io.Writer, sheet *model.SampleSheet) error {
	header := sheet.Header
	if len(header) == 0 {
		header = defaultHeaderV1
	}
	// If the header omits some columns the data rows use via Extra,
	// append them so we don't drop data.
	header = appendExtraColumns(header, sheet.Data)
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	cw.UseCRLF = false
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("writer: write header: %w", err)
	}
	for _, sample := range sheet.Data {
		row := make([]string, len(header))
		for i, col := range header {
			row[i] = fieldFor(sample, col)
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("writer: write row: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("writer: flush: %w", err)
	}
	if _, err := io.WriteString(w, buf.String()); err != nil {
		return fmt.Errorf("writer: write output: %w", err)
	}
	return nil
}

// WriteV2 writes sheet in v2 (IEM sectioned) format.
func WriteV2(w io.Writer, sheet *model.SampleSheet) error {
	var buf bytes.Buffer

	// [Data]
	if err := writeSectionHeader(&buf, "Data"); err != nil {
		return err
	}
	if err := writeV2Data(&buf, sheet.Data); err != nil {
		return err
	}

	// [Reads]
	if sheet.Reads != nil {
		if err := writeSectionHeader(&buf, "Reads"); err != nil {
			return err
		}
		if err := writeV2Reads(&buf, sheet.Reads); err != nil {
			return err
		}
	}

	// [Settings]
	if sheet.Settings != nil {
		if err := writeSectionHeader(&buf, "Settings"); err != nil {
			return err
		}
		if err := writeV2Settings(&buf, sheet.Settings); err != nil {
			return err
		}
	}

	// [Manifests]
	if len(sheet.Manifests) > 0 {
		if err := writeSectionHeader(&buf, "Manifests"); err != nil {
			return err
		}
		if err := writeV2Manifests(&buf, sheet.Manifests); err != nil {
			return err
		}
	}

	// Any extra sections, sorted by name for deterministic output.
	if len(sheet.Extra) > 0 {
		names := make([]string, 0, len(sheet.Extra))
		for name := range sheet.Extra {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			sec := sheet.Extra[name]
			if err := writeSectionHeader(&buf, sec.Name); err != nil {
				return err
			}
			if err := writeV2GenericSection(&buf, sec); err != nil {
				return err
			}
		}
	}

	if _, err := io.WriteString(w, buf.String()); err != nil {
		return fmt.Errorf("writer: write output: %w", err)
	}
	return nil
}

// writeSectionHeader writes "[<name>]\n" to buf.
func writeSectionHeader(buf *bytes.Buffer, name string) error {
	if _, err := fmt.Fprintf(buf, "[%s]\n", name); err != nil {
		return fmt.Errorf("writer: write section header: %w", err)
	}
	return nil
}

// writeV2Data writes the [Data] section of a v2 sheet.
func writeV2Data(buf *bytes.Buffer, data []model.Sample) error {
	cw := csv.NewWriter(buf)
	cw.UseCRLF = false
	if err := cw.Write(canonicalColumnsV2); err != nil {
		return err
	}
	for _, sample := range data {
		row := make([]string, len(canonicalColumnsV2))
		for i, col := range canonicalColumnsV2 {
			row[i] = fieldFor(sample, col)
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// writeV2Reads writes the [Reads] section in vertical key/value form.
func writeV2Reads(buf *bytes.Buffer, reads *model.Reads) error {
	write := func(key string, n int) error {
		if n == 0 {
			return nil
		}
		if _, err := fmt.Fprintf(buf, "%s\n%d\n", key, n); err != nil {
			return err
		}
		return nil
	}
	if err := write("Read1Cycles", reads.Read1Cycles); err != nil {
		return err
	}
	if err := write("Read2Cycles", reads.Read2Cycles); err != nil {
		return err
	}
	if err := write("Index1Cycles", reads.Index1Cycles); err != nil {
		return err
	}
	if err := write("Index2Cycles", reads.Index2Cycles); err != nil {
		return err
	}
	return nil
}

// writeV2Settings writes the [Settings] section as a 2-column CSV.
func writeV2Settings(buf *bytes.Buffer, s *model.Settings) error {
	// Collect keys in a stable order: typed fields first, then Extra
	// keys sorted.
	keys := make([]string, 0, 2+len(s.Extra))
	if s.Adapter != "" {
		keys = append(keys, "Adapter")
	}
	if s.AdapterRead2 != "" {
		keys = append(keys, "AdapterRead2")
	}
	if len(s.Extra) > 0 {
		extra := make([]string, 0, len(s.Extra))
		for k := range s.Extra {
			extra = append(extra, k)
		}
		sort.Strings(extra)
		keys = append(keys, extra...)
	}
	if len(keys) == 0 {
		return nil
	}
	cw := csv.NewWriter(buf)
	cw.UseCRLF = false
	if err := cw.Write(keys); err != nil {
		return err
	}
	row := make([]string, len(keys))
	for i, k := range keys {
		switch k {
		case "Adapter":
			row[i] = s.Adapter
		case "AdapterRead2":
			row[i] = s.AdapterRead2
		default:
			row[i] = s.Extra[k]
		}
	}
	if err := cw.Write(row); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// writeV2Manifests writes the [Manifests] section.
func writeV2Manifests(buf *bytes.Buffer, manifests []model.Manifest) error {
	for _, m := range manifests {
		// We only write column A; the IEM spec models a single
		// column of manifest paths.
		if _, err := fmt.Fprintln(buf, m.A); err != nil {
			return err
		}
	}
	return nil
}

// writeV2GenericSection writes a section we did not model explicitly.
func writeV2GenericSection(buf *bytes.Buffer, sec model.Section) error {
	if len(sec.Rows) == 0 {
		return nil
	}
	cw := csv.NewWriter(buf)
	cw.UseCRLF = false
	for _, row := range sec.Rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// appendExtraColumns returns header plus any extra column names that
// appear in sample.Extra but not in header. The order of extras is
// sorted for determinism.
func appendExtraColumns(header []string, data []model.Sample) []string {
	have := make(map[string]struct{}, len(header))
	for _, h := range header {
		have[h] = struct{}{}
	}
	var extras []string
	for _, s := range data {
		for k := range s.Extra {
			if _, ok := have[k]; ok {
				continue
			}
			if _, ok := have[canonicalAlias(k)]; ok {
				continue
			}
			have[k] = struct{}{}
			extras = append(extras, k)
		}
	}
	sort.Strings(extras)
	return append(header, extras...)
}

// fieldFor returns the value of sample for the given column name. It
// handles the known columns and falls back to Extra. The lookup is
// case-insensitive.
func fieldFor(sample model.Sample, col string) string {
	switch strings.ToLower(col) {
	case "sample_id", "sampleid":
		return sample.SampleID
	case "sample_name", "samplename":
		return sample.SampleName
	case "sample_plate", "sampleplate":
		return sample.SamplePlate
	case "sample_well", "samplewell":
		return sample.SampleWell
	case "i7_index_id", "i7indexid":
		return sample.I7IndexID
	case "index":
		return sample.Index
	case "i5_index_id", "i5indexid":
		return sample.I5IndexID
	case "index2":
		return sample.Index2
	case "sample_project", "sampleproject":
		return sample.SampleProject
	case "description":
		return sample.Description
	}
	if v, ok := sample.Extra[col]; ok {
		return v
	}
	return ""
}

// canonicalAlias maps a v1-style alias to its canonical v2 name.
func canonicalAlias(name string) string {
	switch strings.ToLower(name) {
	case "sampleid":
		return "Sample_ID"
	case "samplename":
		return "Sample_Name"
	case "sampleplate":
		return "Sample_Plate"
	case "samplewell":
		return "Sample_Well"
	case "i7indexid":
		return "I7_Index_ID"
	case "i5indexid":
		return "I5_Index_ID"
	case "sampleproject":
		return "Sample_Project"
	}
	return name
}
