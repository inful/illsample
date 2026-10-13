// Package parser reads Illumina sample sheets from an io.Reader.
//
// Two layouts are supported:
//
//   - v1 (legacy): a single CSV with a header row. The column order does
//     not matter; columns are matched by name.
//
//   - v2 (IEM): a sequence of sections, each introduced by a [Name]
//     header. [Data] is a CSV section, [Reads] is a vertical key/value
//     section, and [Settings] is a 2-column CSV section. Unknown
//     sections are preserved verbatim in SampleSheet.Extra so that
//     round-trip writing is lossless.
//
// The top-level Parse function auto-detects the layout by inspecting
// the first non-empty line. Callers that need a specific variant should
// use ParseV1 or ParseV2.
package parser

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/inful/illsample/internal/model"
)

// Parse reads a sample sheet from r and returns the parsed model. The
// on-disk layout is auto-detected.
func Parse(r io.Reader) (*model.SampleSheet, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read sample sheet: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes is like Parse but takes a byte slice. The supplied slice
// may begin with a UTF-8 BOM, which is stripped before parsing.
func ParseBytes(data []byte) (*model.SampleSheet, error) {
	text := stripBOM(data)
	if !utf8.ValidString(text) {
		return nil, &ParseError{Msg: "input is not valid UTF-8"}
	}
	lines := splitLines(text)
	if len(stripBlankLines(lines)) == 0 {
		return nil, &ParseError{Msg: "empty input"}
	}
	if isV2(lines) {
		return parseV2(lines)
	}
	return parseV1(data)
}

// ParseV1 parses a v1 (legacy single-section CSV) sample sheet.
func ParseV1(r io.Reader) (*model.SampleSheet, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read sample sheet: %w", err)
	}
	return parseV1(data)
}

// ParseV2 parses a v2 (IEM sectioned) sample sheet.
func ParseV2(r io.Reader) (*model.SampleSheet, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read sample sheet: %w", err)
	}
	text := stripBOM(data)
	lines := splitLines(text)
	if len(stripBlankLines(lines)) == 0 {
		return nil, &ParseError{Msg: "empty input"}
	}
	return parseV2(lines)
}

// ParseError describes a failure to parse a sample sheet. The Line
// field is the 1-based line number where the error was detected, or 0
// if the error does not apply to a specific line.
type ParseError struct {
	Line int
	Msg  string
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
	}
	return e.Msg
}

// Is allows errors.Is to match *ParseError values.
func (e *ParseError) Is(target error) bool {
	_, ok := target.(*ParseError)
	return ok
}

// stripBOM removes a leading UTF-8 byte-order mark if present.
func stripBOM(b []byte) string {
	const bom = "\uFEFF"
	s := string(b)
	return strings.TrimPrefix(s, bom)
}

// splitLines breaks s on \r\n, \n, or \r and returns each line without
// the terminator.
func splitLines(s string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	// Sample sheets are small; raise the buffer cap so a pathological
	// quoted field with embedded newlines still fits.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// stripBlankLines returns lines with the leading and trailing empty
// lines removed. Empty lines in the middle are preserved so that line
// numbers stay accurate for error messages.
func stripBlankLines(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

// isV2 reports whether the first non-empty, non-comment line of lines
// is a [Section] header. v2 sheets always start with a section header
// (almost always [Data]); v1 sheets start with the column header row.
func isV2(lines []string) bool {
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return isSectionHeader(t)
	}
	return false
}

// isSectionHeader reports whether s looks like a [Name] section header.
func isSectionHeader(s string) bool {
	return strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") && len(s) >= 2
}

// sectionName extracts the section name from a [Name] header.
func sectionName(s string) string {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	return s
}

// csvRecord is one parsed CSV row. It exists to keep call sites concise.
type csvRecord []string

// readCSV parses the given raw lines (already trimmed of section
// delimiters) as CSV. startLine is the 1-based line number of the first
// line in raw, used for error messages.
func readCSV(raw []string, startLine int) ([]csvRecord, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	r := csv.NewReader(bytes.NewReader([]byte(strings.Join(raw, "\n"))))
	r.FieldsPerRecord = -1 // tolerate ragged rows
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, &ParseError{
			Line: startLine + byteOffsetToLine(raw, err),
			Msg:  fmt.Sprintf("invalid CSV: %v", err),
		}
	}
	out := make([]csvRecord, 0, len(records))
	for _, rec := range records {
		cp := make(csvRecord, len(rec))
		copy(cp, rec)
		out = append(out, cp)
	}
	return out, nil
}

// byteOffsetToLine is a best-effort conversion of a CSV error position
// into a 1-based line number. encoding/csv returns errors of the form
// "xxx on line N"; we fall back to startLine when the position can't be
// determined.
func byteOffsetToLine(raw []string, err error) int {
	msg := err.Error()
	const marker = "line "
	idx := strings.LastIndex(msg, marker)
	if idx < 0 {
		return 0
	}
	tail := msg[idx+len(marker):]
	end := 0
	for end < len(tail) && tail[end] >= '0' && tail[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n := 0
	for i := 0; i < end; i++ {
		n = n*10 + int(tail[i]-'0')
	}
	if n <= 0 || n > len(raw) {
		return 0
	}
	return n - 1
}

// peek returns the line at offset n ahead of the current position
// without advancing.
func (p *lineParser) peek(offset int) (string, bool) {
	if p.pos+offset >= len(p.lines) {
		return "", false
	}
	return p.lines[p.pos+offset], true
}

// lineParser is a small cursor over a slice of lines.
type lineParser struct {
	lines []string
	pos   int
}

// next returns the current line and advances the cursor.
func (p *lineParser) next() (string, bool) {
	if p.pos >= len(p.lines) {
		return "", false
	}
	line := p.lines[p.pos]
	p.pos++
	return line, true
}

// eof reports whether the cursor is past the last line.
func (p *lineParser) eof() bool {
	return p.pos >= len(p.lines)
}

// findNextSection returns the index of the next [Section] header at or
// after the current position, or len(p.lines) if there is none.
//
//nolint:unused // retained for future use
func (p *lineParser) findNextSection() int {
	for i := p.pos; i < len(p.lines); i++ {
		if isSectionHeader(strings.TrimSpace(p.lines[i])) {
			return i
		}
	}
	return len(p.lines)
}

// peekIsNextSection reports whether the next non-blank, non-comment
// line is a [Section] header.
//
//nolint:unused // retained for future use
func (p *lineParser) peekIsNextSection() bool {
	for i := p.pos; i < len(p.lines); i++ {
		t := strings.TrimSpace(p.lines[i])
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return isSectionHeader(t)
	}
	return false
}

// skipBlankAndComments advances past blank and comment lines.
func (p *lineParser) skipBlankAndComments() {
	for p.pos < len(p.lines) {
		t := strings.TrimSpace(p.lines[p.pos])
		if t == "" || strings.HasPrefix(t, "#") {
			p.pos++
			continue
		}
		return
	}
}

// lineNumber returns the 1-based line number of the cursor's current
// position.
func (p *lineParser) lineNumber() int {
	return p.pos + 1
}

// errAt returns a *ParseError anchored at the current line.
func (p *lineParser) errAt(format string, args ...any) error {
	return &ParseError{Line: p.lineNumber(), Msg: fmt.Sprintf(format, args...)}
}

// comment reports whether s (already trimmed) is a comment line.
func comment(s string) bool {
	return strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//")
}
