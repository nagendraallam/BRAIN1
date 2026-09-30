# BRAIN1 — memory in Go

A local, compressed memory store for agents and developers. Use it directly as
a Go package or through the CLI. No Python, ripgrep, shell commands, zstd
executable, database, or server is required at runtime.

## Build

Requires Go 1.25 or newer. The pure-Go compression dependency is vendored, so
builds and tests work offline:

```sh
go build -o bin/brain1 ./cmd/brain1
go test -race ./...
./bin/brain1 --help
```

Install on your PATH from this checkout with `go install ./cmd/brain1`.

## Use the package

```go
import brain1 "github.com/nagendraallam/BRAIN1"

store, err := brain1.Open("") // BRAIN1_DIR, or ~/.config/brain1
if err != nil { return err }
entry, err := store.Add("JWT logic moved to auth.go", "auth_notes")
if err != nil { return err }
text, err := store.Read(entry.Name)
if err != nil { return err }
fmt.Println(text)
matches, err := store.Find("jwt|auth")
if err != nil { return err }
fmt.Println(matches)
entries, err := store.List()
if err != nil { return err }
fmt.Println(entries)
```

The public API is `Open`, `Store.Add`, `Store.Find`, `Store.Read`, `Store.List`, plus streaming `Store.AddReader` and `Store.ReadTo`.
Operations return errors rather than exiting your process. A Store may be used
concurrently. The rewrite must be published before other projects can download
this Go module; for now use a local `replace` directive.

## CLI

```sh
./bin/brain1 add "Finished refactoring auth" --name auth_notes
echo "Database migration complete" | ./bin/brain1 add
./bin/brain1 find "auth|migration"
./bin/brain1 read auth_notes
./bin/brain1 list
```

Names receive `_2`, `_3`, etc. on collision. Read accepts a filename, stem, or
unique prefix; ambiguous prefixes return an error. Use `--` before add text
beginning with a dash. `--help` works globally and after each command.
Set `BRAIN1_DIR` to use another storage folder.

## Compatibility and implementation

- Existing Python `.md.zst` files at `~/.config/brain1` remain readable without
  conversion. The Python implementation and packaging have been removed.
- Compression uses `github.com/klauspost/compress/zstd` v1.20.1 (pure Go).
- Search uses Go's standard `regexp` package, case-insensitively, line by line.
  It retains regex behavior, but Go RE2 syntax differs from ripgrep's Rust
  regex syntax. There is no ripgrep subprocess or Go wrapper.
- Search scans and decompresses files on each call; it is not indexed.
  No speed claim is made against ripgrep.
- AddReader and ReadTo stream large entries; the CLI streams stdin and read output.
  Find streams decompression but retains matching lines. Individual search lines
  are limited to 64 MiB; total entry size has no 64 MiB cap. Corrupt files produce errors.
- New files use owner-only permissions. Complete files are published with an
  atomic hard link to prevent overwrites and partially visible writes. Storage
  must support hard links (normal APFS, ext4 and NTFS directories do).
- The package and CLI make no network calls.

## Memory use

`AddReader` and `ReadTo` process chunks, so a 100 GB entry does not require
100 GB of RAM. `Read` returns the whole string and therefore does require
memory proportional to the uncompressed entry. Search streams decompression,
scans the entire store, and retains matching lines in memory. There is no
random byte seeking or search index. Codec buffers also consume memory.

## Performance results

Tested on an Apple M4 (10 logical CPUs), macOS arm64, Go 1.27.1, on 2026-09-30.
Input was the **first 500,000,000 bytes of enwik9**, the Wikipedia text corpus
from [enwik9.zip](https://mattmahoney.net/dc/enwik9.zip). The full ZIP member
contains 1,000,000,000 bytes. No synthetic data was added to this sample.

| Measurement | Result |
|---|---:|
| Original size | 500.00 MB |
| Compressed size | 153.52 MB (153,524,561 bytes) |
| Space saved | 69.30% (3.26:1) |
| Compression and save, including fsync | 2.536 s (197.1 MB/s) |
| Full streaming read | 0.360 s (1,388.2 MB/s) |
| Search `Alan Turing` (174 matching lines) | 5.226 s |
| Search `quantum mechanics` (910 matching lines) | 4.672 s |
| Search `<title>.*computer.*</title>` (229 matching lines) | 0.612 s |
| Search absent term | 4.789 s |
| Random small-note retrieval among 1,000 entries | 5.565 ms |
| List 1,000 entries | 5.219 ms |

Decimal MB; warm filesystem caches. Read/search/list values are medians of
three runs; compression is one sample. Random retrieval is the median of 100
reads of separate ~2 KB synthetic notes, not random access inside the corpus.
Read discards output, excluding terminal rendering and integrity hashing.
Every search scans the complete corpus. Query structure affects runtime.

SHA-256 round-trip verification passed:
`81dfe6fd07b2f575b368e7228f14fef33871e6b99eafca96ee036b3acbfdc836`.
Search result counts matched an independent uncompressed scan. Race tests and
`go vet` passed; core test coverage was 87.0%, CLI coverage 73.7%.
Downloaded data, generated stores, and example artifacts are not included.

## Run the benchmark

The performance runner is retained at `scripts/performance/main.go`.
See [benchmark instructions](scripts/performance/README.md) for input setup,
reproduction commands, output locations, and interpretation.

```sh
go run ./scripts/performance -input /path/to/text.txt -runs 3
go test -run '^$' -bench . -benchmem .
```

The pure-Go compression dependency is vendored for offline builds; its license
is included at `vendor/github.com/klauspost/compress/LICENSE`.
