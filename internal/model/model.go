// Package model defines the in-memory representation of an Illumina sample
// sheet. The same SampleSheet value can describe either a v1 (legacy CSV)
// or v2 (IEM sectioned) layout; the Format field records which variant is
// in use.
package model

import "strings"

// Format identifies the on-disk layout of a sample sheet.
type Format int

const (
	// FormatUnknown is the zero value. Callers should not produce it; the
	// writer treats it as a synonym for FormatV2.
	FormatUnknown Format = iota
	// FormatV1 is the legacy single-section CSV layout.
	FormatV1
	// FormatV2 is the IEM sectioned layout ([Data], [Reads], [Settings], ...).
	FormatV2
)

// String returns the canonical short name of the format.
func (f Format) String() string {
	switch f {
	case FormatV1:
		return "v1"
	case FormatV2:
		return "v2"
	default:
		return "unknown"
	}
}

// SampleSheet is the in-memory representation of an Illumina sample sheet.
// Fields that only apply to v2 are left nil for v1 sheets and vice versa.
type SampleSheet struct {
	// Format is the layout version. Set by the parser; the writer uses it
	// to choose the appropriate serializer.
	Format Format
	// Header records the column order of the data section, populated
	// from the source for both v1 and v2 sheets. The v2 writer always
	// emits the canonical column order regardless of Header, so Header
	// is informational; the verifier and the v1 writer use it to
	// detect missing columns and to round-trip column order.
	Header []string
	// Reads holds the [Reads] section of a v2 sheet. Nil for v1.
	Reads *Reads
	// Settings holds the [Settings] section of a v2 sheet. Nil for v1.
	Settings *Settings
	// Manifests is the rows of the [Manifests] section of a v2 sheet, if
	// present. bcl2fastq treats an empty Manifests section as "no
	// manifest" so the parser only populates this when at least one row
	// was read.
	Manifests []Manifest
	// Data is the [Data] section (or, for v1, the only section). The
	// order of rows in Data matches the order in the source.
	Data []Sample
	// Extra preserves any v2 sections that the model does not capture
	// explicitly, so round-trip writing is lossless. Nil for v1.
	Extra map[string]Section
}

// Section is a generic v2 section (anything other than the modeled ones).
type Section struct {
	// Name is the section name as it appeared in the source, without the
	// surrounding square brackets (e.g. "Data", "Settings").
	Name string
	// Columns is the first non-empty row of the section, which carries
	// the column names. May be empty for sections that have no header.
	Columns []string
	// Rows is the data rows of the section. For headered sections, the
	// first row of Rows is the same as Columns; subsequent rows are the
	// data.
	Rows [][]string
}

// Sample is a single row in the data section. Columns that the model
// understands are mapped to typed fields; the original column name is used
// to do the mapping, so column order in the source does not matter. Any
// columns the model does not recognise are preserved in Extra so that
// round-trip writing does not drop data.
type Sample struct {
	SampleID      string
	SampleName    string
	SamplePlate   string
	SampleWell    string
	I7IndexID     string
	Index         string
	I5IndexID     string
	Index2        string
	SampleProject string
	Description   string
	// Extra preserves columns the model did not consume, keyed by the
	// original column name from the source header. Nil when there are no
	// such columns.
	Extra map[string]string
	// SourceRow is the zero-based row index in the source document, used
	// for diagnostic messages. -1 for samples that were constructed
	// programmatically.
	SourceRow int
}

// Reads describes the [Reads] section of a v2 sheet. A zero value means the
// section was absent or unspecified.
type Reads struct {
	Read1Cycles  int
	Read2Cycles  int
	Index1Cycles int
	Index2Cycles int
}

// Settings describes the [Settings] section of a v2 sheet. The two most
// common keys are promoted to typed fields; anything else is preserved in
// Extra so that round-trip writing is lossless.
type Settings struct {
	Adapter      string
	AdapterRead2 string
	Extra        map[string]string
}

// Manifest is one row of the [Manifests] section. The IEM spec allows up to
// six columns labelled A through F; we model them as positional fields. A
// row with fewer than six values has empty strings for the missing ones.
type Manifest struct {
	A string
	B string
	C string
	D string
	E string
	F string
}

// Column describes a known column in the [Data] section. The first
// entry of Names is the canonical form emitted by the v2 writer;
// subsequent entries are aliases that parse to the same Sample
// field. Set and Get read and write that field.
//
// The parser and the writer share this single registry; previously
// each maintained its own parallel mapping, which let a renamed or
// added column get out of sync across the two paths. Adding a
// column is one entry here.
type Column struct {
	Names []string
	Set   func(*Sample, string)
	Get   func(*Sample) string
}

// CanonicalName returns the first entry of Names, which is the form
// the v2 writer emits. Returns the input unchanged when the column
// is unknown.
func (c *Column) CanonicalName() string {
	if c == nil || len(c.Names) == 0 {
		return ""
	}
	return c.Names[0]
}

// columns is the typed column registry, in the order the v2 writer
// emits them. The order is the IEM canonical v2 order; do not
// reorder without a corresponding test update.
var columns = []Column{
	{Names: []string{"Sample_ID", "SampleID"}, Set: func(s *Sample, v string) { s.SampleID = v }, Get: func(s *Sample) string { return s.SampleID }},
	{Names: []string{"Sample_Name", "SampleName"}, Set: func(s *Sample, v string) { s.SampleName = v }, Get: func(s *Sample) string { return s.SampleName }},
	{Names: []string{"Sample_Plate", "SamplePlate"}, Set: func(s *Sample, v string) { s.SamplePlate = v }, Get: func(s *Sample) string { return s.SamplePlate }},
	{Names: []string{"Sample_Well", "SampleWell"}, Set: func(s *Sample, v string) { s.SampleWell = v }, Get: func(s *Sample) string { return s.SampleWell }},
	{Names: []string{"I7_Index_ID", "I7IndexID"}, Set: func(s *Sample, v string) { s.I7IndexID = v }, Get: func(s *Sample) string { return s.I7IndexID }},
	{Names: []string{"index"}, Set: func(s *Sample, v string) { s.Index = v }, Get: func(s *Sample) string { return s.Index }},
	{Names: []string{"I5_Index_ID", "I5IndexID"}, Set: func(s *Sample, v string) { s.I5IndexID = v }, Get: func(s *Sample) string { return s.I5IndexID }},
	{Names: []string{"index2"}, Set: func(s *Sample, v string) { s.Index2 = v }, Get: func(s *Sample) string { return s.Index2 }},
	{Names: []string{"Sample_Project", "SampleProject"}, Set: func(s *Sample, v string) { s.SampleProject = v }, Get: func(s *Sample) string { return s.SampleProject }},
	{Names: []string{"Description"}, Set: func(s *Sample, v string) { s.Description = v }, Get: func(s *Sample) string { return s.Description }},
}

// ColumnByName returns the column whose name list contains the
// given header (case-insensitive), or nil. The first match wins;
// register each column with its canonical name first to make the
// lookup deterministic.
func ColumnByName(name string) *Column {
	lower := strings.ToLower(name)
	for i := range columns {
		for _, n := range columns[i].Names {
			if strings.ToLower(n) == lower {
				return &columns[i]
			}
		}
	}
	return nil
}

// CanonicalColumnNames returns the canonical column names in the
// order the v2 writer emits them.
func CanonicalColumnNames() []string {
	out := make([]string, len(columns))
	for i, c := range columns {
		out[i] = c.Names[0]
	}
	return out
}
