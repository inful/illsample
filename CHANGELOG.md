# Changelog

All notable changes to `illsample` are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed
- v1 sheets with the no-underscore `I7IndexID` column now place values
  in `I7IndexID` (previously they were silently mis-routed to
  `I5IndexID`).
- v2 sheets no longer drop unknown `[Data]` columns on round-trip.
- `[Settings]` rows 2+ are no longer silently dropped; last non-empty
  value wins per column.
- `ParseV1` and `ParseV2` now reject non-UTF-8 input, matching
  `Parse`.
- Writer no longer emits cells starting with `=`, `+`, `-`, `@`,
  `\t`, or `\r` verbatim — they are prefixed with `'` to prevent
  CSV-injection when opened in Excel / Google Sheets.
- v1 sheets with whitespace-padded column headers round-trip without
  data loss.
- v2 sheets whose preamble is a `//` comment are now correctly
  detected as v2.
- `[Settings]` keys are now matched case-insensitively.
- Error line numbers in `[Data]`, `[Settings]`, and unknown sections
  are no longer off by the count of leading blank/comment lines.
- `[Manifests]` "A" header heuristic now uses a one-line look-ahead
  so a single-row `[Manifests] A` is no longer silently dropped.
- CLI `validate` now writes errors and warnings to stderr; only the
  summary line goes to stdout.
- CLI `convert` no longer silently flips `FormatUnknown` sheets to
  v1; the user must specify `-f` explicitly.

### Changed
- Code review applied: 25 findings from `01-REVIEW.md` (2 critical,
  3 high, 11 warnings, 9 info) addressed. The remaining 3 (IN-07 row
  indexing convention, IN-09 CRLF, WR-05 `\r` preservation) are
  deferred — see the review for rationale.

### Removed
- Dead helpers `findNextSection` and `peekIsNextSection` (no callers).
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

[Unreleased]: https://github.com/inful/illsample/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/inful/illsample/releases/tag/v0.1.0
