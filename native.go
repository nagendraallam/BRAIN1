package brain1

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Compression controls the native Zstandard speed/size trade-off.
type Compression string

const (
	Fast     Compression = "fast"     // zstd level 1
	Balanced Compression = "balanced" // zstd level 9
	Dense    Compression = "dense"    // zstd level 19; much slower, often smaller
)

// Options configure native tools. Empty paths resolve a bundled sibling binary,
// then PATH. Explicit options take priority over BRAIN1_RG / BRAIN1_ZSTD.
type Options struct {
	Compression Compression
	RipgrepPath string
	ZstdPath    string
}

func (o *Options) normalize() error {
	if o.Compression == "" {
		o.Compression = Compression(os.Getenv("BRAIN1_COMPRESSION"))
	}
	if o.Compression == "" {
		o.Compression = Balanced
	}
	switch o.Compression {
	case Fast, Balanced, Dense:
	default:
		return fmt.Errorf("invalid compression %q (use fast, balanced, dense)", o.Compression)
	}
	return nil
}
func tool(name, explicit, envkey string) (string, error) {
	if explicit == "" {
		explicit = os.Getenv(envkey)
	}
	if explicit != "" {
		p, err := exec.LookPath(explicit)
		if err != nil {
			return "", fmt.Errorf("%s: %w", envkey, err)
		}
		return filepath.Abs(p)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	if self, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(self), name+suffix)
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() {
			return p, nil
		}
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s is required: use a bundled BRAIN1 release, install %s, or set %s", name, name, envkey)
	}
	return filepath.Abs(p)
}

// capBuffer retains only the first 64 KiB of native diagnostics.
type capBuffer struct{ bytes.Buffer }

func (b *capBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (64 << 10) - b.Len()
	if remaining > 0 {
		if remaining > n {
			remaining = n
		}
		_, _ = b.Buffer.Write(p[:remaining])
	}
	return n, nil
}
func (s *Store) compress(input io.Reader, out io.Writer) error {
	binary, err := tool("zstd", s.options.ZstdPath, "BRAIN1_ZSTD")
	if err != nil {
		return err
	}
	level := "-9"
	switch s.options.Compression {
	case Fast:
		level = "-1"
	case Dense:
		level = "-19"
	}
	checked := &contentCheck{dst: io.Discard}
	cmd := exec.Command(binary, "--compress", "--stdout", "--quiet", "--check", "-T2", level)
	cmd.Env = nativeEnv(binary)
	cmd.Stdin = io.TeeReader(input, checked)
	cmd.Stdout = out
	var stderr capBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("zstd compression failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if !checked.nonBlank && len(checked.pending) == 0 {
		return errors.New("no content provided")
	}
	return nil
}

// Find uses real ripgrep, never Go's regexp engine. Native executables are
// invoked directly with argument arrays: no shell, interpolation or scripts.
func (s *Store) Find(query string) ([]Match, error) {
	return s.FindContext(context.Background(), query)
}

// FindContext supports cancellation. Search streams ripgrep's JSON output;
// returned matches occupy memory. There is no limit on total corpus size.
func (s *Store) FindContext(ctx context.Context, query string) ([]Match, error) {
	rg, err := tool("rg", s.options.RipgrepPath, "BRAIN1_RG")
	if err != nil {
		return nil, err
	}
	zstd, err := tool("zstd", s.options.ZstdPath, "BRAIN1_ZSTD")
	if err != nil {
		return nil, err
	}
	// ripgrep discovers its zstd decompressor by the exact executable name on PATH.
	// Require the conventional basename so PATH resolves the supplied tool.
	expected := "zstd"
	if runtime.GOOS == "windows" {
		expected += ".exe"
	}
	if filepath.Base(zstd) != expected {
		return nil, fmt.Errorf("ZstdPath must point to an executable named %s", expected)
	}
	cmd := exec.CommandContext(ctx, rg, "--no-config", "--json", "--text", "--ignore-case", "--search-zip", "--hidden", "--no-ignore", "--maxdepth", "1", "--glob", "*.md.zst", "--color=never", "--", query, ".")
	cmd.Dir = s.dir
	env := nativeEnv(rg)
	if runtime.GOOS == "linux" {
		for i, e := range env {
			if strings.HasPrefix(e, "LD_LIBRARY_PATH=") {
				env[i] = "LD_LIBRARY_PATH=" + filepath.Join(filepath.Dir(zstd), "lib") + ":" + strings.TrimPrefix(e, "LD_LIBRARY_PATH=")
			}
		}
	}
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.ToUpper(env[i]), "PATH=") {
			env = append(env[:i], env[i+1:]...)
		}
	}
	cmd.Env = append(env, "PATH="+filepath.Dir(zstd)+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stderr capBuffer
	cmd.Stderr = &stderr
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	byName := map[string][]string{}
	decoder := json.NewDecoder(pipe)
	for {
		var event struct {
			Type string `json:"type"`
			Data struct {
				Path  jsonText `json:"path"`
				Lines jsonText `json:"lines"`
			} `json:"data"`
		}
		err = decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("ripgrep JSON: %w", err)
		}
		if event.Type != "match" {
			continue
		}
		path, e := event.Data.Path.decode()
		if e != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, e
		}
		lines, e := event.Data.Lines.decode()
		if e != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, e
		}
		name := filepath.Base(path)
		byName[name] = append(byName[name], strings.TrimSpace(lines))
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return nil, fmt.Errorf("ripgrep failed: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	matches := make([]Match, 0, len(names))
	for _, name := range names {
		matches = append(matches, Match{Name: name, Lines: byName[name]})
	}
	return matches, nil
}

type jsonText struct {
	Text  *string `json:"text"`
	Bytes string  `json:"bytes"`
}

func (t jsonText) decode() (string, error) {
	if t.Text != nil {
		return *t.Text, nil
	}
	b, err := base64.StdEncoding.DecodeString(t.Bytes)
	return string(b), err
}

// Release bundles keep non-system Linux shared libraries beside the binaries.
func nativeEnv(binary string) []string {
	env := os.Environ()
	if runtime.GOOS == "linux" {
		for i := len(env) - 1; i >= 0; i-- {
			if strings.HasPrefix(env[i], "LD_LIBRARY_PATH=") {
				env = append(env[:i], env[i+1:]...)
			}
		}
		env = append(env, "LD_LIBRARY_PATH="+filepath.Join(filepath.Dir(binary), "lib")+":"+os.Getenv("LD_LIBRARY_PATH"))
	}
	return env
}
