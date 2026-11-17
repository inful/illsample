# Changelog

All notable changes to `illsample` are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed
- `E_V2_NO_READS` is now `W_V2_NO_READS`, a warning rather than an
  error. Illumina's sequencing specification says that "depending on
  the software using the Sample Sheet file, some sections may be
  optional. However, the Header and Data sections are always
  required." The `[Reads]` section holds the cycle counts, which some
  software takes from the run instead of from the sheet.

### Fixed
- The v2 writer writes the `[Header]` section for every sheet. A sheet
  that records no run metadata got no section at all, and the
  specification says a v2 sample sheet must begin with it — so
  `illsample convert -f v2` wrote a sheet that `illsample validate`
  rejected with `E_V2_NO_HEADER`. The section now holds the file
  format version when the sheet has no records of its own.

## [0.5.2] - 2026-11-17

### Fixed
- A setting that is both a typed field and an `Extra` key is written
  once. The v2 writer wrote the `Adapter` column twice when
  `Settings.Adapter` and `Settings.Extra["Adapter"]` were both set,
  and the specification says the column headers of a section must all
  be unique.
- `illsample info` prints the run metadata of the `[Header]` section.
  The section is a typed field now, so its records had become
  invisible in the summary.

## [0.5.1] - 2026-11-17

### Fixed
- `Parse`, `ParseV1` and `ParseV2` report a nil reader as an error
  instead of panicking. `Write` already reported a nil sheet and
  `Verify` already reported `E_NIL_SHEET`; the parser was the one
  that crashed.
- A v1 sheet that records no columns gets a v1 header. The default
  was the v2 canonical column list, so a sheet built in code came
  out with the IEM columns `Sample_Plate`, `Sample_Well`,
  `I7_Index_ID` and `I5_Index_ID`, which the legacy layout does not
  have. The default is now Illumina's own `bcl2fastq` template:
  `Sample_ID,Sample_Name,index,index2,Sample_Project,Description`.
- `illsample new -f v1` and the writer now use that one definition
  of the v1 header; the command had a list of its own that did not
  match.

## [0.5.0] - 2026-11-17

### Added
- `E_V2_NO_HEADER`: a v2 sheet without the `[Header]` section is an
  error. Illumina's specification says that a sample sheet must begin
  with the Header section and that the Header and Reads sections are
  present in every v2 sample sheet; the library already failed a sheet
  without `[Reads]`, and now fails one without `[Header]` the same
  way.

### Changed
- The test fixtures live in one place. The parser, verifier and
  writer packages each held a copy of the fifteen fixture sheets, so
  a change to a fixture had to be made four times; they now read the
  module's `testdata/` directory.
- The valid v2 fixtures now carry a `[Header]` section, as Illumina's
  own sheets do.

## [0.4.0] - 2026-11-17

### Added
- `illsample new` writes a sample sheet to fill in: a v2 sheet by
  default, `-f v1` for the legacy layout, `-o FILE` to write it
  somewhere other than stdout.
- `SampleSheet.Header` holds the records of the `[Header]` section —
  the run metadata that Illumina's specification puts at the top of
  every v2 sheet — as key/value pairs.

### Changed
- `SampleSheet.Header` no longer holds the column names of the data
  section. That field is now `SampleSheet.Columns`. The old name was
  a trap: in sample-sheet language "Header" is the `[Header]`
  section, and the field it named was silently ignored by the v2
  writer.
- The v2 writer emits the `[Header]` section from
  `SampleSheet.Header`, so the run metadata no longer has to be built
  as an unknown section in `SampleSheet.Extra`.

### Fixed
- A sheet built in code is no longer told that the columns it has
  are missing. The rules that look for a column now fall back to the
  samples when the sheet declares no columns, so a hand-built sheet
  verifies on its own terms.

## [0.3.0] - 2026-11-16

### Added
- `illsample validate -f v1|v2` forces the input format instead of
  auto-detecting it.
- `E_V1_INVALID_SAMPLE_ID`, `E_V2_INVALID_SAMPLE_ID`,
  `E_V1_LONG_SAMPLE_ID` and `E_V2_LONG_SAMPLE_ID`: the specification
  allows only letters, digits, dashes and underscores in a
  `Sample_ID`, and at most 100 characters.
- `W_V1_DUPLICATE_COLUMN` and `W_V2_DUPLICATE_COLUMN`: the section
  requirements say that the column headers of a data section must all
  be unique.
- A fuzz target for the writer (`go test -fuzz=FuzzWrite
  ./internal/writer/`): the writer must not fail, and what it writes
  must parse back to the same number of rows.

### Fixed
- The parser now reads the layouts that Illumina documents, in the
  *Sequencing Sample Sheet Format Specifications* (Pub. No.
  970-2017-004-A) and the *MiSeq Sample Sheet Quick Reference Guide*
  (Part # 15028392):
  - `[Reads]` holds one cycle count per line (read 1, then read 2).
    Such a sheet failed to parse with `unknown [Reads] key "151"`.
  - `[Settings]` holds one `Key,Value` pair per line. Such a sheet
    was read as a table, so the Read 1 adapter came out as the
    literal text `AdapterRead2` and the adapter sequence was lost.
  - `[Manifests]` holds one `Key,Filename` pair per line. The key and
    the file name are now read apart.
  - A section label may be padded with commas (`[Header],,,,,,`).
    Such a label was not recognised, and the sheet was read as v1.
  - A line that holds nothing but commas and whitespace is ignored,
    as the specification asks. It used to become an extra sample row
    with an empty `Sample_ID`.
  - The trailing commas that pad a record are ignored. The v2
    template's `Read1Cycles,125,,` used to fail with "value for
    \"Read1Cycles\" must be an integer, got \"125,,\"".
  - The instruction lines that Illumina's v2 template puts between
    its sections (the ones in angle brackets) are skipped, so the
    template and the NovaSeq 6000 example sheet parse as they are.
- The verifier accepts every spelling of a column that the parser
  accepts: a sheet that writes `SampleID`, as the MiSeq guide does, no
  longer fails with `E_V2_MISSING_SAMPLE_ID`.
- A v1 sheet with a single column no longer loses a cell on the
  round-trip: a cell that is empty was written as a blank line, and
  the reader skips blank lines. Such a cell is written as the quoted
  empty field now.
- `illsample validate -h` printed the usage to stderr and exited 2.
  Help now goes to stdout and exits 0; an unknown flag exits 2 with
  "usage error" on stderr.
- A single-column `[Settings]` section is now read as a vertical
  key/value list. With two keys the second value used to land in the
  first key — the sheet carried the Read 2 adapter as the Read 1
  adapter — and the second key was dropped.
- Two columns that feed the same field (`Sample_ID` and its alias
  `SampleID`) no longer pick a random winner: the parser walked a map,
  so the same sheet could parse differently from one run to the next.
  The first column in the header wins.
- A v2 column header with surrounding whitespace (` Notes `) no longer
  loses its values on round-trip.
- `W_V2_INCONSISTENT_INDEX_IDS` now fires when only one of the two
  index-ID columns is present and only some rows fill it.
- The README's release download URL pointed at an asset name that
  never existed (`illsample_Linux_x86_64.tar.gz`); it now matches the
  published archives, `illsample_<version>_<os>_<arch>.tar.gz`.
- v2 detection no longer requires the first non-empty line to be a
  `[Section]` header. A text preamble (as in Illumina's v2 template)
  no longer makes a v2 sheet look like a v1 CSV.
- Illumina's own v2 dialect is now read: the `[BCLConvert_Data]` and
  `[BCLConvert_Settings]` section names, and a `[Reads]` section
  written as `Key,Value` pairs on one line. Such sheets previously
  failed to parse.
- `Sample_Name` and `Sample_Project` are optional in v2 — Illumina's
  spec requires only `Sample_ID` and `index` — so they are reported
  as warnings instead of errors.
- `W_V2_NO_ADAPTER` no longer fires when the settings section spells
  the Read 1 adapter `AdapterRead1`.
- `E_V2_INDEX_VS_READS` names the `[Reads]` key it compares against.
- `illsample validate -strict` no longer prints a promoted warning
  twice, and counts it once.

### Changed
- The project is licensed under the **GNU General Public License
  version 3 or later**, not MIT. The `LICENSE` file holds the full
  text.
- The sample sheets that Illumina publishes are now part of the test
  suite (`testdata/illumina/`): the v2 template, the NovaSeq 6000
  example sheet and the six 5-base DNA prep templates. Every one of
  them is parsed, verified and written back, and the round-trip must
  not change a sample.

### Removed
- `Manifest.B` … `Manifest.F`: the fields were never read from a
  sheet nor written to one, so a value put there was silently dropped.
- `Manifest.A` is now `Manifest.Filename`, and `Manifest.Key` holds
  the label that the Data section's `Manifest` column refers to.
- `01-REVIEW.md`: the review of the initial release. Every finding in
  it is fixed and listed in this changelog, so the file only dated the
  repo. It stays in the history, at
  [`01-REVIEW.md`](https://github.com/inful/illsample/blob/2a3c049/01-REVIEW.md).

## [0.2.0] - 2026-10-15

### Added
- GitHub Actions workflows in `.github/workflows/`:
  - `ci.yml` runs `go test -race`, `go vet`, `gofmt`, `golangci-lint`
    v2, and a six-target cross-build on every push to `main` and
    every pull request.
  - `release.yml` runs the full test suite then invokes
    `goreleaser release --clean` on every `v*` tag push to produce
    cross-platform binaries and a GitHub release.
  - `snapshot.yml` runs the same GoReleaser flow with `--snapshot`
    on every pull request so reviewers can grab a build to test
    before merge.
- `.goreleaser.yml` configuration: CGO_ENABLED=0 builds for
  linux/darwin/windows/freebsd on amd64/arm64 (+ 386 on
  linux/darwin), archives as `tar.gz` (zip on Windows), includes
  `LICENSE` / `README` / `CHANGELOG`, emits a `SHA256SUMS`, and
  auto-generates release notes from merged PRs.
- `RELEASING.md`: step-by-step release process for future
  maintainers.
- README badges (CI, Release, pkg.go.dev, License) and a
  "From a release" install section.
- `illsample version` now prints the embedded commit and build
  date when the binary was produced by GoReleaser.

### Fixed
- **CR-01 (data corruption)**: v1 sheets with the no-underscore
  `I7IndexID` column now place values in `I7IndexID`. Previously
  the setter was wired to `I5IndexID` and a sequencer would
  demultiplex using the wrong index.
- **CR-02 (CSV injection)**: cells starting with `=`, `+`, `-`,
  `@`, `\t`, or `\r` are now prefixed with `'` so that
  Excel / Google Sheets do not evaluate them as formulas.
- **HI-01 (data loss)**: v2 writer no longer drops unknown
  `[Data]` columns on round-trip; the canonical columns are
  emitted first, then any source-header columns, then any
  `sample.Extra` keys, with alias de-duplication.
- **HI-02 (data loss)**: `[Settings]` rows 2+ are no longer
  silently dropped; last non-empty value wins per column.
- **HI-03 (input validation)**: `ParseV1` and `ParseV2` now
  reject non-UTF-8 input, matching the auto-detecting `Parse`.
- v1 column headers with surrounding whitespace round-trip
  without data loss.
- v2 sheets whose preamble is a `//` comment are now correctly
  detected as v2.
- `[Settings]` keys are now matched case-insensitively.
- Error line numbers in `[Data]`, `[Settings]`, and unknown
  sections are no longer off by the count of leading blank or
  comment lines.
- `[Manifests]` "A" header heuristic now uses a one-line
  look-ahead so a single-row `[Manifests] A` is no longer
  silently dropped.
- CLI `validate` now writes errors and warnings to stderr;
  only the summary line goes to stdout.
- CLI `convert` no longer silently flips `FormatUnknown`
  sheets to v1; the user must specify `-f` explicitly.
- `loadSheet` now wraps every error path with the file
  basename (or `<stdin>` for the `-` path) so the user can
  tell which file failed.

### Changed
- The 5 critical/high findings from the code review (CR-01,
  CR-02, HI-01, HI-02, HI-03) plus 17 of the 20 review warnings
  and info items are addressed in this release. The full review is
  kept in the history, at
  [`01-REVIEW.md`](https://github.com/inful/illsample/blob/2a3c049/01-REVIEW.md);
  the remaining 3 (IN-07 row-indexing convention, IN-09 CRLF
  emission, WR-05 `\r` preservation) are deferred.
- Project pushed to <https://github.com/inful/illsample>.
  All commits authored as `inful <inful@users.noreply.github.com>`.
- Coverage: parser 73.8% → 85.2%, verify 90.6% → 90.5%, writer
  76.8% → 79.9%, root 100% unchanged.

### Removed
- Dead helpers `findNextSection` and `peekIsNextSection`
  (no callers).
- The unusual `ParseError.Is` method (callers should use
  `errors.As`).
- The dead `error` return from `v1RowToSample`.

## [0.1.0] - 2026-10-13

### Added

- Initial release.
- `illsample.Parse` (auto-detect) and `illsample.ParseV1` /
  `illsample.ParseV2` for parsing Illumina sample sheets.
- `illsample.Write` and `illsample.WriteV1` / `illsample.WriteV2` for
  emitting sheets back to disk. Round-trips are lossless for
  well-formed input.
- `illsample.Verify` and `illsample.VerifyWithOptions` for running
  validation rules. Each finding is an `Issue` with a `Severity`
  (error or warning) and a `Location` (sheet / section / row / column).
- Layered severity model with two options: `PromoteWarningsToErrors`
  and `SuppressWarnings`.
- CLI binary `illsample` with `validate`, `convert`, `info`,
  `version`, and `help` subcommands.
- 30 verification rules across both v1 and v2 (10 v1 errors, 13 v2
  errors, 2 v1 warnings, 4 v2 warnings, plus a nil-sheet sentinel; see
  the README for the full list).
- Fuzz test for the parser.
- Test fixtures under `testdata/` covering valid v1 (single, dual,
  extra columns), valid v2 (single, dual, empty manifests), and a
  series of invalid v1/v2 sheets.
- 100% statement coverage on the top-level package; 77% / 91% / 80%
  on the internal packages (parser / verify / writer).

[Unreleased]: https://github.com/inful/illsample/compare/v0.5.2...HEAD
[0.5.2]: https://github.com/inful/illsample/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/inful/illsample/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/inful/illsample/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/inful/illsample/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/inful/illsample/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/inful/illsample/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/inful/illsample/releases/tag/v0.1.0
