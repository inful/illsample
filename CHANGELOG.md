# Changelog

All notable changes to `illsample` are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
  and info items are addressed in this release. See
  [`01-REVIEW.md`](01-REVIEW.md) for the full review; the
  remaining 3 (IN-07 row-indexing convention, IN-09 CRLF
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

[Unreleased]: https://github.com/inful/illsample/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/inful/illsample/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/inful/illsample/releases/tag/v0.1.0
