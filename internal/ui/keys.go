package ui

import (
	tea "charm.land/bubbletea/v2"
	tk "github.com/Gandalfalex/tuikit"

	"github.com/Gandalfalex/vim-kit/keys"
	"s3reader/internal/query"
)

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if m.prompt.Kind == inputCommand {
		return m.handleCommandKey(msg)
	}
	if m.prompt.Kind != inputNone {
		return m.handleInput(msg)
	}
	m.SetStatus("", false) // messages last until the next key

	// esc cancels in-flight work first
	if k == "esc" && m.cancelOp() {
		m.Keys.Reset()
		m.SetStatus("cancelled", false)
		return m, nil
	}

	pager := m.list() == nil
	km := listKeymap
	if pager {
		km = pagerKeymap
	}
	ev, ok := m.Keys.Feed(km, k)
	if !ok {
		return m, nil // count digit or pending g/z
	}
	if ev.Action != actQuit {
		m.Disarm()
	}
	if ev.Seq == ":" {
		m.endVisual()
		return m, m.openCommandLine()
	}
	if pager {
		return m.handlePagerKey(ev)
	}
	return m.handleListKey(ev)
}

func (m *model) handleListKey(ev keys.Event) (tea.Model, tea.Cmd) {
	l := m.list()
	h := m.listHeight()
	defer l.Scroll(h)

	if l.Apply(ev, h) {
		return m, nil
	}
	if m.visualOn() {
		if mm, cmd, ok := m.handleVisualKey(ev, l); ok {
			return mm, cmd
		}
	}
	if fn, ok := listActions[ev.Action]; ok {
		return fn(m, keyCtx{Event: ev, l: l})
	}
	return m, nil
}

func (m *model) handleInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	l := m.list()
	switch msg.String() {
	case "esc":
		if m.prompt.Kind == inputFilter && l != nil {
			q, _ := query.Parse(m.prevFilter)
			l.setQuery(q)
		}
		m.prompt.Close()
		return m, nil
	case "enter":
		mode := m.prompt.Kind
		m.prompt.Close()
		q, err := query.Parse(m.prompt.Input.Value())
		if err != nil {
			m.SetStatus(err.Error(), true)
			return m, nil
		}
		if mode == inputSearch {
			if q.Empty() {
				return m, nil
			}
			return m, m.runSearch(q)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.prompt.Input, cmd = m.prompt.Input.Update(msg)
	if m.prompt.Kind == inputFilter && l != nil {
		q, err := query.Parse(m.prompt.Input.Value())
		if err == nil {
			l.setQuery(q)
			m.SetStatus("", false)
		} else {
			m.SetStatus(err.Error(), true)
		}
	}
	return m, cmd
}

// openSearchInput starts the "?" recursive search prompt for the current dir.
func (m *model) openSearchInput() tea.Cmd {
	m.popToList()
	if m.Stack.Top() != scrObjects && m.Stack.Top() != scrSearch {
		m.SetStatus("open a bucket to search", true)
		return nil
	}
	return m.prompt.Open(inputSearch, tk.ModeSearch, "?", "search s3://"+m.bucket+"/"+m.prefix+" recursively (same syntax as /)", "")
}

// runSearch opens (or replaces) the search results screen.
func (m *model) runSearch(q query.Query) tea.Cmd {
	if m.Stack.Top() == scrSearch {
		m.Stack.Pop()
		m.prefix = m.searchRoot
	}
	m.Stack.Push(scrSearch)
	return m.startSearch(q)
}

func (m *model) handlePagerKey(ev keys.Event) (tea.Model, tea.Cmd) {
	if m.vp.Apply(ev) {
		return m, nil
	}
	if fn, ok := pagerActions[ev.Action]; ok {
		return fn(m, keyCtx{Event: ev})
	}
	return m, nil
}
