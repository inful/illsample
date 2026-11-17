// Package writer serialises a model.SampleSheet back into a v1 or v2
// sample sheet on disk.
//
// The Write function chooses the format from sheet.Format; callers
// that want a specific layout should use WriteV1 or WriteV2. Writing
// is symmetric to the parser: every column or section that the parser
// preserved (e.g. in sheet.Columns or sheet.Extra) is written back so
// that a round-trip is lossless for well-formed input.
//
// Note on line endings: this package emits LF, whereas IEM produces
// CRLF. bcl2fastq and DRAGEN tolerate both, and the parser accepts
// both, so this is a stylistic deviation only — byte-identical
// round-trips are not guaranteed for CRLF inputs. If you need CRLF
// output, post-process the writer's output.
package writer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/inful/illsample/internal/model"
)

// canonicalColumnsV2 is the column order used by the IEM v2 [Data] section.
// It is derived from the model registry so the v2 canonical header cannot
// drift apart from the columns the model knows.
var canonicalColumnsV2 = model.CanonicalColumnNames()

// defaultHeaderV1 is the header a v1 sheet gets when it records none.
// It is Illumina's own bcl2fastq template: the columns the legacy
// layout has, and not the IEM ones (Sample_Plate, Sample_Well,
// I7_Index_ID, I5_Index_ID), which are v2 columns.
//
// A test checks that every name here is a column the model knows, so
// that a rename in the registry cannot leave the writer writing a
// column the parser does not read.
var defaultHeaderV1 = []string{
	"Sample_ID", "Sample_Name", "index", "index2", "Sample_Project", "Description",
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
	header := sheet.Columns
	if len(header) == 0 {
		header = defaultHeaderV1
	}
	// If the header omits some columns the data rows use via Extra,
	// append them so we don't drop data.
	header = appendExtraColumns(header, sheet.Data)
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	cw.UseCRLF = false
	if err := writeCSVRow(&buf, cw, header); err != nil {
		return fmt.Errorf("writer: write header: %w", err)
	}
	for _, sample := range sheet.Data {
		row := make([]string, len(header))
		for i, col := range header {
			row[i] = csvSafe(fieldFor(sample, col))
		}
		if err := writeCSVRow(&buf, cw, row); err != nil {
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

// writeCSVRow writes one CSV record.
//
// A record with a single empty cell would be written as a blank line,
// and the reader skips blank lines: the cell would be lost on the
// round-trip. Such a record is written as the quoted empty field
// instead, which the reader reads back as one empty cell.
func writeCSVRow(buf *bytes.Buffer, cw *csv.Writer, row []string) error {
	if len(row) == 1 && row[0] == "" {
		// The csv writer buffers its rows, so it has to be flushed
		// before anything is written to buf directly.
		cw.Flush()
		if err := cw.Error(); err != nil {
			return err
		}
		buf.WriteString("\"\"\n")
		return nil
	}
	return cw.Write(row)
}

// WriteV2 writes sheet in v2 (IEM sectioned) format.
//
// The specification says that a sample sheet must begin with the
// Header section and end with the Data section; the sections in
// between may be in any order. The writer follows that order, so its
// output is the layout Illumina's own template uses.
func WriteV2(w io.Writer, sheet *model.SampleSheet) error {
	var buf bytes.Buffer

	// [Header] comes first, if the sheet has one.
	if err := writeV2Header(&buf, sheet); err != nil {
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
			if name == "Header" {
				continue // already written
			}
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

	// [Data] comes last.
	if err := writeSectionHeader(&buf, "Data"); err != nil {
		return err
	}
	header := v2DataHeader(sheet)
	if err := writeV2Data(&buf, header, sheet.Data); err != nil {
		return err
	}

	if _, err := io.WriteString(w, buf.String()); err != nil {
		return fmt.Errorf("writer: write output: %w", err)
	}
	return nil
}

// writeV2Header writes the [Header] section: the run metadata that
// Illumina's specification puts at the top of every v2 sample sheet.
//
// The records come from sheet.Header. A sheet that carries the
// section as an unknown section instead is written from there.
//
// A sheet with no records at all still gets the section, with the
// file format version the writer knows: a v2 sheet must begin with
// the Header section, and a sheet the library writes must be one the
// library accepts.
func writeV2Header(buf *bytes.Buffer, sheet *model.SampleSheet) error {
	if len(sheet.Header) == 0 {
		sec, ok := sheet.Extra["Header"]
		if !ok {
			if err := writeSectionHeader(buf, "Header"); err != nil {
				return err
			}
			cw := csv.NewWriter(buf)
			cw.UseCRLF = false
			if err := cw.Write([]string{"FileFormatVersion", "2"}); err != nil {
				return err
			}
			cw.Flush()
			return cw.Error()
		}
		if err := writeSectionHeader(buf, sec.Name); err != nil {
			return err
		}
		return writeV2GenericSection(buf, sec)
	}
	if err := writeSectionHeader(buf, "Header"); err != nil {
		return err
	}
	keys := make([]string, 0, len(sheet.Header))
	for k := range sheet.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	cw := csv.NewWriter(buf)
	cw.UseCRLF = false
	for _, k := range keys {
		if err := cw.Write([]string{csvSafe(k), csvSafe(sheet.Header[k])}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// writeSectionHeader writes "[<name>]\n" to buf.
func writeSectionHeader(buf *bytes.Buffer, name string) error {
	if _, err := fmt.Fprintf(buf, "[%s]\n", name); err != nil {
		return fmt.Errorf("writer: write section header: %w", err)
	}
	return nil
}

// v2DataHeader computes the column list to emit in the [Data] section.
// It starts from the canonical v2 column order, then appends any
// columns from the source header (in source order) that are not already
// present, then appends any keys from sample.Extra that are not yet
// included (sorted for determinism). This preserves the round-trip
// property for sheets that carry DRAGEN- or BaseSpace-specific columns.
//
// Both passes skip columns whose lower-cased name is a known alias of a
// canonical column (e.g. "i7indexid" when "I7_Index_ID" is already
// present). Without this, a v1 sheet using the no-underscore alias
// would be round-tripped as two columns: the canonical one and the
// alias.
func v2DataHeader(sheet *model.SampleSheet) []string {
	cols := append([]string(nil), canonicalColumnsV2...)
	seen := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		seen[strings.ToLower(c)] = struct{}{}
	}
	// Source-header order: append any column that isn't already there.
	for _, h := range sheet.Columns {
		key := strings.ToLower(strings.TrimSpace(h))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		// Recognise aliases of canonical columns. canonicalAlias
		// returns the canonical name (or the input itself if no
		// alias is known); if the canonical name is already in
		// seen, skip the alias column.
		if _, ok := seen[strings.ToLower(canonicalAlias(h))]; ok {
			continue
		}
		seen[key] = struct{}{}
		cols = append(cols, h)
	}
	// Any Extra keys across all samples that aren't yet included.
	var extraKeys []string
	for _, s := range sheet.Data {
		for k := range s.Extra {
			lk := strings.ToLower(k)
			if _, ok := seen[lk]; ok {
				continue
			}
			if _, ok := seen[strings.ToLower(canonicalAlias(k))]; ok {
				continue
			}
			seen[lk] = struct{}{}
			extraKeys = append(extraKeys, k)
		}
	}
	sort.Strings(extraKeys)
	cols = append(cols, extraKeys...)
	return cols
}

// writeV2Data writes the [Data] section of a v2 sheet using the given
// header. The header should already be computed by v2DataHeader.
func writeV2Data(buf *bytes.Buffer, header []string, data []model.Sample) error {
	cw := csv.NewWriter(buf)
	cw.UseCRLF = false
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, sample := range data {
		row := make([]string, len(header))
		for i, col := range header {
			row[i] = csvSafe(fieldFor(sample, col))
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// writeV2Reads writes the [Reads] section in vertical key/value form.
// writeV2Reads writes the [Reads] section.
//
// Illumina's own sheets write the cycle counts as "Key,Value" records,
// one per line ("Read1Cycles,151"), and the specification describes a
// standalone section the same way.
func writeV2Reads(buf *bytes.Buffer, reads *model.Reads) error {
	cw := csv.NewWriter(buf)
	cw.UseCRLF = false
	write := func(key string, n int) error {
		if n == 0 {
			return nil
		}
		return cw.Write([]string{key, strconv.Itoa(n)})
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
	cw.Flush()
	return cw.Error()
}

// writeV2Settings writes the [Settings] section as a 2-column CSV.
// writeV2Settings writes the [Settings] section.
//
// Illumina's own sheets write the settings as "Key,Value" records, one
// per line ("AdapterRead1,AGATCGGAAGAGCACACGTCTGAACTCCAGTCA"), and
// the specification describes a standalone section the same way.
//
// The typed fields come first, then the Extra keys sorted. A key that
// is already a record is skipped: the specification says the column
// headers of a section must all be unique, and the typed field is the
// one that holds the value.
func writeV2Settings(buf *bytes.Buffer, s *model.Settings) error {
	records := make([][]string, 0, 2+len(s.Extra))
	seen := make(map[string]bool, 2+len(s.Extra))
	add := func(key, value string) {
		lower := strings.ToLower(key)
		if seen[lower] {
			return
		}
		seen[lower] = true
		records = append(records, []string{key, value})
	}
	if s.Adapter != "" {
		add("Adapter", s.Adapter)
	}
	if s.AdapterRead2 != "" {
		add("AdapterRead2", s.AdapterRead2)
	}
	extra := make([]string, 0, len(s.Extra))
	for k := range s.Extra {
		extra = append(extra, k)
	}
	sort.Strings(extra)
	for _, k := range extra {
		add(k, s.Extra[k])
	}
	if len(records) == 0 {
		return nil
	}
	cw := csv.NewWriter(buf)
	cw.UseCRLF = false
	for _, rec := range records {
		if err := cw.Write([]string{csvSafe(rec[0]), csvSafe(rec[1])}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// writeV2Manifests writes the [Manifests] section.
func writeV2Manifests(buf *bytes.Buffer, manifests []model.Manifest) error {
	cw := csv.NewWriter(buf)
	cw.UseCRLF = false
	for _, m := range manifests {
		// Illumina's specification writes "Key,Filename" pairs; a
		// manifest without a key is written as its path alone.
		row := []string{csvSafe(m.Filename)}
		if m.Key != "" {
			row = []string{csvSafe(m.Key), csvSafe(m.Filename)}
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// writeV2GenericSection writes a section we did not model explicitly.
func writeV2GenericSection(buf *bytes.Buffer, sec model.Section) error {
	if len(sec.Rows) == 0 {
		return nil
	}
	cw := csv.NewWriter(buf)
	cw.UseCRLF = false
	for _, row := range sec.Rows {
		// Apply csvSafe to every cell; the rows in Extra come from
		// arbitrary user-controlled sections.
		safe := make([]string, len(row))
		for i, cell := range row {
			safe[i] = csvSafe(cell)
		}
		if err := cw.Write(safe); err != nil {
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

// fieldFor returns the value of sample for the given column name. The
// lookup is case-insensitive via the model column registry; unknown
// columns fall back to Extra.
func fieldFor(sample model.Sample, col string) string {
	if c := model.ColumnByName(col); c != nil {
		return c.Get(&sample)
	}
	if v, ok := sample.Extra[col]; ok {
		return v
	}
	return ""
}

// csvSafe escapes a value that may otherwise be interpreted as a
// formula by Excel, Google Sheets, or LibreOffice. We prepend a single
// quote (`'`) which those applications treat as a literal-string
// indicator. encoding/csv does not escape these characters on its own.
//
// The parser strips that prefix again on read (see unescapeCSVSafe in
// the parser package), so a parse → write → parse round-trip returns
// the original value.
//
// Reference: OWASP "CSV Injection".
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// canonicalAlias returns the canonical column name for the given
// header, or the input itself if no alias is known. The v2 writer
// uses this to deduplicate aliased columns (e.g. "I7IndexID" and
// "I7_Index_ID" should not both be emitted).
func canonicalAlias(name string) string {
	if c := model.ColumnByName(name); c != nil {
		return c.CanonicalName()
	}
	return name
}
