# Release Defuddle

Release both Go modules with one version. The root tag publishes GitHub release
artifacts; the `cmd/defuddle/` tag makes `go install
github.com/dotcommander/defuddle/cmd/defuddle@latest` resolve the same release.

## Prepare

Start from `main` with the intended code and documentation committed and no
unrelated changes in the worktree. Keep the CLI module on the current published
library version while running the full workspace checks:

```bash
task verify
task snapshot
git status --short
```

Choose the next semantic version from the user-visible changes and record it in
`CHANGELOG.md`. Versions come from Git tags; the `version = "dev"` values in
`version.go` and `cmd/defuddle/main.go` are build-time fallbacks and are not
edited for a release.

After those checks pass, update only the CLI library requirement and commit it:

```bash
cd cmd/defuddle
go mod edit -require=github.com/dotcommander/defuddle@v0.13.0
cd ../..
git add cmd/defuddle/go.mod
git commit -m "build(cli): require defuddle v0.13.0"
```

Do not run `go mod tidy` for the CLI yet. The new root module version does not
exist remotely, so Go cannot produce its checksum until the root tag is pushed.

## Publish both modules

```bash
task tag VERSION=v0.13.0
```

`task tag` deliberately releases in this order:

1. Run root-module race tests, vet, vulnerability scanning, and GoReleaser
   configuration checks with `GOWORK=off`.
2. Confirm `cmd/defuddle/go.mod` already requires the same `vX.Y.Z` version.
3. Create and push the root `vX.Y.Z` tag. This triggers the GitHub Actions and
   GoReleaser workflow.
4. Run `GOWORK=off go mod tidy`, `GOWORK=off go test ./...`, and
   `GOWORK=off go build ./...` inside
   `cmd/defuddle/`. Disabling the workspace proves the CLI builds against the
   released library rather than the local checkout.
5. Commit any checksum change, then create and push
   `cmd/defuddle/vX.Y.Z`.

The root tag must be available through the Go module proxy before the standalone
CLI verification can succeed. If propagation is delayed, rerun the CLI phase
after the version resolves; do not change the already-correct CLI version pin.
The root release workflow tests the CLI against the tagged workspace and then
restores the generated `go.work.sum` change before GoReleaser validates that the
checkout is clean.

## Verify the public release

```bash
git ls-remote --tags origin v0.13.0 cmd/defuddle/v0.13.0
GOWORK=off go install github.com/dotcommander/defuddle/cmd/defuddle@v0.13.0
defuddle --version
```

Also confirm the [release workflow](https://github.com/dotcommander/defuddle/actions)
completed and the [GitHub release](https://github.com/dotcommander/defuddle/releases)
contains archives and checksums for Linux, macOS, and Windows.

## Recovery

Do not move or overwrite a published tag. Fix the defect on `main` and publish a
new patch release. If the root tag succeeded but the CLI phase failed, leave the
root release intact, fix the CLI module or wait for module propagation, rerun
the standalone CLI checks, and then create the matching CLI tag.
