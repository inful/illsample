package parser

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/inful/illsample/internal/model"
)

// parseV2 parses a v2 (IEM sectioned) sample sheet. The first non-blank
// line is expected to be a [Section] header; any preceding text is
// silently ignored (it is sometimes used for human-readable comments).
// Non-UTF-8 input is rejected with a ParseError so that callers using
// ParseV2 directly get the same behaviour as callers using the
// auto-detecting Parse.
func parseV2(lines []string) (*model.SampleSheet, error) {
	// Validate the entire line slice as UTF-8. This is a fast pass
	// over the slice's underlying string (lines is built by
	// splitLines from a string already validated by parseBytes; for
	// direct ParseV2 callers, we re-validate here).
	for _, line := range lines {
		if !utf8.ValidString(line) {
			return nil, &ParseError{Line: 1, Msg: "input is not valid UTF-8"}
		}
	}
	sheet := &model.SampleSheet{
		Format: model.FormatV2,
	}
	p := &lineParser{lines: lines}

	// Skip any preamble (comments, blank lines, free text) before the
	// first section.
	for !p.eof() {
		t := strings.TrimSpace(p.lines[p.pos])
		if isBlankLine(t) || comment(t) || isInstructionLine(t) {
			p.pos++
			continue
		}
		if isSectionHeader(t) {
			break
		}
		if !preambleLine(t) {
			return nil, p.errAt("expected [Section] header, got %q", t)
		}
		p.pos++
	}

	for !p.eof() {
		// Skip blanks and comments between sections.
		p.skipBlankAndComments()
		if p.eof() {
			break
		}
		header := strings.TrimSpace(p.lines[p.pos])
		if !isSectionHeader(header) {
			return nil, p.errAt("expected [Section] header, got %q", header)
		}
		name := strings.ToLower(sectionName(header))
		p.pos++ // consume the section header

		switch name {
		case "data", "bclconvert_data":
			// Illumina's BCL Convert dialect names the data section
			// [BCLConvert_Data]; IEM names it [Data].
			if err := parseDataSection(p, sheet); err != nil {
				return nil, err
			}
		case "reads":
			if err := parseReadsSection(p, sheet); err != nil {
				return nil, err
			}
		case "settings", "bclconvert_settings":
			// The two dialects spell the settings section
			// differently and lay it out differently: [Settings]
			// is a header row plus a value row, while
			// [BCLConvert_Settings] is one "Key,Value" pair per row.
			if err := parseSettingsSection(p, sheet, name == "bclconvert_settings"); err != nil {
				return nil, err
			}
		case "manifests":
			if err := parseManifestsSection(p, sheet); err != nil {
				return nil, err
			}
		default:
			if err := parseUnknownSection(p, sheet, header); err != nil {
				return nil, err
			}
		}
	}
	return sheet, nil
}

// parseDataSection reads a [Data] section. The section is a header line
// followed by data rows, all CSV. The section ends at the next [Section]
// header or EOF.
func parseDataSection(p *lineParser, sheet *model.SampleSheet) error {
	raw, startLine, err := collectSection(p)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		// An empty [Data] section is valid: it just means no samples.
		// We leave sheet.Data as nil; the verifier will flag a sheet
		// with no samples.
		return nil
	}
	records, err := readCSV(raw, startLine)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	header := records[0]
	// Trim whitespace from the header names, like the v1 parser does.
	// Without this the writer looks the column up as " Notes" in
	// sample.Extra, where the key is "Notes", and the column's values
	// are dropped on round-trip.
	for i, h := range header {
		header[i] = strings.TrimSpace(h)
	}
	sheet.Header = append(sheet.Header, header...)
	colIdx := make(map[string]int, len(header))
	for i, name := range header {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if _, ok := colIdx[key]; !ok {
			colIdx[key] = i
		}
	}
	for rowNum, row := range records[1:] {
		sheet.Data = append(sheet.Data, v1RowToSample(row, header, colIdx, startLine+rowNum+1))
	}
	return nil
}

// parseReadsSection reads a [Reads] section. Three layouts exist.
//
// Illumina's Sample Sheet specification holds one cycle count per
// line, in order: read 1, then read 2. The index reads are not part
// of this section; their length follows from the index sequence.
//
// Illumina's BCL Convert dialect writes "Key,Value" on a single
// line, and IEM puts the key on one line and the value on the next.
// A line that contains a comma is the former, a line that is a plain
// number is the specification's layout, anything else is a key whose
// value is on the next line.
func parseReadsSection(p *lineParser, sheet *model.SampleSheet) error {
	if sheet.Reads == nil {
		sheet.Reads = &model.Reads{}
	}
	var key string
	read := 0 // the next bare cycle count: read 1, then read 2
	for {
		line, ok := p.peek(0)
		if !ok {
			break
		}
		trimmed := strings.TrimSpace(line)
		if isSectionHeader(trimmed) {
			break
		}
		if isBlankLine(trimmed) || comment(trimmed) || isInstructionLine(trimmed) {
			p.next()
			continue
		}
		p.next()
		value := ""
		if key == "" {
			if k, v, found := strings.Cut(trimmed, ","); found {
				// The value is the second field; the commas after it
				// are the padding the specification allows.
				v, _, _ = strings.Cut(v, ",")
				key, value = strings.TrimSpace(k), strings.TrimSpace(v)
			} else if n, err := strconv.Atoi(trimmed); err == nil {
				// A plain number: the cycle count of the next read.
				if read == 2 {
					return p.errAt("[Reads] holds one cycle count per read (read 1, read 2), got a third")
				}
				if read == 0 {
					sheet.Reads.Read1Cycles = n
				} else {
					sheet.Reads.Read2Cycles = n
				}
				read++
				continue
			} else {
				key = trimmed
				continue
			}
		} else {
			value = trimmed
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return p.errAt("[Reads] value for %q must be an integer, got %q", key, value)
		}
		switch key {
		case "Read1Cycles":
			sheet.Reads.Read1Cycles = n
		case "Read2Cycles":
			sheet.Reads.Read2Cycles = n
		case "Index1Cycles":
			sheet.Reads.Index1Cycles = n
		case "Index2Cycles":
			sheet.Reads.Index2Cycles = n
		default:
			return p.errAt("unknown [Reads] key %q", key)
		}
		key = ""
	}
	if key != "" {
		return p.errAt("[Reads] key %q has no value", key)
	}
	return nil
}

// parseSettingsSection reads a settings section.
//
// Three layouts exist. Illumina's specification and the MiSeq guide
// both write one "Key,Value" pair per line. Illumina's BCL Convert
// dialect writes the same pairs under a different section name, and
// IEM's [Settings] is a table: the first row holds the setting names,
// the rows after it hold the values. A section with a single column
// is a vertical list: the key is on one line and its value on the
// next.
//
// In the header/value layout, later rows override earlier ones
// (last-non-empty wins). This is the behaviour bcl2fastq effectively
// implements and is what users editing a sheet by hand usually expect.
func parseSettingsSection(p *lineParser, sheet *model.SampleSheet, pairForm bool) error {
	raw, startLine, err := collectSection(p)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	records, err := readCSV(raw, startLine)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	if sheet.Settings == nil {
		sheet.Settings = &model.Settings{}
	}
	if pairForm || isPairSettings(records) {
		for _, rec := range records {
			if len(rec) == 0 {
				continue
			}
			key := strings.TrimSpace(rec[0])
			if key == "" {
				continue
			}
			value := ""
			if len(rec) > 1 {
				value = strings.TrimSpace(rec[1])
			}
			applySetting(sheet.Settings, key, value)
		}
		return nil
	}
	if len(records[0]) == 1 {
		// A single-column section is a vertical key/value list.
		applyVerticalSettings(sheet.Settings, records)
		return nil
	}
	header := records[0]
	for col, name := range header {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		if len(records) < 2 {
			// Header without any data row is a no-op.
			continue
		}
		// Walk all data rows; the last non-empty value wins.
		var value string
		for _, rec := range records[1:] {
			if col >= len(rec) {
				continue
			}
			v := strings.TrimSpace(rec[col])
			if v != "" {
				value = v
			}
		}
		applySetting(sheet.Settings, key, value)
	}
	return nil
}

// applySetting stores one settings key/value pair. The key lookup is
// case-insensitive. The IEM spec uses canonical case (Adapter,
// AdapterRead2) but real-world sheets sometimes use lowercase, and the
// v1 path is already case-insensitive — we want a consistent story.
//
// "AdapterRead1" (Illumina's name for the Read 1 adapter) is kept in
// Extra rather than mapped onto Adapter, so that a round-trip writes
// the key back under the name the source used.
func applySetting(s *model.Settings, key, value string) {
	switch strings.ToLower(key) {
	case "adapter":
		s.Adapter = value
	case "adapterread2":
		s.AdapterRead2 = value
	default:
		if s.Extra == nil {
			s.Extra = make(map[string]string)
		}
		s.Extra[key] = value
	}
}

// isPairSettings reports whether a settings section is a list of
// "Key,Value" pairs, the layout in Illumina's Sample Sheet
// specification, rather than a table with a header row.
//
// The two layouts look alike when the table has two columns: in the
// pair layout the second field of the first record is a value, in the
// table layout it is the name of the next setting.
func isPairSettings(records []csvRecord) bool {
	if len(records) == 0 || len(records[0]) != 2 {
		return false
	}
	if !isSettingName(records[0][1]) {
		return true
	}
	// With a second record the table layout holds values there, while
	// the pair layout holds the name of the next setting.
	return len(records) > 1 && isSettingName(records[1][0])
}

// isSettingName reports whether a field looks like the name of a
// setting rather than its value. Illumina's setting names are words
// ("Adapter", "outputgenomevcf"); its values are sequences, numbers
// or flags.
func isSettingName(s string) bool {
	if s == "" {
		return false
	}
	if isSettingsKey(s) {
		return true
	}
	// A lower-case word without digits is a name too.
	return s == strings.ToLower(s) && !strings.ContainsAny(s, "0123456789")
}

// applyVerticalSettings reads a single-column [Settings] section: the
// key is on one line and its value on the next.
//
// A line that names a setting starts a new key; every other line is
// another value for the key above it, and the last non-empty value
// wins. Without the "names a setting" test a vertical section with
// two keys would put the second key's value into the first key and
// drop the second key entirely — the sheet would then carry the
// Read 2 adapter as the Read 1 adapter.
func applyVerticalSettings(s *model.Settings, records []csvRecord) {
	key := ""
	value := ""
	for _, rec := range records {
		line := strings.TrimSpace(strings.Join(rec, ","))
		if line == "" {
			continue
		}
		if key == "" || isSettingsKey(line) {
			if key != "" && value != "" {
				applySetting(s, key, value)
			}
			key, value = line, ""
			continue
		}
		value = line
	}
	if key != "" && value != "" {
		applySetting(s, key, value)
	}
}

// isSettingsKey reports whether a line in a single-column [Settings]
// section names a setting.
//
// Illumina setting names are CamelCase ("Adapter", "AdapterRead2",
// "SoftwareVersion") or contain an underscore; their values are DNA
// sequences or version numbers. A line that looks like a name starts
// a new key.
//
// shortcut: a value that looks like a name is read as a key. The
// values Illumina writes are sequences and version numbers, so this
// needs a hand-edited sheet to hit; upgrade by requiring the key to
// be a name the parser knows.
func isSettingsKey(line string) bool {
	switch strings.ToLower(line) {
	case "adapter", "adapterread1", "adapterread2":
		return true
	}
	if strings.Contains(line, "_") {
		return true
	}
	for i := 0; i+1 < len(line); i++ {
		if line[i] >= 'A' && line[i] <= 'Z' && line[i+1] >= 'a' && line[i+1] <= 'z' {
			return true
		}
	}
	return false
}

// parseManifestsSection reads a [Manifests] section.
//
// Illumina's specification writes one "Key,Filename" pair per line;
// the key is the label that the Data section's Manifest column refers
// to. IEM writes one path per line under a column letter, which is
// read as the key of the paths below it.
func parseManifestsSection(p *lineParser, sheet *model.SampleSheet) error {
	// Collect the section's raw lines so we can look ahead.
	var raw []string
	for {
		line, ok := p.peek(0)
		if !ok {
			break
		}
		trimmed := strings.TrimSpace(line)
		if isSectionHeader(trimmed) {
			break
		}
		if isBlankLine(trimmed) || comment(trimmed) || isInstructionLine(trimmed) {
			p.next()
			continue
		}
		raw = append(raw, trimmed)
		p.next()
	}
	// A leading column letter is the key of the paths below it.
	key := ""
	if len(raw) > 0 && isColumnLetter(raw[0]) {
		key = raw[0]
		if len(raw) == 1 {
			// The section holds nothing but the column letter.
			sheet.Manifests = append(sheet.Manifests, model.Manifest{Key: key})
			return nil
		}
		raw = raw[1:]
	}
	for _, line := range raw {
		m := model.Manifest{Key: key}
		if k, filename, found := strings.Cut(line, ","); found {
			m.Key = strings.TrimSpace(k)
			m.Filename = strings.TrimSpace(filename)
		} else {
			m.Filename = line
		}
		sheet.Manifests = append(sheet.Manifests, m)
	}
	return nil
}

// isColumnLetter reports whether s is one of the column letters A to
// F that IEM uses for the [Manifests] section.
func isColumnLetter(s string) bool {
	return len(s) == 1 && s[0] >= 'A' && s[0] <= 'F'
}

// parseUnknownSection preserves a section that the model does not know
// about. Each line is parsed as a single-column CSV record so quoted
// fields are handled correctly.
func parseUnknownSection(p *lineParser, sheet *model.SampleSheet, rawHeader string) error {
	name := sectionName(rawHeader)
	// Collect the raw lines first, then decide whether to parse as CSV.
	raw, startLine, err := collectSection(p)
	if err != nil {
		return err
	}
	sec := model.Section{Name: name}
	if len(raw) == 0 {
		if sheet.Extra == nil {
			sheet.Extra = make(map[string]model.Section)
		}
		sheet.Extra[name] = sec
		return nil
	}
	records, err := readCSV(raw, startLine)
	if err != nil {
		return err
	}
	if len(records) > 0 {
		sec.Columns = records[0]
		sec.Rows = make([][]string, 0, len(records))
		for _, r := range records {
			row := make([]string, len(r))
			copy(row, r)
			sec.Rows = append(sec.Rows, row)
		}
	}
	if sheet.Extra == nil {
		sheet.Extra = make(map[string]model.Section)
	}
	sheet.Extra[name] = sec
	return nil
}

// collectSection gathers the raw lines belonging to the current
// section (i.e. everything up to the next [Section] header or EOF),
// skipping blank lines and comments. The cursor is advanced past the
// section contents but NOT past the next section header. The returned
// startLine is the 1-based line number of the first non-blank,
// non-comment line in the section; callers pass it to readCSV so
// that error messages point at the right source line even when a
// section begins with blank or comment lines.
func collectSection(p *lineParser) ([]string, int, error) {
	var raw []string
	startLine := 0
	for {
		line, ok := p.peek(0)
		if !ok {
			return raw, startLine, nil
		}
		trimmed := strings.TrimSpace(line)
		if isSectionHeader(trimmed) {
			return raw, startLine, nil
		}
		if isBlankLine(trimmed) || comment(trimmed) || isInstructionLine(trimmed) {
			p.next()
			continue
		}
		if startLine == 0 {
			startLine = p.lineNumber()
		}
		raw = append(raw, line)
		p.next()
	}
}
