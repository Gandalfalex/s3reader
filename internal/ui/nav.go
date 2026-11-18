package ui

import (
	"path"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *model) open() (tea.Model, tea.Cmd) {
	l := m.list()
	e, ok := l.Current()
	if !ok {
		return m, nil
	}
	switch m.Stack.Top() {
	case scrProfiles:
		i, _ := strconv.Atoi(e.Key)
		return m, m.selectProfile(i)
	case scrBuckets:
		return m, m.openBucket(e.Key, "")
	case scrObjects:
		if e.IsDir {
			return m, m.loadDir(e.Key, "")
		}
		return m, m.loadDetail(e.Key)
	case scrSearch:
		return m, m.loadDetail(e.Key)
	}
	return m, nil
}

func (m *model) back() (tea.Model, tea.Cmd) {
	switch m.Stack.Top() {
	case scrObjects:
		if m.prefix != "" {
			trimmed := strings.TrimSuffix(m.prefix, "/")
			parent := ""
			if i := strings.LastIndex(trimmed, "/"); i >= 0 {
				parent = trimmed[:i+1]
			}
			return m, m.loadDir(parent, m.prefix)
		}
		if m.Stack.Len() == 1 {
			m.SetStatus("at bucket root", false)
			return m, nil
		}
		m.Stack.Pop()
		if m.Stack.Top() == scrBuckets {
			m.buckets.Select(m.bucket)
		}
	case scrBuckets:
		if m.Stack.Len() > 1 {
			m.Stack.Pop()
		}
	case scrSearch:
		m.Stack.Pop()
	default:
		m.Stack.Pop()
	}
	return m, nil
}

func (m *model) openInfo() (tea.Model, tea.Cmd) {
	switch m.Stack.Top() {
	case scrBuckets:
		if e, ok := m.buckets.Current(); ok {
			return m, m.loadInfo(e.Key, e.Modified)
		}
	case scrObjects, scrSearch:
		return m, m.loadInfo(m.bucket, m.bucketCreated)
	}
	return m, nil
}

func (m *model) reload() (tea.Model, tea.Cmd) {
	switch m.Stack.Top() {
	case scrBuckets:
		m.Stack.Pop()
		return m, m.loadBuckets()
	case scrObjects:
		sel := ""
		if e, ok := m.objects.Current(); ok {
			sel = e.Key
		}
		delete(m.dirs, m.here())
		return m, m.loadDir(m.prefix, sel)
	case scrSearch:
		return m, m.startSearch(m.searchQuery)
	}
	return m, nil
}

// popToList closes pager screens (info, object, help, errors).
func (m *model) popToList() {
	m.Stack.PopWhile(func(s screen) bool { return listOf(m, s) == nil })
}

// resolveDir turns a vim-ish path (.., /, a/b, /abs) into a prefix.
func (m *model) resolveDir(arg string) string {
	if arg == "" {
		return m.prefix
	}
	base := "/" + m.prefix
	if strings.HasPrefix(arg, "/") {
		base = ""
	}
	p := strings.TrimPrefix(path.Clean(path.Join("/", base, arg)), "/")
	if p != "" {
		p += "/"
	}
	return p
}
