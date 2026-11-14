package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Gandalfalex/vim-kit/cmdline"
)

func (m *model) openCommandLine() tea.Cmd {
	m.mode = inputCommand
	m.input.Prompt = ":"
	m.input.Placeholder = "command  (tab: complete, ↑↓: history, :help)"
	m.input.SetValue("")
	m.cmd.Open(m.computeCompletions(""))
	return m.input.Focus()
}

func (m *model) closeCommandLine() {
	m.mode = inputNone
	m.input.Blur()
	m.cmd.Close()
}

func (m *model) handleCommandKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+[":
		m.closeCommandLine()
		return m, nil
	case "enter":
		line := strings.TrimSpace(m.input.Value())
		m.closeCommandLine()
		if line == "" {
			return m, nil
		}
		m.cmd.Commit(line)
		return m, m.execute(line)
	case "tab", "ctrl+n":
		m.cycleCompletion(1)
		return m, nil
	case "shift+tab", "ctrl+p":
		m.cycleCompletion(-1)
		return m, nil
	case "up", "down":
		dir := 1
		if msg.String() == "up" {
			dir = -1
		}
		if val, ok := m.cmd.Recall(dir); ok {
			m.input.SetValue(val)
			m.input.CursorEnd()
			m.cmd.SetItems(m.computeCompletions(val))
		}
		return m, nil
	case "backspace":
		if m.input.Value() == "" { // like vim: backspace on empty line leaves
			m.closeCommandLine()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.cmd.SetItems(m.computeCompletions(m.input.Value()))
	return m, cmd
}

func (m *model) cycleCompletion(dir int) {
	if val, ok := m.cmd.Cycle(dir); ok {
		m.input.SetValue(val)
		m.input.CursorEnd()
	}
}

// renderPopup draws the completion menu, scrolled so the selection is visible.
func (m *model) renderPopup() []string {
	compl, complIdx := m.cmd.Items, m.cmd.Index
	n := len(compl)
	if n == 0 {
		return nil
	}
	win := min(n, maxPopup)
	if n > maxPopup {
		win-- // last row shows "… more"
	}
	start := 0
	if complIdx >= win {
		start = complIdx - win + 1
	}
	labelW := 0
	for _, it := range compl {
		labelW = max(labelW, ansi.StringWidth(it.Label))
	}
	labelW = min(labelW+2, max(10, m.w/2))
	var out []string
	for i := start; i < start+win; i++ {
		it := compl[i]
		line := pad(" "+pad(it.Label, labelW)+sMuted.Render(it.Desc), m.w)
		if i == complIdx {
			line = sSel.Render(ansi.Strip(line))
		} else {
			line = sBar.Render(line)
		}
		out = append(out, line)
	}
	if rest := n - start - win; n > maxPopup {
		out = append(out, sBar.Render(pad(sMuted.Render(fmt.Sprintf(" %d/%d · %d more (tab to cycle)", max(complIdx+1, 0), n, rest)), m.w)))
	}
	return out
}

func (m *model) execute(line string) tea.Cmd {
	name, arg := cmdline.Split(line)
	if n, ok := cmdline.LineNumber(name); ok { // :42 jumps to line 42
		if l := m.list(); l != nil {
			l.Cursor = n - 1
			l.clamp()
			l.Top = l.Cursor - m.listHeight()/2
			l.scroll(m.listHeight())
		} else {
			m.vp.SetYOffset(n - 1)
		}
		return nil
	}
	c, err := commands.Resolve(name)
	if err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	return c.Run(m, arg)
}
