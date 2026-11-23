package ui

import (
	tk "github.com/Gandalfalex/tuikit"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3reader/internal/config"
	"s3reader/internal/query"
	"s3reader/internal/store"
)

type model struct {
	tk.Shell[screen] // stack, status, error log, async ops, keys, size

	settings *config.Settings

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

	prompt     tk.Prompt[inputMode]
	prevFilter string

	vp     tk.Pager
	info   *store.BucketInfo
	stats  *store.Stats
	detail *store.ObjectDetail

	searchRoot    string
	searchQuery   query.Query
	searchScanned int64
	searchDone    bool

	cmd          tk.CommandLine[*model] // ":" commands, history and completion
	pendingStats *string                // scan this prefix once bucket info arrives

	dirs    dirCache
	cursors map[dirKey]string // key under the cursor when a dir was left
	jumps   []dirKey          // ctrl+o / ctrl+i
	jumpIdx int

	dl *download // transfer in flight, nil when idle

	logPath string
}

func newModel(s *config.Settings, logPath string) *model {
	m := &model{
		Shell:    tk.NewShell(theme, scrProfiles),
		settings: s,
		prompt:   tk.NewPrompt[inputMode](),
		cmd:      tk.CommandLine[*model]{Commands: commands, Goto: gotoLine},
		vp:       tk.NewPager(),
		logPath:  logPath,
	}
	m.resetHistory()
	m.prompt.Input.CharLimit = 512
	m.profiles = unsortedList()
	m.buckets, m.objects, m.search = sortedList(sortName, false), sortedList(sortName, false), sortedList(sortName, false)
	var rows []store.Entry
	for i, p := range s.Profiles {
		rows = append(rows, store.Entry{Key: strconv.Itoa(i), Name: p.Name})
	}
	m.profiles.Set(rows)
	m.Brief = shortErr
	return m
}

func (m *model) Init() tea.Cmd {
	if len(m.settings.Profiles) == 1 {
		return tea.Batch(m.selectProfile(0), m.Ops.Tick())
	}
	return nil
}

func (m *model) list() *listState { return listOf(m, m.Stack.Top()) }

// listOf is the list screen s shows, nil for pages.
func listOf(m *model, s screen) *listState {
	switch s {
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

func (m *model) listHeight() int { return max(1, m.H-4) } // title, column header, status, hints

// cancelOp aborts the operation in flight, if any, and drops its result.
func (m *model) cancelOp() bool { return m.Ops.Cancel() }

// fail records an operation error: status bar, error log (E) and log file.
// User cancellations are not errors.
func (m *model) fail(what string, err error) {
	if store.IsCancel(err) {
		return
	}
	if m.Fail(what, err, "profile", m.profile.Name, "bucket", m.bucket, "prefix", m.prefix) && m.Stack.Top() == scrErrors {
		m.refreshViewport()
	}
}
