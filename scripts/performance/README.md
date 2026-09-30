# Performance runner

Run from the repository root with Go 1.25 or newer. Dependencies are vendored.

## Your own text file

```sh
go run ./scripts/performance -input /absolute/path/to/text.txt -runs 3
```

The input is never modified. The runner uses the entire supplied file.
`-mb` controls generated synthetic input only; it does not truncate `-input`.

## Reproduce the 500 MB Wikipedia benchmark

Download https://mattmahoney.net/dc/enwik9.zip yourself. The ZIP contains the
1,000,000,000-byte enwik9 text. Prepare a 500,000,000-byte sample outside this
repository (allow roughly 2 GB of working space):

```sh
mkdir -p /tmp/brain1-benchmark-input
unzip /path/to/enwik9.zip -d /tmp/brain1-benchmark-input
head -c 500000000 /tmp/brain1-benchmark-input/enwik9 > /tmp/brain1-benchmark-input/enwik9-500MB.txt
go run ./scripts/performance -input /tmp/brain1-benchmark-input/enwik9-500MB.txt -runs 3
```

On Windows, extract the ZIP and provide a text file directly; the preparation
commands above target macOS/Linux. The runner itself uses Go on all platforms.

## Generated input

```sh
go run ./scripts/performance -mb 500 -runs 3
```

This generates deterministic synthetic log-like text with seed 42. It is not
Wikipedia and should not be compared as if it were the same corpus.

## Output and cleanup

Each run creates a unique `run-*` directory in the system temporary directory.
Use `-output /existing/directory` to choose another parent directory. The runner
prints the result JSON to stdout and the result path/progress to stderr.
The run directory contains `results.json`, compressed corpus storage, and
1,000 small test notes; synthetic runs also contain their generated input.
Remove that specific run directory when finished. Your supplied input is left
untouched. No files are automatically downloaded or pushed.

## What is measured

- Compression and durable save: one measurement, including fsync.
- Full decompressed read into io.Discard: median/min/max of `-runs` repetitions.
- Case-insensitive regex search: hit and miss queries, full corpus scans.
- Random retrieval: 100 reads among 1,000 separate ~2 KB synthetic notes.
- Listing: all 1,000 small notes, repeated `-runs` times.
- SHA-256 and byte-count round-trip verification before read timing.
- Search result counts compared with an independent uncompressed scan.

Results use decimal MB and warm caches. Read excludes terminal rendering and
hashing. There is no random seeking within a large compressed entry. The
program retains matching search lines; broad queries can use more memory.
Real-input queries may have zero matches depending on the supplied corpus.
Input lines longer than 64 MiB are outside the search API's supported limit.
The historical results in the main README are measurements, not guarantees.

Small allocation/throughput benchmarks are also available:

```sh
go test -run '^$' -bench . -benchmem .
```
