package brain1

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type repeatReader struct {
	block     []byte
	remaining int64
	offset    int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n := 0
	for n < len(p) {
		k := copy(p[n:], r.block[r.offset:])
		n += k
		r.offset = (r.offset + k) % len(r.block)
	}
	r.remaining -= int64(n)
	return n, nil
}

type brokenReader struct{}

func (brokenReader) Read(p []byte) (int, error) {
	return copy(p, "partial"), errors.New("source failed")
}

type brokenWriter struct{}

func (brokenWriter) Write(p []byte) (int, error) { return 0, errors.New("destination failed") }

type tinyReader struct{ io.Reader }

func (r tinyReader) Read(p []byte) (int, error) { return r.Reader.Read(p[:1]) }

func TestStreamingLargeEntry(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const size = int64(65 << 20)
	e, err := s.AddReader(&repeatReader{block: []byte("memory line\n"), remaining: size}, "large")
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.ReadTo(e.Name, io.Discard)
	if err != nil || n != size {
		t.Fatalf("read %d: %v", n, err)
	}
	matches, err := s.Find("not-in-this-file")
	if err != nil || len(matches) != 0 {
		t.Fatalf("find: %v %v", matches, err)
	}
}
func TestStreamErrorsAndWhitespace(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []io.Reader{nil, brokenReader{}, strings.NewReader(""), tinyReader{strings.NewReader(" \t\n\u2003\u00a0")}} {
		if _, err := s.AddReader(r, "bad"); err == nil {
			t.Fatal("invalid input accepted")
		}
		files, err := os.ReadDir(s.dir)
		if err != nil || len(files) != 0 {
			t.Fatalf("failed write left files: %v %v", files, err)
		}
	}
	text := " \u2003 café 世界\n"
	e, err := s.AddReader(tinyReader{strings.NewReader(text)}, "unicode")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Read(e.Name)
	if err != nil || got != text {
		t.Fatalf("%q %v", got, err)
	}
	if _, err = s.ReadTo(e.Name, brokenWriter{}); err == nil {
		t.Fatal("writer failure lost")
	}
	if _, err = s.ReadTo(e.Name, nil); err == nil {
		t.Fatal("nil writer accepted")
	}
	data, err := os.ReadFile(filepath.Join(s.dir, e.Name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, "truncated"+Extension), data[:len(data)-3], 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadTo("truncated", io.Discard); err == nil {
		t.Fatal("truncation accepted")
	}
}
func TestNamesAndPrefix(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e, err := s.Add("hello", "../My notes! 世界")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "My_notes_世界.md.zst" {
		t.Fatal(e)
	}
	if text, err := s.Read("My_"); err != nil || text != "hello" {
		t.Fatalf("%q %v", text, err)
	}
	if _, err := s.Add("x", strings.Repeat("n", 201)); err == nil {
		t.Fatal("long name accepted")
	}
	if e, err := s.Add("x", ""); err != nil || e.Name == Extension {
		t.Fatalf("%v %v", e, err)
	}
}
func BenchmarkReadTo(b *testing.B) {
	s, err := Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	data := bytes.Repeat([]byte("A memory of the deployment and tests.\n"), 30000)
	e, err := s.AddReader(bytes.NewReader(data), "benchmark")
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.ReadTo(e.Name, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkFind(b *testing.B) {
	s, err := Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	data := bytes.Repeat([]byte("A memory of the deployment and tests.\n"), 30000)
	if _, err := s.AddReader(bytes.NewReader(data), "benchmark"); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Find("missing"); err != nil {
			b.Fatal(err)
		}
	}
}
