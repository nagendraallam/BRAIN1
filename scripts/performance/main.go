// A reproducible offline performance example. Run from the repo root:
// go run ./scripts/performance -mb 500 -runs 3
// Supply -input /path/to/real/text.txt to benchmark your own corpus instead.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	brain1 "github.com/nagendraallam/BRAIN1"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type Timing struct {
	MedianMS  float64   `json:"median_ms"`
	MinMS     float64   `json:"min_ms"`
	MaxMS     float64   `json:"max_ms"`
	MBPerSec  float64   `json:"mb_per_sec,omitempty"`
	SamplesMS []float64 `json:"samples_ms"`
}

func timed(runs int, size int64, fn func()) Timing {
	samples := make([]float64, runs)
	for i := range samples {
		start := time.Now()
		fn()
		samples[i] = float64(time.Since(start)) / float64(time.Millisecond)
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + median) / 2
	}
	t := Timing{MedianMS: median, MinMS: sorted[0], MaxMS: sorted[len(sorted)-1], SamplesMS: samples}
	if size > 0 {
		t.MBPerSec = float64(size) / 1e6 / (median / 1000)
	}
	return t
}
func generate(path string, size int64) {
	f, err := os.Create(path)
	check(err)
	w := bufio.NewWriterSize(f, 1<<20)
	rng := rand.New(rand.NewSource(42))
	services := []string{"auth", "payments", "inventory", "search", "notifications", "gateway", "billing", "users"}
	actions := []string{"deployed revision", "investigated latency", "reviewed migration", "resolved timeout", "updated permissions", "verified rollback", "repaired cache", "completed backup"}
	rows := size / 256
	for i := int64(0); i < rows; i++ {
		marker := "ordinary-note"
		if i == 0 {
			marker = "BRAIN1_MARKER_START"
		}
		if i == rows/2 {
			marker = "BRAIN1_MARKER_MIDDLE"
		}
		if i == rows-1 {
			marker = "BRAIN1_MARKER_END"
		}
		line := fmt.Sprintf("record=%09d service=%s action=%s user=%08x duration=%dms trace=%016x%016x%016x status=%d %s", i, services[rng.Intn(len(services))], actions[rng.Intn(len(actions))], rng.Uint32(), rng.Intn(20000), rng.Uint64(), rng.Uint64(), rng.Uint64(), 200+rng.Intn(400), marker)
		if len(line) > 255 {
			panic("row too long")
		}
		line += strings.Repeat(" ", 255-len(line)) + "\n"
		_, err = w.WriteString(line)
		check(err)
	}
	if remainder := size % 256; remainder > 0 {
		_, err = w.WriteString(strings.Repeat("x", int(remainder)))
		check(err)
	}
	check(w.Flush())
	check(f.Close())
}
func hashFile(path string) string {
	f, err := os.Open(path)
	check(err)
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	check(err)
	return hex.EncodeToString(h.Sum(nil))
}
func main() {
	mb := flag.Int64("mb", 500, "synthetic input size in decimal MB")
	runs := flag.Int("runs", 3, "read/search repetitions")
	input := flag.String("input", "", "optional real text file (not modified)")
	output := flag.String("output", "", "parent directory for benchmark artifacts (default: system temp)")
	flag.Parse()
	if *runs < 1 || *mb < 1 {
		panic("mb and runs must be positive")
	}
	dir, err := os.MkdirTemp(*output, "run-")
	check(err)
	synthetic := *input == ""
	if synthetic {
		*input = filepath.Join(dir, "synthetic-memory.txt")
		fmt.Fprintln(os.Stderr, "Generating", *mb, "MB reproducible synthetic text")
		generate(*input, *mb*1000000)
	}
	info, err := os.Stat(*input)
	check(err)
	size := info.Size()
	wantHash := hashFile(*input)
	s, err := brain1.Open(filepath.Join(dir, "store"))
	check(err)
	var entry brain1.Entry
	fmt.Fprintln(os.Stderr, "Compressing", size, "bytes")
	write := timed(1, size, func() {
		f, err := os.Open(*input)
		check(err)
		defer f.Close()
		entry, err = s.AddReader(f, "corpus")
		check(err)
	})
	h := sha256.New()
	n, err := s.ReadTo(entry.Name, h)
	check(err)
	gotHash := hex.EncodeToString(h.Sum(nil))
	if n != size || gotHash != wantHash {
		panic("roundtrip mismatch")
	}
	fmt.Fprintln(os.Stderr, "Timing full streaming reads")
	read := timed(*runs, size, func() {
		n, err := s.ReadTo(entry.Name, io.Discard)
		check(err)
		if n != size {
			panic("wrong read size")
		}
	})
	queries := []string{"BRAIN1_MARKER_START", "BRAIN1_MARKER_MIDDLE", "BRAIN1_MARKER_END", "BRAIN1_NEVER_PRESENT"}
	if !synthetic {
		queries = []string{"Alan Turing", "quantum mechanics", "<title>.*computer.*</title>", "BRAIN1_NEVER_PRESENT"}
	}
	// Independently scan the uncompressed input to check every result count.
	expected := map[string]int{}
	fmt.Fprintln(os.Stderr, "Validating query counts against uncompressed text")
	f, err := os.Open(*input)
	check(err)
	expressions := make([]*regexp.Regexp, len(queries))
	for i, q := range queries {
		expressions[i] = regexp.MustCompile("(?i)" + q)
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 64<<20)
	for scanner.Scan() {
		for i, re := range expressions {
			if re.Match(scanner.Bytes()) {
				expected[queries[i]]++
			}
		}
	}
	check(scanner.Err())
	check(f.Close())
	search := map[string]Timing{}
	for _, q := range queries {
		fmt.Fprintln(os.Stderr, "Timing full search:", q)
		search[q] = timed(*runs, size, func() {
			matches, err := s.Find(q)
			check(err)
			count := 0
			for _, m := range matches {
				count += len(m.Lines)
			}
			if count != expected[q] {
				panic(fmt.Sprintf("wrong result count for %s: got %d want %d", q, count, expected[q]))
			}
		})
	}
	small, err := brain1.Open(filepath.Join(dir, "small-store"))
	check(err)
	fmt.Fprintln(os.Stderr, "Creating 1,000 small memories for random retrieval")
	names := make([]string, 1000)
	for i := range names {
		e, err := small.Add(strings.Repeat(fmt.Sprintf("Memory %d: deployment checked and rollback verified.\n", i), 40), fmt.Sprintf("note-%04d", i))
		check(err)
		names[i] = e.Name
	}
	rng := rand.New(rand.NewSource(9))
	randomRead := timed(100, 0, func() { _, err := small.Read(names[rng.Intn(len(names))]); check(err) })
	listing := timed(*runs, 0, func() {
		entries, err := small.List()
		check(err)
		if len(entries) != 1000 {
			panic("wrong list count")
		}
	})
	result := map[string]any{"input_path": *input, "synthetic": synthetic, "input_bytes": size, "compressed_bytes": entry.Size, "compression_ratio": float64(size) / float64(entry.Size), "space_saved_percent": 100 * (1 - float64(entry.Size)/float64(size)), "sha256": wantHash, "roundtrip_verified": true, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "logical_cpus": runtime.NumCPU(), "write": write, "stream_read": read, "search": search, "matching_lines": expected, "random_read_1000_entries": randomRead, "list_1000_entries": listing, "notes": "Decimal MB. Warm filesystem cache. Write is one sample including fsync; read excludes hashing/output rendering; search collects matching lines and always scans the entire corpus. See synthetic flag: generated input uses seed 42; supplied input is read unchanged. Not a cold-cache benchmark. No random byte seeking inside a compressed entry."}
	data, err := json.MarshalIndent(result, "", "  ")
	check(err)
	check(os.WriteFile(filepath.Join(dir, "results.json"), data, 0600))
	fmt.Println(string(data))
	fmt.Fprintln(os.Stderr, "Results:", filepath.Join(dir, "results.json"))
}
