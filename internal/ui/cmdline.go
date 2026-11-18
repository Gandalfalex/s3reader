package ui

import (
	tea "charm.land/bubbletea/v2"

	tk "github.com/Gandalfalex/tuikit"
)

func (m *model) openCommandLine() tea.Cmd {
	m.cmd.Open(m)
	return m.prompt.Open(inputCommand, tk.ModeCommand, ":", tk.Placeholder, "")
}

func (m *model) handleCommandKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	cmd, err := tk.HandleCommandKey(&m.cmd, m, &m.prompt, msg)
	if err != nil {
		m.SetStatus(err.Error(), true)
	}
	return m, cmd
}

// gotoLine is :42 on whatever the screen shows.
func gotoLine(m *model, n int) {
	if l := m.list(); l != nil {
		l.Cursor = n - 1
		l.Clamp(len(l.Vis))
		l.Top = l.Cursor - m.listHeight()/2
		l.Scroll(m.listHeight())
		return
	}
	m.vp.Goto(n)
}
