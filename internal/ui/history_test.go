package ui

import (
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3reader/internal/store"
)

func cachedModel(dirs map[string][]string) *model {
	m := &model{bclient: &s3.Client{}, bucket: "b", h: 20, w: 80}
	m.resetHistory()
	m.stack = []screen{scrProfiles, scrObjects}
	for prefix, names := range dirs {
		var es []store.Entry
		for _, n := range names {
			es = append(es, store.Entry{Key: prefix + n, Name: n, IsDir: n[len(n)-1] == '/'})
		}
		m.dirs.put(dirKey{"b", prefix}, es)
	}
	m.applyDir("", m.dirs[dirKey{"b", ""}].entries, "")
	return m
}

func TestJumpBackForward(t *testing.T) {
	m := cachedModel(map[string][]string{
		"":     {"a/", "z"},
		"a/":   {"b/", "x"},
		"a/b/": {"f"},
	})
	if cmd := m.loadDir("a/", ""); cmd != nil {
		t.Fatal("cached dir should load synchronously")
	}
	m.loadDir("a/b/", "")
	if m.prefix != "a/b/" {
		t.Fatalf("prefix = %q", m.prefix)
	}
	m.jump(-1)
	if m.prefix != "a/" {
		t.Fatalf("after ctrl+o prefix = %q, want a/", m.prefix)
	}
	m.jump(-1)
	if m.prefix != "" {
		t.Fatalf("after 2x ctrl+o prefix = %q, want root", m.prefix)
	}
	m.jump(-1) // at oldest: stays
	if m.prefix != "" {
		t.Fatalf("prefix moved past oldest jump: %q", m.prefix)
	}
	m.jump(+1)
	m.jump(+1)
	if m.prefix != "a/b/" {
		t.Fatalf("after 2x ctrl+i prefix = %q, want a/b/", m.prefix)
	}
	// a new jump drops the forward history
	m.jump(-1)
	m.loadDir("", "")
	m.jump(+1)
	if m.prefix != "" {
		t.Fatalf("forward history survived a new jump: %q", m.prefix)
	}
}

func TestDirRemembersCursor(t *testing.T) {
	m := cachedModel(map[string][]string{
		"":   {"a/", "b/"},
		"a/": {"1", "2", "3"},
	})
	m.loadDir("a/", "")
	m.objects.Cursor = 2
	m.loadDir("", "a/")
	m.loadDir("a/", "")
	if e, _ := m.objects.current(); e.Key != "a/3" {
		t.Fatalf("cursor on %q, want a/3", e.Key)
	}
}

func TestJumpBackTwiceBeforeLoad(t *testing.T) {
	m := cachedModel(map[string][]string{"": {"a/"}, "a/": {"b/"}, "a/b/": {"f"}})
	m.loadDir("a/", "")
	m.loadDir("a/b/", "")
	delete(m.dirs, dirKey{"b", "a/"}) // force a network load that has not arrived
	m.jump(-1)
	m.jump(-1)
	want := []dirKey{{"b", ""}, {"b", "a/"}, {"b", "a/b/"}}
	if !slices.Equal(m.jumps, want) {
		t.Fatalf("jumps = %v, want %v", m.jumps, want)
	}
}

func TestTrunc(t *testing.T) {
	cases := []struct {
		fn   func(string, int) string
		in   string
		w    int
		want string
	}{
		{truncMid, "short", 10, "short"},
		{truncMid, "export-2024-05-17.parquet", 13, "export…arquet"},
		{truncMid, "abcdef", 1, "…"},
		{truncLeft, "s3://bucket/a/b/c/", 10, "…et/a/b/c/"},
		{truncLeft, "short", 10, "short"},
	}
	for _, c := range cases {
		if got := c.fn(c.in, c.w); got != c.want {
			t.Errorf("trunc(%q, %d) = %q, want %q", c.in, c.w, got, c.want)
		}
	}
}
