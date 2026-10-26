package query

import (
	"testing"
	"time"

	"s3reader/internal/store"
)

func TestDateFilter(t *testing.T) {
	d := func(s string) time.Time {
		tm, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	now := time.Now()
	files := map[string]store.Entry{
		"may":      {Name: "a.csv", Key: "a.csv", Modified: d("2024-05-17 10:00")},
		"june":     {Name: "b.csv", Key: "b.csv", Modified: d("2024-06-01 00:00")},
		"2023":     {Name: "c.csv", Key: "c.csv", Modified: d("2023-12-31 23:59")},
		"named":    {Name: "report-2024-05.csv", Key: "report-2024-05.csv", Modified: d("2025-01-01 00:00")},
		"fresh":    {Name: "d.csv", Key: "d.csv", Modified: now},
		"old-week": {Name: "e.csv", Key: "e.csv", Modified: now.Add(-10 * 24 * time.Hour)},
	}
	dir := store.Entry{Name: "2024-05/", Key: "2024-05/", IsDir: true}

	cases := []struct {
		q    string
		want []string
	}{
		{"2024-05", []string{"may", "named"}}, // name OR date
		{"on:2024-05", []string{"may"}},
		{"date:17.05.2024", []string{"may"}},
		{"05.2024", []string{"may"}},
		{"2024-05-17", []string{"may"}},
		{"2024-05-17T10:00", []string{"may"}},
		{">2024-05", []string{"june", "named", "fresh", "old-week"}},
		{">=2024-05 <2024-06-02", []string{"may", "june"}},
		{"<2024", []string{"2023"}},
		{"<=2024-05", []string{"may", "2023"}},
		{"2024-01..2024-05", []string{"may"}},
		{"2024-05..", []string{"may", "june", "named", "fresh", "old-week"}},
		{"..2023", []string{"2023"}},
		{"<7d", []string{"fresh"}},
		{">1w <2w", []string{"old-week"}},
		{"after:7d", []string{"fresh"}},
		{"before:7d after:2024-12", []string{"named", "old-week"}},
		{"after:2024-05-31 before:2024-06-02", []string{"june"}},
		{"today", []string{"fresh"}},
		{"2023", []string{"2023"}},
		{">2kb csv", []string{"none"}}, // size filter still works
	}
	for _, c := range cases {
		q, err := Parse(c.q)
		if err != nil {
			t.Fatalf("%q: %v", c.q, err)
		}
		want := map[string]bool{}
		for _, w := range c.want {
			want[w] = true
		}
		for id, e := range files {
			if got := q.Match(e); got != want[id] {
				t.Errorf("%q on %s (%s): got %v want %v", c.q, id, e.Modified.Format(time.DateTime), got, want[id])
			}
		}
	}

	// dirs: plain date term matches by name, explicit date range hides them
	if q, _ := Parse("2024-05"); !q.Match(dir) {
		t.Error("dir should match by name")
	}
	if q, _ := Parse("on:2024-05"); q.Match(dir) {
		t.Error("dir should not match date range")
	}
	if _, err := Parse("on:banana"); err == nil {
		t.Error("expected error for bad date")
	}
}

func TestContainsFold(t *testing.T) {
	cases := []struct {
		s, sub string
		want   bool
	}{
		{"Daily-Export.CSV", "export", true},
		{"Daily-Export.CSV", "csv", true},
		{"Daily-Export.CSV", "exports", false},
		{"abc", "", true},
		{"ab", "abc", false},
		{"ÄRGER-Datei.txt", "ärger", true}, // non-ASCII falls back to ToLower
		{"ÄRGER-Datei.txt", "datei", true},
		{"aaab", "aab", true},
	}
	for _, c := range cases {
		if got := containsFold(c.s, c.sub); got != c.want {
			t.Errorf("containsFold(%q, %q) = %v, want %v", c.s, c.sub, got, c.want)
		}
	}
}
