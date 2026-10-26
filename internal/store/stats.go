// Package store wraps the AWS SDK S3 calls used by the UI.
package store

import (
	"path"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Stats is accumulated while walking all objects.
type Stats struct {
	Prefix   string
	Count    int64
	Size     int64
	ByClass  map[string][2]int64 // class -> {count, bytes}
	ByExt    map[string][2]int64
	Largest  Entry
	Newest   Entry
	Oldest   Entry
	Done     bool
	Err      error
	Started  time.Time
	Finished time.Time
}

func NewStats(prefix string) *Stats {
	return &Stats{Prefix: prefix, ByClass: map[string][2]int64{}, ByExt: map[string][2]int64{}, Started: time.Now()}
}

func (s *Stats) Add(objs []types.Object) {
	for _, o := range objs {
		e := ObjEntry(o, aws.ToString(o.Key))
		s.Count++
		s.Size += e.Size
		cls := e.Class
		if cls == "" {
			cls = "STANDARD"
		}
		v := s.ByClass[cls]
		s.ByClass[cls] = [2]int64{v[0] + 1, v[1] + e.Size}
		ext := strings.ToLower(path.Ext(e.Key))
		if ext == "" {
			ext = "(none)"
		}
		v = s.ByExt[ext]
		s.ByExt[ext] = [2]int64{v[0] + 1, v[1] + e.Size}
		if e.Size > s.Largest.Size || s.Largest.Key == "" {
			s.Largest = e
		}
		if e.Modified.After(s.Newest.Modified) || s.Newest.Key == "" {
			s.Newest = e
		}
		if e.Modified.Before(s.Oldest.Modified) || s.Oldest.Key == "" {
			s.Oldest = e
		}
	}
}

func (s *Stats) Clone() *Stats {
	c := *s
	c.ByClass = make(map[string][2]int64, len(s.ByClass))
	for k, v := range s.ByClass {
		c.ByClass[k] = v
	}
	c.ByExt = make(map[string][2]int64, len(s.ByExt))
	for k, v := range s.ByExt {
		c.ByExt[k] = v
	}
	return &c
}

type KV struct {
	K string
	V [2]int64
}

func TopBy(m map[string][2]int64, n int) []KV {
	res := make([]KV, 0, len(m))
	for k, v := range m {
		res = append(res, KV{k, v})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].V[1] > res[j].V[1] })
	if len(res) > n {
		res = res[:n]
	}
	return res
}
