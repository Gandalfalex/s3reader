package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *model) openCommandLine() tea.Cmd {
	m.mode = inputCommand
	m.input.Prompt = ":"
	m.input.Placeholder = "command  (tab: complete, ↑↓: history, :help)"
	m.input.SetValue("")
	m.histIdx = len(m.cmdHist)
	m.compl = m.computeCompletions("")
	m.complIdx = -1
	return m.input.Focus()
}

func (m *model) closeCommandLine() {
	m.mode = inputNone
	m.input.Blur()
	m.compl, m.complIdx = nil, -1
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
		if n := len(m.cmdHist); n == 0 || m.cmdHist[n-1] != line {
			m.cmdHist = append(m.cmdHist, line)
		}
		return m, m.execute(line)
	case "tab", "ctrl+n":
		m.cycleCompletion(1)
		return m, nil
	case "shift+tab", "ctrl+p":
		m.cycleCompletion(-1)
		return m, nil
	case "up", "down":
		if len(m.cmdHist) == 0 {
			return m, nil
		}
		if msg.String() == "up" {
			m.histIdx = max(0, m.histIdx-1)
		} else {
			m.histIdx = min(len(m.cmdHist), m.histIdx+1)
		}
		val := ""
		if m.histIdx < len(m.cmdHist) {
			val = m.cmdHist[m.histIdx]
		}
		m.input.SetValue(val)
		m.input.CursorEnd()
		m.compl, m.complIdx = m.computeCompletions(val), -1
		return m, nil
	case "backspace":
		if m.input.Value() == "" { // like vim: backspace on empty line leaves
			m.closeCommandLine()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.compl, m.complIdx = m.computeCompletions(m.input.Value()), -1
	return m, cmd
}

func (m *model) cycleCompletion(dir int) {
	if len(m.compl) == 0 {
		return
	}
	n := len(m.compl)
	m.complIdx = ((m.complIdx+dir)%n + n) % n
	m.input.SetValue(m.compl[m.complIdx].value)
	m.input.CursorEnd()
}

// renderPopup draws the completion menu, scrolled so the selection is visible.
func (m *model) renderPopup() []string {
	n := len(m.compl)
	if n == 0 {
		return nil
	}
	win := min(n, maxPopup)
	if n > maxPopup {
		win-- // last row shows "… more"
	}
	start := 0
	if m.complIdx >= win {
		start = m.complIdx - win + 1
	}
	labelW := 0
	for _, it := range m.compl {
		labelW = max(labelW, ansi.StringWidth(it.label))
	}
	labelW = min(labelW+2, max(10, m.w/2))
	var out []string
	for i := start; i < start+win; i++ {
		it := m.compl[i]
		line := pad(" "+pad(it.label, labelW)+sMuted.Render(it.desc), m.w)
		if i == m.complIdx {
			line = sSel.Render(ansi.Strip(line))
		} else {
			line = sBar.Render(line)
		}
		out = append(out, line)
	}
	if rest := n - start - win; n > maxPopup {
		out = append(out, sBar.Render(pad(sMuted.Render(fmt.Sprintf(" %d/%d · %d more (tab to cycle)", max(m.complIdx+1, 0), n, rest)), m.w)))
	}
	return out
}

func (m *model) execute(line string) tea.Cmd {
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	if n, err := strconv.Atoi(name); err == nil { // :42 jumps to line 42
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
	c, err := resolveCommand(name)
	if err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	return c.run(m, arg)
}
