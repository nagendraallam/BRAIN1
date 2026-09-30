package brain1

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestStore(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"Auth module\nsecond line", "other note"} {
		if _, err := s.Add(content, "notes"); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := s.List()
	if err != nil || len(entries) != 2 {
		t.Fatalf("%v %v", entries, err)
	}
	if entries[1].Name != "notes_2.md.zst" {
		t.Fatal(entries)
	}
	got, err := s.Read("notes")
	if err != nil || got != "Auth module\nsecond line" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := s.Read("not"); err == nil {
		t.Fatal("ambiguous prefix accepted")
	}
	matches, err := s.Find("AUTH|second")
	if err != nil || len(matches) != 1 || len(matches[0].Lines) != 2 {
		t.Fatalf("%v %v", matches, err)
	}
	if _, err := s.Find("["); err == nil {
		t.Fatal("invalid regex accepted")
	}
	if _, err := s.Read("../notes"); err == nil {
		t.Fatal("traversal accepted")
	}
	if _, err := s.Add("  ", ""); err == nil {
		t.Fatal("empty entry accepted")
	}
	long := strings.Repeat("x", 100000) + " needle"
	if _, err := s.Add(long, "long"); err != nil {
		t.Fatal(err)
	}
	if matches, err := s.Find("needle"); err != nil || len(matches) != 1 {
		t.Fatalf("long line: %v %v", matches, err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, "broken.md.zst"), []byte("not zstd"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Find("missing"); err == nil {
		t.Fatal("corruption silently ignored")
	}
}

func TestConcurrentAdd(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Add("memory", "same"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	entries, err := s.List()
	if err != nil || len(entries) != 12 {
		t.Fatalf("%d %v", len(entries), err)
	}
	for _, e := range entries {
		if got, err := s.Read(e.Name); err != nil || got != "memory" {
			t.Fatalf("%q %v", got, err)
		}
	}
}

func TestDefaultAndSymlink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BRAIN1_DIR", dir)
	s, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	if s.dir != dir {
		t.Fatalf("%s", s.dir)
	}
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(target, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link.md.zst")); err != nil {
		t.Skip(err)
	}
	if _, err := s.Read("link"); err == nil {
		t.Fatal("followed symlink")
	}
}
