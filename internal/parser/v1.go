package parser

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/inful/illsample/internal/model"
)

// parseV1 parses a v1 sample sheet. data may begin with a UTF-8 BOM.
// Non-UTF-8 input is rejected with a ParseError so that callers using
// ParseV1 directly get the same behaviour as callers using the
// auto-detecting Parse.
func parseV1(data []byte) (*model.SampleSheet, error) {
	text := stripBOM(data)
	if !utf8.ValidString(text) {
		return nil, &ParseError{Msg: "input is not valid UTF-8"}
	}
	r := csv.NewReader(bytes.NewReader([]byte(text)))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, &ParseError{Msg: fmt.Sprintf("invalid CSV: %v", err)}
	}
	if len(records) == 0 {
		return nil, &ParseError{Msg: "empty input"}
	}

	header := records[0]
	// Trim whitespace from header values. Without this, a header
	// like "Sample_ID, Sample_Name ,..." stores " Sample_Name "
	// in the column-index map; the writer's case-insensitive lookup
	// would then fail to match the typed-field case and emit an
	// empty cell, losing the data on round-trip.
	for i, h := range header {
		header[i] = strings.TrimSpace(h)
	}
	// Build the column index. Lowercased name -> column position. The
	// original header is preserved on the SampleSheet so the writer can
	// emit columns in their original order.
	colIdx := make(map[string]int, len(header))
	for i, name := range header {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		// First occurrence wins. bcl2fastq ignores duplicate column
		// names; we do the same.
		if _, ok := colIdx[key]; !ok {
			colIdx[key] = i
		}
	}

	sheet := &model.SampleSheet{
		Format: model.FormatV1,
		Header: header,
		Data:   make([]model.Sample, 0, len(records)-1),
	}

	for rowNum, row := range records[1:] {
		sheet.Data = append(sheet.Data, v1RowToSample(row, header, colIdx, rowNum+2))
	}
	return sheet, nil
}

// v1RowToSample converts one CSV row into a Sample, mapping known
// columns to typed fields and preserving the rest in Extra. line is
// the 1-based source line; it is stored on Sample.SourceRow so
// downstream code can produce accurate diagnostic messages.
func v1RowToSample(row []string, header []string, colIdx map[string]int, line int) model.Sample {
	s := model.Sample{SourceRow: line - 1}
	for lcName, col := range colIdx {
		if col >= len(row) {
			continue // row is shorter than the header
		}
		value := unescapeCSVSafe(strings.TrimSpace(row[col]))
		if c := model.ColumnByName(lcName); c != nil {
			c.Set(&s, value)
			continue
		}
		// Unknown column: preserve verbatim under its original name.
		origName := strings.TrimSpace(header[col])
		if origName == "" {
			continue
		}
		if s.Extra == nil {
			s.Extra = make(map[string]string)
		}
		s.Extra[origName] = value
	}
	return s
}
