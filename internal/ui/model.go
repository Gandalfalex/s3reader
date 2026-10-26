package ui

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Gandalfalex/vim-kit/keys"
	"s3reader/internal/config"
	"s3reader/internal/query"
	"s3reader/internal/store"
)

type model struct {
	settings *config.Settings
	w, h     int

	stack []screen

	profiles listState
	buckets  listState
	objects  listState
	search   listState

	profile       config.Profile
	client        *s3.Client // profile-level client
	bclient       *s3.Client // bucket-region client
	bucket        string
	bucketRegion  string
	bucketCreated time.Time
	prefix        string

	mode       inputMode
	input      textinput.Model
	prevFilter string

	keys keys.Input // count prefix and pending g/z sequence

	loading  bool
	what     string
	spin     spinner.Model
	spinning bool // a spinner tick is in flight
	gen      int
	cancel   context.CancelFunc

	vp     viewport.Model
	info   *store.BucketInfo
	stats  *store.Stats
	detail *store.ObjectDetail

	searchRoot    string
	searchQuery   query.Query
	searchScanned int64
	searchDone    bool

	status    string
	statusErr bool

	cmdHist      []string // ":" history
	histIdx      int
	compl        []complItem // ":" completion popup
	complIdx     int         // selected completion, -1 = none
	pendingStats *string     // scan this prefix once bucket info arrives

	opStart time.Time
	errs    []errEntry // newest last, capped at maxErrs
	logPath string
}

type errEntry struct {
	at   time.Time
	what string
	err  error
}

const maxErrs = 200

func newModel(s *config.Settings, logPath string) *model {
	in := textinput.New()
	in.Prompt = "/"
	in.CharLimit = 512

	m := &model{
		settings: s,
		input:    in,
		spin:     spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(cAccent))),
		vp:       viewport.New(),
		logPath:  logPath,
	}
	m.profiles.noSort = true
	var rows []store.Entry
	for i, p := range s.Profiles {
		rows = append(rows, store.Entry{Key: strconv.Itoa(i), Name: p.Name})
	}
	m.profiles.set(rows)
	m.stack = []screen{scrProfiles}
	return m
}

func (m *model) Init() tea.Cmd {
	if len(m.settings.Profiles) == 1 {
		cmd := m.selectProfile(0)
		m.spinning = true
		return tea.Batch(cmd, m.spin.Tick)
	}
	return nil
}

func (m *model) top() screen { return m.stack[len(m.stack)-1] }

func (m *model) push(s screen) { m.stack = append(m.stack, s) }

func (m *model) pop() {
	if len(m.stack) > 1 {
		m.stack = m.stack[:len(m.stack)-1]
	}
}

func (m *model) list() *listState {
	switch m.top() {
	case scrProfiles:
		return &m.profiles
	case scrBuckets:
		return &m.buckets
	case scrObjects:
		return &m.objects
	case scrSearch:
		return &m.search
	}
	return nil
}

func (m *model) listHeight() int { return max(1, m.h-4) } // title, column header, status, hints

func (m *model) newOp(what string) context.Context {
	if m.cancel != nil {
		m.cancel()
	}
	m.gen++
	m.loading = true
	m.what = what
	m.opStart = time.Now()
	slog.Debug("op start", "op", what, "gen", m.gen)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	return ctx
}

func (m *model) done() {
	if m.loading {
		slog.Debug("op done", "op", m.what, "took", time.Since(m.opStart).Round(time.Millisecond))
	}
	m.loading = false
	m.what = ""
}

func (m *model) setStatus(s string, isErr bool) {
	m.status, m.statusErr = s, isErr
}

// fail records an operation error: status bar, error history (E) and log file.
// User cancellations are not errors.
func (m *model) fail(what string, err error) {
	if err == nil || store.IsCancel(err) {
		return
	}
	slog.Error(what, "err", err, "profile", m.profile.Name, "bucket", m.bucket, "prefix", m.prefix)
	m.errs = append(m.errs, errEntry{at: time.Now(), what: what, err: err})
	if len(m.errs) > maxErrs {
		m.errs = m.errs[len(m.errs)-maxErrs:]
	}
	m.setStatus(what+": "+shortErr(err)+"  (E: error log)", true)
	if m.top() == scrErrors {
		m.refreshViewport()
	}
}
