package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
	"s3reader/internal/query"
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

func quit(m *model, _ keyCtx) (tea.Model, tea.Cmd) { return m, tea.Quit }

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
)

var listKeymap = keys.MustKeymap(motion.VerticalBindings(), keys.Bindings{
	actQuit:    {"q"},
	actErrors:  {"E"},
	actSearch:  {"?", "s"},
	actFilter:  {"/"},
	actEsc:     {"esc"},
	actSort:    {"o"},
	actReverse: {"O"},
	actOpen:    {"enter", "l", "right"},
	actBack:    {"h", "left", "backspace", "-"},
	actInfo:    {"i", "gi"},
	actRoot:    {"gr"},
	actReload:  {"r", "ctrl+r"},
	actYank:    {"y"},
	actYankKey: {"Y"},
	actReveal:  {"D"},
})

var pagerKeymap = keys.MustKeymap(pagerMotions(), keys.Bindings{
	actBack:        {"q", "esc", "h", "left", "backspace", "-"},
	actStats:       {"c"},
	actStatsPrefix: {"C"},
	actYank:        {"y"},
	actReload:      {"r"},
	actErrors:      {"E"},
	actSearch:      {"?"},
})

// pagerMotions adds less-style keys to the vim defaults.
func pagerMotions() keys.Bindings {
	b := motion.VerticalBindings()
	b[motion.Down] = append(b[motion.Down], "enter")
	b[motion.PageDown] = append(b[motion.PageDown], "space")
	return b
}

var listActions = map[keys.Action]keyFn{
	actQuit:   quit,
	actErrors: do(func(m *model, _ keyCtx) { m.openErrors() }),
	actSearch: cmd((*model).openSearchInput),
	actFilter: func(m *model, c keyCtx) (tea.Model, tea.Cmd) {
		m.mode = inputFilter
		m.prevFilter = c.l.filter.Raw
		m.input.Prompt = "/"
		m.input.Placeholder = "filter: text !exclude re:regex ext:csv >1MB 2024-05 >2024-01 <7d class:glacier type:dir"
		m.input.SetValue(c.l.filter.Raw)
		m.input.CursorEnd()
		return m, m.input.Focus()
	},
	actEsc: func(m *model, c keyCtx) (tea.Model, tea.Cmd) {
		if !c.l.filter.Empty() {
			c.l.setFilter(query.Query{})
			return m, nil
		}
		return m.back()
	},
	actSort: do(func(m *model, c keyCtx) {
		c.l.sortBy = (c.l.sortBy + 1) % len(sortNames)
		m.resort(c.l)
		m.setStatus("sort: "+sortNames[c.l.sortBy], false)
	}),
	actReverse: do(func(m *model, c keyCtx) {
		c.l.reverse = !c.l.reverse
		m.resort(c.l)
	}),
	actOpen: nav((*model).open),
	actBack: nav((*model).back),
	actInfo: nav((*model).openInfo),
	actRoot: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) {
		if m.top() == scrObjects && m.prefix != "" {
			return m, m.loadDir("", "")
		}
		return m, nil
	},
	actReload: nav((*model).reload),
	actYank:   cmd((*model).cmdYank),
	actYankKey: func(m *model, c keyCtx) (tea.Model, tea.Cmd) {
		if e, ok := c.l.current(); ok {
			return m, yank(e.Key)
		}
		return m, nil
	},
	actReveal: func(m *model, c keyCtx) (tea.Model, tea.Cmd) {
		// reveal search result in its directory
		if m.top() == scrSearch {
			if e, ok := c.l.current(); ok {
				dir := e.Key[:strings.LastIndex(e.Key, "/")+1]
				m.pop()
				return m, m.loadDir(dir, e.Key)
			}
		}
		return m, nil
	},
}

var pagerActions = map[keys.Action]keyFn{
	actBack: do(func(m *model, _ keyCtx) { m.pop() }),
	actStats: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) {
		if m.top() == scrInfo && m.info != nil {
			return m, m.scanStats("")
		}
		return m, nil
	},
	actStatsPrefix: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) {
		if m.top() == scrInfo && m.info != nil && m.info.Name == m.bucket && m.prefix != "" {
			return m, m.scanStats(m.prefix)
		}
		return m, nil
	},
	actYank: cmd((*model).cmdYank),
	actReload: func(m *model, _ keyCtx) (tea.Model, tea.Cmd) {
		if m.top() == scrObject && m.detail != nil {
			m.pop()
			return m, m.loadDetail(m.detail.Key)
		}
		return m, nil
	},
	actErrors: do(func(m *model, _ keyCtx) {
		if m.top() != scrErrors {
			m.openErrors()
		}
	}),
	actSearch: cmd((*model).openSearchInput),
}
