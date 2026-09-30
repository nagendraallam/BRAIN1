// Package brain1 provides an offline, compressed memory store without subprocesses.
package brain1

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
)

const Extension = ".md.zst"
const maxLineSize = 64 << 20

// Store can be shared by goroutines. Each operation owns its codec resources.
type Store struct{ dir string }
type Entry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}
type Match struct {
	Name  string   `json:"name"`
	Lines []string `json:"lines"`
}

// Open uses BRAIN1_DIR, then the original Python location ~/.config/brain1,
// when dir is empty. Existing Python .md.zst entries need no migration.
func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = os.Getenv("BRAIN1_DIR")
	}
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(home, ".config", "brain1")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func sanitize(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), " ", "_")
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-' {
			return r
		}
		return -1
	}, name)
	if name == "" {
		return "entry"
	}
	return name
}

// Add stores an in-memory string. Use AddReader for large inputs.
func (s *Store) Add(content, name string) (Entry, error) {
	return s.AddReader(strings.NewReader(content), name)
}

// AddReader compresses a stream without buffering the complete input.
// A complete entry is published atomically; failed or empty inputs leave no entry.
func (s *Store) AddReader(content io.Reader, name string) (entry Entry, err error) {
	if content == nil {
		return entry, errors.New("nil content reader")
	}
	if name == "" {
		name = fmt.Sprint(time.Now().UnixMilli())
	}
	name = sanitize(name)
	if len(name) > 200 {
		return entry, errors.New("entry name exceeds 200 bytes")
	}
	f, err := os.CreateTemp(s.dir, ".brain1-*")
	if err != nil {
		return entry, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	enc, err := zstd.NewWriter(f, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(9)), zstd.WithEncoderConcurrency(1))
	if err != nil {
		return entry, err
	}
	defer enc.Close()
	checked := &contentCheck{dst: enc}
	if _, err = io.CopyBuffer(checked, content, make([]byte, 128<<10)); err != nil {
		return entry, err
	}
	if !checked.nonBlank && len(checked.pending) == 0 {
		return entry, errors.New("no content provided")
	}
	if err = enc.Close(); err != nil {
		return entry, err
	}
	if err = f.Sync(); err != nil {
		return entry, err
	}
	info, err := f.Stat()
	if err != nil {
		return entry, err
	}
	if err = f.Close(); err != nil {
		return entry, err
	}
	for i := 1; ; i++ {
		stem := name
		if i > 1 {
			stem = fmt.Sprintf("%s_%d", name, i)
		}
		filename := stem + Extension
		err = os.Link(f.Name(), filepath.Join(s.dir, filename))
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return entry, fmt.Errorf("publish entry: %w", err)
		}
		return Entry{Name: filename, Size: info.Size()}, nil
	}
}

// Validate Unicode whitespace even when a rune is split across reader chunks.
// Once content is detected the remaining stream passes through uninspected.
type contentCheck struct {
	dst      io.Writer
	nonBlank bool
	pending  []byte
}

func (c *contentCheck) Write(p []byte) (int, error) {
	if !c.nonBlank {
		data := p
		if len(c.pending) > 0 {
			data = append(c.pending, data...)
			c.pending = nil
		}
		for len(data) > 0 {
			if !utf8.FullRune(data) {
				c.pending = append([]byte(nil), data...)
				break
			}
			r, n := utf8.DecodeRune(data)
			if !unicode.IsSpace(r) {
				c.nonBlank = true
				c.pending = nil
				break
			}
			data = data[n:]
		}
	}
	return c.dst.Write(p)
}

func (s *Store) List() ([]Entry, error) {
	files, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0)
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), Extension) || !f.Type().IsRegular() {
			continue
		}
		info, err := f.Info()
		if err != nil {
			return nil, err
		}
		entries = append(entries, Entry{Name: f.Name(), Size: info.Size()})
	}
	return entries, nil
}

func (s *Store) resolve(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return "", errors.New("invalid entry name")
	}
	entries, err := s.List()
	if err != nil {
		return "", err
	}
	var candidates []string
	for _, e := range entries {
		if e.Name == name || e.Name == name+Extension {
			return e.Name, nil
		}
		if strings.HasPrefix(e.Name, name) {
			candidates = append(candidates, e.Name)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no entry found matching %q", name)
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("multiple entries match %q: %s", name, strings.Join(candidates, ", "))
	}
	return candidates[0], nil
}

// Read returns an entire entry as a string. For large entries use ReadTo.
func (s *Store) Read(name string) (string, error) {
	var out strings.Builder
	if _, err := s.ReadTo(name, &out); err != nil {
		return "", err
	}
	return out.String(), nil
}

// ReadTo streams an entry to dst, returning uncompressed bytes written.
// If decompression or dst fails, dst may already contain partial output.
func (s *Store) ReadTo(name string, dst io.Writer) (int64, error) {
	if dst == nil {
		return 0, errors.New("nil destination writer")
	}
	filename, err := s.resolve(name)
	if err != nil {
		return 0, err
	}
	var n int64
	err = s.withReader(filename, func(r io.Reader) error { var e error; n, e = io.Copy(dst, r); return e })
	return n, err
}

func (s *Store) withReader(name string, fn func(io.Reader) error) error {
	f, err := os.Open(filepath.Join(s.dir, name))
	if err != nil {
		return err
	}
	defer f.Close()
	dec, err := zstd.NewReader(f, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(128<<20))
	if err != nil {
		return err
	}
	defer dec.Close()
	if err = fn(dec); err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	return nil
}

// Find searches each line using a case-insensitive Go regular expression.
// Input is streamed, but returned matching lines occupy memory. Individual
// lines are limited to 64 MiB. Go's regex syntax differs slightly from ripgrep.
func (s *Store) Find(query string) ([]Match, error) {
	re, err := regexp.Compile("(?i)" + query)
	if err != nil {
		return nil, err
	}
	entries, err := s.List()
	if err != nil {
		return nil, err
	}
	matches := make([]Match, 0)
	for _, e := range entries {
		m := Match{Name: e.Name}
		err = s.withReader(e.Name, func(r io.Reader) error {
			scanner := bufio.NewScanner(r)
			scanner.Buffer(make([]byte, 64<<10), maxLineSize+1)
			for scanner.Scan() {
				if re.Match(scanner.Bytes()) {
					m.Lines = append(m.Lines, strings.TrimSpace(scanner.Text()))
				}
			}
			return scanner.Err()
		})
		if err != nil {
			return nil, err
		}
		if len(m.Lines) > 0 {
			matches = append(matches, m)
		}
	}
	return matches, nil
}
