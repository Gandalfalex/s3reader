package ui

import (
	"fmt"
	"log/slog"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"s3reader/internal/query"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if r, ok := msg.(opResult); ok && r.opGen() != m.gen {
		return m, nil // result of a superseded or cancelled operation
	}
	cmd := m.update(msg)
	if m.loading && !m.spinning {
		m.spinning = true
		cmd = tea.Batch(cmd, m.spin.Tick)
	}
	return m, cmd
}

func (m *model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.SetWidth(max(10, m.w-4))
		m.vp.SetWidth(m.w)
		m.vp.SetHeight(max(1, m.h-3))
		m.refreshViewport()
		return nil

	case spinner.TickMsg:
		if !m.loading {
			m.spinning = false
			return nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return cmd

	case statusMsg:
		m.setStatus(string(msg), false)
		return nil

	case clientMsg:
		m.done()
		if msg.err != nil {
			m.fail("connect "+msg.profile.Name, msg.err)
			return nil
		}
		slog.Info("connected", "profile", msg.profile.Name, "endpoint", msg.profile.Endpoint, "region", msg.client.Options().Region)
		m.client, m.profile = msg.client, msg.profile
		if m.profile.Bucket != "" {
			return m.openBucket(m.profile.Bucket, m.profile.Prefix)
		}
		return m.loadBuckets()

	case bucketsMsg:
		m.done()
		if msg.err != nil {
			m.fail("list buckets", msg.err)
			return nil
		}
		m.buckets = listState{}
		m.buckets.set(msg.buckets)
		m.push(scrBuckets)
		m.setStatus(fmt.Sprintf("%d buckets", len(msg.buckets)), false)
		return nil

	case bucketOpenedMsg:
		m.done()
		if msg.err != nil {
			m.fail("open bucket "+msg.bucket, msg.err)
			return nil
		}
		m.bucket, m.bucketRegion, m.bclient, m.prefix = msg.bucket, msg.region, msg.client, msg.prefix
		m.bucketCreated = time.Time{}
		for _, b := range m.buckets.all {
			if b.Key == msg.bucket {
				m.bucketCreated = b.Modified
				break
			}
		}
		sortBy, rev := m.objects.sortBy, m.objects.reverse
		m.objects = listState{sortBy: sortBy, reverse: rev}
		m.objects.set(msg.entries)
		m.push(scrObjects)
		m.setStatus("", false)
		return nil

	case dirMsg:
		m.done()
		if msg.err != nil {
			m.fail("list s3://"+m.bucket+"/"+msg.prefix, msg.err)
			return nil
		}
		if msg.prefix != m.prefix { // keep the filter on reload, drop it on cd
			m.objects.filter = query.Query{}
		}
		m.prefix = msg.prefix
		m.objects.Cursor, m.objects.Top = 0, 0
		m.objects.set(msg.entries)
		if msg.sel != "" {
			m.objects.selectKey(msg.sel)
		}
		m.objects.Top = m.objects.Cursor - m.listHeight()/2
		m.objects.scroll(m.listHeight())
		return nil

	case searchMsg:
		m.searchScanned = msg.scanned
		m.search.appendSorted(msg.found)
		if msg.done {
			m.done()
			m.searchDone = true
			if msg.err != nil {
				m.fail("search "+m.searchQuery.Raw, msg.err)
			} else {
				m.setStatus(fmt.Sprintf("%d matches in %d objects", len(m.search.all), msg.scanned), false)
			}
			return nil
		}
		return waitFor(msg.ch)

	case infoMsg:
		m.done()
		m.info = &msg.info
		m.stats = nil
		m.openPage(scrInfo, false)
		if p := m.pendingStats; p != nil {
			m.pendingStats = nil
			return m.scanStats(*p)
		}
		return nil

	case statsMsg:
		m.stats = msg.stats
		if msg.stats.Done {
			m.done()
			m.fail("scan s3://"+m.info.Name+"/"+msg.stats.Prefix, msg.stats.Err)
		}
		m.refreshViewport()
		if msg.stats.Done {
			return nil
		}
		return waitFor(msg.ch)

	case detailMsg:
		m.done()
		m.detail = &msg.detail
		if msg.detail.Head == nil && msg.detail.Err != nil {
			m.fail("head "+msg.detail.Key, msg.detail.Err)
			return nil
		}
		if msg.detail.Err != nil {
			slog.Warn("object preview failed", "key", msg.detail.Key, "err", msg.detail.Err)
		}
		m.openPage(scrObject, false)
		return nil

	case tea.KeyPressMsg:
		_, cmd := m.handleKey(msg)
		return cmd
	}

	if m.mode != inputNone {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return cmd
	}
	return nil
}
