# Release Defuddle

Release both Go modules with one version. The root tag publishes the library
module; the `cmd/defuddle/` tag makes `go install
github.com/dotcommander/defuddle/cmd/defuddle@latest` resolve the same release.

## Prepare

Start from `main` with the intended code and documentation committed and a clean
worktree. Complete the full workspace checks before the unpublished CLI pin, or
run equivalent bounded gates with that pin and the workspace's local library.
`task verify` includes tidy steps that may need the published version:

```bash
task verify
git status --short
```

Choose the next semantic version from the user-visible changes and record it in
`CHANGELOG.md`. Versions come from Git tags; the `version = "dev"` values in
`version.go` and `cmd/defuddle/main.go` are build-time fallbacks and are not
edited for a release.

Update the CLI library requirement before creating either release tag:

```bash
cd cmd/defuddle
go mod edit -require=github.com/dotcommander/defuddle@v0.16.0
cd ../..
git add cmd/defuddle/go.mod
git commit -m "build(cli): require defuddle v0.16.0"
```

The CLI's new library version cannot resolve remotely until the root tag is
published. For prepublication workspace build/test/vet/lint gates, add this
**temporary**, version-qualified mapping to the actual `go.work`:

```go
replace github.com/dotcommander/defuddle v0.16.0 => .
```

Remove only that mapping after the gates pass; never commit it. Required workspace
commands maintain `go.work.sum`; retain their legitimate checksum updates and
commit them before the root tag. Do not use `go work sync` here: it can promote
module requirements and attempt to fetch the unpublished pin. Defer standalone
CLI tidy/checksum finalization until root publication and complete it before the
CLI tag. Both tags must reference commits whose `go.mod` already has the intended
version. Workspace checks do not establish standalone CLI acceptance.

The module minimum remains Go 1.26.4. Run the vulnerability gate with an installed,
patched toolchain (Go 1.27.1 for this release) and record its actual version. A
passed scan with that toolchain does not establish that Go 1.26.4 is free of
standard-library vulnerabilities.

## Publish both modules

```bash
task tag VERSION=v0.16.0
```

`task tag` deliberately releases in this order:

1. Run root-module race tests, vet, and vulnerability scanning with
   `GOWORK=off`.
2. Confirm `cmd/defuddle/go.mod` already requires the same `vX.Y.Z` version.
3. Create and push the root `vX.Y.Z` tag.
4. Run `GOWORK=off go mod tidy`, `GOWORK=off go test ./...`, and
   `GOWORK=off go build ./...` inside
   `cmd/defuddle/`. Disabling the workspace proves the CLI builds against the
   released library rather than the local checkout.
5. Commit any checksum change, then create and push
   `cmd/defuddle/vX.Y.Z`.

The root tag must be available through the Go module proxy before the standalone
CLI verification can succeed. If propagation is delayed, rerun the CLI phase
after the version resolves; do not change the already-correct CLI version pin.

## Manual publication with existing receipts

When a verifier has already completed the applicable checks on the unchanged
release tree, reuse those receipts rather than rerunning `task tag`'s gates.
Create the annotated root `v0.16.0` tag on the prepared commit and push the branch
and root tag. Then, inside `cmd/defuddle`, run the standalone phase with
`GOWORK=off`: tidy, build, tests, vet, configured lint, and module verification.
Commit resulting module sums and workspace-maintained `go.work.sum` before creating and
pushing `cmd/defuddle/v0.16.0`. Do not alter the already-published root tag.

Create a GitHub release for the root tag with the v0.16.0 changelog notes. The
CLI tag publishes the nested Go module; it does not require a second GitHub
release. Consumer projects such as jinn and webfetch retain their existing calls
and can bump the library dependency after publication; no engine migration is
needed in those projects.

## Verify the public release

```bash
git ls-remote --tags origin v0.16.0 cmd/defuddle/v0.16.0
# Use a scoped GOBIN to avoid replacing an existing installation.
release_bin=$(mktemp -d)
GOBIN="$release_bin" GOWORK=off go install github.com/dotcommander/defuddle/cmd/defuddle@v0.16.0
"$release_bin/defuddle" --version
```

No prebuilt archives are published. The supported binary distribution path is
`go install` from the CLI module tag.

## Recovery

Do not move or overwrite a published tag. Fix the defect on `main` and publish a
new patch release. If the root tag succeeded but the CLI phase failed, leave the
root release intact, fix the CLI module or wait for module propagation, rerun
the standalone CLI checks, and then create the matching CLI tag.
