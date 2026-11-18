package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"s3reader/internal/query"

	tk "github.com/Gandalfalex/tuikit"
)

func (m *model) needBucket() bool {
	if m.bclient == nil || m.bucket == "" {
		m.SetStatus("no bucket open (:bucket <name>)", true)
		return false
	}
	return true
}

func (m *model) cmdCd(arg string) tea.Cmd {
	if !m.needBucket() {
		return nil
	}
	if !m.Stack.PopTo(scrObjects) {
		m.Stack.Push(scrObjects)
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
	if !m.Stack.PopTo(scrObjects) {
		m.Stack.Push(scrObjects)
	}
	for _, e := range m.objects.All {
		if e.Name == arg || e.Key == arg {
			m.objects.Select(e.Key)
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
		m.SetStatus("no profile connected", true)
		return nil
	}
	if arg == "" {
		return m.cmdBuckets()
	}
	if !m.Stack.PopTo(scrBuckets) {
		m.Stack.Reset()
	}
	m.buckets.Select(arg)
	return m.openBucket(arg, "")
}

func (m *model) cmdBuckets() tea.Cmd {
	if m.client == nil {
		m.SetStatus("no profile connected", true)
		return nil
	}
	if m.Stack.PopTo(scrBuckets) {
		m.Stack.Pop() // bucketsMsg pushes a fresh list
	} else {
		m.Stack.Reset()
	}
	return m.loadBuckets()
}

func (m *model) cmdProfile(arg string) tea.Cmd {
	for i, p := range m.settings.Profiles {
		if p.Name == arg {
			m.Stack.Reset()
			m.profiles.Select(strconv.Itoa(i))
			m.bucket, m.bclient, m.prefix = "", nil, ""
			m.buckets = sortedList(sortName, false)
			return m.selectProfile(i)
		}
	}
	m.SetStatus("unknown profile: "+arg, true)
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
		m.SetStatus(err.Error(), true)
		return nil
	}
	l.setQuery(q)
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
		m.SetStatus(err.Error(), true)
		return nil
	}
	if !m.Stack.PopTo(scrSearch) && !m.Stack.PopTo(scrObjects) {
		m.Stack.Push(scrObjects)
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
		m.SetStatus("usage: :sort name|size|modified [desc]", true)
		return nil
	}
	idx := -1
	for i, n := range sortNames {
		if strings.HasPrefix(n, f[0]) {
			idx = i
		}
	}
	if idx < 0 {
		m.SetStatus("unknown sort field: "+f[0], true)
		return nil
	}
	l.order(idx, len(f) > 1 && strings.HasPrefix("desc", f[1]))
	l.Resort()
	return nil
}

// cmdInfo opens bucket info; with stats != nil it also scans that prefix
// once the info has loaded.
func (m *model) cmdInfo(stats *string) tea.Cmd {
	m.popToList()
	if m.Stack.Top() == scrProfiles {
		if m.bucket == "" {
			m.SetStatus("no bucket open", true)
			return nil
		}
		m.Stack.Push(scrObjects)
	}
	m.pendingStats = stats
	_, c := m.openInfo()
	return c
}

func (m *model) cmdYank() tea.Cmd {
	switch m.Stack.Top() {
	case scrObject:
		if m.detail != nil {
			return tk.YankValue("s3://" + m.bucket + "/" + m.detail.Key)
		}
	case scrInfo:
		if m.info != nil {
			return tk.YankValue("s3://" + m.info.Name)
		}
	}
	if l := m.list(); l != nil {
		if e, ok := l.Current(); ok {
			switch m.Stack.Top() {
			case scrBuckets:
				return tk.YankValue("s3://" + e.Key)
			case scrObjects, scrSearch:
				return tk.YankValue("s3://" + m.bucket + "/" + e.Key)
			}
		}
	}
	if m.bucket != "" {
		return tk.YankValue("s3://" + m.bucket + "/" + m.prefix)
	}
	return nil
}

// commandNames lists commands for the help screen.
