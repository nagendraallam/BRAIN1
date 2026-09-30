package brain1

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingToolsAndOptions(t *testing.T) {
	if _, err := OpenWithOptions(t.TempDir(), Options{Compression: "not-a-mode"}); err == nil {
		t.Fatal("bad preset accepted")
	}
	s, err := OpenWithOptions(t.TempDir(), Options{RipgrepPath: filepath.Join(t.TempDir(), "missing"), ZstdPath: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Find("x"); err == nil || !strings.Contains(err.Error(), "BRAIN1_RG") {
		t.Fatalf("%v", err)
	}
	if _, err := s.Add("x", "note"); err == nil {
		t.Fatal("missing compressor accepted")
	}
	files, err := os.ReadDir(s.dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("partial files: %v %v", files, err)
	}
}
func TestPresetsAndNativeRegex(t *testing.T) {
	for _, preset := range []Compression{Fast, Balanced, Dense} {
		t.Run(string(preset), func(t *testing.T) {
			s, err := OpenWithOptions(t.TempDir(), Options{Compression: preset})
			if err != nil {
				t.Fatal(err)
			}
			text := "Unicode café 世界\nAUTH code 123\n-literal\n"
			if _, err := s.Add(text, "test"); err != nil {
				t.Fatal(err)
			}
			if got, err := s.Read("test"); err != nil || got != text {
				t.Fatalf("%q %v", got, err)
			}
			for _, q := range []string{"auth", "-literal", "世界", "[0-9]+"} {
				if m, err := s.Find(q); err != nil || len(m) != 1 || len(m[0].Lines) != 1 {
					t.Fatalf("%s %v %v", q, m, err)
				}
			}
		})
	}
}
func TestCancellationAndEmptyRegexValidation(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Find("["); err == nil {
		t.Fatal("invalid regex accepted on empty store")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.FindContext(ctx, "x"); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}
func TestBinaryJSONAndSymlinkSearch(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("needle \xff\x00 test\n", "binary"); err != nil {
		t.Fatal(err)
	}
	m, err := s.Find("needle")
	if err != nil || len(m) != 1 || m[0].Lines[0] != "needle \xff\x00 test" {
		t.Fatalf("%v %v", m, err)
	}
	other, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e, err := other.Add("secret needle", "outside")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(other.dir, e.Name), filepath.Join(s.dir, "link.md.zst")); err != nil {
		t.Skip(err)
	}
	m, err = s.Find("secret")
	if err != nil || len(m) != 0 {
		t.Fatalf("symlink followed: %v %v", m, err)
	}
}
