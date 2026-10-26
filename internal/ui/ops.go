package ui

import (
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"s3reader/internal/query"
	"s3reader/internal/store"
)

func (m *model) selectProfile(i int) tea.Cmd {
	p := m.settings.Profiles[i]
	ctx := m.newOp("connecting to " + p.Name)
	gen := m.gen
	return func() tea.Msg {
		c, err := store.NewClient(ctx, p)
		return clientMsg{op: op(gen), client: c, profile: p, err: err}
	}
}

func (m *model) loadBuckets() tea.Cmd {
	ctx := m.newOp("listing buckets")
	gen, c := m.gen, m.client
	return func() tea.Msg {
		b, err := store.ListBuckets(ctx, c)
		return bucketsMsg{op: op(gen), buckets: b, err: err}
	}
}

func (m *model) openBucket(bucket, prefix string) tea.Cmd {
	ctx := m.newOp("opening " + bucket)
	gen, c, p := m.gen, m.client, m.profile
	return func() tea.Msg {
		bc, region := store.ClientForBucket(ctx, c, p, bucket)
		entries, err := store.ListDir(ctx, bc, bucket, prefix)
		return bucketOpenedMsg{op: op(gen), bucket: bucket, region: region, client: bc, prefix: prefix, entries: entries, err: err}
	}
}

func (m *model) loadDir(prefix, sel string) tea.Cmd {
	ctx := m.newOp("listing s3://" + m.bucket + "/" + prefix)
	gen, c, b := m.gen, m.bclient, m.bucket
	return func() tea.Msg {
		entries, err := store.ListDir(ctx, c, b, prefix)
		return dirMsg{op: op(gen), prefix: prefix, entries: entries, sel: sel, err: err}
	}
}

func (m *model) startSearch(q query.Query) tea.Cmd {
	ctx := m.newOp("searching")
	gen, c, b, root := m.gen, m.bclient, m.bucket, m.prefix
	m.searchRoot, m.searchQuery, m.searchScanned, m.searchDone = root, q, 0, false
	m.search = listState{sortBy: m.objects.sortBy, reverse: m.objects.reverse}
	ch := make(chan tea.Msg, 4)
	go func() {
		defer close(ch)
		var scanned int64
		err := store.Walk(ctx, c, b, root, func(objs []types.Object) error {
			var found []store.Entry
			for _, o := range objs {
				e := store.ObjEntry(o, strings.TrimPrefix(aws.ToString(o.Key), root))
				if q.Match(e) {
					found = append(found, e)
				}
			}
			scanned += int64(len(objs))
			select {
			case ch <- searchMsg{op: op(gen), found: found, scanned: scanned, ch: ch}:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		ch <- searchMsg{op: op(gen), scanned: scanned, done: true, err: err, ch: ch}
	}()
	return waitFor(ch)
}

func (m *model) loadInfo(bucket string, created time.Time) tea.Cmd {
	ctx := m.newOp("fetching bucket info")
	gen, c, p := m.gen, m.client, m.profile
	bc, region := m.bclient, m.bucketRegion
	if bucket != m.bucket || bc == nil {
		bc = nil
	}
	return func() tea.Msg {
		if bc == nil {
			bc, region = store.ClientForBucket(ctx, c, p, bucket)
		}
		return infoMsg{op: op(gen), info: store.FetchBucketInfo(ctx, bc, p, bucket, region, created)}
	}
}

func (m *model) startStats(bucket, prefix string) tea.Cmd {
	ctx := m.newOp("scanning objects")
	gen, c, p := m.gen, m.client, m.profile
	bc := m.bclient
	if bucket != m.bucket {
		bc = nil
	}
	ch := make(chan tea.Msg, 2)
	st := store.NewStats(prefix)
	m.stats = st.Clone() // st is owned by the goroutine below
	go func() {
		defer close(ch)
		if bc == nil {
			bc, _ = store.ClientForBucket(ctx, c, p, bucket)
		}
		last := time.Time{}
		err := store.Walk(ctx, bc, bucket, prefix, func(objs []types.Object) error {
			st.Add(objs)
			if time.Since(last) > 150*time.Millisecond {
				last = time.Now()
				select {
				case ch <- statsMsg{op: op(gen), stats: st.Clone(), ch: ch}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		st.Done, st.Err, st.Finished = true, err, time.Now()
		ch <- statsMsg{op: op(gen), stats: st.Clone(), ch: ch}
	}()
	return waitFor(ch)
}

func (m *model) loadDetail(key string) tea.Cmd {
	ctx := m.newOp("fetching " + path.Base(key))
	gen, c, b := m.gen, m.bclient, m.bucket
	return func() tea.Msg {
		return detailMsg{op: op(gen), detail: store.FetchObject(ctx, c, b, key)}
	}
}

func yank(s string) tea.Cmd {
	cmds := []tea.Cmd{tea.SetClipboard(s)}
	if runtime.GOOS == "darwin" {
		cmds = append(cmds, func() tea.Msg {
			c := exec.Command("pbcopy")
			c.Stdin = strings.NewReader(s)
			_ = c.Run()
			return nil
		})
	}
	cmds = append(cmds, func() tea.Msg { return statusMsg("yanked " + s) })
	return tea.Batch(cmds...)
}
