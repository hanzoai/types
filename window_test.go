package types

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 7, 26, 15, 4, 5, 0, time.UTC)

func TestParseWindowRelativeLabels(t *testing.T) {
	for _, tc := range []struct {
		label string
		back  time.Duration
		want  Interval
	}{
		{"", 24 * time.Hour, Hour},
		{"24h", 24 * time.Hour, Hour},
		{"1d", 24 * time.Hour, Hour},
		{"day", 24 * time.Hour, Hour},
		{"today", 24 * time.Hour, Hour},
		{"7d", 7 * 24 * time.Hour, Day},
		{"week", 7 * 24 * time.Hour, Day},
		{"30d", 30 * 24 * time.Hour, Day},
		{"month", 30 * 24 * time.Hour, Day},
		{"  7D  ", 7 * 24 * time.Hour, Day},
	} {
		t.Run(strings.TrimSpace(tc.label), func(t *testing.T) {
			w, err := ParseWindow(tc.label, "", "", now)
			if err != nil {
				t.Fatalf("ParseWindow(%q) errored: %v", tc.label, err)
			}
			if !w.End.Equal(now) {
				t.Errorf("End = %v, want %v", w.End, now)
			}
			if !w.Start.Equal(now.Add(-tc.back)) {
				t.Errorf("Start = %v, want %v", w.Start, now.Add(-tc.back))
			}
			if w.Interval != tc.want {
				t.Errorf("Interval = %q, want %q", w.Interval, tc.want)
			}
		})
	}
}

// The label a response echoes is the caller's, defaulted when absent. Every
// caller used to re-derive this after the call; the window now carries it.
func TestParseWindowLabelIsDefaultedNotCanonicalised(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "24h"},
		{"   ", "24h"},
		{"7d", "7d"},
		{" 30d ", "30d"},
		{"7D", "7D"},
		{"custom", "custom"},
	} {
		in := tc.in
		start := ""
		if strings.TrimSpace(in) == "custom" {
			start = "2026-07-01T00:00:00Z"
		}
		w, err := ParseWindow(in, start, "", now)
		if err != nil {
			t.Fatalf("ParseWindow(%q) errored: %v", in, err)
		}
		if w.Label != tc.want {
			t.Errorf("ParseWindow(%q).Label = %q, want %q", in, w.Label, tc.want)
		}
	}
}

func TestParseWindowCustom(t *testing.T) {
	t.Run("rfc3339 under 48h buckets hourly", func(t *testing.T) {
		w, err := ParseWindow("custom", "2026-07-25T00:00:00Z", "2026-07-26T00:00:00Z", now)
		if err != nil {
			t.Fatalf("errored: %v", err)
		}
		if w.Interval != Hour {
			t.Errorf("Interval = %q, want %q", w.Interval, Hour)
		}
		if !w.Start.Equal(time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("Start = %v", w.Start)
		}
	})

	t.Run("over 48h buckets daily", func(t *testing.T) {
		w, err := ParseWindow("custom", "2026-07-01T00:00:00Z", "2026-07-26T00:00:00Z", now)
		if err != nil {
			t.Fatalf("errored: %v", err)
		}
		if w.Interval != Day {
			t.Errorf("Interval = %q, want %q", w.Interval, Day)
		}
	})

	t.Run("exactly 48h stays hourly", func(t *testing.T) {
		w, err := ParseWindow("custom", "2026-07-24T00:00:00Z", "2026-07-26T00:00:00Z", now)
		if err != nil {
			t.Fatalf("errored: %v", err)
		}
		if w.Interval != Hour {
			t.Errorf("Interval = %q, want %q", w.Interval, Hour)
		}
	})

	t.Run("unix seconds accepted", func(t *testing.T) {
		w, err := ParseWindow("custom", "1753401600", "1753488000", now)
		if err != nil {
			t.Fatalf("errored: %v", err)
		}
		if w.Start.Location() != time.UTC || w.End.Location() != time.UTC {
			t.Errorf("bounds must be UTC, got %v/%v", w.Start.Location(), w.End.Location())
		}
	})

	t.Run("absent end defaults to now", func(t *testing.T) {
		w, err := ParseWindow("custom", "2026-07-25T00:00:00Z", "", now)
		if err != nil {
			t.Fatalf("errored: %v", err)
		}
		if !w.End.Equal(now) {
			t.Errorf("End = %v, want %v", w.End, now)
		}
	})
}

// A rejected window must not hand back a zero Window a caller could mistake for
// a valid range.
func TestParseWindowRejects(t *testing.T) {
	for _, tc := range []struct{ name, label, start, end, wantErr string }{
		{"unknown label", "yesterday", "", "", `unknown range "yesterday"`},
		{"unknown label keeps caller casing", "LastWeek", "", "", `unknown range "LastWeek"`},
		{"custom without start", "custom", "", "", "custom range requires a valid start: empty time"},
		{"custom bad start", "custom", "nonsense", "", `custom range requires a valid start: not RFC3339 or unix seconds: "nonsense"`},
		{"custom bad end", "custom", "2026-07-01T00:00:00Z", "nope", `custom range has an invalid end: not RFC3339 or unix seconds: "nope"`},
		{"custom end before start", "custom", "2026-07-26T00:00:00Z", "2026-07-01T00:00:00Z", "custom range end must be after start"},
		{"custom end equals start", "custom", "2026-07-01T00:00:00Z", "2026-07-01T00:00:00Z", "custom range end must be after start"},
		{"custom non-positive unix start", "custom", "0", "", `custom range requires a valid start: not RFC3339 or unix seconds: "0"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, err := ParseWindow(tc.label, tc.start, tc.end, now)
			if err == nil {
				t.Fatalf("want error, got window %+v", w)
			}
			if err.Error() != tc.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tc.wantErr)
			}
			if w != (Window{}) {
				t.Errorf("rejected window must be zero, got %+v", w)
			}
		})
	}
}

func TestIntervalStep(t *testing.T) {
	if got := Hour.Step(); got != time.Hour {
		t.Errorf("Hour.Step() = %v, want %v", got, time.Hour)
	}
	if got := Day.Step(); got != 24*time.Hour {
		t.Errorf("Day.Step() = %v, want %v", got, 24*time.Hour)
	}
}

// ParseWindow only ever yields Hour or Day, so a caller interpolating the
// interval into a query has no third case to guard against.
func TestParseWindowOnlyYieldsTheClosedSet(t *testing.T) {
	labels := []string{"", "24h", "1d", "day", "today", "7d", "week", "30d", "month", "custom"}
	for _, l := range labels {
		start := ""
		if l == "custom" {
			start = "2026-07-01T00:00:00Z"
		}
		w, err := ParseWindow(l, start, "", now)
		if err != nil {
			t.Fatalf("ParseWindow(%q) errored: %v", l, err)
		}
		if w.Interval != Hour && w.Interval != Day {
			t.Fatalf("ParseWindow(%q) yielded interval %q outside {hour, day}", l, w.Interval)
		}
	}
}

// resolveLegacy is the implementation this package replaced, kept verbatim as
// the oracle the parity test compares against.
func resolveLegacy(rangeLabel, startStr, endStr string, now time.Time) (start, end time.Time, interval string, err error) {
	parseParam := func(s string) (time.Time, error) {
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

	now = now.UTC()
	switch strings.ToLower(strings.TrimSpace(rangeLabel)) {
	case "", "24h", "1d", "day", "today":
		return now.Add(-24 * time.Hour), now, "hour", nil
	case "7d", "week":
		return now.Add(-7 * 24 * time.Hour), now, "day", nil
	case "30d", "month":
		return now.Add(-30 * 24 * time.Hour), now, "day", nil
	case "custom":
		start, err = parseParam(startStr)
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("custom range requires a valid start: %w", err)
		}
		if strings.TrimSpace(endStr) == "" {
			end = now
		} else if end, err = parseParam(endStr); err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("custom range has an invalid end: %w", err)
		}
		if !end.After(start) {
			return time.Time{}, time.Time{}, "", fmt.Errorf("custom range end must be after start")
		}
		interval = "hour"
		if end.Sub(start) > 48*time.Hour {
			interval = "day"
		}
		return start.UTC(), end.UTC(), interval, nil
	default:
		return time.Time{}, time.Time{}, "", fmt.Errorf("unknown range %q", rangeLabel)
	}
}

// Bounds, interval and error text must match the replaced implementation
// exactly: these values reach a live dashboard and a 400 body.
func TestParseWindowMatchesTheReplacedImplementation(t *testing.T) {
	labels := []string{
		"", "   ", "24h", "1d", "day", "today", "7d", "week", "30d", "month",
		"24H", " 7d ", "custom", "CUSTOM", "yesterday", "1h", "all",
	}
	bounds := []string{
		"", "   ", "2026-07-01T00:00:00Z", "2026-07-25T12:00:00Z", "2026-07-26T00:00:00Z",
		"1753401600", "0", "-5", "nonsense", "2026-07-01",
	}

	cases := 0
	for _, l := range labels {
		for _, s := range bounds {
			for _, e := range bounds {
				cases++
				w, gotErr := ParseWindow(l, s, e, now)
				wantStart, wantEnd, wantInterval, wantErr := resolveLegacy(l, s, e, now)

				switch {
				case gotErr == nil && wantErr != nil:
					t.Fatalf("ParseWindow(%q,%q,%q) succeeded; legacy errored %v", l, s, e, wantErr)
				case gotErr != nil && wantErr == nil:
					t.Fatalf("ParseWindow(%q,%q,%q) errored %v; legacy succeeded", l, s, e, gotErr)
				case gotErr != nil:
					if gotErr.Error() != wantErr.Error() {
						t.Fatalf("ParseWindow(%q,%q,%q) error = %q, legacy = %q", l, s, e, gotErr, wantErr)
					}
					continue
				}

				if !w.Start.Equal(wantStart) {
					t.Fatalf("ParseWindow(%q,%q,%q) Start = %v, legacy = %v", l, s, e, w.Start, wantStart)
				}
				if !w.End.Equal(wantEnd) {
					t.Fatalf("ParseWindow(%q,%q,%q) End = %v, legacy = %v", l, s, e, w.End, wantEnd)
				}
				if string(w.Interval) != wantInterval {
					t.Fatalf("ParseWindow(%q,%q,%q) Interval = %q, legacy = %q", l, s, e, w.Interval, wantInterval)
				}
				if got, want := w.Interval.Step(), legacyStep(wantInterval); got != want {
					t.Fatalf("Step(%q) = %v, legacy = %v", w.Interval, got, want)
				}
			}
		}
	}
	t.Logf("compared %d label/start/end combinations against the replaced implementation", cases)
}

// legacyStep is the bucket-width helper this package replaced.
func legacyStep(interval string) time.Duration {
	if strings.EqualFold(interval, "day") {
		return 24 * time.Hour
	}
	return time.Hour
}
