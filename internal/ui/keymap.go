package ui

import (
	tk "github.com/Gandalfalex/tuikit"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
)

// keyCtx is what a key action needs besides the model: the key event with
// its count and, for list screens, the active list.
type keyCtx struct {
	keys.Event
	l *listState
}

type keyFn func(m *model, c keyCtx) (tea.Model, tea.Cmd)

// do adapts an action that never returns a command.
func do(fn func(m *model, c keyCtx)) keyFn {
	return func(m *model, c keyCtx) (tea.Model, tea.Cmd) {
		fn(m, c)
		return m, nil
	}
}

// nav adapts navigation methods like (*model).open.
func nav(fn func(*model) (tea.Model, tea.Cmd)) keyFn {
	return func(m *model, _ keyCtx) (tea.Model, tea.Cmd) { return fn(m) }
}

// cmd adapts methods that only produce a command, like (*model).cmdYank.
func cmd(fn func(*model) tea.Cmd) keyFn {
	return func(m *model, _ keyCtx) (tea.Model, tea.Cmd) { return m, fn(m) }
}

// App actions; motions come from vimkit/motion.
const (
	actQuit        keys.Action = "quit"
	actErrors      keys.Action = "errors"
	actSearch      keys.Action = "search"
	actFilter      keys.Action = "filter"
	actEsc         keys.Action = "clear_or_back"
	actSort        keys.Action = "sort"
	actReverse     keys.Action = "reverse"
	actOpen        keys.Action = "open"
	actBack        keys.Action = "back"
	actInfo        keys.Action = "info"
	actRoot        keys.Action = "bucket_root"
	actReload      keys.Action = "reload"
	actYank        keys.Action = "yank"
	actYankKey     keys.Action = "yank_key"
	actReveal      keys.Action = "reveal"
	actStats       keys.Action = "stats"
	actStatsPrefix keys.Action = "stats_prefix"
	actOlder       keys.Action = "jump_back"
	actNewer       keys.Action = "jump_forward"
	actDownload    keys.Action = "download"
)

var listKeymap = keys.MustKeymap(motion.VerticalBindings(), keys.Bindings{
	actQuit:     {"q"},
	actErrors:   {"E"},
	actSearch:   {"?", "s"},
	actFilter:   {"/"},
	actEsc:      {"esc"},
	actSort:     {"o"},
	actReverse:  {"O"},
	actOpen:     {"enter", "l", "right"},
	actBack:     {"h", "left", "backspace", "-"},
	actInfo:     {"i", "gi"},
	actRoot:     {"gr"},
	actReload:   {"r", "ctrl+r"},
	actYank:     {"y"},
	actYankKey:  {"Y"},
	actReveal:   {"D"},
	actDownload: {"d"},
	actOlder:    {"ctrl+o"},
	actNewer:    {"ctrl+i", "tab"}, // most terminals send tab for ctrl+i
})

var pagerKeymap = keys.MustKeymap(tk.PagerMotions(), keys.Bindings{
	actBack:        {"q", "esc", "h", "left", "backspace", "-"},
	actStats:       {"c"},
	actStatsPrefix: {"C"},
	actYank:        {"y"},
	actReload:      {"r"},
	actErrors:      {"E"},
	actSearch:      {"?"},
	actDownload:    {"d"},
})

// quitList quits at the top level; deeper in a bucket q must be pressed twice
// so a stray q does not lose your place.
func quitList(m *model, _ keyCtx) (tea.Model, tea.Cmd) {
	home := m.prefix == "" || (m.bucket == m.profile.Bucket && m.prefix == m.profile.Prefix)
	top := m.Stack.Top()
	return m, m.Quit(top == scrProfiles || top == scrBuckets || (top == scrObjects && home))
}

var listActions = map[keys.Action]keyFn{
	actQuit:   quitList,
	actErrors: do(func(m *model, _ keyCtx) { m.openErrors() }),
	actSearch: cmd((*model).openSearchInput),
	actFilter: func(m *model, c keyCtx) (tea.Model, tea.Cmd) {
		m.prevFilter = c.l.Filter
		return m, m.prompt.Open(inputFilter, tk.ModeFilter, "/", "filter: text !exclude re:regex ext:csv >1MB 2024-05 >2024-01 <7d class:glacier type:dir", c.l.Filter)
	},
	actEsc: func(m *model, c keyCtx) (tea.Model, tea.Cmd) {
		if c.l.Filter != "" {
			c.l.SetFilter("")
			return m, nil
		}
		return m.back()
	},
	actSort: do(func(m *model, c keyCtx) {
		c.l.order((c.l.sortBy+1)%len(sortNames), c.l.reverse)
		c.l.Resort()
		m.SetStatus("sort: "+sortNames[c.l.sortBy], false)
	}),
	actReverse: do(func(m *model, c keyCtx) {
		c.l.order(c.l.sortBy, !c.l.reverse)
		c.l.Resort()
	}),
	actOpen: nav((*model).open),
	actBack: nav((*model).back),
	actInfo: nav((*model).openInfo),
	actRoot: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) {
		if m.Stack.Top() == scrObjects && m.prefix != "" {
			return m, m.loadDir("", "")
		}
		return m, nil
	},
	actReload:   nav((*model).reload),
	actOlder:    func(m *model, _ keyCtx) (tea.Model, tea.Cmd) { return m, m.jump(-1) },
	actNewer:    func(m *model, _ keyCtx) (tea.Model, tea.Cmd) { return m, m.jump(+1) },
	actYank:     cmd((*model).cmdYank),
	actDownload: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) { return m, m.cmdDownload("", false) },
	actYankKey: func(m *model, c keyCtx) (tea.Model, tea.Cmd) {
		if e, ok := c.l.Current(); ok {
			return m, tk.YankValue(e.Key)
		}
		return m, nil
	},
	actReveal: func(m *model, c keyCtx) (tea.Model, tea.Cmd) {
		// reveal search result in its directory
		if m.Stack.Top() == scrSearch {
			if e, ok := c.l.Current(); ok {
				dir := e.Key[:strings.LastIndex(e.Key, "/")+1]
				m.Stack.Pop()
				return m, m.loadDir(dir, e.Key)
			}
		}
		return m, nil
	},
}

var pagerActions = map[keys.Action]keyFn{
	actBack: do(func(m *model, _ keyCtx) { m.Stack.Pop() }),
	actStats: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) {
		if m.Stack.Top() == scrInfo && m.info != nil {
			return m, m.scanStats("")
		}
		return m, nil
	},
	actStatsPrefix: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) {
		if m.Stack.Top() == scrInfo && m.info != nil && m.info.Name == m.bucket && m.prefix != "" {
			return m, m.scanStats(m.prefix)
		}
		return m, nil
	},
	actYank:     cmd((*model).cmdYank),
	actDownload: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) { return m, m.cmdDownload("", false) },
	actReload: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) {
		if m.Stack.Top() == scrObject && m.detail != nil {
			m.Stack.Pop()
			return m, m.loadDetail(m.detail.Key)
		}
		return m, nil
	},
	actErrors: do(func(m *model, _ keyCtx) {
		if m.Stack.Top() != scrErrors {
			m.openErrors()
		}
	}),
	actSearch: cmd((*model).openSearchInput),
}
