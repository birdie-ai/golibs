package xtime_test

import (
	"testing"
	"time"

	"github.com/birdie-ai/golibs/xtime"
	"github.com/google/go-cmp/cmp"
)

// tm returns a time with the given hour and minute, and other fields hard-coded.
func tm(hour, minute int) time.Time {
	return time.Date(2023, 1, 1, hour, minute, 0, 0, time.UTC)
}

func TestRangeContains(t *testing.T) {
	cases := []struct {
		from, to, t time.Time
		want        bool
	}{
		{tm(1, 0), tm(2, 0), tm(0, 59), false},
		{tm(1, 0), tm(2, 0), tm(1, 0), true},
		{tm(1, 0), tm(2, 0), tm(1, 1), true},
		{tm(1, 0), tm(2, 0), tm(1, 59), true},
		{tm(1, 0), tm(2, 0), tm(2, 0), false},
		{tm(1, 0), tm(2, 0), tm(2, 1), false},
	}
	for _, c := range cases {
		got := xtime.Range{c.from, c.to}.Contains(c.t)
		if got != c.want {
			t.Errorf("xtime.Range{%v, %v}.Contains(%v) == %v, want %v",
				c.from, c.to, c.t, got, c.want)
		}
	}
}

func TestRangeDuration(t *testing.T) {
	cases := []struct {
		from, to time.Time
		want     time.Duration
	}{
		{tm(1, 0), tm(1, 0), time.Duration(0)},
		{tm(1, 0), tm(2, 0), 1 * time.Hour},
		{tm(1, 0), tm(3, 0), 2 * time.Hour},
	}
	for _, c := range cases {
		got := xtime.Range{c.from, c.to}.Duration()
		if got != c.want {
			t.Errorf("xtime.Range{%v, %v}.Duration() == %v, want %v",
				c.from, c.to, got, c.want)
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
			[]xtime.Range{{tm(1, 0), tm(2, 0)}},
		},
		{
			tm(1, 0),
			tm(2, 0),
			1 * time.Hour,
			[]xtime.Range{{tm(1, 0), tm(2, 0)}},
		},
		{
			tm(1, 0),
			tm(2, 0),
			30 * time.Minute,
			[]xtime.Range{{tm(1, 0), tm(1, 30)}, {tm(1, 30), tm(2, 0)}},
		},
		{
			tm(1, 0),
			tm(2, 0),
			20 * time.Minute,
			[]xtime.Range{{tm(1, 0), tm(1, 20)}, {tm(1, 20), tm(1, 40)}, {tm(1, 40), tm(2, 0)}},
		},
		{
			tm(1, 0),
			tm(2, 0),
			25 * time.Minute,
			[]xtime.Range{{tm(1, 0), tm(1, 25)}, {tm(1, 25), tm(1, 50)}, {tm(1, 50), tm(2, 0)}},
		},
	}
	for _, c := range cases {
		r := xtime.Range{c.from, c.to}
		got := r.Split(c.max)
		if diff := cmp.Diff(c.want, got); diff != "" {
			t.Errorf("split xtime.Range mismatch (-want +got):\n%s", diff)
		}
	}
}
