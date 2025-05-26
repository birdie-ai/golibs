package xtime_test

import (
	"testing"
	"time"

	"github.com/birdie-ai/golibs/xtime"
	"github.com/google/go-cmp/cmp"
)

func TestRangeNewValidation(t *testing.T) {
	start := time.Now()
	end := start
	_, err := xtime.NewRange(start, end)
	if err != nil {
		t.Fatalf("valid range got error: %v", err)
	}

	end = end.Add(1)
	r, err := xtime.NewRange(start, end)
	if err != nil {
		t.Fatalf("valid range got error: %v", err)
	}

	if r.Start() != start {
		t.Fatalf("r.Start()=%v; want %v", r.Start(), start)
	}
	if r.End() != end {
		t.Fatalf("r.End()=%v; want %v", r.End(), end)
	}

	start = end.Add(1)
	_, err = xtime.NewRange(start, end)
	if err == nil {
		t.Fatal("invalid range got no error")
	}

	var zero time.Time
	_, err = xtime.NewRange(zero, zero)
	if err == nil {
		t.Fatal("invalid range got no error")
	}
}

func TestRangeContains(t *testing.T) {
	cases := []struct {
		start, end, t time.Time
		want          bool
	}{
		{tm(1, 0), tm(2, 0), tm(0, 59), false},
		{tm(1, 0), tm(2, 0), tm(1, 0), true},
		{tm(1, 0), tm(2, 0), tm(1, 1), true},
		{tm(1, 0), tm(2, 0), tm(1, 59), true},
		{tm(1, 0), tm(2, 0), tm(2, 0), false},
		{tm(1, 0), tm(2, 0), tm(2, 1), false},
	}
	for _, c := range cases {
		got := newRange(c.start, c.end).Contains(c.t)
		if got != c.want {
			t.Errorf("xtime.Range{%v, %v}.Contains(%v) == %v, want %v",
				c.start, c.end, c.t, got, c.want)
		}
	}
}

func TestRangeDuration(t *testing.T) {
	cases := []struct {
		start, end time.Time
		want       time.Duration
	}{
		{tm(1, 0), tm(1, 0), time.Duration(0)},
		{tm(1, 0), tm(2, 0), time.Hour},
		{tm(1, 0), tm(3, 0), 2 * time.Hour},
	}
	for _, c := range cases {
		got := newRange(c.start, c.end).Duration()
		if got != c.want {
			t.Errorf("xtime.Range{%v, %v}.Duration() == %v, want %v",
				c.start, c.end, got, c.want)
		}
	}
}

func TestRangeSplit(t *testing.T) {
	cases := []struct {
		from, to time.Time
		max      time.Duration
		want     []xtime.Range
	}{
		{
			tm(1, 0),
			tm(2, 0),
			2 * time.Hour,
			[]xtime.Range{newRange(tm(1, 0), tm(2, 0))},
		},
		{
			tm(1, 0),
			tm(2, 0),
			time.Hour,
			[]xtime.Range{newRange(tm(1, 0), tm(2, 0))},
		},
		{
			tm(1, 0),
			tm(2, 0),
			30 * time.Minute,
			[]xtime.Range{newRange(tm(1, 0), tm(1, 30)), newRange(tm(1, 30), tm(2, 0))},
		},
		{
			tm(1, 0),
			tm(2, 0),
			20 * time.Minute,
			[]xtime.Range{newRange(tm(1, 0), tm(1, 20)), newRange(tm(1, 20), tm(1, 40)), newRange(tm(1, 40), tm(2, 0))},
		},
		{
			tm(1, 0),
			tm(2, 0),
			25 * time.Minute,
			[]xtime.Range{newRange(tm(1, 0), tm(1, 25)), newRange(tm(1, 25), tm(1, 50)), newRange(tm(1, 50), tm(2, 0))},
		},
	}
	for _, c := range cases {
		r := newRange(c.from, c.to)
		got := r.Split(c.max)
		comparer := cmp.Comparer(func(a xtime.Range, b xtime.Range) bool {
			return (a.Start().Equal(b.Start())) && (a.End().Equal(b.End()))
		})

		if diff := cmp.Diff(c.want, got, comparer); diff != "" {
			t.Errorf("split xtime.Range mismatch (-want +got):\n%s", diff)
		}
	}
}

func newRange(start, end time.Time) xtime.Range {
	tr, err := xtime.NewRange(start, end)
	if err != nil {
		panic(err)
	}
	return tr
}

// tm returns a time with the given hour and minute, and other fields hard-coded.
func tm(hour, minute int) time.Time {
	return time.Date(2023, 1, 1, hour, minute, 0, 0, time.UTC)
}
