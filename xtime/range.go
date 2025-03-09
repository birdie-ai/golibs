// Package xtime extends Go's time. So far it contains an implementation of a time range.
package xtime

import (
	"fmt"
	"time"
)

// Range defines a time range ensuring the invariant that [Range.Start()] <= [Range.End()].
type Range struct {
	start, end time.Time
}

// Start returns this range's start.
// It is guaranteed to be <= than [Range.End()].
func (r Range) Start() time.Time {
	return r.start
}

// End returns this range's end.
// It is guaranteed to be >= than [Range.Start()].
func (r Range) End() time.Time {
	return r.end
}

// Contains returns true if [t] is within the range.
func (r Range) Contains(t time.Time) bool {
	return !t.Before(r.start) && t.Before(r.end)
}

// Duration returns the duration of the time range.
func (r Range) Duration() time.Duration {
	return r.end.Sub(r.start)
}

// Split returns a list of time ranges of at most [max] length that together make up [r].
func (r Range) Split(maxDuration time.Duration) []Range {
	var result []Range
	for maxDuration != 0 && r.end.Sub(r.start) > maxDuration {
		next := r.start.Add(maxDuration)
		result = append(result, Range{
			start: r.start,
			end:   next,
		})
		r.start = next
	}
	result = append(result, r)
	return result
}

// NewRange creates a new [Range] validating start/end.
// It ensures the invariant that [Range] always has start <= end.
func NewRange(start, end time.Time) (Range, error) {
	if start.IsZero() || end.IsZero() {
		return Range{}, fmt.Errorf("creating range: start %v and end %v can't be zero value", start, end)
	}
	if start.After(end) {
		return Range{}, fmt.Errorf("creating range: start %v can't be after end %v", start, end)
	}
	return Range{start, end}, nil
}
