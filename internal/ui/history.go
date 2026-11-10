package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"s3reader/internal/store"
)

// Directory listings are cached per bucket+prefix so moving back and forth
// is instant; r reloads.
const (
	dirCacheTTL  = 5 * time.Minute
	dirCacheSize = 64
	maxJumps     = 100
)

type dirKey struct{ bucket, prefix string }

type dirEntry struct {
	entries []store.Entry
	at      time.Time
}

type dirCache map[dirKey]dirEntry

func (c dirCache) get(k dirKey) (dirEntry, bool) {
	e, ok := c[k]
	return e, ok && time.Since(e.at) < dirCacheTTL
}

func (c dirCache) put(k dirKey, entries []store.Entry) {
	if _, ok := c[k]; !ok && len(c) >= dirCacheSize {
		var oldest dirKey
		var at time.Time
		for k, e := range c {
			if at.IsZero() || e.at.Before(at) {
				oldest, at = k, e.at
			}
		}
		delete(c, oldest)
	}
	c[k] = dirEntry{entries, time.Now()}
}

func (m *model) here() dirKey { return dirKey{m.bucket, m.prefix} }

func (m *model) resetHistory() {
	m.dirs, m.cursors = dirCache{}, map[dirKey]string{}
	m.jumps, m.jumpIdx = nil, 0
}

// leave remembers the cursor of the dir being left; with jump it also adds
// the dir to the jump list, dropping forward history like vim.
func (m *model) leave(jump bool) {
	if e, ok := m.objects.current(); ok {
		m.cursors[m.here()] = e.Key
	}
	if !jump {
		return
	}
	m.jumps = m.jumps[:min(m.jumpIdx, len(m.jumps))]
	if n := len(m.jumps); n == 0 || m.jumps[n-1] != m.here() {
		m.jumps = append(m.jumps, m.here())
	}
	if len(m.jumps) > maxJumps {
		m.jumps = m.jumps[1:]
	}
	m.jumpIdx = len(m.jumps)
}

// jump moves through the jump list: -1 older (ctrl+o), +1 newer (ctrl+i).
func (m *model) jump(dir int) tea.Cmd {
	if m.bclient == nil {
		return nil
	}
	if m.jumpIdx >= len(m.jumps) && dir < 0 {
		m.leave(true) // so ctrl+i can come back here
		m.jumpIdx--
	}
	next := m.jumpIdx + dir
	if next < 0 || next >= len(m.jumps) {
		msg := "at newest jump"
		if dir < 0 {
			msg = "at oldest jump"
		}
		m.setStatus(msg, false)
		return nil
	}
	m.leave(false)
	m.jumpIdx = next
	to := m.jumps[next]
	if to.bucket != m.bucket {
		if !m.popTo(scrBuckets) {
			m.stack = m.stack[:1]
		}
		return m.fetchBucket(to.bucket, to.prefix)
	}
	if !m.popTo(scrObjects) {
		m.push(scrObjects)
	}
	return m.listDir(to.prefix, "")
}
