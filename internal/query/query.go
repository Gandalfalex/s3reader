// Package query parses and evaluates filter/search expressions.
package query

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"s3reader/internal/store"
)

type span struct{ from, to time.Time }

func (s span) contains(t time.Time) bool {
	return (s.from.IsZero() || !t.Before(s.from)) && (s.to.IsZero() || t.Before(s.to))
}

type dateTerm struct {
	text string
	sp   span
}

type Query struct {
	Raw      string
	terms    []string
	negTerms []string
	res      []*regexp.Regexp
	exts     []string
	minSize  int64
	maxSize  int64
	ranges   []span     // modified must fall in every range
	dates    []dateTerm // name contains text OR modified in span
	class    string
	kind     string
	caseSens bool
}

// Option applies one parsed token to a Query.
type Option func(*Query) error

// Each parser below inspects one token (lf = lowercased, f = as typed) and
// returns an Option when it claims the token, nil otherwise.

func Ext(lf, f string) Option {
	v, ok := strings.CutPrefix(lf, "ext:")
	if !ok {
		return nil
	}
	return func(q *Query) error {
		for _, e := range strings.Split(v, ",") {
			if e = strings.TrimPrefix(e, "."); e != "" {
				q.exts = append(q.exts, "."+e)
			}
		}
		return nil
	}
}

func Regex(lf, f string) Option {
	if !strings.HasPrefix(lf, "re:") {
		return nil
	}
	return func(q *Query) error {
		pat := f[3:]
		if !q.caseSens {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return fmt.Errorf("bad regex %q: %w", f[3:], err)
		}
		q.res = append(q.res, re)
		return nil
	}
}

func Before(lf, f string) Option {
	v, ok := strings.CutPrefix(lf, "before:")
	if !ok {
		return nil
	}
	return func(q *Query) error {
		op := "<"
		if _, isAge := parseAge(v); isAge {
			op = ">" // before:7d = older than 7 days
		}
		sp, err := parseCompare(op, v)
		if err != nil {
			return err
		}
		q.ranges = append(q.ranges, sp)
		return nil
	}
}

func After(lf, f string) Option {
	v, ok := strings.CutPrefix(lf, "after:")
	if !ok {
		return nil
	}
	return func(q *Query) error {
		op := ">"
		if _, isAge := parseAge(v); isAge {
			op = "<" // after:7d = newer than 7 days
		}
		sp, err := parseCompare(op, v)
		if err != nil {
			return err
		}
		q.ranges = append(q.ranges, sp)
		return nil
	}
}

func Date(lf, f string) Option {
	if !strings.HasPrefix(lf, "on:") && !strings.HasPrefix(lf, "date:") {
		return nil
	}
	return func(q *Query) error {
		v := lf[strings.IndexByte(lf, ':')+1:]
		sp, ok := parseSpan(v)
		if !ok {
			sp, ok = parseRange(v)
		}
		if !ok {
			return fmt.Errorf("bad date %q (use 2024-05-17, 17.05.2024, 2024-05, today, A..B)", v)
		}
		q.ranges = append(q.ranges, sp)
		return nil
	}
}

func Class(lf, f string) Option {
	if !strings.HasPrefix(lf, "class:") {
		return nil
	}
	return func(q *Query) error {
		q.class = strings.ToUpper(f[6:])
		return nil
	}
}

func Type(lf, f string) Option {
	v, ok := strings.CutPrefix(lf, "type:")
	if !ok {
		return nil
	}
	return func(q *Query) error {
		q.kind = v
		return nil
	}
}

// Compare handles >X, <X, >=X, <=X where X is a size or a date/age.
func Compare(lf, f string) Option {
	if len(lf) < 2 || (lf[0] != '>' && lf[0] != '<') {
		return nil
	}
	return func(q *Query) error {
		op, v := lf[:1], lf[1:]
		if rest, ok := strings.CutPrefix(v, "="); ok {
			op, v = op+"=", rest
		}
		if !isYear(v) {
			if n, err := parseSize(v); err == nil {
				if op[0] == '>' {
					q.minSize = n
				} else {
					q.maxSize = n
				}
				return nil
			}
		}
		if sp, err := parseCompare(op, v); err == nil {
			q.ranges = append(q.ranges, sp)
			return nil
		}
		q.terms = append(q.terms, f) // neither size nor date: literal text
		return nil
	}
}

func Negative(lf, f string) Option {
	if len(lf) < 2 || lf[0] != '!' {
		return nil
	}
	return func(q *Query) error {
		q.negTerms = append(q.negTerms, f[1:])
		return nil
	}
}

// Default always claims the token: date range, date-or-name, or plain text.
func Default(lf, f string) Option {
	return func(q *Query) error {
		if sp, ok := parseRange(lf); ok {
			q.ranges = append(q.ranges, sp)
		} else if sp, ok := parseSpan(lf); ok {
			q.dates = append(q.dates, dateTerm{text: f, sp: sp})
		} else {
			q.terms = append(q.terms, f)
		}
		return nil
	}
}

// parsers are tried in order; the first one that claims a token wins.
var parsers = []func(lf, f string) Option{
	Ext, Regex, Before, After, Date, Class, Type, Compare, Negative, Default,
}

func Parse(s string) (Query, error) {
	q := Query{Raw: s, minSize: -1, maxSize: -1}
	q.caseSens = strings.ToLower(s) != s
	for _, f := range strings.Fields(s) {
		lf := strings.ToLower(f)
		for _, p := range parsers {
			if opt := p(lf, f); opt != nil {
				if err := opt(&q); err != nil {
					return q, err
				}
				break
			}
		}
	}
	return q, nil
}

func (q *Query) Empty() bool { return strings.TrimSpace(q.Raw) == "" }

// contains reports whether name contains t, ignoring case unless the query is
// case-sensitive. Terms are already lowercase when case is ignored.
func (q *Query) contains(name, t string) bool {
	if q.caseSens {
		return strings.Contains(name, t)
	}
	return containsFold(name, t)
}

func (q *Query) Match(e store.Entry) bool {
	name := e.Name
	for _, t := range q.terms {
		if !q.contains(name, t) {
			return false
		}
	}
	for _, t := range q.negTerms {
		if q.contains(name, t) {
			return false
		}
	}
	for _, re := range q.res {
		if !re.MatchString(e.Name) {
			return false
		}
	}
	for _, d := range q.dates {
		if !q.contains(name, d.text) && (e.IsDir || !d.sp.contains(e.Modified)) {
			return false
		}
	}
	switch q.kind {
	case "dir", "d":
		if !e.IsDir {
			return false
		}
	case "file", "f":
		if e.IsDir {
			return false
		}
	}
	// Remaining predicates only make sense for objects; dirs pass through
	// only when no object-specific predicate is set.
	objPred := len(q.exts) > 0 || q.minSize >= 0 || q.maxSize >= 0 || len(q.ranges) > 0 || q.class != ""
	if e.IsDir {
		return !objPred
	}
	if len(q.exts) > 0 {
		ext := path.Ext(e.Key)
		ok := false
		for _, x := range q.exts {
			if strings.EqualFold(ext, x) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if q.minSize >= 0 && e.Size <= q.minSize {
		return false
	}
	if q.maxSize >= 0 && e.Size >= q.maxSize {
		return false
	}
	for _, r := range q.ranges {
		if !r.contains(e.Modified) {
			return false
		}
	}
	if q.class != "" {
		cls := e.Class
		if cls == "" {
			cls = "STANDARD"
		}
		if !strings.Contains(cls, q.class) {
			return false
		}
	}
	return true
}

// containsFold is strings.Contains(strings.ToLower(s), sub) for a lowercase
// sub, without allocating when s is ASCII.
func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return strings.Contains(strings.ToLower(s), sub)
		}
	}
	first := sub[0]
	for i := 0; i+len(sub) <= len(s); i++ {
		if lower(s[i]) != first {
			continue
		}
		j := 1
		for j < len(sub) && lower(s[i+j]) == sub[j] {
			j++
		}
		if j == len(sub) {
			return true
		}
	}
	return false
}

func lower(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// parseSpan parses an absolute date into the period it denotes.
func parseSpan(s string) (span, bool) {
	day := func(t time.Time) span { return span{t, t.AddDate(0, 0, 1)} }
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	switch s {
	case "today":
		return day(today), true
	case "yesterday":
		return day(today.AddDate(0, 0, -1)), true
	}
	if isYear(s) {
		y, _ := strconv.Atoi(s)
		t := time.Date(y, 1, 1, 0, 0, 0, 0, time.Local)
		return span{t, t.AddDate(1, 0, 0)}, true
	}
	if t, err := time.Parse(time.RFC3339, strings.ToUpper(s)); err == nil {
		return span{t, t.Add(time.Second)}, true
	}
	layouts := []struct {
		layout string
		next   func(time.Time) time.Time
	}{
		{"2006-01-02t15:04", func(t time.Time) time.Time { return t.Add(time.Minute) }},
		{"2006-01-02", func(t time.Time) time.Time { return t.AddDate(0, 0, 1) }},
		{"02.01.2006", func(t time.Time) time.Time { return t.AddDate(0, 0, 1) }},
		{"2006-01", func(t time.Time) time.Time { return t.AddDate(0, 1, 0) }},
		{"01.2006", func(t time.Time) time.Time { return t.AddDate(0, 1, 0) }},
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l.layout, s, time.Local); err == nil {
			return span{t, l.next(t)}, true
		}
	}
	return span{}, false
}

// parseRange parses "A..B" where either side may be empty.
func parseRange(s string) (span, bool) {
	a, b, ok := strings.Cut(s, "..")
	if !ok || (a == "" && b == "") {
		return span{}, false
	}
	var r span
	if a != "" {
		sa, ok := parseSpan(a)
		if !ok {
			return span{}, false
		}
		r.from = sa.from
	}
	if b != "" {
		sb, ok := parseSpan(b)
		if !ok {
			return span{}, false
		}
		r.to = sb.to
	}
	return r, true
}

// parseAge parses relative ages: 12h, 7d, 2w, 3mo, 1y.
func parseAge(s string) (time.Duration, bool) {
	units := []struct {
		suffix string
		d      time.Duration
	}{{"mo", 30 * 24 * time.Hour}, {"h", time.Hour}, {"d", 24 * time.Hour}, {"w", 7 * 24 * time.Hour}, {"y", 365 * 24 * time.Hour}}
	for _, u := range units {
		if n, ok := strings.CutSuffix(s, u.suffix); ok {
			if v, err := strconv.Atoi(n); err == nil && v >= 0 {
				return time.Duration(v) * u.d, true
			}
			return 0, false
		}
	}
	return 0, false
}

// parseCompare turns "op value" into a modified-time range. For absolute dates
// > and < exclude the named period, >= and <= include it. For ages, < means
// newer than and > means older than.
func parseCompare(op, v string) (span, error) {
	if sp, ok := parseSpan(v); ok {
		switch op {
		case ">":
			return span{from: sp.to}, nil
		case ">=":
			return span{from: sp.from}, nil
		case "<":
			return span{to: sp.from}, nil
		case "<=":
			return span{to: sp.to}, nil
		}
	}
	if d, ok := parseAge(v); ok {
		cut := time.Now().Add(-d)
		if op[0] == '<' {
			return span{from: cut}, nil
		}
		return span{to: cut}, nil
	}
	return span{}, fmt.Errorf("bad date %q (use 2024-05-17, 17.05.2024, 2024-05, today, 7d, 2w, 3mo)", v)
}

func isYear(s string) bool {
	if len(s) != 4 {
		return false
	}
	y, err := strconv.Atoi(s)
	return err == nil && y >= 1970 && y < 2200
}

func parseSize(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.TrimSuffix(strings.TrimSuffix(s, "IB"), "B")
	mult := int64(1)
	if n := len(s); n > 0 {
		switch s[n-1] {
		case 'K':
			mult = 1 << 10
		case 'M':
			mult = 1 << 20
		case 'G':
			mult = 1 << 30
		case 'T':
			mult = 1 << 40
		}
		if mult > 1 {
			s = s[:n-1]
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f * float64(mult)), nil
}
