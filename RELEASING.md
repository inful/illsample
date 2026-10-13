# Releasing

The release process is fully automated via [GoReleaser] and
GitHub Actions. A maintainer only needs to push a tag.

## Process

1. Make sure `main` is green (CI passes).

2. Decide on the next version. The project follows [Semantic
   Versioning]: `vMAJOR.MINOR.PATCH`.
   - Backwards-compatible changes → bump PATCH
   - Backwards-compatible features → bump MINOR
   - Breaking changes → bump MAJOR

3. Make sure `CHANGELOG.md` has an entry for the new version under
   the `[Unreleased]` heading. Move it under a new
   `## [vX.Y.Z] - YYYY-MM-DD` heading.

4. Tag the release commit:

   ```sh
   git tag -s v0.2.0 -m "v0.2.0"
   git push origin v0.2.0
   ```

   The `release.yml` workflow runs GoReleaser which builds the
   binaries, drafts a GitHub release (auto-promoted unless the
   tag is `-alpha`/`-beta`/`-rc`), and uploads the artifacts.

5. Verify the release at <https://github.com/inful/illsample/releases>.

## What the workflow does

`release.yml` runs on every `v*` tag push. It:

1. Checks out the repo with full history.
2. Sets up Go 1.22.
3. Runs the test suite (with `-race`).
4. Runs `goreleaser release --clean` which:
   - Cleans the `dist/` directory.
   - Runs `go mod tidy` and `go test -race` (configured in the
     `before.hooks` block of `.goreleaser.yml`).
   - Builds binaries for `linux/darwin/windows/freebsd` on
     `amd64/arm64` and `linux/darwin` on `386`.
   - Packages each binary as `tar.gz` (or `zip` on Windows) with
     `LICENSE`, `README.md`, and `CHANGELOG.md` included.
   - Generates a `SHA256SUMS` file.
   - Auto-generates the changelog from merged PRs since the
     previous tag.
   - Creates a GitHub release with all of the above.

## Snapshot builds

Every pull request runs the `snapshot.yml` workflow, which builds
the same artifacts as a release but does not publish anything.
Reviewers can download the artifacts from the workflow run page
to test the change before it lands.

## Local dry run

To verify the release config locally without publishing:

```sh
# Requires goreleaser installed: go install github.com/goreleaser/goreleaser/v2@latest
goreleaser release --clean --snapshot
ls -la dist/
```

## Versioning in the binary

The release workflow injects version metadata into the CLI binary
via `-ldflags`:

```sh
-X main.version=0.2.0
-X main.commit=abc1234
-X main.date=2026-10-13T10:00:00Z
```

Run `illsample version` to see what was baked in.

[GoReleaser]: https://goreleaser.com
[Semantic Versioning]: https://semver.org
