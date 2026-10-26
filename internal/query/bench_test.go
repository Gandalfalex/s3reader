package query

import (
	"fmt"
	"testing"
	"time"

	"s3reader/internal/store"
)

func benchEntries() []store.Entry {
	es := make([]store.Entry, 10000)
	now := time.Now()
	for i := range es {
		name := fmt.Sprintf("Reports/2024/Daily-Export_%05d.CSV", i)
		es[i] = store.Entry{Name: name, Key: name, Size: int64(i * 100), Modified: now.Add(-time.Duration(i) * time.Hour)}
	}
	return es
}

func BenchmarkMatch(b *testing.B) {
	es := benchEntries()
	for _, qs := range []string{"export", "export !tmp ext:csv", "export >1kb <30d", "Export"} {
		q, err := Parse(qs)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(qs, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for _, e := range es {
					q.Match(e)
				}
			}
		})
	}
}
