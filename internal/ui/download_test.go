package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadDest(t *testing.T) {
	dir := t.TempDir()
	home, _ := os.UserHomeDir()
	cases := map[string]string{
		"":            "file.csv",
		dir:           filepath.Join(dir, "file.csv"),
		"new/":        filepath.Join("new", "file.csv"),
		"renamed.csv": "renamed.csv",
		"~/x.csv":     filepath.Join(home, "x.csv"),
		"~":           filepath.Join(home, "file.csv"),
	}
	for arg, want := range cases {
		if got, err := downloadDest(arg, "a/b/file.csv"); err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", arg, got, err, want)
		}
	}
}
