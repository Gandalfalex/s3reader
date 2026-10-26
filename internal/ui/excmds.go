package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"s3reader/internal/query"
)

func (m *model) needBucket() bool {
	if m.bclient == nil || m.bucket == "" {
		m.setStatus("no bucket open (:bucket <name>)", true)
		return false
	}
	return true
}

func (m *model) cmdCd(arg string) tea.Cmd {
	if !m.needBucket() {
		return nil
	}
	if !m.popTo(scrObjects) {
		m.push(scrObjects)
	}
	if arg == "" {
		arg = "/"
	}
	return m.loadDir(m.resolveDir(arg), "")
}

func (m *model) cmdOpen(arg string) tea.Cmd {
	if !m.needBucket() {
		return nil
	}
	if arg == "" {
		m.popToList()
		_, c := m.open()
		return c
	}
	if !m.popTo(scrObjects) {
		m.push(scrObjects)
	}
	for _, e := range m.objects.all {
		if e.Name == arg || e.Key == arg {
			m.objects.selectKey(e.Key)
			if e.IsDir {
				return m.loadDir(e.Key, "")
			}
			return m.loadDetail(e.Key)
		}
	}
	if strings.HasSuffix(arg, "/") {
		return m.loadDir(m.resolveDir(arg), "")
	}
	key := strings.TrimPrefix(arg, "/")
	if !strings.HasPrefix(arg, "/") {
		key = m.prefix + arg
	}
	return m.loadDetail(key)
}

func (m *model) cmdBucket(arg string) tea.Cmd {
	if m.client == nil {
		m.setStatus("no profile connected", true)
		return nil
	}
	if arg == "" {
		return m.cmdBuckets()
	}
	if !m.popTo(scrBuckets) {
		m.stack = m.stack[:1]
	}
	m.buckets.selectKey(arg)
	return m.openBucket(arg, "")
}

func (m *model) cmdBuckets() tea.Cmd {
	if m.client == nil {
		m.setStatus("no profile connected", true)
		return nil
	}
	if m.popTo(scrBuckets) {
		m.pop() // bucketsMsg pushes a fresh list
	} else {
		m.stack = m.stack[:1]
	}
	return m.loadBuckets()
}

func (m *model) cmdProfile(arg string) tea.Cmd {
	for i, p := range m.settings.Profiles {
		if p.Name == arg {
			m.stack = m.stack[:1]
			m.profiles.selectKey(strconv.Itoa(i))
			m.bucket, m.bclient, m.prefix = "", nil, ""
			m.buckets = listState{}
			return m.selectProfile(i)
		}
	}
	m.setStatus("unknown profile: "+arg, true)
	return nil
}

func (m *model) cmdFilter(arg string) tea.Cmd {
	m.popToList()
	l := m.list()
	if l == nil {
		return nil
	}
	q, err := query.Parse(arg)
	if err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	l.setFilter(q)
	return nil
}

func (m *model) cmdSearch(arg string) tea.Cmd {
	if !m.needBucket() {
		return nil
	}
	q, err := query.Parse(arg)
	if err != nil || q.Empty() {
		if err == nil {
			err = fmt.Errorf("usage: :search <query>")
		}
		m.setStatus(err.Error(), true)
		return nil
	}
	if !m.popTo(scrSearch) && !m.popTo(scrObjects) {
		m.push(scrObjects)
	}
	return m.runSearch(q)
}

func (m *model) cmdSort(arg string) tea.Cmd {
	m.popToList()
	l := m.list()
	if l == nil {
		return nil
	}
	f := strings.Fields(arg)
	if len(f) == 0 {
		m.setStatus("usage: :sort name|size|modified [desc]", true)
		return nil
	}
	idx := -1
	for i, n := range sortNames {
		if strings.HasPrefix(n, f[0]) {
			idx = i
		}
	}
	if idx < 0 {
		m.setStatus("unknown sort field: "+f[0], true)
		return nil
	}
	l.sortBy = idx
	l.reverse = len(f) > 1 && strings.HasPrefix("desc", f[1])
	m.resort(l)
	return nil
}

// cmdInfo opens bucket info; with stats != nil it also scans that prefix
// once the info has loaded.
func (m *model) cmdInfo(stats *string) tea.Cmd {
	m.popToList()
	if m.top() == scrProfiles {
		if m.bucket == "" {
			m.setStatus("no bucket open", true)
			return nil
		}
		m.push(scrObjects)
	}
	m.pendingStats = stats
	_, c := m.openInfo()
	return c
}

func (m *model) cmdYank() tea.Cmd {
	switch m.top() {
	case scrObject:
		if m.detail != nil {
			return yank("s3://" + m.bucket + "/" + m.detail.Key)
		}
	case scrInfo:
		if m.info != nil {
			return yank("s3://" + m.info.Name)
		}
	}
	if l := m.list(); l != nil {
		if e, ok := l.current(); ok {
			switch m.top() {
			case scrBuckets:
				return yank("s3://" + e.Key)
			case scrObjects, scrSearch:
				return yank("s3://" + m.bucket + "/" + e.Key)
			}
		}
	}
	if m.bucket != "" {
		return yank("s3://" + m.bucket + "/" + m.prefix)
	}
	return nil
}

// commandNames lists commands for the help screen.
func commandHelp() string {
	cs := append([]command(nil), commands...)
	sort.Slice(cs, func(i, j int) bool { return cs[i].name < cs[j].name })
	var b strings.Builder
	for _, c := range cs {
		label := ":" + c.name
		if c.args != "" {
			label += " " + c.args
		}
		alias := ""
		if len(c.aliases) > 0 {
			alias = " (:" + strings.Join(c.aliases, ", :") + ")"
		}
		b.WriteString("    " + pad(label, 26) + c.desc + alias + "\n")
	}
	return b.String()
}
