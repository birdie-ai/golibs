// Package xtime extends Go's time. So far it contains an implementation of a time range.
package xtime

import "time"

// Range defines a time range that contains time t if Start <= t < End.
type Range struct {
	Start, End time.Time
}

// Contains returns true if [t] is within the range.
func (r Range) Contains(t time.Time) bool {
	return !t.Before(r.Start) && t.Before(r.End)
}

// Duration returns the duration of the time range.
func (r Range) Duration() time.Duration {
	return r.End.Sub(r.Start)
}

// Split returns a list of time ranges of at most [max] length that together make up [r].
func (r Range) Split(max time.Duration) []Range {
	var result []Range
	for max != 0 && r.End.Sub(r.Start) > max {
		next := r.Start.Add(max)
		result = append(result, Range{
			Start: r.Start,
			End:   next,
		})
		r.Start = next
	}
	result = append(result, r)
	return result
}
