package ui

import (
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"s3reader/internal/store"

	tk "github.com/Gandalfalex/tuikit"
)

func (m *model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "s3reader"
	return v
}

func (m *model) render() string {
	if m.W == 0 {
		return ""
	}
	var body string
	switch m.Stack.Top() {
	case scrInfo, scrObject, scrHelp, scrErrors:
		body = m.vp.View()
	default:
		body = m.renderList()
	}
	var popup []string
	if m.prompt.Kind == inputCommand {
		popup = tk.Popup(theme, &m.cmd.Line, m.W, maxPopup)
	}
	return tk.Frame(m.titleBar(), tk.Overlay(body, popup, false), m.statusBar(), m.hints())
}

// titleBar shows where we are; a path that does not fit is cut from the
// left, since its deepest part is the one that matters.
func (m *model) titleBar() string {
	var before, after []string
	if m.profile.Name != "" {
		before = append(before, sCrumb.Render(m.profile.Name))
	}
	path := ""
	switch m.Stack.Top() {
	case scrObjects, scrSearch:
		path = "s3://" + m.bucket + "/" + m.prefix
	case scrObject:
		path = "s3://" + m.bucket + "/" + m.detail.Key
	case scrInfo:
		path = "s3://" + m.info.Name
	}
	switch m.Stack.Top() {
	case scrInfo:
		after = append(after, sMuted.Render("bucket info"))
	case scrHelp:
		after = append(after, sMuted.Render("help"))
	case scrErrors:
		after = append(after, sErr.Render("error log"))
	case scrSearch:
		after = append(after, sWarn.Render("search: "+m.searchQuery.Raw))
	}
	return tk.TitleBarPath(theme, m.W, "s3reader", before, path, after)
}

func (m *model) renderList() string {
	l := m.list()
	h := m.listHeight()
	nameW := m.W - 2
	var header string
	switch m.Stack.Top() {
	case scrProfiles:
		header = tk.Pad("PROFILE", 24) + tk.Pad("ENDPOINT / REGION", 40) + "BUCKET"
	case scrBuckets:
		nameW = m.W - 2 - 20
		header = tk.Pad("BUCKET", nameW) + "CREATED"
	default:
		nameW = m.W - 2 - 11 - 18 - 14
		if nameW < 20 {
			nameW = m.W - 2 - 11
		}
		header = tk.Pad("NAME", nameW) + tk.PadLeft("SIZE", 10) + " "
		if nameW == m.W-2-11 {
			break
		}
		header += " " + tk.Pad("MODIFIED", 17) + tk.Pad("CLASS", 14)
	}
	empty := ""
	if m.Ops.Loading() {
		empty = m.Ops.Spinner()
	}
	sel := m.visualSet(l)
	return l.View(theme, m.W, h, tk.ListView[store.Entry]{
		Header: header,
		Row: func(e store.Entry, i int) string {
			if sel[i] {
				return visualRow(m.renderRow(e, nameW), m.W-2)
			}
			return m.renderRow(e, nameW)
		},
		Empty: empty,
	})
}

func (m *model) renderRow(e store.Entry, nameW int) string {
	switch m.Stack.Top() {
	case scrProfiles:
		i, _ := strconv.Atoi(e.Key)
		p := m.settings.Profiles[i]
		ep := p.Endpoint
		if ep == "" {
			ep = "aws"
		}
		if p.Region != "" {
			ep += " (" + p.Region + ")"
		}
		bucket := p.Bucket
		if bucket == "" {
			bucket = sMuted.Render("(all)")
		}
		return tk.Pad(e.Name, 24) + sMuted.Render(tk.Pad(ep, 40)) + bucket
	case scrBuckets:
		return tk.Pad(tk.TruncMid(e.Name, nameW), nameW) + sMuted.Render(fmtTime(e.Modified))
	}
	name := tk.TruncMid(e.Name, nameW)
	if e.IsDir {
		return tk.Pad(sDir.Render(name), nameW)
	}
	row := tk.Pad(name, nameW) + tk.PadLeft(tk.FmtSize(e.Size), 10) + " "
	if nameW == m.W-2-11 {
		return row
	}
	cls := e.Class
	if cls == "" {
		cls = "STANDARD"
	}
	return row + " " + sMuted.Render(tk.Pad(fmtTime(e.Modified), 17)+tk.Pad(cls, 14))
}

func (m *model) statusBar() string {
	if m.prompt.Active() {
		return tk.PromptBar(theme, m.W, m.prompt.Mode, m.prompt.Input)
	}
	var mid []string
	if l := m.list(); l != nil {
		mid = append(mid, l.Position()...)
		if m.Stack.Top() == scrObjects || m.Stack.Top() == scrSearch {
			mid = append(mid, tk.FmtSize(l.totalSize()))
			arrow := "↑"
			if l.reverse {
				arrow = "↓"
			}
			mid = append(mid, "sort:"+sortNames[l.sortBy]+arrow)
		}
		if l.Filter != "" {
			mid = append(mid, sOK.Render("/"+l.Filter))
		}
		if m.visualOn() {
			mid = append(mid, sWarn.Render(strconv.Itoa(m.visualCount())+" selected"))
		}
		if m.Stack.Top() == scrSearch && !m.searchDone {
			mid = append(mid, fmt.Sprintf("scanned %d", m.searchScanned))
		}
	} else {
		mid = append(mid, m.vp.Percent())
	}
	if s := m.downloadStatus(); s != "" {
		mid = append(mid, sOK.Render(s))
	}
	mode := tk.ModeNormal
	if m.visualOn() {
		mode = tk.ModeVLine
	}
	return tk.StatusBar(theme, m.W, mode, m.StatusParts(mid, m.Stack.Top() == scrErrors))
}

func (m *model) hints() string {
	if m.prompt.Kind == inputCommand {
		return tk.HintBar(theme, m.W, "tab/S-tab complete · ↑↓ history · enter run · esc cancel")
	}
	if m.visualOn() {
		return tk.HintBar(theme, m.W, "j/k gg/G extend · y yank s3:// URIs · Y yank keys · esc/v cancel")
	}
	var h string
	switch m.Stack.Top() {
	case scrProfiles:
		h = "j/k move · l/enter select · / filter · : command · :help · q quit"
	case scrBuckets:
		h = "j/k move · l open · i info · / filter · o sort · y yank · r reload · : command · :help"
	case scrObjects:
		h = "j/k gg/G ^d/^u · l open · h up · ^o/^i jump · / filter · ? search · i info · o/O sort · : command · :help"
	case scrSearch:
		h = "l details · D reveal in dir · / filter results · ? new search · h back · esc cancel"
	case scrInfo:
		h = "c compute stats (bucket) · C stats (current prefix) · j/k scroll · y yank · h back"
	case scrObject:
		h = "j/k ^d/^u scroll · gg/G · y yank uri · r reload · h back"
	case scrHelp:
		h = "j/k scroll · q back"
	case scrErrors:
		h = "j/k scroll · G newest · q back · full log: " + m.logPath
	}
	return tk.HintBar(theme, m.W, h)
}
