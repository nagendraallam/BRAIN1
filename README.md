# BRAIN1

Local compressed memory for Go programs, AI agents, and the terminal.
**Real ripgrep search, native Zstandard compression, and streaming Go APIs.**
No Python, shell interpolation, database, or network service is used at runtime.

## Install the CLI

Use a platform archive built by the release workflow or `scripts/release`.
Extract the entire folder and run `./brain1` (`brain1.exe` on Windows). Keep
`rg`, `zstd`, and the `lib` folder beside it; add that folder to PATH if desired.
Bundled releases include the native tools and non-system libraries. Mac bundles
are ad-hoc signed, not notarized; Linux bundles require a compatible glibc system.

To build from source, install Go 1.25+, ripgrep and Zstandard:

```sh
# macOS
brew install go ripgrep zstd
# Ubuntu/Debian (install a current Go toolchain separately)
sudo apt-get install ripgrep zstd

go build -o bin/brain1 ./cmd/brain1
go test -race ./...
```

`go install github.com/nagendraallam/BRAIN1/cmd/brain1@<published-tag>` installs
only the Go executable; it does not install native dependencies. The source
build uses vendored Go decompression code and can build offline.

## CLI

```sh
brain1 add "Moved JWT validation into auth.go" --name auth
cat /path/to/large.txt | brain1 add --name archive
brain1 find 'jwt|validation'
brain1 read auth
brain1 list
BRAIN1_COMPRESSION=dense brain1 add 'Long-lived notes' --name notes
```

Storage defaults to `~/.config/brain1`; override with `BRAIN1_DIR`.
Duplicate names receive `_2`, `_3`, etc. Read accepts a name, filename or unique
prefix. Existing `.md.zst` memories remain compatible. Data is published
atomically; failed writes leave no visible entry.

## Use as a Go package

After publishing a version tag, consumers can run:

```sh
go get github.com/nagendraallam/BRAIN1@<published-tag>
```

Go modules are distributed from Git tags; there is no separate package upload.
Until published, use a local `replace` in the consuming application's go.mod.
A complete API example:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    brain1 "github.com/nagendraallam/BRAIN1"
)

func main() {
    store, err := brain1.OpenWithOptions("", brain1.Options{
        Compression: brain1.Balanced,
        // Optionally specify bundled tools using absolute paths:
        // RipgrepPath: "/path/to/bundle/rg",
        // ZstdPath: "/path/to/bundle/zstd",
    })
    if err != nil { log.Fatal(err) }
    entry, err := store.Add("JWT validation moved into auth.go", "auth")
    if err != nil { log.Fatal(err) }
    matches, err := store.FindContext(context.Background(), "jwt")
    if err != nil { log.Fatal(err) }
    fmt.Println(matches)
    if _, err := store.ReadTo(entry.Name, os.Stdout); err != nil { log.Fatal(err) }
}
```

`AddReader(io.Reader, name)` and `ReadTo(name, io.Writer)` handle large streams.
`Add`/`Read` are string conveniences; `Read` materializes the full text.
`List`, `Find`, and cancellable `FindContext` return structured values/errors.
Store operations may run concurrently. The application must provide the native
tools: this API invokes real executables, not an in-process ripgrep binding.
Resolution order is explicit options, `BRAIN1_RG`/`BRAIN1_ZSTD`, executable
siblings, then PATH. Missing dependencies produce errors, never a slower
silent fallback. ZstdPath must use the filename `zstd` (`zstd.exe` on Windows).

## Compression and memory

| Preset | Native Zstandard level | Intended use |
|---|---:|---|
| `fast` | 1 | Prioritize ingestion speed |
| `balanced` (default) | 9 | Balance size and write time |
| `dense` | 19 | Favor smaller storage; much slower writes |

Two compression workers are used. Higher levels are not guaranteed to make
every input smaller. No compressor or finder is universally best; these are
proven engines with explicit trade-offs, not a claim of a world record.

A 100 GB entry is processed in chunks by AddReader/ReadTo. Search streams
ripgrep JSON but retains returned matching lines. Broad queries can therefore
use significant memory. ReadTo uses the Go Zstandard decoder with a 128 MiB
window/memory bound; supported presets generate compatible windows. Find uses
ripgrep's default Rust regex engine, case-insensitively, scanning the complete
store with no index. There is no random byte seeking inside a compressed file.

## Measured results

500,000,000 bytes from the start of the Wikipedia `enwik9` corpus, supplied as
[enwik9.zip](https://mattmahoney.net/dc/enwik9.zip), tested on an Apple M4,
macOS arm64, Go 1.27.1, ripgrep 15.2.0, Zstandard 1.5.7 (2026-09-30).
Both current tools searched the same `.zst`; matching lines were identical.

| Measurement | BRAIN1 with native engines | Direct ripgrep |
|---|---:|---:|
| `Alan Turing` (174 lines) | 0.431 s | 0.420 s |
| `quantum mechanics` (910 lines) | 0.426 s | 0.411 s |
| `<title>.*computer.*</title>` (229 lines) | 0.419 s | 0.411 s |
| Missing term | 0.419 s | 0.409 s |

Warm-cache medians of 3 end-to-end CLI runs, alternating tool order, outputs
discarded after correctness checks. Small differences are not proof of a
universal performance ranking. Native compression produced **143,700,673 bytes
(143.70 MB, 71.26% smaller)** in **4.50 s** (one measurement).

The earlier implementation used Go regexp: phrase/miss searches took
4.90–5.45 s and compression produced 153.52 MB. The current measured phrase
searches are about 12× faster; title regex improvement is smaller. Those old
measurements used the old compressed output, so they compare complete versions,
not an isolated regex-engine microbenchmark. Peak RAM was not reliably measured.

The input sample SHA-256 was
`81dfe6fd07b2f575b368e7228f14fef33871e6b99eafca96ee036b3acbfdc836`.
Do not infer random-access speed or cold-cache performance from these tests.

## Benchmarks and distribution

- [Performance runner](scripts/performance/README.md): reproduce corpus tests.
- [Release instructions](scripts/release/README.md): native bundles, archives,
  checksums, and publishing Go module tags.
- `go test -race ./...` and `go vet ./...` validate the package and CLI.

MIT license for BRAIN1. Native tools/libraries retain their own notices, included
in release archives. Go dependencies are vendored with their license files.
