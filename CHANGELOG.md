# Changelog

All notable changes to `illsample` are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
- 18 verification rules across both v1 and v2 (see the README for
  the full list).
- Fuzz test for the parser.
- Test fixtures under `testdata/` covering valid v1 (single, dual,
  extra columns), valid v2 (single, dual, empty manifests), and a
  series of invalid v1/v2 sheets.
- 100% statement coverage on the top-level package; 73% / 91% / 77%
  on the internal packages.

[Unreleased]: https://github.com/inful/illsample/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/inful/illsample/releases/tag/v0.1.0
