package ui

import (
	"cmp"
	"unicode"
	"unicode/utf8"

	tk "github.com/Gandalfalex/tuikit"
	"s3reader/internal/query"
	"s3reader/internal/store"
)

const (
	sortName = iota
	sortSize
	sortModified
)

var sortNames = []string{"name", "size", "modified"}

// listState is a list of entries filtered with the query language of /.
type listState struct {
	tk.List[store.Entry]
	sortBy  int
	reverse bool
}

// unsortedList keeps the entries in the order given (profiles).
func unsortedList() listState {
	l := listState{List: tk.NewList(func(e store.Entry) string { return e.Key })}
	l.Match = func(f string) func(store.Entry) bool {
		q, err := query.Parse(f)
		if err != nil {
			return func(store.Entry) bool { return false }
		}
		return q.Match
	}
	return l
}

// sortedList orders its entries by a sort field.
func sortedList(by int, reverse bool) listState {
	l := unsortedList()
	l.order(by, reverse)
	return l
}

// order sets the sort field and direction; Resort applies it.
func (l *listState) order(by int, reverse bool) {
	l.sortBy, l.reverse = by, reverse
	l.Compare = func(a, b store.Entry) int { return compare(by, reverse, a, b) }
}

// setQuery filters by a parsed query.
func (l *listState) setQuery(q query.Query) {
	if q.Empty() {
		l.SetFilter("")
		return
	}
	l.SetFilter(q.Raw)
}

// compare orders directories first, then by the sort field.
func compare(by int, reverse bool, a, b store.Entry) int {
	if a.IsDir != b.IsDir {
		if a.IsDir {
			return -1
		}
		return 1
	}
	var c int
	switch by {
	case sortSize:
		c = cmp.Compare(a.Size, b.Size)
	case sortModified:
		c = a.Modified.Compare(b.Modified)
	default:
		c = compareFold(a.Name, b.Name)
	}
	if reverse {
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

func (l *listState) totalSize() (n int64) {
	for _, i := range l.Vis {
		n += l.All[i].Size
	}
	return
}
