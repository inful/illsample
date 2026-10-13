---
phase: 01-illsample-review
reviewed: 2026-10-13T09:30:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - samplesheet.go
  - example_test.go
  - samplesheet_test.go
  - internal/model/model.go
  - internal/parser/parser.go
  - internal/parser/v1.go
  - internal/parser/v2.go
  - internal/parser/parser_test.go
  - internal/writer/writer.go
  - internal/writer/writer_test.go
  - internal/verify/verify.go
  - internal/verify/rules.go
  - internal/verify/verify_test.go
  - cmd/illsample/main.go
  - cmd/illsample/cli_test.go
findings:
  critical: 2
  high: 3
  warning: 11
  info: 9
  total: 25
status: issues_found
---

# Phase 01: Code Review Report — illsample

**Reviewed:** 2026-10-13T09:30:00Z
**Depth:** standard
**Files Reviewed:** 14 source + 7 test + 5 docs/config
**Status:** issues_found

## Summary

The codebase is small, well-organised, and clearly the work of a single author who cares about
documentation. `go vet` and `golangci-lint run` are both clean, the parser is fuzz-tested, the
test suite passes with -race, and the public API is layered nicely (root package re-exports the
`internal/` types via Go type aliases so consumers don't have to import the sub-packages).

However, I found **two correctness bugs that would corrupt data on real-world input** (one of
which — the v1 column-alias map — silently mis-assigns index identifiers, which is the kind of
bug a lab would only notice after a failed demultiplex), plus **a v2 round-trip data-loss bug**
that contradicts the README's "lossless" promise, and **a CSV-injection vulnerability** in the
writer (any cell starting with `=`, `+`, `-`, or `@` is emitted verbatim and would be
evaluated as a formula when the file is opened in Excel / Google Sheets).

I also found a number of small inconsistencies between the v1 and v2 paths (UTF-8 validation,
header trimming, error line numbering), some dead-but-nolint'd code, a few documentation
staleness issues, and a couple of CLI UX problems. None of those are ship-blockers on their
own, but they are listed below in case the team wants to clean them up.

Confirmed during review (in addition to the code reading):

- `I7IndexID` column values are silently stored in `I5IndexID` (parser/v1.go:24)
- v2 [Data] round-trip drops unknown columns (writer/writer.go:175-192)
- Writer emits `=2+2`, `@SUM(...)` unescaped — CSV-injection vector
- `ParseV1` accepts non-UTF-8 input that `Parse` rejects
- A v2 sheet with multiple [Settings] data rows loses all but the first
- A v1 header with trailing whitespace makes that column's data vanish on round-trip

---

## Critical Issues

### CR-01: `I7IndexID` (no-underscore) alias maps to the wrong field

**File:** `internal/parser/v1.go:24`
**Issue:** The knownColumns map entry for the legacy-typo alias `"i7indexid"` sets
`s.I5IndexID`, not `s.I7IndexID`. The comment on the same line (`// legacy typo alias`)
suggests the author meant to handle the case where a v1 sheet uses the no-underscore form
of the column name, but the body assigns the value to the *opposite* field.

```go
"i7_index_id": func(s *model.Sample, v string) { s.I7IndexID = v },
"i7indexid":   func(s *model.Sample, v string) { s.I5IndexID = v }, // legacy typo alias
"i5_index_id": func(s *model.Sample, v string) { s.I5IndexID = v },
"i5indexid":   func(s *model.Sample, v string) { s.I5IndexID = v },
```

I confirmed the bug with a real parse:

```
input = "Sample_ID,Sample_Name,index,I7IndexID,Sample_Project\nid1,name1,ATCG,UDI001,P1\n"
sheet.Data[0].I7IndexID == ""
sheet.Data[0].I5IndexID == "UDI001"   // BUG: should be I7IndexID
```

**Why it matters:** bcl2fastq / DRAGEN will demultiplex using the value in the
wrong slot, silently assigning reads to the wrong sample. This is exactly the class
of bug that ruins a sequencing run, and it is invisible in the parsed model because
the column name looks correct.

**Fix:**

```go
"i7indexid":   func(s *model.Sample, v string) { s.I7IndexID = v },
```

Add a regression test that parses a v1 sheet with the `I7IndexID` no-underscore
column and asserts the value lands in `I7IndexID`, not `I5IndexID`.

---

### CR-02: Writer emits CSV-injection payloads verbatim

**File:** `internal/writer/writer.go` (entire file, but most importantly the
`writeV2Data`, `WriteV1`, `writeV2Settings`, and `writeV2GenericSection` paths)
**Issue:** `encoding/csv` quotes only what the CSV grammar requires (commas, quotes,
newlines). It does not escape cells that begin with `=`, `+`, `-`, `@`, `\t`,
or `\r`, which are treated as formula prefixes by Excel and Google Sheets. The writer
passes user-controlled strings straight through.

Confirmed:

```
input:  Sample_Name = "=2+2" and "@SUM(A1:A10)"
output: id1,=2+2,ATCG,P1
        id2,@SUM(A1:A10),GCTA,P2
```

The `writeV2Manifests` function is even worse: it uses `fmt.Fprintln(buf, m.A)` and
bypasses CSV quoting entirely, so a manifest path containing a comma, quote, or
newline would corrupt the output file.

**Why it matters:** Sample sheets end up in lab LIMS systems and are routinely
opened in Excel. A maliciously-constructed `Sample_Name` such as
`=HYPERLINK("evil", "Click for results")` or `=cmd|' /C calc'!A1` (DDE) will
execute as soon as the sheet is opened. The `Description` field is even more
exposed because the README calls it out as free-form text.

**Fix:** Either (a) prefix dangerous cells with a tab (Excel still evaluates, but
most parsers treat the cell as text) — `"\t" + value` — or (b) prepend a single
quote `'` so spreadsheet apps treat the cell as a literal string. Apply the
transform in a single helper called from every `cw.Write` and from
`writeV2Manifests`.

```go
// csvSafe escapes a value that may otherwise be interpreted as a formula
// in Excel / Google Sheets. The encoding/csv package handles the
// quoting for commas and quotes; this handles the formula-injection case.
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
```

Add a regression test with a sample name that starts with each of `=`, `+`, `-`, `@`
and assert the re-parsed value equals the input.

---

## High Issues

### HI-01: v2 [Data] writer drops unknown columns (data loss; contradicts README)

**File:** `internal/writer/writer.go:175-192` (function `writeV2Data`)
**Issue:** The v2 [Data] writer hard-codes `canonicalColumnsV2` and writes only those
ten columns. Any column carried in `sample.Extra` is silently dropped. This contradicts
both the package doc (`"a round-trip is lossless for well-formed input"`) and the
README (`"Round-trips are lossless: unknown columns and unknown sections are preserved"`).

Confirmed with a synthetic sheet containing a `CustomField` column: the field is
present in the input, the writer emits the canonical ten columns, and `CustomField`
is gone after the round-trip.

```
output: [Data]
        Sample_ID,Sample_Name,Sample_Plate,...,Sample_Project,Description
        id1,name1,,,,ATCGATCG,,,P1,
        (no CustomField column)
```

For comparison, the v1 writer does call `appendExtraColumns` and so preserves extras.

**Why it matters:** v2 sheets from DRAGEN and BaseSpace routinely include custom
columns (assay, pool, lane, custom indexes). Every time a user runs
`illsample convert -f v2` on a v1 sheet that has been enriched with extras, those
extras vanish. This is a real lab workflow.

**Fix:** Decide whether the v2 writer should (a) emit a single header that includes
both the canonical ten and any `sample.Extra` keys, or (b) preserve the original
header order. Option (a) is closer to the documented behaviour:

```go
func writeV2Data(buf *bytes.Buffer, header []string, data []model.Sample) error {
    // Start from the canonical columns; if the user supplied a header,
    // prepend any columns that appear in Extra but not in the canonical set.
    cols := append([]string(nil), canonicalColumnsV2...)
    seen := make(map[string]struct{}, len(cols))
    for _, c := range cols {
        seen[c] = struct{}{}
    }
    if len(header) > 0 {
        for _, h := range header {
            lk := strings.ToLower(strings.TrimSpace(h))
            if _, ok := seen[lk]; ok || lk == "" {
                continue
            }
            cols = append(cols, h)
            seen[lk] = struct{}{}
        }
    }
    // ... rest of the function
}
```

Then add a round-trip test that uses a v2 sheet with an extra column and asserts
the column survives.

---

### HI-02: parseSettingsSection silently drops all but the first data row

**File:** `internal/parser/v2.go:186-210`
**Issue:** The Settings parser only ever reads `records[1]`:

```go
header := records[0]
for col, name := range header {
    ...
    if len(records) < 2 {
        continue
    }
    var value string
    if col < len(records[1]) {
        value = strings.TrimSpace(records[1][col])
    }
    switch key { ... }
}
```

If a v2 sheet has a [Settings] section with more than one data row (which is allowed
by the IEM spec and observed in the wild for multi-lane sheets), rows 2+ are
silently discarded.

**Why it matters:** This is data loss in the round-trip direction the user doesn't
expect. There is no warning, no log, nothing.

**Fix:** Either (a) iterate over `records[1:]` and concatenate the values for
duplicate keys, or (b) error out with a clear "multiple data rows in [Settings]
are not supported" message. Option (a) is safer for migration scenarios:

```go
for _, rec := range records[1:] {
    for col, name := range header {
        key := strings.TrimSpace(name)
        if key == "" || col >= len(rec) {
            continue
        }
        value := strings.TrimSpace(rec[col])
        // ... same switch as before
    }
}
```

Add a test where the input has two [Settings] data rows and assert both
appear in the round-tripped output.

---

### HI-03: parseV1 skips the UTF-8 check that Parse/ParseV2 perform

**File:** `internal/parser/v1.go:35-44`
**Issue:** The auto-detecting `Parse` function rejects non-UTF-8 input, but
`ParseV1` (and the v2-only path of `ParseV2`) does not. A binary file accidentally
fed to `ParseV1` will be parsed as v1, producing a sheet with garbage `Sample_ID`
and `Sample_Name` values that no downstream tool will catch.

Confirmed:

```
input: \xff\xfe + "Sample_ID,sample,ATCG,P1\n"
Parse (auto):   error: input is not valid UTF-8
ParseV1:        <nil>            // silently accepted
```

**Why it matters:** A small inconsistency like this is the kind of thing that
turns a user-reported "the parser crashed" into "wait, only *sometimes* it
crashed." The user-visible behaviour of the three entry points should be
identical up to the format-detection step.

**Fix:** Either drop the UTF-8 check in `ParseBytes` and apply it consistently in
all three entry points, or move the check into `parseV1` and `parseV2` so the
explicit entry points also reject garbage. The latter is closer to the current
intent:

```go
func parseV1(data []byte) (*model.SampleSheet, error) {
    text := stripBOM(data)
    if !utf8.ValidString(text) {
        return nil, &ParseError{Msg: "input is not valid UTF-8"}
    }
    ...
}
```

Add a unit test for `ParseV1` with non-UTF-8 bytes.

---

## Warnings

### WR-01: v1 header column names retain surrounding whitespace

**File:** `internal/parser/v1.go:49`, `internal/writer/writer.go:321-348`
**Issue:** The v1 parser stores the raw header (`header := records[0]`) without
trimming, but the colIdx is built from `strings.ToLower(strings.TrimSpace(name))`.
The writer's `fieldFor` then lowercases the *original* (un-trimmed) column name and
fails to match any of the typed-field cases, returning `""`.

Confirmed:

```
input:  "Sample_ID, Sample_Name ,index,Sample_Project\nid1,name1,ATCG,P1\n"
parsed: Header = ["Sample_ID", " Sample_Name ", "index", "Sample_Project"]
        Data[0].SampleName = "name1"   // correctly stored
write:  "Sample_ID,\" Sample_Name \",index,Sample_Project\nid1,,ATCG,P1\n"
reparsed: Data[0].SampleName = ""     // lost
```

**Why it matters:** A v1 sheet with whitespace in its column header (which Excel
and the IEM happily produce) loses the data in that column on round-trip. This
affects the same "lossless round-trip" promise the README makes.

**Fix:** Trim header values when storing them:

```go
header := make([]string, len(records[0]))
for i, h := range records[0] {
    header[i] = strings.TrimSpace(h)
}
```

(Or apply the trim in `fieldFor` and the colIdx builder consistently.)

---

### WR-02: `isV2` only filters `#` comments, not `//` (inconsistent with `comment()`)

**File:** `internal/parser/parser.go:141-150`
**Issue:** The v2 parser has a `comment()` helper that recognises both `#` and
`//`, but the v2-detection `isV2()` only skips `#`. A v2 file that begins
with `// generated by foo\n` (a perfectly reasonable comment style) is misclassified
as v1 and then mis-parses: the v1 parser treats the first line as a single-field
header and produces a v1 sheet with all values in `Extra`.

**Why it matters:** Mis-detection of v1 vs v2 means the verifier applies the
wrong rule set, the writer emits the wrong format, and the user has to debug
*why* their perfectly valid v2 sheet has zero samples and three errors.

**Fix:**

```go
func isV2(lines []string) bool {
    for _, line := range lines {
        t := strings.TrimSpace(line)
        if t == "" || comment(t) {
            continue
        }
        return isSectionHeader(t)
    }
    return false
}
```

Add a parse test with a `//` comment in the v2 preamble.

---

### WR-03: [Manifests] "A" header heuristic is fragile

**File:** `internal/parser/v2.go:233-236`
**Issue:**

```go
if trimmed == "A" && len(sheet.Manifests) == 0 {
    continue
}
```

The parser assumes the first `A` is the IEM column-letter header and skips it.
This silently drops any manifest that legitimately has the identifier `A` and
is the only row in the section. The check is also not case-insensitive (`a`
would not be skipped, but the IEM spec is).

**Why it matters:** Edge-case data loss for a real, if uncommon, manifest
identifier.

**Fix:** Only skip the header when the *next* line looks like a manifest
(`read one line ahead`), or skip the header only when `m.A == "A"` AND the
section header came from IEM (we don't have that signal, so the lookahead
approach is simpler).

---

### WR-04: parseSettingsSection is case-sensitive on key names

**File:** `internal/parser/v2.go:199-209`
**Issue:** `key := strings.TrimSpace(name)` is used to look up the typed
fields, but the switch is on the original (not lowercased) value. A [Settings]
section with the header `adapter,AdapterRead2` puts the value in
`Settings.Extra["adapter"]` instead of `Settings.Adapter`. The verifier's
`W_V2_NO_ADAPTER` then fires spuriously.

The IEM spec uses canonical case, but the v1 path is case-insensitive everywhere
and the parser is supposed to be lenient.

**Fix:** Lowercase before the switch:

```go
switch strings.ToLower(key) {
case "adapter":
    sheet.Settings.Adapter = value
...
}
```

---

### WR-05: readCSV loses `\r` in embedded line endings

**File:** `internal/parser/parser.go:174`
**Issue:** `bytes.NewReader([]byte(strings.Join(raw, "\n")))` always uses LF as
the row separator when handing the section back to the CSV reader. If the
original sheet used CRLF and a quoted field had a literal `\r\n` in the middle
of it, the `\r` is dropped and the cell value differs from the source.

Unlikely to fire in practice (sample sheets rarely have `\r\n` in cell values),
but a true round-trip preserves every byte.

**Fix:** Detect the dominant line ending (`\r\n` vs `\n`) once and join with
the same sequence.

---

### WR-06: Error line numbers are off when a section has leading blanks/comments

**File:** `internal/parser/v2.go:75, 167, 219, 261`
**Issue:** Functions capture `startLine := p.lineNumber()` *before* calling
`collectSection`, but `collectSection` skips blanks and comments without
adjusting the start. The result is that `readCSV` reports line numbers that
are too low by the number of leading blank/comment lines in the section.

**Why it matters:** Diagnosability. When a sheet is rejected with
`line 7: invalid CSV: ...`, the user expects to be able to jump to line 7
of the source.

**Fix:** Have `collectSection` return both the raw lines and the start line
of the first non-blank, non-comment line:

```go
func collectSection(p *lineParser) (raw []string, startLine int, err error) {
    // ... existing loop
    if !isSectionHeader(trimmed) && trimmed != "" && !comment(trimmed) && startLine == 0 {
        startLine = p.pos + 1
    }
    // ...
}
```

---

### WR-07: CLI writes verification output to stdout, not stderr

**File:** `cmd/illsample/main.go:241-258` (and call site at line 96)
**Issue:** `printResult(os.Stdout, ...)` writes errors and warnings to
stdout. The README example writes them to stderr. Unix convention is that
diagnostics go to stderr, especially for a tool that has a `convert` subcommand
whose stdout is meant to be a parsed CSV.

**Why it matters:** `illsample validate bad.csv | sed ...` will mix error
messages into the parsed stream.

**Fix:** Pass `os.Stderr` (or an `io.Writer` from the caller) and only
the `"OK: no issues found"` line should go to stdout in the success case.
Or split into a `printSummary(w, result, ...)` for stdout and
`printDiagnostics(w, result, ...)` for stderr.

---

### WR-08: loadSheet error wrapping is inconsistent

**File:** `cmd/illsample/main.go:217-235`
**Issue:** A parse error gets wrapped with the file basename
(`"bad.csv: line 5: invalid CSV: ..."`), but an `os.Open` error
(`"open bad.csv: no such file or directory"`) does not, and the stdin path
(`"-"`) skips wrapping entirely. Same kind of failure, three different
output formats.

**Fix:** Apply the basename wrap to *all* three error returns, or document
explicitly that stdin is anonymous and skip the wrap only for the `-` case.

---

### WR-09: convert flips `FormatUnknown` sheets to v1

**File:** `cmd/illsample/main.go:120-127`
**Issue:**

```go
if sheet.Format == illsample.FormatV1 {
    sheet.Format = illsample.FormatV2
} else {
    sheet.Format = illsample.FormatV1
}
```

A programmatically constructed sheet (FormatUnknown, e.g. one that came from
a builder that forgot to set the field) gets flipped to v1. The writer
treats FormatUnknown as v2, so the flip is also inconsistent with the writer's
own default.

**Fix:** Treat FormatUnknown as the sentinel it is and require the user to
set Format explicitly:

```go
case "":
    if sheet.Format == illsample.FormatUnknown {
        return fmt.Errorf("convert: cannot flip FormatUnknown sheet; use -f v1 or -f v2")
    }
    if sheet.Format == illsample.FormatV1 {
        sheet.Format = illsample.FormatV2
    } else {
        sheet.Format = illsample.FormatV1
    }
```

---

### WR-10: README and CHANGELOG claim "18 rules" — there are 30

**File:** `README.md:192`, `CHANGELOG.md:27`
**Issue:** The README table lists 30 codes (`E_NIL_SHEET` + 11 v1 + 18 v2 = 30),
and the code defines 29 rules + 1 nil-sheet sentinel, but both the README
prose and the CHANGELOG say "18 verification rules". Anyone counting rule
implementations to estimate test coverage or effort will be off by ~70%.

**Fix:** Update both files. The CHANGELOG entry is the easier one to fix
(`"- 30 verification rules (29 sheet rules + 1 nil-sheet sentinel) across
both v1 and v2"`). The README's table is correct, only the prose number is stale.

---

### WR-11: FuzzParse docstring overstates the property under test

**File:** `internal/parser/parser_test.go:346-380`
**Issue:** The doc comment promises two properties:

> 1. Parse never panics, regardless of input.
> 2. A successful parse followed by a Write followed by a Parse is either
>    equal to the original or, in the rare case of ambiguous column mapping,
>    equivalent up to Extra normalisation.

But the f.Fuzz body only checks (1) and a trivial format check. Anyone running
`go test -fuzz=FuzzParse -fuzztime=1h` thinking they're testing round-trip
isn't.

**Fix:** Either implement the round-trip property:

```go
sheet, err := Parse(strings.NewReader(input))
if err != nil {
    return
}
var buf bytes.Buffer
if err := Write(&buf, sheet); err != nil {
    t.Errorf("Write: %v", err)
    return
}
if _, err := Parse(&buf); err != nil {
    t.Errorf("re-parse of Write output failed: %v\n%s", err, buf.String())
}
```

...or trim the docstring to match the actual coverage.

---

## Info

### IN-01: Dead code annotated with `nolint:unused`

**File:** `internal/parser/parser.go:255-278`
**Issue:** `findNextSection` and `peekIsNextSection` are marked
`//nolint:unused // retained for future use`. They are not called anywhere
in the codebase, and "retained for future use" is the kind of comment that
survives every code review until the project is retired.

**Fix:** Delete them. If a future feature needs them, git has the history.

---

### IN-02: `byteOffsetToLine` is at 0% coverage

**File:** `internal/parser/parser.go:197-220`
**Issue:** Coverage report shows 0% for `byteOffsetToLine`. The function is
only called by `readCSV`, which only fires on CSV parse errors. The fuzz
seeds do not contain any malformed CSV that would trigger it. The function
is also not used by the v1 path.

**Fix:** Add a unit test that feeds the parser a quoted field with a stray
quote, e.g. `"a","b"c\n`, and assert that the resulting `ParseError.Line`
is in the expected range.

---

### IN-03: `Location.String` has a switch with three identical arms

**File:** `internal/verify/verify.go:66-84`
**Issue:**

```go
switch l.Section {
case "":
    b.WriteString("Sheet")
case "Data", "Reads", "Settings", "Manifests":
    b.WriteString(l.Section)
default:
    b.WriteString(l.Section)
}
```

The second and third cases do the same thing. Simplify to:

```go
if l.Section == "" {
    b.WriteString("Sheet")
} else {
    b.WriteString(l.Section)
}
```

---

### IN-04: `ParseError.Is` method is unusual

**File:** `internal/parser/parser.go:97-101`
**Issue:**

```go
func (e *ParseError) Is(target error) bool {
    _, ok := target.(*ParseError)
    return ok
}
```

This makes `errors.Is(err, &ParseError{})` match *any* `*ParseError`, which
is not generally what `Is` is for. There's no `Unwrap` method either, so
`errors.Is(err, target)` will walk through the `fmt.Errorf("%s: %w", ..., pe)`
chain and then match any `*ParseError`. The intent is presumably to allow
`errors.Is(err, parser.ErrParse)` or similar, but no such sentinel exists.

**Fix:** Either remove `Is` (callers can use `errors.As`) or add sentinel
errors (`ErrInvalidCSV`, `ErrEmptyInput`, `ErrInvalidUTF8`) and make
`Is` check against them by Code.

---

### IN-05: `hasInvalidNameChars` comment disagrees with code

**File:** `internal/verify/verify.go:246-259`
**Issue:** The comment claims the function rejects whitespace, `/`, and
`\`, but the implementation only rejects whitespace and control chars. The
accompanying test `{"sample/one", false}` (verify_test.go:334) confirms
the implementation, not the comment.

**Fix:** Update the comment to match, or extend the function to reject `/`
and `\` if the empirical restriction is real.

---

### IN-06: `v1RowToSample` has a dead error return

**File:** `internal/parser/v1.go:85-107`
**Issue:** The function signature is
`func v1RowToSample(row []string, header []string, colIdx map[string]int, line int) (model.Sample, error)`
but the body never returns a non-nil error. Every caller either passes the
error up the chain (`parser/v2.go:106-109`) or ignores it. The
`line int` parameter is also used only to set `SourceRow: line - 1`, with
no plan for future error messages.

**Fix:** Drop the error return. If a future enhancement needs to fail
mid-row (e.g. for the index-character check), reintroduce it then.

---

### IN-07: SourceRow / Location.Row / error messages all use 0-based row indexing

**File:** `internal/model/model.go:95-99`, `internal/verify/verify.go:66-84`,
`internal/verify/rules.go:151-157`
**Issue:** `SourceRow` is 0-based (`line - 1` in the parser),
`Location.Row` is 0-based, and the duplicate-SampleID message uses
0-based (`"is also used in row %d"`). `ParseError.Line`, on the other
hand, is 1-based. Users reading the output have to remember which is which.

**Fix:** Pick one convention and apply it everywhere. `Location` is exposed
in the public API, so the change is technically breaking; document it in the
CHANGELOG and bump the version.

---

### IN-08: `extraNames` has a comment that misleads

**File:** `cmd/illsample/main.go:204-213`
**Issue:** The comment claims `"Go map iteration order is already randomised,
which is fine for human consumption."` Random order is not fine when the user
wants to compare two runs of `illsample info`, or copy/paste the output into
a bug report.

**Fix:** `sort.Strings(names)` before returning. One line.

---

### IN-09: `csv.UseCRLF = false` doesn't match IEM convention

**File:** `internal/writer/writer.go:77, 177, 243, 283`
**Issue:** IEM emits CRLF; the writer emits LF. bcl2fastq and DRAGEN tolerate
both, so this is a stylistic difference, not a bug. But the v1 parser
also calls `r.LazyQuotes = true` which is a known CSV deviation; combining
the two means a v1 round-trip is never byte-identical to the source.

**Fix:** Document the deviation in the package doc, or expose a
`WriteOptions{UseCRLF bool}` for callers who need it.

---

## Test Gaps

I did not find a single test that would have caught any of CR-01, CR-02, HI-01, HI-02, or HI-03.
The tests are good for what they cover (round-trip of canonical inputs, all 29 rules fire on
their respective fixtures), but there is no negative testing for:

| Area | Missing test |
|---|---|
| CR-01 | A v1 sheet with the `I7IndexID` (no-underscore) column |
| CR-02 | A sample name that starts with `=`, `+`, `-`, or `@` |
| HI-01 | A v2 sheet with an extra `[Data]` column |
| HI-02 | A v2 sheet with multiple `[Settings]` data rows |
| HI-03 | Non-UTF-8 input to `ParseV1` |
| WR-01 | A v1 sheet with whitespace in the header |
| WR-02 | A v2 sheet whose preamble is a `//` comment |
| WR-04 | A `[Settings]` section with lowercase keys |
| `ParseError.Is` | Any test at all |
| `Location.String` | Cases where `Row == 0` |
| `Sample.SourceRow` | Asserted at the right 0-based value after a CRLF input |
| `ParseBytes` | Direct unit test (only exercised through `Parse`) |

The fuzz test catches the "no panic" class of bug but, per WR-11, does not exercise the
round-trip property it advertises.

---

## Documentation

- The `SeverityError` doc says `"the sheet will not run"`, but the [Settings] field
  is not a hard requirement (bcl2fastq treats it as advisory). `E_V1_INDEX_LENGTH_MISMATCH`
  is also non-fatal in some configurations. The doc is a touch more absolute than reality.
- `parseSettingsSection` returns `sheet.Settings = &model.Settings{}` even when the
  section is empty (`len(records) < 2`, no data row). The README says
  `"Empty [Settings] and empty [Manifests] sections are treated as 'no settings / no manifests'"`,
  but the implementation actually does allocate a `Settings` struct in this case. This affects
  whether the v2 writer omits or emits the `[Settings]` header on round-trip — the writer
  *will* emit it if `Settings` is non-nil, which it will be.
- `Model.Section.Columns` doc says `"first non-empty row"`, but the parser actually
  uses `records[0]` regardless of whether it's empty.
- The `Model.Sample.SourceRow` doc is good. `Model.SampleSheet.Header` could use a
  line clarifying that v2 round-trips do *not* preserve the original column order
  (after the HI-01 fix is applied, this paragraph should be updated accordingly).

---

## Maintainability

- Naming: consistent, except `v1RowToSample` is also used by the v2 path
  (`parser/v2.go:106`). Rename to `rowToSample` or `csvRowToSample`.
- File structure: `internal/parser/v1.go`, `internal/parser/v2.go`, `internal/parser/parser.go`
  is a clean split. The shared `lineParser` and `readCSV` live in `parser.go`. Good.
- Magic numbers: 100 in `W_V1_LONG_SAMPLE_NAME` and `W_V2_LONG_SAMPLE_NAME` could be
  a named constant (`maxSampleNameLen`) for documentation. Same for the
  4*1024*1024 buffer cap in `splitLines`.
- `Makefile`: `cover` target pipes through `tail -20`, which may truncate
  the total-coverage line if the project grows past 20 functions. Use
  `go tool cover -func=coverage.out | grep -E 'total|^[a-z]' ` or
  `awk '/^total/ {print; exit}' ` instead.
- `internal/model`: 0% test coverage. The package has no exported behaviour beyond
  the field types, but `Format.String` is exercised only through the public API,
  and a regression in `(Format).String()` would be invisible without a test
  here. A 10-line test file would close the gap.

---

## Build / Tooling Health

- `go test -race ./...` — passes (0.45s for parser, 0.27s for verify, 0.28s for writer).
- `go vet ./...` — clean.
- `golangci-lint run` — 0 issues (the linter is set up but the rules don't catch
  the CR-01 / HI-01 / HI-03 class of bug, which is a static-analysis limitation, not
  a project miss).
- `gofmt -l .` — clean.
- `go test -coverprofile=cov.out ./...` then `go tool cover -func=cov.out`:
  - top-level `illsample`: 100.0% (claimed in CHANGELOG ✓)
  - `internal/parser`: 73.8% (claimed 73% ✓)
  - `internal/verify`: 90.6% (claimed 91% ✓)
  - `internal/writer`: 76.8% (claimed 77% ✓)
  - `internal/model`: 0% — *not mentioned in the CHANGELOG* (CHANGELOG says "the internal
    packages" which is ambiguous; with only 3 packages mentioned but 4 in the repo,
    model is silently missing from the coverage report).

---

## Recommended Order of Operations

If you want a sensible PR breakdown, I'd attack them in this order:

1. **CR-01** — one-line fix + a regression test. This is the only "lab equipment
   ruined" risk in the codebase.
2. **CR-02** — small helper, four call sites, one regression test. Security,
   straightforward.
3. **HI-01** — read the data, follow the IEM convention, write a round-trip test.
   The trickiest of the three because it requires deciding the v2 column
   ordering policy.
4. **HI-02** — iterate over `records[1:]` instead of taking `records[1]`; one
   regression test.
5. **HI-03** — copy the UTF-8 check from `ParseBytes` into `parseV1` and
   `parseV2`; one regression test.
6. The WR-* items can be PR'd individually as cleanup; none are urgent.

---

_Reviewed: 2026-10-13T09:30:00Z_
_Reviewer: gsd-code-reviewer (adversarial pass)_
_Depth: standard_
