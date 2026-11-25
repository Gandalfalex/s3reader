package ui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"s3reader/internal/config"
	"s3reader/internal/store"
)

func TestVisualSelection(t *testing.T) {
	m := newModel(&config.Settings{Profiles: []config.Profile{{Name: "aws"}}}, "")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 14})
	m.bucket, m.prefix = "data", "logs/"
	m.Stack.Push(scrObjects)
	m.showEntries([]store.Entry{
		{Key: "logs/a.txt", Name: "a.txt"},
		{Key: "logs/b.txt", Name: "b.txt"},
		{Key: "logs/c.txt", Name: "c.txt"},
		{Key: "logs/d.txt", Name: "d.txt"},
	}, "")
	press := func(ks ...string) {
		for _, k := range ks {
			msg := tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
			if k == "esc" {
				msg = tea.KeyPressMsg{Code: tea.KeyEscape}
			}
			m.Update(msg)
		}
	}

	press("j", "V", "j", "j")
	if !m.visualOn() {
		t.Fatal("V should start visual mode")
	}
	want := []string{"s3://data/logs/b.txt", "s3://data/logs/c.txt", "s3://data/logs/d.txt"}
	if got := m.selectedLines(&m.objects, false); !slices.Equal(got, want) {
		t.Errorf("uris: got %q, want %q", got, want)
	}
	press("k", "k", "k") // past the anchor: selection flips upwards
	if got := m.selectedLines(&m.objects, true); !slices.Equal(got, []string{"logs/a.txt", "logs/b.txt"}) {
		t.Errorf("keys: got %q", got)
	}
	press("y")
	if m.visualOn() {
		t.Error("y should leave visual mode")
	}
	press("v", "esc")
	if m.visualOn() || m.Stack.Top() != scrObjects {
		t.Error("esc should only leave visual mode")
	}
}
