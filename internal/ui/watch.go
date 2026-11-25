package ui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	tk "github.com/Gandalfalex/tuikit"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"s3reader/internal/store"
)

const (
	defaultWatchEvery = 10 * time.Second
	minWatchEvery     = 2 * time.Second
	watchHistory      = 40 // completed scans kept for rates and the sparkline
)

// watch re-scans a prefix on an interval while another program writes to
// it. Like a download it runs outside Ops, so browsing does not stop it;
// :watch off, :cancel or quitting does.
type watch struct {
	bucket, prefix string
	every          time.Duration
	target         int64 // expected object count, 0 when unknown
	cancel         context.CancelFunc
	started        time.Time

	scanning bool
	scanned  int64 // objects seen so far in the running scan
	base     *watchSample
	samples  []watchSample // most recent last
	uploads  store.Uploads
	err      error // last scan error, nil after a good scan
}

type watchSample struct {
	at     time.Time // scan finished
	took   time.Duration
	count  int64
	size   int64
	newest store.Entry
}

type watchMsg struct {
	scanned int64        // progress of the running scan
	sample  *watchSample // set when a scan finished
	uploads store.Uploads
	err     error
	ch      <-chan tea.Msg
}

// cmdWatch handles ":watch [dir] [interval] [target]" and ":watch off".
func (m *model) cmdWatch(arg string) tea.Cmd {
	f := strings.Fields(arg)
	if len(f) == 1 && (f[0] == "off" || f[0] == "stop") {
		if !m.stopWatch() {
			m.SetStatus("no watch running", false)
		}
		return nil
	}
	if len(f) == 0 && m.watch != nil {
		m.showWatch()
		return nil
	}
	if !m.needBucket() {
		return nil
	}
	prefix, every, target := m.prefix, defaultWatchEvery, int64(0)
	for _, a := range f {
		if d, err := time.ParseDuration(a); err == nil {
			every = max(d, minWatchEvery)
		} else if n, err := strconv.ParseInt(a, 10, 64); err == nil && n > 0 {
			target = n
		} else {
			prefix = m.resolveDir(a)
		}
	}
	m.stopWatch()
	ctx, cancel := context.WithCancel(context.Background())
	w := &watch{bucket: m.bucket, prefix: prefix, every: every, target: target, cancel: cancel, started: time.Now(), scanning: true}
	m.watch = w
	slog.Info("watch start", "bucket", w.bucket, "prefix", prefix, "every", every, "target", target)
	ch := make(chan tea.Msg, 2)
	go runWatch(ctx, m.bclient, w.bucket, prefix, every, ch)
	m.showWatch()
	return waitFor(ch)
}

func (m *model) stopWatch() bool {
	if m.watch == nil {
		return false
	}
	m.watch.cancel()
	slog.Info("watch stop", "bucket", m.watch.bucket, "prefix", m.watch.prefix)
	m.watch = nil
	if m.Stack.Top() == scrWatch {
		m.Stack.Pop()
	}
	m.SetStatus("watch stopped", false)
	return true
}

func (m *model) showWatch() {
	if m.Stack.Top() == scrWatch {
		m.refreshViewport()
		return
	}
	m.openPage(scrWatch, false)
}

// runWatch scans, reports, sleeps, repeats until ctx is cancelled.
func runWatch(ctx context.Context, c *s3.Client, bucket, prefix string, every time.Duration, ch chan tea.Msg) {
	defer close(ch)
	uploadsOff := false // endpoint does not support listing them
	for {
		start := time.Now()
		st := store.NewStats(prefix)
		last := time.Time{}
		err := store.Walk(ctx, c, bucket, prefix, func(objs []types.Object) error {
			st.Add(objs)
			if time.Since(last) > 250*time.Millisecond {
				last = time.Now()
				select {
				case ch <- watchMsg{scanned: st.Count, ch: ch}:
				default: // UI busy; skip this update
				}
			}
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		msg := watchMsg{scanned: st.Count, err: err, ch: ch}
		if err == nil {
			msg.sample = &watchSample{at: time.Now(), took: time.Since(start), count: st.Count, size: st.Size, newest: st.Newest}
			if !uploadsOff {
				msg.uploads = store.ListUploads(ctx, c, bucket, prefix)
				uploadsOff = unsupported(msg.uploads.Err)
			} else {
				msg.uploads.Err = errUploadsUnsupported
			}
		}
		select {
		case ch <- msg:
		case <-ctx.Done():
			return
		}
		select {
		case <-time.After(every):
		case <-ctx.Done():
			return
		}
	}
}

var errUploadsUnsupported = errors.New("not supported by this endpoint")

func unsupported(err error) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "NotImplemented", "XNotImplemented":
			return true
		}
	}
	return false
}

func (m *model) onWatch(msg watchMsg) tea.Cmd {
	w := m.watch
	if w == nil {
		return nil // stopped; the goroutine has been cancelled
	}
	w.scanned = msg.scanned
	if s := msg.sample; s != nil {
		w.scanning, w.err, w.uploads = false, nil, msg.uploads
		if w.base == nil {
			w.base = s
		}
		w.samples = append(w.samples, *s)
		if len(w.samples) > watchHistory {
			w.samples = w.samples[len(w.samples)-watchHistory:]
		}
	} else if msg.err != nil {
		w.scanning, w.err = false, msg.err
		m.fail("watch s3://"+w.bucket+"/"+w.prefix, msg.err)
	} else {
		w.scanning = true
	}
	if m.Stack.Top() == scrWatch {
		m.refreshViewport()
	}
	return waitFor(msg.ch)
}

// rate is objects and bytes per second between two samples.
func rate(a, b watchSample) (obj, bytes float64) {
	dt := b.at.Sub(a.at).Seconds()
	if dt <= 0 {
		return 0, 0
	}
	return float64(b.count-a.count) / dt, float64(b.size-a.size) / dt
}

func (m *model) renderWatch() string {
	w := m.watch
	if w == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n" + sSection.Render(" WATCH") + "\n")
	b.WriteString(row("Scope", "s3://"+w.bucket+"/"+w.prefix))
	b.WriteString(row("Every", w.every.String()+sMuted.Render("  running "+time.Since(w.started).Round(time.Second).String()+" · :watch off stops")))
	state := sOK.Render("idle")
	if w.scanning {
		state = sWarn.Render("scanning… " + tk.FmtInt(w.scanned) + " objects")
	}
	if n := len(w.samples); n > 0 {
		s := w.samples[n-1]
		state += sMuted.Render(fmt.Sprintf("  last scan %s, took %s", s.at.Local().Format(time.TimeOnly), s.took.Round(time.Millisecond)))
	}
	b.WriteString(row("Status", state))
	if w.err != nil {
		b.WriteString(row("Last error", sErr.Render(shortErr(w.err))))
	}
	if len(w.samples) == 0 {
		return b.String()
	}

	cur := w.samples[len(w.samples)-1]
	b.WriteString("\n" + sSection.Render(" OBJECTS") + "\n")
	b.WriteString(row("Objects", tk.FmtInt(cur.count)+sMuted.Render("  +"+tk.FmtInt(cur.count-w.base.count)+" since start")))
	b.WriteString(row("Total size", tk.FmtSize(cur.size)+sMuted.Render("  +"+tk.FmtSize(max(0, cur.size-w.base.size))+" since start")))
	if len(w.samples) > 1 {
		o, by := rate(w.samples[len(w.samples)-2], cur)
		ao, aby := rate(*w.base, cur)
		b.WriteString(row("Rate now", fmt.Sprintf("%.1f obj/s  %s/s", o, tk.FmtSize(int64(by)))))
		b.WriteString(row("Rate avg", fmt.Sprintf("%.1f obj/s  %s/s", ao, tk.FmtSize(int64(aby)))))
		b.WriteString(row("History", sparkline(w.samples)+sMuted.Render("  new objects per scan")))
		if w.target > 0 {
			b.WriteString(row("Progress", progress(cur.count, w.target, o)))
		}
	} else if w.target > 0 {
		b.WriteString(row("Progress", progress(cur.count, w.target, 0)))
	}
	if cur.newest.Key != "" {
		idle := time.Since(cur.newest.Modified)
		s := idle.Round(time.Second).String() + " ago"
		if idle > max(3*w.every, time.Minute) && cur.count > w.base.count {
			s = sWarn.Render(s + " (stalled?)")
		}
		b.WriteString(row("Last write", s+"  "+sMuted.Render(cur.newest.Key)))
	}

	b.WriteString("\n" + sSection.Render(" OPEN MULTIPART UPLOADS") + "\n")
	u := w.uploads
	switch {
	case u.Err != nil:
		b.WriteString(row("Status", sMuted.Render(shortErr(u.Err))))
	case u.Count == 0:
		b.WriteString(row("Open", sOK.Render("none")))
	default:
		b.WriteString(row("Open", tk.FmtInt(int64(u.Count))+sMuted.Render("  in flight or abandoned")))
		stale := tk.FmtInt(int64(u.Stale))
		if u.Stale > 0 {
			stale = sWarn.Render(stale)
		}
		b.WriteString(row(fmt.Sprintf("Stale (>%dh)", int(store.StaleUpload.Hours())), stale+sMuted.Render("  likely abandoned, still billed")))
		parts := tk.FmtSize(u.Bytes)
		if u.Sized < u.Count {
			parts += sMuted.Render(fmt.Sprintf("  (oldest %d of %d sized)", u.Sized, u.Count))
		}
		b.WriteString(row("Bytes in parts", parts))
		b.WriteString(row("Oldest", fmtTime(u.Oldest.Initiated)+sMuted.Render("  ("+ago(u.Oldest.Initiated)+")  "+u.Oldest.Key)))
	}
	return b.String()
}

func progress(n, target int64, objPerSec float64) string {
	pct := min(100, float64(n)/float64(target)*100)
	barW := 20
	filled := int(pct / 100 * float64(barW))
	s := sOK.Render(strings.Repeat("█", filled)) + sMuted.Render(strings.Repeat("░", barW-filled))
	s += fmt.Sprintf(" %5.1f%%  %s / %s", pct, tk.FmtInt(n), tk.FmtInt(target))
	if left := target - n; left > 0 && objPerSec > 0 {
		s += sMuted.Render("  ETA " + shortDur(time.Duration(float64(left)/objPerSec*float64(time.Second))))
	}
	return s
}

// sparkline draws new objects per scan, scaled to the busiest scan.
func sparkline(s []watchSample) string {
	const bars = "▁▂▃▄▅▆▇█"
	if len(s) < 2 {
		return ""
	}
	d := make([]int64, len(s)-1)
	var top int64
	for i := 1; i < len(s); i++ {
		d[i-1] = max(0, s[i].count-s[i-1].count)
		top = max(top, d[i-1])
	}
	var b strings.Builder
	for _, v := range d {
		i := 0
		if top > 0 {
			i = int(v * 7 / top)
		}
		b.WriteString(string([]rune(bars)[i]))
	}
	return b.String()
}

func shortDur(d time.Duration) string {
	switch {
	case d >= time.Hour:
		return strings.TrimSuffix(d.Round(time.Minute).String(), "0s")
	case d >= time.Minute:
		return d.Round(time.Second).String()
	}
	return d.Round(100 * time.Millisecond).String()
}

// watchStatus is the status bar note while a watch runs, "" when idle.
func (m *model) watchStatus() string {
	w := m.watch
	if w == nil || m.Stack.Top() == scrWatch {
		return ""
	}
	if len(w.samples) == 0 {
		return "◉ watch scanning"
	}
	cur := w.samples[len(w.samples)-1]
	return "◉ watch " + tk.FmtInt(cur.count) + " (+" + tk.FmtInt(cur.count-w.base.count) + ")"
}
