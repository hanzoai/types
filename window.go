package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Interval is the width of one bucket in a metered time series.
//
// The set is closed at Hour and Day. A bucket function is not a bindable
// parameter in the engines that serve these series, so the interval reaches a
// query by interpolation; keeping the set closed means a request cannot put
// anything else there.
type Interval string

const (
	Hour Interval = "hour"
	Day  Interval = "day"
)

// Step is how much time one bucket covers.
func (i Interval) Step() time.Duration {
	if i == Day {
		return 24 * time.Hour
	}
	return time.Hour
}

// Window is a half-open time range [Start, End) and the interval its series
// buckets to.
//
// Label is the range as the caller asked for it, defaulted when the caller
// asked for nothing, so a response can say which window it answers without the
// caller re-deriving it.
type Window struct {
	Label    string
	Start    time.Time
	End      time.Time
	Interval Interval
}

// ParseWindow resolves a range request into an absolute window.
//
// "24h", "7d" and "30d", with their aliases, are relative to now. "custom"
// reads start and end, each RFC3339 or unix seconds; end defaults to now and
// must follow start. An unrecognised label is an error rather than a silent
// default, so a typo cannot quietly change the window a caller is shown.
//
// now is a parameter so the window is a function of the arguments alone.
func ParseWindow(label, start, end string, now time.Time) (Window, error) {
	now = now.UTC()

	w := Window{Label: strings.TrimSpace(label)}
	if w.Label == "" {
		w.Label = "24h"
	}

	switch strings.ToLower(strings.TrimSpace(label)) {
	case "", "24h", "1d", "day", "today":
		w.Start, w.End, w.Interval = now.Add(-24*time.Hour), now, Hour
	case "7d", "week":
		w.Start, w.End, w.Interval = now.Add(-7*24*time.Hour), now, Day
	case "30d", "month":
		w.Start, w.End, w.Interval = now.Add(-30*24*time.Hour), now, Day
	case "custom":
		from, err := parseTime(start)
		if err != nil {
			return Window{}, fmt.Errorf("custom range requires a valid start: %w", err)
		}
		to := now
		if strings.TrimSpace(end) != "" {
			if to, err = parseTime(end); err != nil {
				return Window{}, fmt.Errorf("custom range has an invalid end: %w", err)
			}
		}
		if !to.After(from) {
			return Window{}, fmt.Errorf("custom range end must be after start")
		}
		w.Start, w.End, w.Interval = from.UTC(), to.UTC(), Hour
		if w.End.Sub(w.Start) > 48*time.Hour {
			w.Interval = Day
		}
	default:
		return Window{}, fmt.Errorf("unknown range %q", label)
	}

	return w, nil
}

// parseTime accepts RFC3339 or a unix-seconds string.
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if secs, err := strconv.ParseInt(s, 10, 64); err == nil && secs > 0 {
		return time.Unix(secs, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("not RFC3339 or unix seconds: %q", s)
}
