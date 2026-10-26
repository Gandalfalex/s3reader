package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"s3reader/internal/store"
)

func (m *model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "s3reader"
	return v
}

func (m *model) render() string {
	if m.w == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.titleBar())
	b.WriteByte('\n')

	switch m.top() {
	case scrInfo, scrObject, scrHelp, scrErrors:
		b.WriteString(m.vp.View())
		b.WriteByte('\n')
	default:
		b.WriteString(m.renderList())
	}
	body := b.String()
	if popup := m.renderPopup(); m.mode == inputCommand && len(popup) > 0 {
		// draw the completion menu over the bottom of the body, like vim's pum
		lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
		k := min(len(popup), len(lines)-1) // keep the title bar
		copy(lines[len(lines)-k:], popup[len(popup)-k:])
		body = strings.Join(lines, "\n") + "\n"
	}
	return body + m.statusBar() + "\n" + m.hints()
}

func (m *model) titleBar() string {
	parts := []string{sTitle.Render("s3reader")}
	if m.profile.Name != "" {
		parts = append(parts, sCrumb.Render(m.profile.Name))
	}
	switch m.top() {
	case scrObjects, scrSearch, scrObject:
		parts = append(parts, sCrumb.Render("s3://"+m.bucket+"/"+m.prefix))
	case scrInfo:
		if m.info != nil {
			parts = append(parts, sCrumb.Render("s3://"+m.info.Name), sMuted.Render("bucket info"))
		}
	case scrHelp:
		parts = append(parts, sMuted.Render("help"))
	case scrErrors:
		parts = append(parts, sErr.Render("error log"))
	}
	if m.top() == scrSearch {
		parts = append(parts, sWarn.Render("search: "+m.searchQuery.Raw))
	}
	if m.top() == scrObject && m.detail != nil {
		parts[len(parts)-1] = sCrumb.Render("s3://" + m.bucket + "/" + m.detail.Key)
	}
	s := strings.Join(parts, sMuted.Render(" › "))
	return ansi.Truncate(s, m.w, "…")
}

func (m *model) renderList() string {
	l := m.list()
	h := m.listHeight()
	l.scroll(h)

	var b strings.Builder
	nameW := m.w - 2
	var header string
	switch m.top() {
	case scrProfiles:
		header = pad("PROFILE", 24) + pad("ENDPOINT / REGION", 40) + "BUCKET"
	case scrBuckets:
		nameW = m.w - 2 - 20
		header = pad("BUCKET", nameW) + "CREATED"
	default:
		nameW = m.w - 2 - 11 - 18 - 14
		if nameW < 20 {
			nameW = m.w - 2 - 11
		}
		header = pad("NAME", nameW) + padLeft("SIZE", 10) + " "
		if nameW == m.w-2-11 {
			break
		}
		header += " " + pad("MODIFIED", 17) + pad("CLASS", 14)
	}
	b.WriteString(sHeader.Render(ansi.Truncate("  "+header, m.w, "")))
	b.WriteByte('\n')

	lines := 0
	if len(l.vis) == 0 {
		msg := "  (empty)"
		if !l.filter.Empty() {
			msg = "  no matches for filter: " + l.filter.Raw
		}
		if m.loading {
			msg = "  " + m.spin.View() + " " + m.what
		}
		b.WriteString(sMuted.Render(msg))
		b.WriteByte('\n')
		lines++
	}
	for i := l.Top; i < len(l.vis) && i < l.Top+h; i++ {
		e := l.all[l.vis[i]]
		row := m.renderRow(e, nameW)
		if i == l.Cursor {
			row = sSel.Render(pad("▌ "+ansi.Strip(row), m.w))
		} else {
			row = "  " + row
		}
		b.WriteString(ansi.Truncate(row, m.w, ""))
		b.WriteByte('\n')
		lines++
	}
	for ; lines < h; lines++ {
		b.WriteString(sMuted.Render("~"))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m *model) renderRow(e store.Entry, nameW int) string {
	switch m.top() {
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
		return pad(e.Name, 24) + sMuted.Render(pad(ep, 40)) + bucket
	case scrBuckets:
		return pad(e.Name, nameW) + sMuted.Render(fmtTime(e.Modified))
	}
	name := e.Name
	if e.IsDir {
		name = sDir.Render(trunc(name, nameW))
		return pad(name, nameW)
	}
	row := pad(trunc(name, nameW), nameW) + padLeft(humanSize(e.Size), 10) + " "
	if nameW == m.w-2-11 {
		return row
	}
	cls := e.Class
	if cls == "" {
		cls = "STANDARD"
	}
	return row + " " + sMuted.Render(pad(fmtTime(e.Modified), 17)+pad(cls, 14))
}

func (m *model) statusBar() string {
	modeName, modeColor := "NORMAL", cAccent
	switch m.mode {
	case inputFilter:
		modeName, modeColor = "FILTER", cOK
	case inputSearch:
		modeName, modeColor = "SEARCH", cWarn
	case inputCommand:
		modeName, modeColor = "COMMAND", cDir
	}
	if m.mode != inputNone {
		left := sMode.Background(modeColor).Render(modeName) + " "
		return left + ansi.Truncate(m.input.View(), m.w-lipgloss.Width(left), "")
	}

	left := sMode.Background(modeColor).Render(modeName)
	var mid []string
	if m.loading {
		mid = append(mid, m.spin.View()+" "+m.what)
	}
	if l := m.list(); l != nil {
		pos := 0
		if len(l.vis) > 0 {
			pos = l.Cursor + 1
		}
		mid = append(mid, fmt.Sprintf("%d/%d", pos, len(l.vis)))
		if len(l.vis) != len(l.all) {
			mid = append(mid, fmt.Sprintf("(of %d)", len(l.all)))
		}
		if m.top() == scrObjects || m.top() == scrSearch {
			mid = append(mid, humanSize(l.totalSize()))
			arrow := "↑"
			if l.reverse {
				arrow = "↓"
			}
			mid = append(mid, "sort:"+sortNames[l.sortBy]+arrow)
		}
		if !l.filter.Empty() {
			mid = append(mid, sOK.Render("/"+l.filter.Raw))
		}
		if m.top() == scrSearch && !m.searchDone {
			mid = append(mid, fmt.Sprintf("scanned %d", m.searchScanned))
		}
	} else {
		mid = append(mid, fmt.Sprintf("%3.0f%%", m.vp.ScrollPercent()*100))
	}
	if n := len(m.errs); n > 0 && m.top() != scrErrors {
		mid = append(mid, sErr.Render(fmt.Sprintf("E:%d", n)))
	}
	if p := m.keys.Partial(); p != "" {
		mid = append(mid, sWarn.Render(p))
	}
	if m.status != "" {
		st := m.status
		if m.statusErr {
			st = sErr.Render(st)
		}
		mid = append(mid, st)
	}
	line := left + " " + strings.Join(mid, sMuted.Render(" │ "))
	line = ansi.Truncate(line, m.w, "…")
	return sBar.Render(pad(line, m.w))
}

func (m *model) hints() string {
	if m.mode == inputCommand {
		return sMuted.Render(ansi.Truncate(" tab/S-tab complete · ↑↓ history · enter run · esc cancel", m.w, "…"))
	}
	var h string
	switch m.top() {
	case scrProfiles:
		h = "j/k move · l/enter select · / filter · : command · :help · q quit"
	case scrBuckets:
		h = "j/k move · l open · i info · / filter · o sort · y yank · r reload · : command · :help"
	case scrObjects:
		h = "j/k gg/G ^d/^u · l open · h up · / filter · ? search · i info · o/O sort · : command · :help"
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
	return sMuted.Render(ansi.Truncate(" "+h, m.w, "…"))
}
