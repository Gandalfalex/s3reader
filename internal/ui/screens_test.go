package ui

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"s3reader/internal/config"
	"s3reader/internal/store"
)

var clock = regexp.MustCompile(`\b\d\d:\d\d:\d\d\b`)

var update = flag.Bool("update", false, "rewrite the screen snapshots in testdata")

// TestScreens walks through the app with canned results and compares each
// screen with its snapshot in testdata (go test -update rewrites them).
func TestScreens(t *testing.T) {
	time.Local = time.UTC
	day := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	m := newModel(&config.Settings{Profiles: []config.Profile{
		{Name: "aws", Region: "eu-central-1"},
		{Name: "minio", Endpoint: "http://localhost:9000", Bucket: "data"},
	}}, "/tmp/s3reader.log")
	send := func(msg tea.Msg) { m.Update(msg) }
	press := func(keys ...string) {
		for _, k := range keys {
			msg := tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
			switch k {
			case "enter":
				msg = tea.KeyPressMsg{Code: tea.KeyEnter}
			case "esc":
				msg = tea.KeyPressMsg{Code: tea.KeyEscape}
			case "tab":
				msg = tea.KeyPressMsg{Code: tea.KeyTab}
			}
			m.Update(msg)
		}
	}
	typeText := func(s string) {
		for _, r := range s {
			press(string(r))
		}
	}
	var shots []string
	shot := func(name string) {
		// the error log shows the time of day
		screen := clock.ReplaceAllString(ansi.Strip(m.View().Content), "hh:mm:ss")
		shots = append(shots, "== "+name+"\n"+screen)
	}

	send(tea.WindowSizeMsg{Width: 100, Height: 14})
	shot("profiles")
	send(bucketsMsg{buckets: []store.Entry{
		{Key: "data", Name: "data", Modified: day},
		{Key: "logs", Name: "logs", Modified: day.AddDate(0, 1, 0)},
	}})
	shot("buckets")
	press("j")
	shot("buckets moved")
	send(bucketOpenedMsg{bucket: "logs", prefix: "", entries: []store.Entry{
		{Key: "2024/", Name: "2024/", IsDir: true},
		{Key: "b.csv", Name: "b.csv", Size: 2048, Modified: day, Class: "STANDARD"},
		{Key: "a.json", Name: "a.json", Size: 10, Modified: day.AddDate(0, 0, 3)},
		{Key: "c.csv", Name: "c.csv", Size: 5 << 20, Modified: day.AddDate(0, 0, 1), Class: "GLACIER"},
	}})
	shot("objects")
	press("o")
	shot("sorted by size")
	press("O")
	shot("reversed")
	press("/")
	typeText("csv")
	shot("filtering")
	press("enter")
	shot("filtered")
	press("esc")
	shot("filter cleared")
	press(":")
	typeText("he")
	shot("command line")
	press("esc")
	send(dirMsg{prefix: "2024/", err: errors.New("access denied\nrequest id 123")})
	shot("error")
	press("E")
	shot("error log")
	press("q")
	press("q")
	shot("q armed")

	got := strings.Join(shots, "\n")
	path := filepath.Join("testdata", "screens.txt")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range max(len(gl), len(wl)) {
			var g, w string
			if i < len(gl) {
				g = gl[i]
			}
			if i < len(wl) {
				w = wl[i]
			}
			if g != w {
				t.Errorf("line %d differs:\n got: %q\nwant: %q", i+1, g, w)
			}
		}
	}
}
