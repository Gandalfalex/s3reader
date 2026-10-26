package ui

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
	"s3reader/internal/query"
)

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if m.mode == inputCommand {
		return m.handleCommandKey(msg)
	}
	if m.mode != inputNone {
		return m.handleInput(msg)
	}
	m.setStatus("", false) // messages last until the next key

	// esc cancels in-flight work first
	if k == "esc" && m.loading && m.cancel != nil {
		m.cancel()
		m.done()
		m.gen++
		m.keys.Reset()
		m.setStatus("cancelled", false)
		return m, nil
	}

	pager := m.list() == nil
	km := listKeymap
	if pager {
		km = pagerKeymap
	}
	ev, ok := m.keys.Feed(km, k)
	if !ok {
		return m, nil // count digit or pending g/z
	}
	if ev.Seq == ":" {
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
	defer l.scroll(h)

	if l.Apply(ev, len(l.vis), h) {
		return m, nil
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
		if m.mode == inputFilter && l != nil {
			q, _ := query.Parse(m.prevFilter)
			l.setFilter(q)
		}
		m.mode = inputNone
		m.input.Blur()
		return m, nil
	case "enter":
		mode := m.mode
		m.mode = inputNone
		m.input.Blur()
		q, err := query.Parse(m.input.Value())
		if err != nil {
			m.setStatus(err.Error(), true)
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
	m.input, cmd = m.input.Update(msg)
	if m.mode == inputFilter && l != nil {
		q, err := query.Parse(m.input.Value())
		if err == nil {
			l.setFilter(q)
			m.setStatus("", false)
		} else {
			m.setStatus(err.Error(), true)
		}
	}
	return m, cmd
}

// openSearchInput starts the "?" recursive search prompt for the current dir.
func (m *model) openSearchInput() tea.Cmd {
	m.popToList()
	if m.top() != scrObjects && m.top() != scrSearch {
		m.setStatus("open a bucket to search", true)
		return nil
	}
	m.mode = inputSearch
	m.input.Prompt = "?"
	m.input.Placeholder = "search s3://" + m.bucket + "/" + m.prefix + " recursively (same syntax as /)"
	m.input.SetValue("")
	return m.input.Focus()
}

// runSearch opens (or replaces) the search results screen.
func (m *model) runSearch(q query.Query) tea.Cmd {
	if m.top() == scrSearch {
		m.pop()
		m.prefix = m.searchRoot
	}
	m.push(scrSearch)
	return m.startSearch(q)
}

func (m *model) handlePagerKey(ev keys.Event) (tea.Model, tea.Cmd) {
	if motion.ApplyPager(pager{&m.vp}, ev) {
		return m, nil
	}
	if fn, ok := pagerActions[ev.Action]; ok {
		return fn(m, keyCtx{Event: ev})
	}
	return m, nil
}

// pager adapts the bubbles viewport to motion.Pager.
type pager struct{ *viewport.Model }

func (p pager) Top()    { p.GotoTop() }
func (p pager) Bottom() { p.GotoBottom() }
