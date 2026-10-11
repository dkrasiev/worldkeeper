package snapshot

import (
	"sort"
	"time"
)

// Retention decides which automatic snapshots rotation keeps. It follows
// restic's forget --keep-last/--keep-daily/--keep-weekly/--keep-monthly,
// so both backends rotate alike: each rule picks its own snapshots and a
// snapshot is kept if any rule picks it. A zero count turns its rule off.
type Retention struct {
	// Last keeps the newest Last snapshots.
	Last int
	// Daily keeps the newest snapshot of each of the Daily most recent
	// days that have one. Days without snapshots are skipped, not counted.
	Daily int
	// Weekly does the same for ISO weeks (Monday to Sunday).
	Weekly int
	// Monthly does the same for calendar months.
	Monthly int
}

// Empty reports whether no rule is on. Rotation then deletes nothing, as
// restic does, rather than every automatic snapshot.
func (r Retention) Empty() bool {
	return r.Last <= 0 && r.Daily <= 0 && r.Weekly <= 0 && r.Monthly <= 0
}

// Keep reports which of times r keeps; the result is parallel to times.
// Days, weeks and months are calendar periods in loc. Equal times are
// taken in input order, so pass them newest first, as indexes store them.
func (r Retention) Keep(times []time.Time, loc *time.Location) []bool {
	order := make([]int, len(times))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return times[order[a]].After(times[order[b]]) })

	rules := []struct {
		left   int
		period func(time.Time) int // nil: every snapshot is its own period
		last   int
		seen   bool
	}{
		{left: r.Last},
		{left: r.Daily, period: func(t time.Time) int { y, m, d := t.Date(); return y*10000 + int(m)*100 + d }},
		{left: r.Weekly, period: func(t time.Time) int { y, w := t.ISOWeek(); return y*100 + w }},
		{left: r.Monthly, period: func(t time.Time) int { y, m, _ := t.Date(); return y*100 + int(m) }},
	}
	keep := make([]bool, len(times))
	for _, i := range order {
		t := times[i].In(loc)
		for j := range rules {
			rule := &rules[j]
			if rule.left <= 0 {
				continue
			}
			if rule.period != nil {
				p := rule.period(t)
				if rule.seen && p == rule.last {
					continue // a newer snapshot already stands for this period
				}
				rule.last, rule.seen = p, true
			}
			rule.left--
			keep[i] = true
		}
	}
	return keep
}
