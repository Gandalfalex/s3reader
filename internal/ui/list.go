package ui

import (
	"cmp"
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/Gandalfalex/vim-kit/motion"
	"s3reader/internal/query"
	"s3reader/internal/store"
)

const (
	sortName = iota
	sortSize
	sortModified
)

var sortNames = []string{"name", "size", "modified"}

type listState struct {
	all []store.Entry
	vis []int
	motion.Window
	filter  query.Query
	sortBy  int
	reverse bool
	noSort  bool // keep insertion order (profiles)
}

func (l *listState) set(items []store.Entry) {
	l.all = items
	l.apply()
}

// apply re-sorts and re-filters; use refilter when only the filter changed.
func (l *listState) apply() {
	if !l.noSort {
		slices.SortStableFunc(l.all, l.compare)
	}
	l.refilter()
}

// setFilter replaces the filter and moves the cursor to the first match.
func (l *listState) setFilter(q query.Query) {
	l.filter = q
	l.Cursor, l.Top = 0, 0
	l.refilter()
}

func (l *listState) refilter() {
	l.vis = l.vis[:0]
	all := l.filter.Empty()
	for i, e := range l.all {
		if all || l.filter.Match(e) {
			l.vis = append(l.vis, i)
		}
	}
	l.clamp()
}

// appendSorted merges a batch into the already sorted list in O(n), keeping
// the cursor on the same entry. Used for streamed search results.
func (l *listState) appendSorted(items []store.Entry) {
	if len(items) == 0 {
		return
	}
	key := ""
	if e, ok := l.current(); ok {
		key = e.Key
	}
	if l.noSort {
		l.all = append(l.all, items...)
	} else {
		slices.SortStableFunc(items, l.compare)
		merged := make([]store.Entry, 0, len(l.all)+len(items))
		i, j := 0, 0
		for i < len(l.all) && j < len(items) {
			if l.compare(items[j], l.all[i]) < 0 {
				merged = append(merged, items[j])
				j++
			} else {
				merged = append(merged, l.all[i])
				i++
			}
		}
		merged = append(append(merged, l.all[i:]...), items[j:]...)
		l.all = merged
	}
	l.refilter()
	if key != "" {
		l.selectKey(key)
	}
}

// compare orders directories first, then by the sort field.
func (l *listState) compare(a, b store.Entry) int {
	if a.IsDir != b.IsDir {
		if a.IsDir {
			return -1
		}
		return 1
	}
	var c int
	switch l.sortBy {
	case sortSize:
		c = cmp.Compare(a.Size, b.Size)
	case sortModified:
		c = a.Modified.Compare(b.Modified)
	default:
		c = compareFold(a.Name, b.Name)
	}
	if l.reverse {
		return -c
	}
	return c
}

// compareFold compares case-insensitively without allocating.
func compareFold(a, b string) int {
	for a != "" && b != "" {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if ra != rb {
			if la, lb := unicode.ToLower(ra), unicode.ToLower(rb); la != lb {
				return cmp.Compare(la, lb)
			}
		}
		a, b = a[na:], b[nb:]
	}
	return cmp.Compare(len(a), len(b))
}

func (l *listState) clamp() { l.Clamp(len(l.vis)) }

func (l *listState) current() (store.Entry, bool) {
	if len(l.vis) == 0 {
		return store.Entry{}, false
	}
	return l.all[l.vis[l.Cursor]], true
}

func (l *listState) selectKey(key string) {
	for i, idx := range l.vis {
		if l.all[idx].Key == key {
			l.Cursor = i
			return
		}
	}
}

// scroll keeps the cursor visible with a small scrolloff.
func (l *listState) scroll(height int) { l.Scroll(len(l.vis), height) }

func (l *listState) totalSize() (n int64) {
	for _, i := range l.vis {
		n += l.all[i].Size
	}
	return
}
