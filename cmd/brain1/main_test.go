package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	t.Setenv("BRAIN1_DIR", t.TempDir())
	cases := []struct {
		args        []string
		input, want string
		fail        bool
	}{
		{[]string{"add", "Auth memory", "--name", "meeting"}, "", "meeting.md.zst", false},
		{[]string{"add", "--name=piped"}, "piped memory", "piped.md.zst", false},
		{[]string{"find", "AUTH"}, "", "Auth memory", false},
		{[]string{"read", "meeting"}, "", "Auth memory", false},
		{[]string{"list"}, "", "Stored entries (2)", false},
		{[]string{"find", "absent"}, "", "No matches", false},
		{[]string{"read", "missing"}, "", "", true},
		{[]string{"find", "["}, "", "", true},
		{[]string{"add", "--name"}, "", "", true},
		{[]string{"list", "extra"}, "", "", true},
		{[]string{"add"}, " ", "", true},
		{[]string{"unknown"}, "", "", true},
	}
	for _, c := range cases {
		var out bytes.Buffer
		err := run(c.args, strings.NewReader(c.input), &out)
		if (err != nil) != c.fail || !strings.Contains(out.String(), c.want) {
			t.Errorf("%v: %q %v", c.args, out.String(), err)
		}
	}
}
