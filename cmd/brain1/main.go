package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	brain1 "github.com/nagendraallam/BRAIN1"
)

const help = `BRAIN1 — local compressed memory

Usage:
  brain1 add [text] [--name NAME]  Save text (or read stdin)
  brain1 find <query>             Search case-insensitive regular expressions
  brain1 read <name>              Read filename, stem, or unique prefix
  brain1 list                     List memories

Set BRAIN1_DIR to override ~/.config/brain1.
No Python, ripgrep, zstd executable, or server required.`

func run(args []string, in io.Reader, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, help)
		return nil
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(out, help)
		return nil
	}
	command, rest := args[0], args[1:]
	if command != "add" && command != "find" && command != "read" && command != "list" {
		return fmt.Errorf("unknown command %q", command)
	}
	if len(rest) == 1 && (rest[0] == "--help" || rest[0] == "-h") {
		fmt.Fprintln(out, help)
		return nil
	}
	s, err := brain1.Open("")
	if err != nil {
		return err
	}
	switch command {
	case "add":
		name := ""
		var words []string
		for i := 0; i < len(rest); i++ {
			a := rest[i]
			if a == "--" {
				words = append(words, rest[i+1:]...)
				break
			}
			if a == "--name" || a == "-n" {
				if i+1 >= len(rest) {
					return fmt.Errorf("%s requires a name", a)
				}
				i++
				name = rest[i]
			} else if strings.HasPrefix(a, "--name=") {
				name = strings.TrimPrefix(a, "--name=")
			} else if strings.HasPrefix(a, "-") {
				return fmt.Errorf("unknown option %q; use -- before literal text", a)
			} else {
				words = append(words, a)
			}
		}
		var content io.Reader = in
		if len(words) > 0 {
			content = strings.NewReader(strings.Join(words, " "))
		}
		entry, err := s.AddReader(content, name)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Saved  →  %s  (%d bytes compressed)\n", entry.Name, entry.Size)
	case "find":
		if len(rest) != 1 {
			return fmt.Errorf("usage: brain1 find <query>")
		}
		matches, err := s.Find(rest[0])
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			fmt.Fprintf(out, "No matches found for '%s'.\n", rest[0])
			return nil
		}
		fmt.Fprintf(out, "Found '%s' in %d file(s):\n\n", rest[0], len(matches))
		for _, m := range matches {
			fmt.Fprintf(out, "  %s\n", m.Name)
			for _, line := range m.Lines {
				fmt.Fprintf(out, "    > %s\n", line)
			}
		}
	case "read":
		if len(rest) != 1 {
			return fmt.Errorf("usage: brain1 read <name>")
		}
		if _, err := fmt.Fprintf(out, "=== %s ===\n\n", rest[0]); err != nil {
			return err
		}
		if _, err := s.ReadTo(rest[0], out); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}

	case "list":
		if len(rest) != 0 {
			return fmt.Errorf("usage: brain1 list")
		}
		entries, err := s.List()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Fprintln(out, "No entries stored yet. Add some with: brain1 add <text>")
			return nil
		}
		fmt.Fprintf(out, "Stored entries (%d):\n\n", len(entries))
		for _, e := range entries {
			fmt.Fprintf(out, "  %-50s  %d bytes\n", e.Name, e.Size)
		}
	}
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
