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
	// splitLines from a string already validated by ParseBytes; for
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

	// Skip any preamble (comments, blank lines) before the first section.
	for !p.eof() {
		t := strings.TrimSpace(p.lines[p.pos])
		if t == "" || comment(t) {
			p.pos++
			continue
		}
		if isSectionHeader(t) {
			break
		}
		return nil, p.errAt("expected [Section] header, got %q", t)
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
		case "data":
			if err := parseDataSection(p, sheet); err != nil {
				return nil, err
			}
		case "reads":
			if err := parseReadsSection(p, sheet); err != nil {
				return nil, err
			}
		case "settings":
			if err := parseSettingsSection(p, sheet); err != nil {
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
	startLine := p.lineNumber()
	raw, err := collectSection(p)
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
		sample, err := v1RowToSample(row, header, colIdx, startLine+rowNum+1)
		if err != nil {
			return err
		}
		sheet.Data = append(sheet.Data, sample)
	}
	return nil
}

// parseReadsSection reads a [Reads] section, which is a vertical
// key/value list: keys on one line, values on the next, repeating.
func parseReadsSection(p *lineParser, sheet *model.SampleSheet) error {
	if sheet.Reads == nil {
		sheet.Reads = &model.Reads{}
	}
	var key string
	for {
		line, ok := p.peek(0)
		if !ok {
			break
		}
		trimmed := strings.TrimSpace(line)
		if isSectionHeader(trimmed) {
			break
		}
		if trimmed == "" || comment(trimmed) {
			p.next()
			continue
		}
		p.next()
		if key == "" {
			key = trimmed
			continue
		}
		n, err := strconv.Atoi(trimmed)
		if err != nil {
			return p.errAt("[Reads] value for %q must be an integer, got %q", key, trimmed)
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

// parseSettingsSection reads a [Settings] section, which is a 2-column
// CSV: the first row is the header, subsequent rows are values. When
// the section contains more than one data row, later rows override
// earlier ones (last-non-empty wins). This is the behaviour bcl2fastq
// effectively implements and is what users editing a sheet by hand
// usually expect.
func parseSettingsSection(p *lineParser, sheet *model.SampleSheet) error {
	startLine := p.lineNumber()
	raw, err := collectSection(p)
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
		switch key {
		case "Adapter":
			sheet.Settings.Adapter = value
		case "AdapterRead2":
			sheet.Settings.AdapterRead2 = value
		default:
			if sheet.Settings.Extra == nil {
				sheet.Settings.Extra = make(map[string]string)
			}
			sheet.Settings.Extra[key] = value
		}
	}
	return nil
}

// parseManifestsSection reads a [Manifests] section. IEM allows a single
// column of manifest identifiers (A, B, ...). Some real-world sheets
// have a header row that duplicates the column letters; we treat any
// row whose single field is just a column letter as a header to skip.
func parseManifestsSection(p *lineParser, sheet *model.SampleSheet) error {
	for {
		line, ok := p.peek(0)
		if !ok {
			break
		}
		trimmed := strings.TrimSpace(line)
		if isSectionHeader(trimmed) {
			break
		}
		if trimmed == "" || comment(trimmed) {
			p.next()
			continue
		}
		p.next()
		// Skip a literal "A" header line; treat anything else as a row.
		if trimmed == "A" && len(sheet.Manifests) == 0 {
			continue
		}
		m := model.Manifest{A: trimmed}
		sheet.Manifests = append(sheet.Manifests, m)
	}
	return nil
}

// parseUnknownSection preserves a section that the model does not know
// about. Each line is parsed as a single-column CSV record so quoted
// fields are handled correctly.
func parseUnknownSection(p *lineParser, sheet *model.SampleSheet, rawHeader string) error {
	name := sectionName(rawHeader)
	// Collect the raw lines first, then decide whether to parse as CSV.
	raw, err := collectSection(p)
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
	records, err := readCSV(raw, p.lineNumber()-len(raw))
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

// collectSection gathers the raw lines belonging to the current section
// (i.e. everything up to the next [Section] header or EOF), skipping
// blank lines and comments. The cursor is advanced past the section
// contents but NOT past the next section header.
func collectSection(p *lineParser) ([]string, error) {
	var raw []string
	for {
		line, ok := p.peek(0)
		if !ok {
			return raw, nil
		}
		trimmed := strings.TrimSpace(line)
		if isSectionHeader(trimmed) {
			return raw, nil
		}
		if trimmed == "" || comment(trimmed) {
			p.next()
			continue
		}
		raw = append(raw, line)
		p.next()
	}
}
