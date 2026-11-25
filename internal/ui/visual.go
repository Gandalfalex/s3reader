package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	tk "github.com/Gandalfalex/tuikit"
	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/visual"

	"s3reader/internal/store"
)

// sVisual marks rows inside a visual selection (the cursor row keeps sSel).
var sVisual = sSel.UnsetBold().Foreground(cWarn)

// visualState is a line-wise selection over the visible rows of one list
// screen. Rows have no columns, so v and V both select whole rows.
type visualState struct {
	visual.Selection
	scr screen // the list screen it was started on
}

// visualOn reports whether a selection is active on the current screen.
func (m *model) visualOn() bool {
	return m.vis.Active() && m.vis.scr == m.Stack.Top() && m.list() != nil && m.prompt.Kind == inputNone
}

func (m *model) startVisual(l *listState) {
	if len(l.Vis) == 0 {
		return
	}
	m.vis.Start(visual.Line, visual.Pos{Line: l.Cursor})
	m.vis.scr = m.Stack.Top()
}

func (m *model) endVisual() { m.vis = visualState{} }

// visualRange is the selected span of l.Vis, inclusive.
func (m *model) visualRange(l *listState) (from, to int) {
	from, to = m.vis.Anchor.Line, l.Cursor
	if to < from {
		from, to = to, from
	}
	return max(from, 0), min(to, len(l.Vis)-1)
}

// visualSet is the selected rows as indexes into l.All, for rendering.
func (m *model) visualSet(l *listState) map[int]bool {
	if !m.visualOn() {
		return nil
	}
	from, to := m.visualRange(l)
	set := make(map[int]bool, to-from+1)
	for i := from; i <= to; i++ {
		set[l.Vis[i]] = true
	}
	return set
}

func (m *model) visualCount() int {
	from, to := m.visualRange(m.list())
	return max(0, to-from+1)
}

// handleVisualKey runs a key in visual mode; ok is false when the key ends
// the selection and should run as in normal mode.
func (m *model) handleVisualKey(ev keys.Event, l *listState) (tea.Model, tea.Cmd, bool) {
	switch ev.Action {
	case actVisual, actEsc:
		m.endVisual()
		return m, nil, true
	case actYank, actYankKey:
		cmd := m.yankSelection(l, ev.Action == actYankKey)
		m.endVisual()
		return m, cmd, true
	}
	m.endVisual()
	return m, nil, false
}

// yankSelection copies the selected rows, one per line: s3:// URIs, or raw
// keys when raw is set.
func (m *model) yankSelection(l *listState, raw bool) tea.Cmd {
	lines := m.selectedLines(l, raw)
	switch len(lines) {
	case 0:
		return nil
	case 1:
		return tk.YankValue(lines[0])
	}
	return tk.Yank(strings.Join(lines, "\n"), strconv.Itoa(len(lines))+" names")
}

func (m *model) selectedLines(l *listState, raw bool) []string {
	from, to := m.visualRange(l)
	var lines []string
	for i := from; i <= to; i++ {
		e := l.All[l.Vis[i]]
		if raw {
			lines = append(lines, m.keyOf(e))
		} else {
			lines = append(lines, m.uriOf(e))
		}
	}
	return lines
}

// uriOf is the s3:// URI of a list row (the name, for profiles).
func (m *model) uriOf(e store.Entry) string {
	switch m.Stack.Top() {
	case scrBuckets:
		return "s3://" + e.Key
	case scrObjects, scrSearch:
		return "s3://" + m.bucket + "/" + e.Key
	}
	return e.Name
}

// keyOf is the raw key of a list row (the name, for profiles).
func (m *model) keyOf(e store.Entry) string {
	if m.Stack.Top() == scrProfiles {
		return e.Name
	}
	return e.Key
}

// visualRow highlights a selected row across the width.
func visualRow(row string, w int) string { return sVisual.Render(tk.Pad(ansi.Strip(row), w)) }
