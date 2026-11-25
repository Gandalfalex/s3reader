package ui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	tk "github.com/Gandalfalex/tuikit"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"s3reader/internal/store"
)

// download is the transfer in flight. It runs outside Ops so navigating
// does not cancel it; only :cancel or quitting does.
type download struct {
	key, dest   string
	done, total int64
	cancel      context.CancelFunc
	start       time.Time
}

type downloadMsg struct {
	done, total int64
	finished    bool
	err         error
	ch          <-chan tea.Msg
}

// downloadTarget is the object under the cursor or open in the pager.
func (m *model) downloadTarget() (key string, class types.StorageClass, ok bool) {
	switch top := m.Stack.Top(); top {
	case scrObject:
		if m.detail != nil && m.detail.Head != nil {
			return m.detail.Key, m.detail.Head.StorageClass, true
		}
	case scrObjects, scrSearch:
		if e, found := m.list().Current(); found {
			if e.IsDir {
				m.SetStatus("can't download a directory", true)
				return "", "", false
			}
			return e.Key, types.StorageClass(e.Class), true
		}
	}
	m.SetStatus("no object selected", true)
	return "", "", false
}

// downloadDest resolves the :download argument: empty means the current
// directory, a trailing slash or an existing directory gets the object name.
func downloadDest(arg, key string) (string, error) {
	name := path.Base(key)
	if arg == "" {
		arg = "."
	}
	if arg == "~" || strings.HasPrefix(arg, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		arg = filepath.Join(home, arg[1:])
	}
	if strings.HasSuffix(arg, "/") {
		return filepath.Join(arg, name), nil
	}
	if fi, err := os.Stat(arg); err == nil && fi.IsDir() {
		return filepath.Join(arg, name), nil
	}
	return arg, nil
}

func (m *model) cmdDownload(arg string, overwrite bool) tea.Cmd {
	if m.dl != nil {
		m.SetStatus("download of "+path.Base(m.dl.key)+" in progress (:cancel stops it)", true)
		return nil
	}
	if !m.needBucket() {
		return nil
	}
	key, class, ok := m.downloadTarget()
	if !ok {
		return nil
	}
	if class == types.StorageClassGlacier || class == types.StorageClassDeepArchive {
		m.SetStatus(string(class)+" object must be restored before download", true)
		return nil
	}
	dest, err := downloadDest(arg, key)
	if err != nil {
		m.fail("download "+key, err)
		return nil
	}
	if abs, err := filepath.Abs(dest); err == nil {
		dest = abs
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.dl = &download{key: key, dest: dest, total: -1, cancel: cancel, start: time.Now()}
	c, b := m.bclient, m.bucket
	ch := make(chan tea.Msg, 2)
	go func() {
		defer close(ch)
		defer cancel()
		last := time.Time{}
		n, err := store.Download(ctx, c, b, key, dest, overwrite, func(done, total int64) {
			if time.Since(last) < 150*time.Millisecond {
				return
			}
			last = time.Now()
			select {
			case ch <- downloadMsg{done: done, total: total, ch: ch}:
			default: // UI busy; skip this update
			}
		})
		ch <- downloadMsg{done: n, total: n, finished: true, err: err, ch: ch}
	}()
	m.SetStatus("downloading "+path.Base(key)+" → "+dest, false)
	return waitFor(ch)
}

func (m *model) cmdCancel() tea.Cmd {
	if m.dl != nil {
		m.dl.cancel()
		return nil
	}
	if m.stopWatch() {
		return nil
	}
	if m.cancelOp() {
		m.SetStatus("cancelled", false)
		return nil
	}
	m.SetStatus("nothing to cancel", false)
	return nil
}

func (m *model) onDownload(msg downloadMsg) tea.Cmd {
	d := m.dl
	if d == nil {
		return nil
	}
	if !msg.finished {
		d.done, d.total = msg.done, msg.total
		return waitFor(msg.ch)
	}
	m.dl = nil
	switch {
	case errors.Is(msg.err, store.ErrExists):
		m.SetStatus(d.dest+": "+msg.err.Error(), true)
	case store.IsCancel(msg.err):
		m.SetStatus("download cancelled", false)
	case msg.err != nil:
		m.fail("download "+d.key, msg.err)
	default:
		took := time.Since(d.start)
		slog.Info("downloaded", "key", d.key, "dest", d.dest, "bytes", msg.done, "took", took.Round(time.Millisecond))
		m.SetStatus(fmt.Sprintf("saved %s (%s) → %s", path.Base(d.key), tk.FmtSize(msg.done), d.dest), false)
	}
	return nil
}

// downloadStatus is the progress shown in the status bar, "" when idle.
func (m *model) downloadStatus() string {
	d := m.dl
	if d == nil {
		return ""
	}
	s := "↓ " + tk.TruncMid(path.Base(d.key), 24) + " " + tk.FmtSize(d.done)
	if d.total > 0 {
		s += fmt.Sprintf("/%s %d%%", tk.FmtSize(d.total), d.done*100/d.total)
	}
	return s
}
