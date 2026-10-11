package snapshot

import (
	"reflect"
	"testing"
	"time"
	_ "time/tzdata" // Europe/Berlin on machines without a zone database
)

// at parses "2006-01-02 15:04" in loc.
func at(t *testing.T, loc *time.Location, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// kept lists the inputs r keeps, by their position.
func kept(r Retention, times []time.Time, loc *time.Location) []int {
	out := []int{}
	for i, k := range r.Keep(times, loc) {
		if k {
			out = append(out, i)
		}
	}
	return out
}

func TestRetentionKeep(t *testing.T) {
	utc := time.UTC
	// Newest first, as indexes store them. 2026-10-05 is a Monday.
	times := []time.Time{
		at(t, utc, "2026-10-12 20:00"), // 0  Mon, week 42
		at(t, utc, "2026-10-11 22:00"), // 1  Sun, week 41
		at(t, utc, "2026-10-11 09:00"), // 2  Sun
		at(t, utc, "2026-10-10 18:00"), // 3  Sat
		at(t, utc, "2026-10-05 00:00"), // 4  Mon, week 41
		at(t, utc, "2026-10-04 23:59"), // 5  Sun, week 40
		at(t, utc, "2026-09-30 12:00"), // 6  September
		at(t, utc, "2026-09-02 12:00"), // 7  September
		at(t, utc, "2026-08-15 12:00"), // 8  August
		at(t, utc, "2025-12-31 12:00"), // 9  last year
	}
	cases := []struct {
		name string
		r    Retention
		want []int
	}{
		{"last", Retention{Last: 3}, []int{0, 1, 2}},
		{"last beyond count", Retention{Last: 50}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{"daily skips days without saves", Retention{Daily: 4}, []int{0, 1, 3, 4}},
		{"daily newest of each day", Retention{Daily: 100}, []int{0, 1, 3, 4, 5, 6, 7, 8, 9}},
		{"weekly is Monday to Sunday", Retention{Weekly: 3}, []int{0, 1, 5}},
		{"monthly", Retention{Monthly: 3}, []int{0, 6, 8}},
		{"monthly across a year", Retention{Monthly: 10}, []int{0, 6, 8, 9}},
		// Each rule counts on its own; a snapshot picked by several rules
		// uses up a slot in each.
		{"union", Retention{Last: 2, Daily: 3, Weekly: 2, Monthly: 2}, []int{0, 1, 3, 6}},
		{"empty keeps nothing", Retention{}, []int{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := kept(c.r, times, utc); !reflect.DeepEqual(got, c.want) {
				t.Errorf("kept %v, want %v", got, c.want)
			}
		})
	}
}

func TestRetentionEmpty(t *testing.T) {
	if !(Retention{}).Empty() || !(Retention{Last: -1}).Empty() {
		t.Error("no positive count must be empty")
	}
	for _, r := range []Retention{{Last: 1}, {Daily: 1}, {Weekly: 1}, {Monthly: 1}} {
		if r.Empty() {
			t.Errorf("%+v is not empty", r)
		}
	}
	if got := (Retention{Last: 3}).Keep(nil, time.UTC); len(got) != 0 {
		t.Errorf("no snapshots: %v", got)
	}
}

// Days are calendar days where the user lives, not UTC days.
func TestRetentionTimeZone(t *testing.T) {
	utc := time.UTC
	moscow := time.FixedZone("UTC+3", 3*60*60)
	times := []time.Time{
		at(t, utc, "2026-10-11 01:00"), // 04:00 on the 11th in Moscow
		at(t, utc, "2026-10-10 22:30"), // 01:30 on the 11th in Moscow
		at(t, utc, "2026-10-10 20:00"), // 23:00 on the 10th in Moscow
	}
	if got, want := kept(Retention{Daily: 5}, times, utc), []int{0, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("UTC days: kept %v, want %v", got, want)
	}
	if got, want := kept(Retention{Daily: 5}, times, moscow), []int{0, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("Moscow days: kept %v, want %v", got, want)
	}

	// Sunday 23:30 in New York is already Monday in UTC: a new ISO week.
	ny := time.FixedZone("UTC-4", -4*60*60)
	week := []time.Time{
		at(t, ny, "2026-10-11 23:30"), // Sun, week 41 in New York; Mon, week 42 in UTC
		at(t, ny, "2026-10-11 10:00"), // Sun, week 41 everywhere
	}
	if got, want := kept(Retention{Weekly: 5}, week, ny), []int{0}; !reflect.DeepEqual(got, want) {
		t.Errorf("New York weeks: kept %v, want %v", got, want)
	}
	if got, want := kept(Retention{Weekly: 5}, week, utc), []int{0, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("UTC weeks: kept %v, want %v", got, want)
	}

	// The last evening of a month is the next month in UTC+3.
	month := []time.Time{
		at(t, utc, "2026-10-31 22:00"), // November in Moscow
		at(t, utc, "2026-10-31 12:00"),
	}
	if got, want := kept(Retention{Monthly: 5}, month, moscow), []int{0, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("Moscow months: kept %v, want %v", got, want)
	}
	if got, want := kept(Retention{Monthly: 5}, month, utc), []int{0}; !reflect.DeepEqual(got, want) {
		t.Errorf("UTC months: kept %v, want %v", got, want)
	}
}

// A day with a daylight saving change is still one calendar day.
func TestRetentionDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	// Clocks go back on 2026-10-25, so that day has 25 hours.
	times := []time.Time{
		at(t, berlin, "2026-10-25 23:50"),
		at(t, berlin, "2026-10-25 00:10"), // 24h40m earlier, same day
		at(t, berlin, "2026-10-24 23:50"),
	}
	if got, want := kept(Retention{Daily: 5}, times, berlin), []int{0, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("kept %v, want %v", got, want)
	}
}

func TestRetentionWeekAcrossYears(t *testing.T) {
	utc := time.UTC
	// 2027-01-03 is a Sunday in ISO week 53 of 2026, like 2026-12-28.
	times := []time.Time{
		at(t, utc, "2027-01-04 12:00"), // Mon, 2027-W01
		at(t, utc, "2027-01-03 12:00"), // Sun, 2026-W53
		at(t, utc, "2026-12-28 12:00"), // Mon, 2026-W53
		at(t, utc, "2026-12-27 12:00"), // Sun, 2026-W52
	}
	if got, want := kept(Retention{Weekly: 10}, times, utc), []int{0, 1, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("kept %v, want %v", got, want)
	}
	// Same day-of-year and month a year apart are different periods.
	years := []time.Time{at(t, utc, "2026-10-11 12:00"), at(t, utc, "2025-10-11 12:00")}
	if got, want := kept(Retention{Daily: 5, Monthly: 5}, years, utc), []int{0, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("a year apart: kept %v, want %v", got, want)
	}
}

func TestRetentionOrderAndTies(t *testing.T) {
	utc := time.UTC
	// The result follows the input, whatever its order.
	times := []time.Time{
		at(t, utc, "2026-10-09 12:00"),
		at(t, utc, "2026-10-11 08:00"),
		at(t, utc, "2026-10-10 12:00"),
		at(t, utc, "2026-10-11 20:00"),
	}
	if got, want := kept(Retention{Last: 2}, times, utc), []int{1, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("last: kept %v, want %v", got, want)
	}
	if got, want := kept(Retention{Daily: 2}, times, utc), []int{2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("daily: kept %v, want %v", got, want)
	}

	// Equal times: the one listed first wins.
	same := at(t, utc, "2026-10-11 12:00")
	tie := []time.Time{same, same, at(t, utc, "2026-10-10 12:00")}
	if got, want := kept(Retention{Last: 1}, tie, utc), []int{0}; !reflect.DeepEqual(got, want) {
		t.Errorf("tie, last: kept %v, want %v", got, want)
	}
	if got, want := kept(Retention{Daily: 2}, tie, utc), []int{0, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("tie, daily: kept %v, want %v", got, want)
	}
	// The same instant in another zone is still a tie.
	tie[1] = same.In(time.FixedZone("UTC+3", 3*60*60))
	if got, want := kept(Retention{Last: 1}, tie, utc), []int{0}; !reflect.DeepEqual(got, want) {
		t.Errorf("tie across zones: kept %v, want %v", got, want)
	}
}
