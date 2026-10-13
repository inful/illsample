# illsample

A Go library and CLI for parsing, writing, and verifying Illumina sample sheets.

Illumina sequencers consume a *sample sheet* — a small CSV-ish file — that tells the
basecaller which samples are in which wells, what indices to expect, and how to
demultiplex the run. The format has had two main versions in the wild: the
[v1 legacy CSV](#v1-legacy-csv) and the [v2 IEM sectioned format](#v2-iem-sections),
both supported by `bcl2fastq`, DRAGEN, and BaseSpace. `illsample` reads both,
writes both, and runs a battery of validation rules so you can catch problems
before they reach the sequencer.

## Features

- **Parse v1 (legacy CSV) and v2 (IEM sectioned) sample sheets** with the same
  auto-detecting `illsample.Parse` function. Use `illsample.ParseV1` or
  `illsample.ParseV2` to force a specific format.
- **Write** sample sheets back to disk in either format. Round-trips are
  lossless: unknown columns and unknown sections are preserved.
- **Verify** with a layered model. Each finding is an `Issue` tagged with a
  `Severity` (error or warning) and a `Location` (sheet / section / row / column).
  Use `VerifyOptions` to suppress warnings or promote them to errors.
- **CLI** with `validate`, `convert`, and `info` subcommands for shell pipelines.
- **Zero external dependencies.** Pure standard library.
- **Fuzzed.** The parser is exercised by `go test -fuzz` to guard against
  panics on malformed input.

## Installation

```bash
go get github.com/inful/illsample
```

To install the CLI:

```bash
go install github.com/inful/illsample/cmd/illsample@latest
```

## Quick start

### Library

```go
package main

import (
    "fmt"
    "os"

    "github.com/inful/illsample"
)

func main() {
    f, err := os.Open("SampleSheet.csv")
    if err != nil {
        panic(err)
    }
    defer f.Close()

    sheet, err := illsample.Parse(f)
    if err != nil {
        panic(err)
    }

    result := illsample.Verify(sheet)
    if result.HasErrors() {
        for _, iss := range result.Errors() {
            fmt.Fprintf(os.Stderr, "error %s: %s [%s]\n",
                iss.Code, iss.Message, iss.Location)
        }
        os.Exit(1)
    }
    for _, iss := range result.Warnings() {
        fmt.Fprintf(os.Stderr, "warning %s: %s [%s]\n",
            iss.Code, iss.Message, iss.Location)
    }

    // Re-emit as v2.
    if err := illsample.WriteV2(os.Stdout, sheet); err != nil {
        panic(err)
    }
}
```

### CLI

```bash
# Validate a sheet; exit 0 on success, 1 on errors, 2 on usage errors.
illsample validate SampleSheet.csv

# Strict mode: treat warnings as errors.
illsample validate -strict SampleSheet.csv

# Print a summary of the sheet.
illsample info SampleSheet.csv

# Convert v1 to v2.
illsample convert -f v2 SampleSheet.csv > Sheet_v2.csv

# Read from stdin.
cat SampleSheet.csv | illsample validate -
```

## The two formats

### v1 (legacy CSV)

A single CSV with a header row. Column order does not matter; columns are
matched by name. Required columns for `bcl2fastq`:

- `Sample_ID` (or `SampleID`)
- `Sample_Name` (or `SampleName`)
- `index` (and optionally `index2` for dual-indexed runs)
- `Sample_Project` (or `SampleProject`)

A minimal v1 sheet:

```csv
Sample_ID,Sample_Name,index,Sample_Project
sample_01,sample_one,ATCGATCG,Project_Alpha
sample_02,sample_two,GCTAGCTA,Project_Alpha
```

### v2 (IEM sections)

A sequence of sections, each introduced by a `[Name]` header. The
Illumina Experiment Manager (IEM) emits this layout; `bcl2fastq` and
DRAGEN both accept it. The canonical v2 layout is:

```ini
[Data]
Sample_ID,Sample_Name,Sample_Plate,Sample_Well,I7_Index_ID,index,I5_Index_ID,index2,Sample_Project,Description
sample_01,sample_one,plate1,A1,UDI001,ATCGATCG,,,RNAseq,first

[Reads]
Read1Cycles
151
Read2Cycles
151
Index1Cycles
8

[Settings]
Adapter,AdapterRead2
AGATCGGAAGAGCACACGTCTGAACTCCAGTCA,AGATCGGAAGAGCGTCGTGTAGGGAAAGAGTGT
```

- `[Data]` is a CSV section with the canonical column order above.
- `[Reads]` is a vertical key/value list: key on one line, value on the next.
- `[Settings]` is a 2-column CSV: header row, then one or more value rows.
- `[Manifests]` (optional) is a single column of manifest paths.
- Any other section is preserved verbatim in `SampleSheet.Extra` so that
  round-trips are lossless.

## Verification rules

| Code | Severity | What it catches |
|------|----------|-----------------|
| `E_NIL_SHEET`               | error   | `Verify` was called with a nil sheet |
| `E_V1_NO_DATA`              | error   | v1 sheet has no data rows |
| `E_V1_MISSING_SAMPLE_ID`    | error   | `Sample_ID` column missing |
| `E_V1_MISSING_SAMPLE_NAME`  | error   | `Sample_Name` column missing |
| `E_V1_MISSING_INDEX`        | error   | neither `index` nor `index2` present |
| `E_V1_MISSING_SAMPLE_PROJECT` | error | `Sample_Project` column missing |
| `E_V1_EMPTY_SAMPLE_ID`      | error   | a row has an empty `Sample_ID` |
| `E_V1_DUPLICATE_SAMPLE_ID`  | error   | the same `Sample_ID` appears twice |
| `E_V1_INVALID_INDEX`        | error   | index contains characters outside A/C/G/T/N |
| `E_V1_INDEX_LENGTH_MISMATCH`| error   | `index` length varies between rows |
| `E_V1_INVALID_SAMPLE_NAME`  | error   | `Sample_Name` contains characters that bcl2fastq will reject |
| `E_V2_NO_DATA`              | error   | `[Data]` section has no rows |
| `E_V2_NO_READS`             | error   | `[Reads]` section is missing |
| `E_V2_MISSING_SAMPLE_ID`    | error   | `Sample_ID` column missing |
| `E_V2_MISSING_SAMPLE_NAME`  | error   | `Sample_Name` column missing |
| `E_V2_MISSING_SAMPLE_PROJECT` | error | `Sample_Project` column missing |
| `E_V2_MISSING_INDEX`        | error   | `index` column missing |
| `E_V2_READ1_MISSING`        | error   | `[Reads]` Read1Cycles not specified |
| `E_V2_EMPTY_SAMPLE_ID`      | error   | a row has an empty `Sample_ID` |
| `E_V2_DUPLICATE_SAMPLE_ID`  | error   | the same `Sample_ID` appears twice |
| `E_V2_INVALID_INDEX`        | error   | index contains characters outside A/C/G/T/N |
| `E_V2_INDEX_LENGTH_MISMATCH`| error   | `index` length varies between rows |
| `E_V2_INDEX_VS_READS`       | error   | `index` length does not match `[Reads]` Index1Cycles |
| `E_V2_INVALID_SAMPLE_NAME`  | error   | `Sample_Name` contains illegal characters |
| `W_V1_INCONSISTENT_PLATE_WELL` | warning | some rows have plate/well, others don't |
| `W_V1_LONG_SAMPLE_NAME`     | warning | `Sample_Name` is over 100 characters |
| `W_V2_NO_ADAPTER`           | warning | no `Adapter` sequence in `[Settings]` |
| `W_V2_INCONSISTENT_PLATE_WELL` | warning | some rows have plate/well, others don't |
| `W_V2_INCONSISTENT_INDEX_IDS` | warning | `I7_Index_ID` / `I5_Index_ID` only on some rows |
| `W_V2_LONG_SAMPLE_NAME`     | warning | `Sample_Name` is over 100 characters |

The exact list of rules is defined in
[`internal/verify/rules.go`](internal/verify/rules.go). Adding a rule is
a single struct literal in the `allRules` slice.

## API reference

The full API reference is on [pkg.go.dev](https://pkg.go.dev/github.com/inful/illsample).
The most useful entry points are:

- `illsample.Parse(r)` — auto-detect v1/v2 and parse.
- `illsample.ParseV1(r)` / `illsample.ParseV2(r)` — force a format.
- `illsample.Write(w, sheet)` / `illsample.WriteV1` / `illsample.WriteV2` — emit a sheet.
- `illsample.Verify(sheet)` / `illsample.VerifyWithOptions(sheet, opts)` — run rules.

The data model is documented in
[`internal/model/model.go`](internal/model/model.go). The two most
useful types are `SampleSheet` (a parsed sheet) and `Sample` (a single
data row).

## Quirks the parser handles

The v1 and v2 specs are loose, and real-world sheets bend them in
various ways. `illsample` is designed to parse what people actually
send it, not just what the spec says:

- **UTF-8 BOM** at the start of the file is stripped.
- **CRLF or LF** line endings both work.
- **Section names** are case-insensitive (`[Data]`, `[data]`, `[DATA]` are
  equivalent).
- **Column names** are case-insensitive.
- **Comment lines** starting with `#` (or `//`) are ignored everywhere
  in v2 sheets.
- **Empty `[Settings]`** and empty `[Manifests]` sections are treated
  as "no settings / no manifests" rather than as errors.
- **Unknown sections** are preserved verbatim in `sheet.Extra`, so a
  v2 sheet that contains DRAGEN-specific sections round-trips
  losslessly.
- **Unknown columns** in `[Data]` are preserved per-row in
  `sample.Extra`.
- **Ragged rows** (rows shorter than the header) are tolerated; the
  trailing fields are simply empty.

## Development

```bash
# Run all tests.
go test -race ./...

# Run a quick fuzz pass.
go test -fuzz=FuzzParse -fuzztime=10s ./internal/parser/

# Check coverage.
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## License

MIT. See [LICENSE](LICENSE).
