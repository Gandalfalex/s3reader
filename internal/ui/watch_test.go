package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"s3reader/internal/config"
	"s3reader/internal/store"
)

func TestWatchRender(t *testing.T) {
	m := newModel(&config.Settings{Profiles: []config.Profile{{Name: "p"}}}, "")
	_, cancel := context.WithCancel(context.Background())
	t0 := time.Now().Add(-time.Minute)
	m.watch = &watch{bucket: "b", prefix: "in/", every: 10 * time.Second, target: 1000, cancel: cancel, started: t0}
	m.Stack.Push(scrWatch)

	sample := func(at time.Time, n int64) watchMsg {
		return watchMsg{scanned: n, sample: &watchSample{at: at, count: n, size: n * 1024,
			newest: store.Entry{Key: "in/last", Modified: at}},
			uploads: store.Uploads{Count: 3, Stale: 1, Sized: 3, Bytes: 5 << 20,
				Oldest: store.OpenUpload{Key: "in/big", Initiated: t0.Add(-2 * time.Hour)}}}
	}
	m.onWatch(sample(t0, 100))
	m.onWatch(watchMsg{scanned: 50}) // next scan in progress
	if !m.watch.scanning {
		t.Fatal("progress message should mark the watch as scanning")
	}
	m.onWatch(sample(t0.Add(10*time.Second), 300))

	out := ansi.Strip(m.renderWatch())
	for _, want := range []string{"+200 since start", "20.0 obj/s", "30.0%", "ETA", "Stale (>1h)", "in/big"} {
		if !strings.Contains(out, want) {
			t.Errorf("watch page lacks %q:\n%s", want, out)
		}
	}
	if m.watch.scanning {
		t.Error("finished scan should clear scanning")
	}

	m.Stack.Pop()
	if s := m.watchStatus(); !strings.Contains(s, "300 (+200)") {
		t.Errorf("status = %q", s)
	}
	if !m.stopWatch() || m.watch != nil {
		t.Error("stopWatch should clear the watch")
	}
}

func TestSparkline(t *testing.T) {
	s := []watchSample{{count: 0}, {count: 10}, {count: 10}, {count: 15}}
	if got := sparkline(s); got != "█▁▄" {
		t.Errorf("sparkline = %q", got)
	}
}
