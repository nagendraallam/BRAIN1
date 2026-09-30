# Build and distribute BRAIN1

## Local host-platform release

Install Go 1.25+, ripgrep and Zstandard. Run from the repository root:

```sh
go run ./scripts/release -version v0.2.0
```

This creates `dist/brain1-v0.2.0-<os>-<arch>/`, a tar.gz (ZIP on Windows),
and a SHA-256 checksum. It builds the Go CLI, bundles real `rg` and `zstd`,
collects native dependencies and notices, and records tool versions. Supply
`-rg /path/to/rg -zstd /path/to/zstd` to select binaries. Do not supply Windows
package-manager shims; use the real executable paths.

Mac packaging uses otool/install_name_tool to relocate non-system libraries,
then ad-hoc signs them. It currently supports Homebrew's native dependency
layout; custom @rpath layouts may require explicit changes. No Homebrew paths
are needed when running the completed bundle. It is not Apple-notarized.
Linux packaging collects ldd dependencies except glibc components; build on the
oldest glibc distribution you support. This is not a musl/Alpine package.
Windows packaging copies adjacent DLLs; use standalone upstream releases.
Native dependency licensing remains the distributor's responsibility; inspect
the licenses directory before publishing binaries from custom locations.

The archive is for the build host's OS/architecture. Cross-compiling only the
Go executable is insufficient because native tools must match. The workflow
builds on separate macOS, Linux and Windows hosts and uploads artifacts. A tag
run also creates a **draft** GitHub release for review; it does not publish it.

## Verify a bundle

Keep every file in the archive together. Run its brain1 with a PATH that does
not contain system ripgrep/zstd, then add, find and read a memory. Check the
archive checksum before distribution. An extracted bundle should work on a
compatible OS without separate ripgrep or Zstandard installation.

## Publish as a Go module

Go uses Git tags and the module path already in go.mod. No npm-style publish
command exists. After committing and pushing the tested source, choose an unused
semantic version (v0.2.0 below is an example, not an already-published release):

```sh
git tag v0.2.0
git push origin master
git push origin v0.2.0
```

Consumers then run:

```sh
go get github.com/nagendraallam/BRAIN1@v0.2.0
go install github.com/nagendraallam/BRAIN1/cmd/brain1@v0.2.0
```

Those commands fetch/build Go code only. Consumers must install native tools
or point Options.RipgrepPath/ZstdPath at an extracted BRAIN1 bundle. Importing
the Go module does not download or execute installers. Local prepublication
integration uses `go mod edit -replace github.com/nagendraallam/BRAIN1=/path/to/BRAIN1`.
