// Package xtime extends Go's time. So far it contains an implementation of a time range.
package xtime

import "time"

// Range defines a time range that contains time t if From <= t < To.
type Range struct {
	From, To time.Time
}

// Contains returns true if [t] is within the range.
func (r Range) Contains(t time.Time) bool {
	return !t.Before(r.From) && t.Before(r.To)
}

// Duration returns the duration of the time range.
func (r Range) Duration() time.Duration {
	return r.To.Sub(r.From)
}

// Split returns a list of time ranges of at most [max] length that together make up [r].
func (r Range) Split(max time.Duration) []Range {
	var result []Range
	for max != 0 && r.To.Sub(r.From) > max {
		next := r.From.Add(max)
		result = append(result, Range{
			From: r.From,
			To:   next,
		})
		r.From = next
	}
	result = append(result, r)
	return result
}
