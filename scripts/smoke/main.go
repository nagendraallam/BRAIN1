// Checks a release bundle without native search/compression tools on PATH.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	bundle := flag.String("bundle", "", "extracted release directory")
	flag.Parse()
	if *bundle == "" {
		panic("-bundle required")
	}
	path, err := filepath.Abs(*bundle)
	if err != nil {
		panic(err)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	temp, err := os.MkdirTemp("", "brain1-bundle-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(temp)
	env := []string{}
	for _, e := range os.Environ() {
		k := strings.ToUpper(strings.SplitN(e, "=", 2)[0])
		if k != "PATH" && !strings.HasPrefix(k, "BRAIN1_") {
			env = append(env, e)
		}
	}
	systemPath := "/usr/bin:/bin"
	if runtime.GOOS == "windows" {
		systemPath = filepath.Join(os.Getenv("SystemRoot"), "System32")
	}
	env = append(env, "PATH="+systemPath, "BRAIN1_DIR="+temp)
	for _, args := range [][]string{{"add", "bundled searchable café 世界 memory", "--name", "test"}, {"find", "SEARCHABLE"}, {"read", "test"}, {"list"}} {
		cmd := exec.Command(filepath.Join(path, "brain1"+suffix), args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%v: %v %s", args, err, out))
		}
		if !strings.Contains(string(out), "test") && !strings.Contains(string(out), "searchable") {
			panic("unexpected output: " + string(out))
		}
	}
	fmt.Println("PASS: bundled add/find/read/list with system-only PATH")
}
