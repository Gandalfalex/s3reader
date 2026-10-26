package ui

import (
	"path"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *model) resort(l *listState) {
	key := ""
	if e, ok := l.current(); ok {
		key = e.Key
	}
	l.apply()
	if key != "" {
		l.selectKey(key)
	}
}

func (m *model) open() (tea.Model, tea.Cmd) {
	l := m.list()
	e, ok := l.current()
	if !ok {
		return m, nil
	}
	switch m.top() {
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
	switch m.top() {
	case scrObjects:
		if m.prefix != "" {
			trimmed := strings.TrimSuffix(m.prefix, "/")
			parent := ""
			if i := strings.LastIndex(trimmed, "/"); i >= 0 {
				parent = trimmed[:i+1]
			}
			return m, m.loadDir(parent, m.prefix)
		}
		if len(m.stack) == 1 {
			m.setStatus("at bucket root", false)
			return m, nil
		}
		m.pop()
		if m.top() == scrBuckets {
			m.buckets.selectKey(m.bucket)
		}
	case scrBuckets:
		if len(m.stack) > 1 {
			m.pop()
		}
	case scrSearch:
		m.pop()
	default:
		m.pop()
	}
	return m, nil
}

func (m *model) openInfo() (tea.Model, tea.Cmd) {
	switch m.top() {
	case scrBuckets:
		if e, ok := m.buckets.current(); ok {
			return m, m.loadInfo(e.Key, e.Modified)
		}
	case scrObjects, scrSearch:
		return m, m.loadInfo(m.bucket, m.bucketCreated)
	}
	return m, nil
}

func (m *model) reload() (tea.Model, tea.Cmd) {
	switch m.top() {
	case scrBuckets:
		m.pop()
		return m, m.loadBuckets()
	case scrObjects:
		sel := ""
		if e, ok := m.objects.current(); ok {
			sel = e.Key
		}
		return m, m.loadDir(m.prefix, sel)
	case scrSearch:
		return m, m.startSearch(m.searchQuery)
	}
	return m, nil
}

func (m *model) popTo(s screen) bool {
	for i := len(m.stack) - 1; i >= 0; i-- {
		if m.stack[i] == s {
			m.stack = m.stack[:i+1]
			return true
		}
	}
	return false
}

// popToList closes pager screens (info, object, help, errors).
func (m *model) popToList() {
	for len(m.stack) > 1 && m.list() == nil {
		m.pop()
	}
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
