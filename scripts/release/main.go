// Builds a host-platform bundle containing brain1, rg, zstd, and native libraries.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}
func run(cmd string, args ...string) string {
	c := exec.Command(cmd, args...)
	b, err := c.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("%s: %v\n%s", cmd, err, b))
	}
	return string(b)
}
func copyFile(src, dst string) {
	in, err := os.Open(src)
	check(err)
	defer in.Close()
	st, err := in.Stat()
	check(err)
	check(os.MkdirAll(filepath.Dir(dst), 0755))
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, st.Mode().Perm()|0200)
	check(err)
	_, err = io.Copy(out, in)
	check(err)
	check(out.Close())
}

var bundled = map[string]string{}

func licenses(source, stage string) {
	real, err := filepath.EvalSymlinks(source)
	check(err)
	roots := []string{filepath.Dir(real), filepath.Dir(filepath.Dir(real))}
	for _, root := range roots {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			u := strings.ToUpper(e.Name())
			if !e.IsDir() && (strings.HasPrefix(u, "LICENSE") || strings.HasPrefix(u, "LICENCE") || strings.HasPrefix(u, "COPYING")) {
				copyFile(filepath.Join(root, e.Name()), filepath.Join(stage, "licenses", filepath.Base(root)+"-"+e.Name()))
			}
		}
	}
	if runtime.GOOS == "linux" {
		// Distribution packages include the full native component notices here.
		for _, pkg := range []string{"ripgrep", "zstd", "libzstd1", "libpcre2-8-0", "liblz4-1", "liblzma5", "libgcc-s1"} {
			p := filepath.Join("/usr/share/doc", pkg, "copyright")
			if _, err := os.Stat(p); err == nil {
				copyFile(p, filepath.Join(stage, "licenses", pkg+"-copyright"))
			}
		}
	}

}
func macDeps(original, target, stage string) {
	out := run("otool", "-L", original)
	lines := strings.Split(out, "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		dep := fields[0]
		if strings.HasPrefix(dep, "/usr/lib/") || strings.HasPrefix(dep, "/System/Library/") {
			continue
		}
		real := dep
		if strings.HasPrefix(dep, "@rpath/") {
			real = filepath.Join(filepath.Dir(original), "..", "lib", strings.TrimPrefix(dep, "@rpath/"))
		}
		if strings.HasPrefix(dep, "@loader_path/") {
			real = filepath.Join(filepath.Dir(original), strings.TrimPrefix(dep, "@loader_path/"))
		}
		resolved, err := filepath.EvalSymlinks(real)
		check(err)
		// dylib's first otool entry may be its own install ID.
		originalReal, err := filepath.EvalSymlinks(original)
		check(err)
		if resolved == originalReal {
			continue
		}
		base := filepath.Base(dep)
		dst := filepath.Join(stage, "lib", base)
		if prev, ok := bundled[base]; ok {
			if prev != resolved {
				panic("library basename collision: " + base)
			}
		} else {
			bundled[base] = resolved
			copyFile(resolved, dst)
			licenses(resolved, stage)
			macDeps(resolved, dst, stage)
			run("install_name_tool", "-id", "@loader_path/"+base, dst)
		}
		replacement := "@loader_path/lib/" + base
		if filepath.Dir(target) == filepath.Join(stage, "lib") {
			replacement = "@loader_path/" + base
		}
		run("install_name_tool", "-change", dep, replacement, target)
	}
}
func linuxDeps(original, stage string) {
	cmd := exec.Command("ldd", original)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "not a dynamic") || strings.Contains(string(out), "statically linked") {
			return
		}
		check(fmt.Errorf("ldd: %s", out))
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "=>" {
			continue
		}
		path := fields[2]
		if path == "not" {
			panic("unresolved library: " + line)
		}
		if !filepath.IsAbs(path) {
			continue
		}
		name := filepath.Base(path)
		// The target system supplies its libc, loader and other glibc components.
		if name == "libc.so.6" || name == "libm.so.6" || name == "libpthread.so.0" || name == "libdl.so.2" || name == "librt.so.1" {
			continue
		}
		if _, ok := bundled[name]; ok {
			continue
		}
		bundled[name] = path
		copyFile(path, filepath.Join(stage, "lib", name))
		// Debian/Ubuntu distribution copyright files accompany native dependencies.
		copyright := filepath.Join("/usr/share/doc", strings.Split(name, ".so")[0]+"1", "copyright")
		if _, err := os.Stat(copyright); err == nil {
			copyFile(copyright, filepath.Join(stage, "licenses", name+"-copyright"))
		}
	}
}
func pack(stage, archive string) {
	f, err := os.Create(archive)
	check(err)
	defer f.Close()
	if runtime.GOOS == "windows" {
		w := zip.NewWriter(f)
		check(filepath.Walk(stage, func(p string, i os.FileInfo, e error) error {
			if e != nil {
				return e
			}
			if i.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(filepath.Dir(stage), p)
			if e != nil {
				return e
			}
			dst, e := w.Create(filepath.ToSlash(rel))
			if e != nil {
				return e
			}
			src, e := os.Open(p)
			if e != nil {
				return e
			}
			defer src.Close()
			_, e = io.Copy(dst, src)
			return e
		}))
		check(w.Close())
		return
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	check(filepath.Walk(stage, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		h, e := tar.FileInfoHeader(i, "")
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(filepath.Dir(stage), p)
		if e != nil {
			return e
		}
		h.Name = filepath.ToSlash(rel)
		if e = tw.WriteHeader(h); e != nil {
			return e
		}
		if !i.Mode().IsRegular() {
			return nil
		}
		src, e := os.Open(p)
		if e != nil {
			return e
		}
		defer src.Close()
		_, e = io.Copy(tw, src)
		return e
	}))
	check(tw.Close())
	check(gz.Close())
}
func main() {
	version := flag.String("version", "dev", "release version")
	rgFlag := flag.String("rg", "rg", "path to real ripgrep binary (not a package-manager shim)")
	zstdFlag := flag.String("zstd", "zstd", "path to real zstd binary")
	flag.Parse()
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`).MatchString(*version) {
		panic("invalid version")
	}
	name := "brain1-" + *version + "-" + runtime.GOOS + "-" + runtime.GOARCH
	stage, err := filepath.Abs(filepath.Join("dist", name))
	check(err)
	if _, err := os.Stat(stage); err == nil {
		panic("output already exists: " + stage)
	}
	check(os.MkdirAll(stage, 0755))
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	run("go", "build", "-trimpath", "-ldflags=-X main.version="+*version, "-o", filepath.Join(stage, "brain1"+suffix), "./cmd/brain1")
	versions := []string{"BRAIN1 " + *version, runtime.Version()}
	for _, tool := range []struct{ name, path string }{{"rg", *rgFlag}, {"zstd", *zstdFlag}} {
		source, err := exec.LookPath(tool.path)
		check(err)
		source, err = filepath.EvalSymlinks(source)
		check(err)
		target := filepath.Join(stage, tool.name+suffix)
		copyFile(source, target)
		licenses(source, stage)
		versions = append(versions, run(source, "--version"))
		switch runtime.GOOS {
		case "darwin":
			macDeps(source, target, stage)
		case "linux":
			linuxDeps(source, stage)
		case "windows":
			files, err := filepath.Glob(filepath.Join(filepath.Dir(source), "*.dll"))
			check(err)
			for _, dll := range files {
				copyFile(dll, filepath.Join(stage, filepath.Base(dll)))
			}
		}
	}
	copyFile("README.md", filepath.Join(stage, "README.md"))
	copyFile("LICENSE", filepath.Join(stage, "LICENSE"))
	copyFile("vendor/github.com/klauspost/compress/LICENSE", filepath.Join(stage, "licenses", "klauspost-compress-LICENSE"))
	check(os.WriteFile(filepath.Join(stage, "VERSIONS.txt"), []byte(strings.Join(versions, "\n")), 0644))
	if runtime.GOOS == "darwin" {
		files, err := filepath.Glob(filepath.Join(stage, "lib", "*"))
		check(err)
		for _, p := range files {
			run("codesign", "--force", "--sign", "-", p)
		}
		for _, n := range []string{"rg", "zstd", "brain1"} {
			run("codesign", "--force", "--sign", "-", filepath.Join(stage, n))
		}
	}
	archive := stage + ".tar.gz"
	if runtime.GOOS == "windows" {
		archive = stage + ".zip"
	}
	pack(stage, archive)
	f, err := os.Open(archive)
	check(err)
	h := sha256.New()
	_, err = io.Copy(h, f)
	check(err)
	check(f.Close())
	checksum := hex.EncodeToString(h.Sum(nil)) + "  " + filepath.Base(archive) + "\n"
	check(os.WriteFile(archive+".sha256", []byte(checksum), 0644))
	fmt.Print(checksum)
}
